package services

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

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
