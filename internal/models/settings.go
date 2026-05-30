package models

// DiscoveryHost represents a single Docker host for auto-discovery.
type DiscoveryHost struct {
	IP    string `json:"ip"`
	Label string `json:"label,omitempty"`
}

// AppSettings holds persistent application settings stored in settings.json.
type AppSettings struct {
	DiscoveryHosts []DiscoveryHost `json:"discovery_hosts"`
}
