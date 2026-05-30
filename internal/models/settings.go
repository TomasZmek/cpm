package models

// AppSettings holds persistent application settings stored in settings.json.
type AppSettings struct {
	DiscoveryIP string `json:"discovery_ip"`
}
