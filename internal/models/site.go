package models

import (
	"fmt"
	"strings"
	"time"
)

// Site represents a proxy rule
type Site struct {
	Filename           string    `json:"filename"`
	Filepath           string    `json:"filepath"`
	Domains            []string  `json:"domains"`
	TargetIP           string    `json:"target_ip"`
	TargetPort         string    `json:"target_port"`
	IsHTTPSBackend     bool      `json:"is_https_backend"`
	IsInternal         bool      `json:"is_internal"`
	Snippets           []string  `json:"snippets"`
	Tags               []string  `json:"tags"`
	AdditionalBackends []string  `json:"additional_backends"`
	LBPolicy           string    `json:"lb_policy"`
	EnableWebSocket    bool      `json:"enable_websocket"`
	HealthCheckPath    string    `json:"health_check_path"`
	TimeoutSeconds     int       `json:"timeout_seconds"`
	BasicAuthEnabled   bool      `json:"basic_auth_enabled"`
	BasicAuthUsers     []string  `json:"basic_auth_users"`
	ExtraConfig        string    `json:"extra_config"`
	RawContent         string    `json:"raw_content"`
	ModifiedAt         time.Time `json:"modified_at"`
}

// PrimaryDomain returns the first domain
func (s *Site) PrimaryDomain() string {
	if len(s.Domains) == 0 {
		return s.Filename
	}
	return s.Domains[0]
}

// DomainsString returns domains as comma-separated string
func (s *Site) DomainsString() string {
	return strings.Join(s.Domains, ", ")
}

// AccessIcon returns icon based on access type
func (s *Site) AccessIcon() string {
	if s.BasicAuthEnabled {
		return "🔐"
	}
	if s.IsInternal {
		return "🔒"
	}
	return "🌍"
}

// TargetURL returns the full target URL
func (s *Site) TargetURL() string {
	protocol := "http"
	if s.IsHTTPSBackend {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s:%s", protocol, s.TargetIP, s.TargetPort)
}

// AllBackends returns all backend URLs including main
func (s *Site) AllBackends() []string {
	protocol := "http"
	if s.IsHTTPSBackend {
		protocol = "https"
	}

	main := fmt.Sprintf("%s://%s:%s", protocol, s.TargetIP, s.TargetPort)
	backends := []string{main}

	for _, backend := range s.AdditionalBackends {
		if backend = strings.TrimSpace(backend); backend != "" {
			if !strings.HasPrefix(backend, "http") {
				backend = fmt.Sprintf("%s://%s", protocol, backend)
			}
			backends = append(backends, backend)
		}
	}

	return backends
}

// ToCaddyfile generates Caddyfile content for this site
func (s *Site) ToCaddyfile() string {
	var lines []string

	// Tags as comment
	if len(s.Tags) > 0 {
		lines = append(lines, fmt.Sprintf("# @tags: %s", strings.Join(s.Tags, ", ")))
	}

	// Domain header
	lines = append(lines, fmt.Sprintf("%s {", strings.Join(s.Domains, ", ")))

	// Import snippets
	for _, snippet := range s.Snippets {
		if snippet != "" {
			lines = append(lines, fmt.Sprintf("    import %s", snippet))
		}
	}

	// Legacy support - add internal_only if internal and not in snippets
	if s.IsInternal && !contains(s.Snippets, "internal_only") {
		lines = append(lines, "    import internal_only")
	}

	// Basic Auth
	if s.BasicAuthEnabled && len(s.BasicAuthUsers) > 0 {
		lines = append(lines, "    basic_auth {")
		for _, userHash := range s.BasicAuthUsers {
			lines = append(lines, fmt.Sprintf("        %s", userHash))
		}
		lines = append(lines, "    }")
	}

	// Extra config
	if extra := strings.TrimSpace(s.ExtraConfig); extra != "" {
		for _, line := range strings.Split(extra, "\n") {
			lines = append(lines, fmt.Sprintf("    %s", strings.TrimSpace(line)))
		}
	}

	// Reverse proxy
	lines = append(lines, s.generateReverseProxy()...)

	lines = append(lines, "}")

	return strings.Join(lines, "\n") + "\n"
}

func (s *Site) generateReverseProxy() []string {
	var lines []string
	backends := s.AllBackends()

	// Simple proxy without extra settings
	simpleProxy := len(backends) == 1 &&
		!s.IsHTTPSBackend &&
		!s.EnableWebSocket &&
		s.HealthCheckPath == "" &&
		s.TimeoutSeconds == 0

	if simpleProxy {
		lines = append(lines, fmt.Sprintf("    reverse_proxy %s:%s", s.TargetIP, s.TargetPort))
		return lines
	}

	// Complex proxy
	backendStr := strings.Join(backends, " ")
	lines = append(lines, fmt.Sprintf("    reverse_proxy %s {", backendStr))

	// Load balancing policy
	if len(backends) > 1 && s.LBPolicy != "" {
		lines = append(lines, fmt.Sprintf("        lb_policy %s", s.LBPolicy))
	}

	// Health check
	if s.HealthCheckPath != "" {
		lines = append(lines, fmt.Sprintf("        health_uri %s", s.HealthCheckPath))
		lines = append(lines, "        health_interval 30s")
	}

	// WebSocket headers
	if s.EnableWebSocket {
		lines = append(lines, "        header_up Host {host}")
		lines = append(lines, "        header_up X-Real-IP {remote_host}")
		lines = append(lines, "        header_up X-Forwarded-For {remote_host}")
		lines = append(lines, "        header_up X-Forwarded-Proto {scheme}")
	}

	// Transport settings
	if s.IsHTTPSBackend || s.TimeoutSeconds > 0 {
		lines = append(lines, "        transport http {")
		if s.IsHTTPSBackend {
			lines = append(lines, "            tls_insecure_skip_verify")
		}
		if s.TimeoutSeconds > 0 {
			lines = append(lines, fmt.Sprintf("            dial_timeout %ds", s.TimeoutSeconds))
			lines = append(lines, fmt.Sprintf("            response_header_timeout %ds", s.TimeoutSeconds))
		}
		lines = append(lines, "        }")
	}

	lines = append(lines, "    }")

	return lines
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
