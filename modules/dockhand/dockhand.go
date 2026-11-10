// Package dockhand is a CPM module that reads containers of all Docker
// environments managed by Dockhand (https://dockhand.pro) through its REST
// API, so CPM knows which containers run on which host without connecting to
// each Docker daemon itself.
//
// API used (Dockhand ≥ 1.0):
//
//	GET /api/environments            → [{id, name, connectionType, host, publicIp, …}]
//	GET /api/containers?env=ID&all=true → [{id, name, state, ports:[{IP, PrivatePort, PublicPort}], …}]
//
// Authentication is an API token (Bearer dh_…), needed only when Dockhand
// has authentication enabled.
package dockhand

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TomasZmek/cpm/sdk"
)

const (
	requestTimeout = 10 * time.Second
	maxBody        = 8 << 20 // 8 MB per response
	envConcurrency = 4
	// Dockhand blocks an IP for 5 minutes after 10 failed token attempts;
	// after a rejected token CPM waits this long before trying again.
	authBackoff = 5 * time.Minute
)

func init() {
	sdk.Register(sdk.Module{
		ID:   "dockhand",
		Icon: "⚓",
		Name: sdk.Text{"en": "Dockhand", "cs": "Dockhand", "ko": "Dockhand"},
		Description: sdk.Text{
			"en": "Containers of all Docker environments managed by Dockhand (local and remote hosts via Hawser).",
			"cs": "Kontejnery všech Docker prostředí spravovaných Dockhandem (lokální i vzdálené hosty přes Hawser).",
			"ko": "Dockhand가 관리하는 모든 Docker 환경의 컨테이너 (로컬 및 Hawser를 통한 원격 호스트).",
		},
		Fields: []sdk.Field{
			{
				Key: "url", Type: sdk.FieldURL, Required: true, Placeholder: "http://192.168.1.10:3000",
				Label: sdk.Text{"en": "Dockhand URL", "cs": "Adresa Dockhandu", "ko": "Dockhand URL"},
				Help: sdk.Text{
					"en": "Address CPM can reach Dockhand on, e.g. http://192.168.1.10:3000.",
					"cs": "Adresa, na které CPM Dockhand najde, např. http://192.168.1.10:3000.",
					"ko": "CPM이 Dockhand에 접속할 주소, 예: http://192.168.1.10:3000.",
				},
			},
			{
				Key: "token", Type: sdk.FieldPassword,
				Label: sdk.Text{"en": "API token", "cs": "API token", "ko": "API 토큰"},
				Help: sdk.Text{
					"en": "Dockhand → Profile → API tokens (starts with dh_). Not needed when Dockhand has authentication disabled.",
					"cs": "Dockhand → Profil → API tokeny (začíná dh_). Není potřeba, pokud má Dockhand vypnuté přihlašování.",
					"ko": "Dockhand → 프로필 → API 토큰 (dh_로 시작). Dockhand 인증이 꺼져 있으면 필요 없습니다.",
				},
			},
			{
				Key: "host_ips", Type: sdk.FieldText, Placeholder: "1=192.168.1.10, 2=192.168.1.20",
				Label: sdk.Text{"en": "Host IPs of environments", "cs": "IP adresy hostů prostředí", "ko": "환경별 호스트 IP"},
				Help: sdk.Text{
					"en": "Optional, environment ID = IP as used in proxy rules. Without it CPM uses the environment's Public IP set in Dockhand, then its host address.",
					"cs": "Volitelné, ID prostředí = IP, jak ji používají proxy pravidla. Bez toho CPM použije Public IP nastavenou u prostředí v Dockhandu, případně jeho adresu hosta.",
					"ko": "선택 사항, 환경 ID = 프록시 규칙에서 사용하는 IP. 없으면 Dockhand에 설정된 환경의 Public IP, 그다음 호스트 주소를 사용합니다.",
				},
			},
			{
				Key: "environments", Type: sdk.FieldText, Placeholder: "1, 2",
				Label: sdk.Text{"en": "Environments", "cs": "Prostředí", "ko": "환경"},
				Help: sdk.Text{
					"en": "Optional, comma-separated environment IDs to read. Empty: all environments.",
					"cs": "Volitelné, čárkou oddělená ID prostředí, která se mají načítat. Prázdné: všechna.",
					"ko": "선택 사항, 읽을 환경 ID (쉼표로 구분). 비우면 모든 환경.",
				},
			},
		},
		New: newSource,
	})
}

type source struct {
	base     *url.URL
	token    string
	hostIPs  map[int]string
	envOnly  map[int]bool
	client   *http.Client
	mu       sync.Mutex
	authWait time.Time // no requests before this time after a rejected token
}

