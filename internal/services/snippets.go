package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TomasZmek/cpm/internal/config"
	"github.com/TomasZmek/cpm/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// SnippetsService handles snippet configuration
type SnippetsService struct {
	config     *config.Config
	configPath string
}

// NewSnippetsService creates a new snippets service
func NewSnippetsService(cfg *config.Config) *SnippetsService {
	return &SnippetsService{
		config:     cfg,
		configPath: filepath.Join(cfg.ConfigDir, ".snippets_config.json"),
	}
}

// GetConfig returns the current snippet configuration
func (s *SnippetsService) GetConfig() (*models.SnippetConfig, error) {
	if _, err := os.Stat(s.configPath); os.IsNotExist(err) {
		return models.DefaultSnippetConfig(), nil
	}

	content, err := os.ReadFile(s.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// First try new format
	var cfg models.SnippetConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		// Try to parse with flexible basic_auth format for migration
		var rawCfg map[string]json.RawMessage
		if jsonErr := json.Unmarshal(content, &rawCfg); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}

		// Parse each section manually
		cfg = *models.DefaultSnippetConfig()

		if raw, ok := rawCfg["cloudflare_dns"]; ok {
			json.Unmarshal(raw, &cfg.CloudflareDNS)
		}
		if raw, ok := rawCfg["internal_only"]; ok {
			json.Unmarshal(raw, &cfg.InternalOnly)
		}
		if raw, ok := rawCfg["security_headers"]; ok {
			json.Unmarshal(raw, &cfg.SecurityHeaders)
		}
		if raw, ok := rawCfg["compression"]; ok {
			json.Unmarshal(raw, &cfg.Compression)
		}
		if raw, ok := rawCfg["rate_limit"]; ok {
			json.Unmarshal(raw, &cfg.RateLimit)
		}

		// Handle basic_auth specially - might be old format
		if raw, ok := rawCfg["basic_auth"]; ok {
			// Try new format first
			var basicAuth models.BasicAuthConfig
			if err := json.Unmarshal(raw, &basicAuth); err != nil {
				// Try old format with array users
				var oldFormat struct {
					Enabled bool `json:"enabled"`
					Users   []struct {
						Username     string `json:"username"`
						PasswordHash string `json:"password_hash"`
					} `json:"users"`
				}
				if err := json.Unmarshal(raw, &oldFormat); err == nil {
					cfg.BasicAuth.Enabled = oldFormat.Enabled
					cfg.BasicAuth.Users = make(map[string]string)
					for _, u := range oldFormat.Users {
						cfg.BasicAuth.Users[u.Username] = u.PasswordHash
					}
				}
			} else {
				cfg.BasicAuth = basicAuth
			}
		}

		// Ensure Users map is initialized
		if cfg.BasicAuth.Users == nil {
			cfg.BasicAuth.Users = make(map[string]string)
		}

		// Save in new format for future
		s.SaveConfig(&cfg)
	}

	// Ensure Users map is initialized
	if cfg.BasicAuth.Users == nil {
		cfg.BasicAuth.Users = make(map[string]string)
	}

	return &cfg, nil
}

