package services

import (
	"fmt"
	"log"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
)

// DiscoveryResult represents a single discoverable container+port combination.
type DiscoveryResult struct {
	ContainerName string
	Port          uint16 // PrivatePort — container's internal port
	PublicPort    uint16 // PublicPort — host-mapped port (0 if not mapped)
	DiscoveryIP   string
	HostLabel     string
	Paired        bool
	PairedDomain  string
	SuggestedName string
}

// DiscoverContainers lists running containers for a single host and matches them against existing proxy rules.
func DiscoverContainers(dockerSvc *DockerService, caddySvc *CaddyService, host models.DiscoveryHost) ([]DiscoveryResult, error) {
	sites, _ := caddySvc.GetAllSites()
	return discoverHostSites(dockerSvc, host, sites)
}

// DiscoverAllHosts runs discovery for all configured hosts and merges the results.
// Errors on individual hosts are logged but do not abort the whole operation.
func DiscoverAllHosts(dockerSvc *DockerService, caddySvc *CaddyService, hosts []models.DiscoveryHost) ([]DiscoveryResult, error) {
	sites, _ := caddySvc.GetAllSites()

	var all []DiscoveryResult
	for _, host := range hosts {
		results, err := discoverHostSites(dockerSvc, host, sites)
		if err != nil {
			log.Printf("Discovery error for host %s: %v", host.IP, err)
			continue
		}
		all = append(all, results...)
	}
	return all, nil
}

// discoverHostSites is the shared implementation used by both public discovery functions.
func discoverHostSites(dockerSvc *DockerService, host models.DiscoveryHost, sites []*models.Site) ([]DiscoveryResult, error) {
	containers, err := dockerSvc.ListDiscoverableContainers()
	if err != nil {
		return nil, err
	}

	var results []DiscoveryResult
	for _, c := range containers {
		port := uint16(0)
		publicPort := uint16(0)
		if len(c.Ports) > 0 {
			port = c.Ports[0].PrivatePort
			publicPort = c.Ports[0].PublicPort
		}

		result := DiscoveryResult{
			ContainerName: c.Name,
			Port:          port,
			PublicPort:    publicPort,
			DiscoveryIP:   host.IP,
			HostLabel:     host.Label,
			SuggestedName: SuggestContainerName(c.Name),
		}

		result.Paired, result.PairedDomain = findPairedSite(sites, host.IP, c.Name, port, publicPort)

		results = append(results, result)
	}

	return results, nil
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

func findPairedSite(sites []*models.Site, ip, name string, privatePort, publicPort uint16) (bool, string) {
	privStr := fmt.Sprintf("%d", privatePort)
	pubStr := fmt.Sprintf("%d", publicPort)
	for _, s := range sites {
		// Rules created with a service-name target store the container name in
		// TargetIP (e.g. "reverse_proxy jellyfin:8096"). The name uniquely
		// identifies the container, so a name match counts as paired regardless of
		// port (the container's first exposed port may differ from the rule's).
		if name != "" && s.TargetIP == name {
			return true, s.PrimaryDomain()
		}
		// IP-based rules: an IP can host many containers, so require a port match.
		if s.TargetIP != ip {
			continue
		}
		if s.TargetPort == privStr || (publicPort > 0 && s.TargetPort == pubStr) {
			return true, s.PrimaryDomain()
		}
	}
	return false, ""
}
