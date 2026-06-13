package models

// DiscoveryHost represents a single Docker host for auto-discovery and quick-select.
type DiscoveryHost struct {
	IP            string `json:"ip"`
	Label         string `json:"label,omitempty"`
	IsLocalDocker bool   `json:"is_local_docker,omitempty"`
}

// AppSettings holds persistent application settings stored in settings.json.
type AppSettings struct {
	DiscoveryHosts []DiscoveryHost `json:"discovery_hosts"`
}
