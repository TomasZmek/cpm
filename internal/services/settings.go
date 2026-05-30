package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
)

// SettingsService manages persistent application settings (settings.json).
type SettingsService struct {
	path string
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(configDir string) *SettingsService {
	return &SettingsService{
		path: filepath.Join(configDir, "settings.json"),
	}
}

// migrationSettings handles backward-compatible deserialization of legacy discovery_ip.
type migrationSettings struct {
	DiscoveryIP    string                 `json:"discovery_ip"`
	DiscoveryHosts []models.DiscoveryHost `json:"discovery_hosts"`
}

// Get loads settings from disk. Returns empty defaults if the file doesn't exist yet.
// Automatically migrates legacy discovery_ip to discovery_hosts on first load.
func (s *SettingsService) Get() (*models.AppSettings, error) {
	settings := &models.AppSettings{}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return settings, err
	}

	var migration migrationSettings
	if err := json.Unmarshal(data, &migration); err != nil {
		log.Printf("Error parsing settings.json: %v", err)
		return &models.AppSettings{}, nil
	}

	// Migrate legacy single discovery_ip to the new hosts list.
	if migration.DiscoveryIP != "" && len(migration.DiscoveryHosts) == 0 {
		migration.DiscoveryHosts = []models.DiscoveryHost{
			{IP: migration.DiscoveryIP},
		}
	}

	settings.DiscoveryHosts = migration.DiscoveryHosts
	return settings, nil
}

// DetectLocalIP scans network interfaces and returns the best candidate for the local Docker host IP.
// It skips loopback, docker/bridge/veth interfaces, and the 172.16–31.x range.
// IPs in 192.168.x.x are preferred.
func DetectLocalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var candidates []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		n := iface.Name
		if strings.HasPrefix(n, "docker") || strings.HasPrefix(n, "br-") || strings.HasPrefix(n, "veth") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.To4() == nil || ip.IsLoopback() {
				continue
			}
			// Skip 172.16.0.0/12
			if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
				continue
			}
			candidates = append(candidates, ip.String())
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no suitable local IP address found")
	}
	for _, ip := range candidates {
		if strings.HasPrefix(ip, "192.168.") {
			return ip, nil
		}
	}
	return candidates[0], nil
}

// Save writes settings to disk.
func (s *SettingsService) Save(settings *models.AppSettings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return err
	}

	log.Printf("App settings saved to %s", s.path)
	return nil
}
