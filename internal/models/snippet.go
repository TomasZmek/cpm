package models

// SnippetConfig represents the configuration for all snippets
type SnippetConfig struct {
	CloudflareDNS   CloudflareDNSConfig   `json:"cloudflare_dns"`
	InternalOnly    InternalOnlyConfig    `json:"internal_only"`
	SecurityHeaders SecurityHeadersConfig `json:"security_headers"`
	Compression     CompressionConfig     `json:"compression"`
	RateLimit       RateLimitConfig       `json:"rate_limit"`
	BasicAuth       BasicAuthConfig       `json:"basic_auth"`
}

// CloudflareDNSConfig holds Cloudflare DNS challenge settings
type CloudflareDNSConfig struct {
	Enabled  bool   `json:"enabled"`
	UseEnv   bool   `json:"use_env"`   // Use CF_API_TOKEN env var
	APIToken string `json:"api_token"` // Direct token (if not using env)
}

// InternalOnlyConfig holds internal network restriction settings
type InternalOnlyConfig struct {
	Enabled         bool     `json:"enabled"`
	AllowedNetworks []string `json:"allowed_networks"` // CIDR ranges
}

// SecurityHeadersConfig holds security headers settings
type SecurityHeadersConfig struct {
	Enabled               bool   `json:"enabled"`
	HSTSMaxAge            int    `json:"hsts_max_age"`
	HSTSIncludeSubdomains bool   `json:"hsts_include_subdomains"`
	XContentTypeOptions   bool   `json:"x_content_type_options"`
	XFrameOptions         string `json:"x_frame_options"` // DENY, SAMEORIGIN
	ReferrerPolicy        string `json:"referrer_policy"`
	HideServer            bool   `json:"hide_server"`
}

// CompressionConfig holds compression settings
type CompressionConfig struct {
	Enabled bool `json:"enabled"`
	Zstd    bool `json:"zstd"`
	Gzip    bool `json:"gzip"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	Enabled    bool `json:"enabled"`
	Requests   int  `json:"requests"`    // Requests per window
	WindowSecs int  `json:"window_secs"` // Window in seconds
}

// BasicAuthConfig holds basic auth settings
type BasicAuthConfig struct {
	Enabled bool              `json:"enabled"`
	Users   map[string]string `json:"users"` // username -> bcrypt hash
}

// DefaultSnippetConfig returns default configuration
func DefaultSnippetConfig() *SnippetConfig {
	return &SnippetConfig{
		CloudflareDNS: CloudflareDNSConfig{
			Enabled: true,
			UseEnv:  true,
		},
		InternalOnly: InternalOnlyConfig{
			Enabled: true,
			AllowedNetworks: []string{
				"192.168.0.0/16",
				"172.16.0.0/12",
				"10.0.0.0/8",
			},
		},
		SecurityHeaders: SecurityHeadersConfig{
			Enabled:               true,
			HSTSMaxAge:            31536000,
			HSTSIncludeSubdomains: true,
			XContentTypeOptions:   true,
			XFrameOptions:         "DENY",
			ReferrerPolicy:        "strict-origin-when-cross-origin",
			HideServer:            true,
		},
		Compression: CompressionConfig{
			Enabled: true,
			Zstd:    true,
			Gzip:    true,
		},
		RateLimit: RateLimitConfig{
			Enabled:    false,
			Requests:   100,
			WindowSecs: 60,
		},
		BasicAuth: BasicAuthConfig{
			Enabled: false,
			Users:   make(map[string]string),
		},
	}
}

// Snippet represents a known snippet
type Snippet struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Enabled     bool   `json:"enabled"`
}

// KnownSnippets returns all known snippets
func KnownSnippets() []Snippet {
	return []Snippet{
		{
			ID:          "cloudflare_dns",
			Name:        "Cloudflare DNS",
			Description: "Automatic SSL via DNS challenge",
			Icon:        "☁️",
		},
		{
			ID:          "internal_only",
			Name:        "Internal Only",
			Description: "Restrict to LAN networks",
			Icon:        "🔒",
		},
		{
			ID:          "security_headers",
			Name:        "Security Headers",
			Description: "HSTS, X-Frame-Options, etc.",
			Icon:        "🛡️",
		},
		{
			ID:          "compression",
			Name:        "Compression",
			Description: "Zstd and Gzip compression",
			Icon:        "📦",
		},
		{
			ID:          "rate_limit",
			Name:        "Rate Limit",
			Description: "Request throttling",
			Icon:        "⏱️",
		},
		{
			ID:          "basic_auth",
			Name:        "Basic Auth",
			Description: "HTTP authentication",
			Icon:        "🔐",
		},
	}
}
