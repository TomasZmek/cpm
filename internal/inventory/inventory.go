// Package inventory collects containers from all enabled module sources and
// answers "what is behind this proxy rule?" without blocking page requests:
// a background loop refreshes the containers and probes rule targets that no
// source knows about, and handlers only read the latest snapshot.
package inventory

import (
	"context"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/modules"
	"github.com/TomasZmek/cpm/sdk"
)

// State of a rule's backend.
type State string

const (
	Running     State = "running"     // matched container is running
	Stopped     State = "stopped"     // matched container exists but is not running
	Reachable   State = "reachable"   // no container matched, TCP connect succeeded
	Unreachable State = "unreachable" // no container matched, TCP connect failed
	Unknown     State = "unknown"     // not checked (yet), or not checkable
)

// Status describes a rule's backend.
type Status struct {
	State     State  `json:"state"`
	Container string `json:"container,omitempty"`
	Host      string `json:"host,omitempty"`
}

// SourceState reports the last refresh of one module source.
type SourceState struct {
	ModuleID string
	Name     sdk.Text
	Icon     string
	Count    int
	Err      string
}

const (
	defaultInterval  = 30 * time.Second
	sourceTimeout    = 10 * time.Second
	probeTimeout     = time.Second
	probeConcurrency = 8
)

// Service is the inventory. Create it with New and run Start once.
type Service struct {
	mods  *modules.Manager
	sites func() []*models.Site

	interval time.Duration
	dial     func(ctx context.Context, network, addr string) (net.Conn, error)
	trigger  chan struct{}

	mu         sync.RWMutex
	containers []sdk.Container
	sources    []SourceState
	probes     map[string]bool // "host:port" → reachable
	updated    time.Time
}

// New creates the inventory. sites returns the current proxy rules.
func New(mods *modules.Manager, sites func() []*models.Site) *Service {
	d := &net.Dialer{Timeout: probeTimeout}
	s := &Service{
		mods:     mods,
		sites:    sites,
		interval: defaultInterval,
		dial:     d.DialContext,
		trigger:  make(chan struct{}, 1),
		probes:   map[string]bool{},
	}
	mods.OnChange(s.Trigger)
	return s
}

// Start runs the refresh loop until ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			s.Refresh(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-s.trigger:
			}
		}
	}()
}

// Trigger requests a refresh soon (e.g. after a rule or module changed).
func (s *Service) Trigger() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// Refresh queries all sources and probes unmatched rule targets.
func (s *Service) Refresh(ctx context.Context) {
	active := s.mods.Active()
	states := make([]SourceState, len(active))
	lists := make([][]sdk.Container, len(active))

	var wg sync.WaitGroup
	for i, a := range active {
		states[i] = SourceState{ModuleID: a.Module.ID, Name: a.Module.Name, Icon: a.Module.Icon}
		if a.Err != nil {
			states[i].Err = a.Err.Error()
			continue
		}
		wg.Add(1)
		go func(i int, src sdk.Source) {
			defer wg.Done()
			sctx, cancel := context.WithTimeout(ctx, sourceTimeout)
			defer cancel()
			// A source may return containers together with a warning.
			list, err := src.Containers(sctx)
			if err != nil {
				states[i].Err = err.Error()
			}
			lists[i] = list
			states[i].Count = len(list)
		}(i, a.Source)
	}
	wg.Wait()

	// Several sources can report the same daemon (e.g. Docker (local) and
	// Dockhand's local environment): keep each container once.
	var containers []sdk.Container
	seen := map[string]bool{}
	for _, l := range lists {
		for _, c := range l {
			if key := containerKey(c); key != "" {
				if seen[key] {
					continue
				}
				seen[key] = true
			}
			containers = append(containers, c)
		}
	}

	// Probe targets of rules no container matched.
	targets := map[string]bool{}
	for _, site := range s.sites() {
		if match(site, containers) != nil {
			continue
		}
		if addr, ok := probeAddr(site); ok {
			targets[addr] = false
		}
	}
	s.probe(ctx, targets)

	s.mu.Lock()
	s.containers, s.sources, s.probes, s.updated = containers, states, targets, time.Now()
	s.mu.Unlock()
	for _, st := range states {
		if st.Err != "" {
			log.Printf("inventory: module %s: %s", st.ModuleID, st.Err)
		}
	}
}