func newSource(cfg sdk.Config) (sdk.Source, error) {
	base, err := url.Parse(strings.TrimRight(cfg["url"], "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("invalid Dockhand URL")
	}
	token := cfg["token"]
	if token != "" && !strings.HasPrefix(token, "dh_") {
		return nil, fmt.Errorf("a Dockhand API token starts with dh_")
	}
	hostIPs, err := parseHostIPs(cfg["host_ips"])
	if err != nil {
		return nil, err
	}
	envOnly, err := parseIDs(cfg["environments"])
	if err != nil {
		return nil, err
	}
	return &source{
		base:    base,
		token:   token,
		hostIPs: hostIPs,
		envOnly: envOnly,
		client:  &http.Client{Timeout: requestTimeout},
	}, nil
}

// parseHostIPs parses "1=192.168.1.10, 2=192.168.1.20".
func parseHostIPs(s string) (map[int]string, error) {
	out := map[int]string{}
	for _, part := range splitList(s) {
		k, v, ok := strings.Cut(part, "=")
		id, err := strconv.Atoi(strings.TrimSpace(k))
		ip := strings.TrimSpace(v)
		if !ok || err != nil || id <= 0 || net.ParseIP(ip) == nil {
			return nil, fmt.Errorf("host IPs: expected ID=IP, got %q", part)
		}
		out[id] = ip
	}
	return out, nil
}

func parseIDs(s string) (map[int]bool, error) {
	out := map[int]bool{}
	for _, part := range splitList(s) {
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("environments: invalid ID %q", part)
		}
		out[id] = true
	}
	return out, nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

type environment struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	ConnectionType string `json:"connectionType"`
	Host           string `json:"host"`
	PublicIP       string `json:"publicIp"`
}

type container struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	Ports []struct {
		IP          string `json:"IP"`
		PrivatePort uint16 `json:"PrivatePort"`
		PublicPort  uint16 `json:"PublicPort"`
	} `json:"ports"`
}

var errUnauthorized = errors.New("Dockhand rejected the API token (check it in Settings → Modules)")

func (s *source) get(ctx context.Context, path string, query url.Values, out any) error {
	u := *s.base
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Dockhand unreachable: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		s.mu.Lock()
		s.authWait = time.Now().Add(authBackoff)
		s.mu.Unlock()
		if s.token == "" {
			return errors.New("Dockhand requires an API token")
		}
		return errUnauthorized
	case resp.StatusCode == http.StatusTooManyRequests:
		s.mu.Lock()
		s.authWait = time.Now().Add(authBackoff)
		s.mu.Unlock()
		return errors.New("Dockhand is rate limiting CPM (too many failed token attempts); retrying in 5 minutes")
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("Dockhand %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out)
}

// Containers implements sdk.Source.
func (s *source) Containers(ctx context.Context) ([]sdk.Container, error) {
	s.mu.Lock()
	wait := s.authWait
	s.mu.Unlock()
	if time.Now().Before(wait) {
		return nil, fmt.Errorf("Dockhand rejected the last request; retrying after %s", wait.Format("15:04"))
	}

	var envs []environment
	if err := s.get(ctx, "/api/environments", nil, &envs); err != nil {
		return nil, err
	}
	sort.Slice(envs, func(i, j int) bool { return envs[i].ID < envs[j].ID })

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		all  []sdk.Container
		errs []string
		sem  = make(chan struct{}, envConcurrency)
	)
	for _, env := range envs {
		if len(s.envOnly) > 0 && !s.envOnly[env.ID] {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(env environment) {
			defer func() { <-sem; wg.Done() }()
			var list []container
			q := url.Values{"env": {strconv.Itoa(env.ID)}, "all": {"true"}}
			err := s.get(ctx, "/api/containers", q, &list)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", env.Name, err))
				return
			}
			all = append(all, convert(list, s.hostIP(env), env.Name)...)
		}(env)
	}
	wg.Wait()

	sort.SliceStable(all, func(i, j int) bool {
		if all[i].HostIP != all[j].HostIP {
			return all[i].HostIP < all[j].HostIP
		}
		return all[i].Name < all[j].Name
	})
	if len(errs) > 0 {
		sort.Strings(errs)
		return all, errors.New(strings.Join(errs, "; "))
	}
	return all, nil
}

// hostIP is the address proxy rules use for an environment's host: the
// configured mapping, Dockhand's Public IP, or the host it connects to.
func (s *source) hostIP(env environment) string {
	if ip := s.hostIPs[env.ID]; ip != "" {
		return ip
	}
	if ip := net.ParseIP(strings.TrimSpace(env.PublicIP)); ip != nil {
		return ip.String()
	}
	host := strings.TrimSpace(env.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && !ip.IsLoopback() {
		return ip.String()
	}
	return ""
}

func convert(list []container, hostIP, hostName string) []sdk.Container {
	out := make([]sdk.Container, 0, len(list))
	for _, c := range list {
		ctr := sdk.Container{
			ID:     c.ID,
			Name:   strings.TrimPrefix(c.Name, "/"),
			State:  c.State,
			HostIP: hostIP,
			Host:   hostName,
		}
		seen := map[sdk.Port]bool{}
		for _, p := range c.Ports {
			if ip := net.ParseIP(p.IP); ip != nil && ip.IsLoopback() {
				continue
			}
			port := sdk.Port{Private: p.PrivatePort, Public: p.PublicPort}
			if !seen[port] {
				seen[port] = true
				ctr.Ports = append(ctr.Ports, port)
			}
		}
		out = append(out, ctr)
	}
	return out
}
