package services

import (
	"sort"
	"strconv"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/sdk"
)

// DiscoveryResult represents a single discoverable container+port combination.
type DiscoveryResult struct {
	ContainerName string
	Port          uint16 // port published on the Docker host (0 if not published)
	PrivatePort   uint16 // port inside the container
	DiscoveryIP   string
	HostLabel     string
	Paired        bool
	PairedDomain  string
	SuggestedName string
}

// Discover turns the containers known to the inventory into discovery rows:
// one row per running container and published port, paired with an existing
// proxy rule that targets it (by container name, or by Docker host IP and
// published port). skip filters out containers that must not be offered
// (Caddy, CPM itself).
func Discover(containers []sdk.Container, sites []*models.Site, hosts []models.DiscoveryHost, skip func(sdk.Container) bool) []DiscoveryResult {
	labels := map[string]string{}
	for _, h := range hosts {
		labels[h.IP] = h.Label
	}

	var results []DiscoveryResult
	for _, c := range containers {
		if !c.Running() || (skip != nil && skip(c)) {
			continue
		}
		label := labels[c.HostIP]
		if label == "" {
			label = c.Host
		}
		row := func(private, public uint16) DiscoveryResult {
			r := DiscoveryResult{
				ContainerName: c.Name,
				Port:          public,
				PrivatePort:   private,
				DiscoveryIP:   c.HostIP,
				HostLabel:     label,
				SuggestedName: SuggestContainerName(c.Name),
			}
			r.Paired, r.PairedDomain = findPairedSite(sites, c.HostIP, c.Name, public)
			return r
		}
		published := false
		for _, p := range c.Ports {
			if p.Public > 0 {
				results = append(results, row(p.Private, p.Public))
				published = true
			}
		}
		if !published {
			var private uint16
			if len(c.Ports) > 0 {
				private = c.Ports[0].Private
			}
			results = append(results, row(private, 0))
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].DiscoveryIP != results[j].DiscoveryIP {
			return results[i].DiscoveryIP < results[j].DiscoveryIP
		}
		return strings.ToLower(results[i].ContainerName) < strings.ToLower(results[j].ContainerName)
	})
	return results
}

// SuggestContainerName derives a URL-safe name from a container name.
func SuggestContainerName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", "-")

	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}

	return strings.Trim(b.String(), "-")
}

// findPairedSite returns the rule that targets a container: rules created
// with a service-name target store the container name in TargetIP; IP-based
// rules must use the Docker host IP and the published port (several
// containers share one host IP).
func findPairedSite(sites []*models.Site, hostIP, name string, publicPort uint16) (bool, string) {
	pub := strconv.Itoa(int(publicPort))
	for _, s := range sites {
		if name != "" && strings.EqualFold(s.TargetIP, name) {
			return true, s.PrimaryDomain()
		}
		if hostIP != "" && publicPort > 0 && s.TargetIP == hostIP && s.TargetPort == pub {
			return true, s.PrimaryDomain()
		}
	}
	return false, ""
}