// containerKey identifies a container across sources by its ID (short or
// full form); containers without an ID are never merged.
func containerKey(c sdk.Container) string {
	if len(c.ID) >= 12 {
		return c.ID[:12]
	}
	return ""
}

// probe connects to every target and records whether it accepted the
// connection. The map's keys are collected first; goroutines only write
// results under a mutex.
func (s *Service) probe(ctx context.Context, targets map[string]bool) {
	addrs := make([]string, 0, len(targets))
	for addr := range targets {
		addrs = append(addrs, addr)
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, probeConcurrency)
	for _, addr := range addrs {
		wg.Add(1)
		sem <- struct{}{}
		go func(addr string) {
			defer func() { <-sem; wg.Done() }()
			pctx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			conn, err := s.dial(pctx, "tcp", addr)
			if err == nil {
				conn.Close()
			}
			mu.Lock()
			targets[addr] = err == nil
			mu.Unlock()
		}(addr)
	}
	wg.Wait()
}

// probeAddr returns host:port for rules whose target can be probed from CPM:
// IP addresses and dotted host names. Single-label names are Docker service
// names that usually only resolve inside Caddy's network.
func probeAddr(site *models.Site) (string, bool) {
	host := strings.TrimSpace(site.TargetIP)
	port, err := strconv.Atoi(strings.TrimSpace(site.TargetPort))
	if host == "" || err != nil || port < 1 || port > 65535 {
		return "", false
	}
	if net.ParseIP(host) == nil && !strings.Contains(host, ".") {
		return "", false
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), true
}

// match finds the container behind a rule: by container name (service-name
// targets) or by Docker host IP + published port.
func match(site *models.Site, containers []sdk.Container) *sdk.Container {
	target := strings.TrimSpace(site.TargetIP)
	if target == "" {
		return nil
	}
	port, _ := strconv.Atoi(strings.TrimSpace(site.TargetPort))

	var byName *sdk.Container
	for i := range containers {
		c := &containers[i]
		if strings.EqualFold(c.Name, target) && (byName == nil || c.Running()) {
			byName = c
		}
	}
	if byName != nil {
		return byName
	}
	if port <= 0 {
		return nil
	}
	for i := range containers {
		c := &containers[i]
		if c.HostIP == "" || c.HostIP != target {
			continue
		}
		for _, p := range c.Ports {
			if int(p.Public) == port {
				return c
			}
		}
	}
	return nil
}

// StatusFor returns the backend status of a rule from the latest snapshot.
func (s *Service) StatusFor(site *models.Site) Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c := match(site, s.containers); c != nil {
		st := Status{State: Stopped, Container: c.Name, Host: c.Host}
		if c.Running() {
			st.State = Running
		}
		return st
	}
	if addr, ok := probeAddr(site); ok {
		if up, checked := s.probes[addr]; checked {
			if up {
				return Status{State: Reachable}
			}
			return Status{State: Unreachable}
		}
		// A new or changed rule: check it soon instead of at the next tick.
		s.Trigger()
	}
	return Status{State: Unknown}
}

// Containers returns the containers of the latest snapshot.
func (s *Service) Containers() []sdk.Container {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]sdk.Container(nil), s.containers...)
}

// Sources returns the state of each module source at the last refresh.
func (s *Service) Sources() []SourceState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]SourceState(nil), s.sources...)
}

// Updated returns the time of the last refresh (zero before the first one).
func (s *Service) Updated() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updated
}
