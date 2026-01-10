package models

// WildcardDomain represents a wildcard SSL certificate configuration
type WildcardDomain struct {
	Domain   string `json:"domain"`   // Base domain (e.g., "example.com")
	Provider string `json:"provider"` // DNS provider (e.g., "cloudflare")
	UseEnv   bool   `json:"use_env"`  // Use environment variable for API token
	APIToken string `json:"api_token,omitempty"` // API token (if not using env)
}

// WildcardConfig holds all wildcard domain configurations
type WildcardConfig struct {
	Domains []WildcardDomain `json:"domains"`
}