// SaveConfig saves the snippet configuration
func (s *SnippetsService) SaveConfig(cfg *models.SnippetConfig) error {
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(s.configPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Regenerate snippets.caddy
	return s.GenerateSnippetsFile(cfg)
}

// GenerateSnippetsFile generates the snippets.caddy file from config
func (s *SnippetsService) GenerateSnippetsFile(cfg *models.SnippetConfig) error {
	var lines []string

	lines = append(lines, "# CPM - Caddy Proxy Manager - Generated Snippets")
	lines = append(lines, "# ================================================")
	lines = append(lines, "# DO NOT EDIT MANUALLY - Use the CPM web interface")
	lines = append(lines, "")

	// Cloudflare DNS
	if cfg.CloudflareDNS.Enabled {
		lines = append(lines, "# --- CLOUDFLARE DNS CHALLENGE ---")
		lines = append(lines, "(cloudflare_dns) {")
		lines = append(lines, "    tls {")
		if cfg.CloudflareDNS.UseEnv {
			lines = append(lines, "        dns cloudflare {env.CF_API_TOKEN}")
		} else if cfg.CloudflareDNS.APIToken != "" {
			lines = append(lines, fmt.Sprintf("        dns cloudflare %s", cfg.CloudflareDNS.APIToken))
		}
		lines = append(lines, "    }")
		lines = append(lines, "}")
		lines = append(lines, "")
	}

	// Internal Only
	if cfg.InternalOnly.Enabled && len(cfg.InternalOnly.AllowedNetworks) > 0 {
		lines = append(lines, "# --- INTERNAL NETWORK ONLY ---")
		lines = append(lines, "(internal_only) {")
		lines = append(lines, fmt.Sprintf("    @denied not client_ip %s", strings.Join(cfg.InternalOnly.AllowedNetworks, " ")))
		lines = append(lines, "    handle @denied {")
		lines = append(lines, "        error 403")
		lines = append(lines, "    }")
		lines = append(lines, "    handle_errors {")
		lines = append(lines, "        @403 {")
		lines = append(lines, "            expression {http.error.status_code} == 403")
		lines = append(lines, "        }")
		lines = append(lines, "        handle @403 {")
		lines = append(lines, "            root * /usr/share/caddy/pages")
		lines = append(lines, "            rewrite * /403.html")
		lines = append(lines, "            file_server")
		lines = append(lines, "        }")
		lines = append(lines, "    }")
		lines = append(lines, "}")
		lines = append(lines, "")
	}

	// Security Headers
	if cfg.SecurityHeaders.Enabled {
		lines = append(lines, "# --- SECURITY HEADERS ---")
		lines = append(lines, "(security_headers) {")
		lines = append(lines, "    header {")

		if cfg.SecurityHeaders.HSTSMaxAge > 0 {
			hsts := fmt.Sprintf("max-age=%d", cfg.SecurityHeaders.HSTSMaxAge)
			if cfg.SecurityHeaders.HSTSIncludeSubdomains {
				hsts += "; includeSubDomains"
			}
			lines = append(lines, fmt.Sprintf(`        Strict-Transport-Security "%s"`, hsts))
		}

		if cfg.SecurityHeaders.XContentTypeOptions {
			lines = append(lines, `        X-Content-Type-Options "nosniff"`)
		}

		if cfg.SecurityHeaders.XFrameOptions != "" {
			lines = append(lines, fmt.Sprintf(`        X-Frame-Options "%s"`, cfg.SecurityHeaders.XFrameOptions))
		}

		if cfg.SecurityHeaders.ReferrerPolicy != "" {
			lines = append(lines, fmt.Sprintf(`        Referrer-Policy "%s"`, cfg.SecurityHeaders.ReferrerPolicy))
		}

		if cfg.SecurityHeaders.HideServer {
			lines = append(lines, "        -Server")
		}

		lines = append(lines, "    }")
		lines = append(lines, "}")
		lines = append(lines, "")
	}

	// Compression
	if cfg.Compression.Enabled {
		lines = append(lines, "# --- COMPRESSION ---")
		lines = append(lines, "(compression) {")
		var encodings []string
		if cfg.Compression.Zstd {
			encodings = append(encodings, "zstd")
		}
		if cfg.Compression.Gzip {
			encodings = append(encodings, "gzip")
		}
		if len(encodings) > 0 {
			lines = append(lines, fmt.Sprintf("    encode %s", strings.Join(encodings, " ")))
		}
		lines = append(lines, "}")
		lines = append(lines, "")
	}

	// Rate Limit
	if cfg.RateLimit.Enabled {
		lines = append(lines, "# --- RATE LIMITING ---")
		lines = append(lines, "(rate_limit) {")
		lines = append(lines, fmt.Sprintf("    rate_limit {remote.ip} %d %ds", cfg.RateLimit.Requests, cfg.RateLimit.WindowSecs))
		lines = append(lines, "}")
		lines = append(lines, "")
	}

	// Basic Auth
	if cfg.BasicAuth.Enabled && len(cfg.BasicAuth.Users) > 0 {
		lines = append(lines, "# --- BASIC AUTHENTICATION ---")
		lines = append(lines, "(basic_auth) {")
		lines = append(lines, "    basic_auth {")
		for username, hash := range cfg.BasicAuth.Users {
			lines = append(lines, fmt.Sprintf("        %s %s", username, hash))
		}
		lines = append(lines, "    }")
		lines = append(lines, "}")
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	snippetsPath := filepath.Join(s.config.ConfigDir, "snippets.caddy")

	return os.WriteFile(snippetsPath, []byte(content), 0644)
}

// GetAvailableSnippets returns list of enabled snippets
func (s *SnippetsService) GetAvailableSnippets() ([]string, error) {
	cfg, err := s.GetConfig()
	if err != nil {
		return nil, err
	}

	var available []string

	if cfg.CloudflareDNS.Enabled {
		available = append(available, "cloudflare_dns")
	}
	if cfg.InternalOnly.Enabled {
		available = append(available, "internal_only")
	}
	if cfg.SecurityHeaders.Enabled {
		available = append(available, "security_headers")
	}
	if cfg.Compression.Enabled {
		available = append(available, "compression")
	}
	if cfg.RateLimit.Enabled {
		available = append(available, "rate_limit")
	}
	if cfg.BasicAuth.Enabled {
		available = append(available, "basic_auth")
	}

	return available, nil
}

// AddBasicAuthUser adds a new basic auth user
func (s *SnippetsService) AddBasicAuthUser(username, password string) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}

	// Hash password with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if cfg.BasicAuth.Users == nil {
		cfg.BasicAuth.Users = make(map[string]string)
	}

	cfg.BasicAuth.Users[username] = string(hash)

	return s.SaveConfig(cfg)
}

// RemoveBasicAuthUser removes a basic auth user
func (s *SnippetsService) RemoveBasicAuthUser(username string) error {
	cfg, err := s.GetConfig()
	if err != nil {
		return err
	}

	delete(cfg.BasicAuth.Users, username)

	return s.SaveConfig(cfg)
}

// ValidateIPorCIDR validates IP or CIDR format
func ValidateIPorCIDR(value string) bool {
	// Simple validation - check if it looks like IP or CIDR
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}

	// CIDR format: x.x.x.x/y
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		if len(parts) != 2 {
			return false
		}
		// Check mask is number
		for _, c := range parts[1] {
			if c < '0' || c > '9' {
				return false
			}
		}
	}

	// Check IP part has valid format
	ipPart := strings.Split(value, "/")[0]
	octets := strings.Split(ipPart, ".")
	if len(octets) != 4 {
		return false
	}

	for _, octet := range octets {
		if octet == "" {
			return false
		}
		for _, c := range octet {
			if c < '0' || c > '9' {
				return false
			}
		}
	}

	return true
}
