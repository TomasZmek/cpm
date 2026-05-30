package services

import (
	"fmt"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
)

// DiscoveryResult represents a single discoverable container+port combination.
type DiscoveryResult struct {
	ContainerName string
	Port          uint16
	DiscoveryIP   string
	Paired        bool
	PairedDomain  string
	SuggestedName string
}

// DiscoverContainers lists running containers and matches them against existing proxy rules.
func DiscoverContainers(dockerSvc *DockerService, caddySvc *CaddyService, discoveryIP string) ([]DiscoveryResult, error) {
	containers, err := dockerSvc.ListDiscoverableContainers()
	if err != nil {
		return nil, err
	}

	sites, _ := caddySvc.GetAllSites()

	var results []DiscoveryResult
	for _, c := range containers {
		port := uint16(0)
		if len(c.Ports) > 0 {
			port = c.Ports[0].PrivatePort
		}

		result := DiscoveryResult{
			ContainerName: c.Name,
			Port:          port,
			DiscoveryIP:   discoveryIP,
			SuggestedName: SuggestContainerName(c.Name),
		}

		if port > 0 {
			result.Paired, result.PairedDomain = findPairedSite(sites, discoveryIP, fmt.Sprintf("%d", port))
		}

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

func findPairedSite(sites []*models.Site, ip, port string) (bool, string) {
	for _, s := range sites {
		if s.TargetIP == ip && s.TargetPort == port {
			return true, s.PrimaryDomain()
		}
	}
	return false, ""
}
