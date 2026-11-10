package models

// DiscoveryHost represents a single Docker host for auto-discovery and quick-select.
type DiscoveryHost struct {
	IP            string `json:"ip"`
	Label         string `json:"label,omitempty"`
	IsLocalDocker bool   `json:"is_local_docker,omitempty"`
}

// AppSettings holds persistent application settings stored in settings.json.
type AppSettings struct {
	DiscoveryHosts []DiscoveryHost   `json:"discovery_hosts"`
	AccessLog      AccessLogSettings `json:"access_log"`
}

// Access log defaults and limits (Settings → Logs).
const (
	DefaultAccessLogRollSizeMB = 10
	DefaultAccessLogRollKeep   = 5
	MaxAccessLogRollSizeMB     = 100
	MaxAccessLogRollKeep       = 50
)

// AccessLogSettings configures the access log Caddy writes for all rules.
type AccessLogSettings struct {
	Enabled    bool `json:"enabled"`
	RollSizeMB int  `json:"roll_size_mb,omitempty"`
	RollKeep   int  `json:"roll_keep,omitempty"`
	// MaskIPs truncates client addresses (IPv4 /24, IPv6 /48) before they are written.
	MaskIPs bool `json:"mask_ips,omitempty"`
}

// Normalized returns the settings with defaults applied and values clamped
// to their allowed range.
func (a AccessLogSettings) Normalized() AccessLogSettings {
	if a.RollSizeMB <= 0 {
		a.RollSizeMB = DefaultAccessLogRollSizeMB
	} else if a.RollSizeMB > MaxAccessLogRollSizeMB {
		a.RollSizeMB = MaxAccessLogRollSizeMB
	}
	if a.RollKeep <= 0 {
		a.RollKeep = DefaultAccessLogRollKeep
	} else if a.RollKeep > MaxAccessLogRollKeep {
		a.RollKeep = MaxAccessLogRollKeep
	}
	return a
}
