package models

// ServiceTemplate represents a pre-configured template for common services
type ServiceTemplate struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Icon            string   `json:"icon"`
	Category        string   `json:"category"`
	TargetPort      string   `json:"target_port"`
	IsHTTPSBackend  bool     `json:"is_https_backend"`
	IsInternal      bool     `json:"is_internal"`
	Snippets        []string `json:"snippets"`
	EnableWebSocket bool     `json:"enable_websocket"`
	HealthCheckPath string   `json:"health_check_path"`
	ExtraConfig     string   `json:"extra_config"`
}

// TemplateCategory represents a category of templates
type TemplateCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// GetTemplateCategories returns all template categories
func GetTemplateCategories() []TemplateCategory {
	return []TemplateCategory{
		{ID: "web", Name: "Web Applications", Icon: "🌐"},
		{ID: "media", Name: "Media Servers", Icon: "🎬"},
		{ID: "docker", Name: "Docker & Containers", Icon: "🐳"},
		{ID: "dev", Name: "Development", Icon: "💻"},
		{ID: "monitoring", Name: "Monitoring", Icon: "📊"},
		{ID: "home", Name: "Home Automation", Icon: "🏠"},
		{ID: "nas", Name: "NAS & Storage", Icon: "💾"},
		{ID: "api", Name: "API & Services", Icon: "⚡"},
	}
}

// GetServiceTemplates returns all available service templates
func GetServiceTemplates() []ServiceTemplate {
	return []ServiceTemplate{
		// Web Applications
		{
			ID:          "generic_web",
			Name:        "Web Application",
			Description: "Generic web application (HTTP backend)",
			Icon:        "🌐",
			Category:    "web",
			TargetPort:  "80",
			Snippets:    []string{"cloudflare_dns", "security_headers", "compression"},
		},
		{
			ID:          "nextcloud",
			Name:        "Nextcloud",
			Description: "Nextcloud cloud storage",
			Icon:        "☁️",
			Category:    "web",
			TargetPort:  "80",
			Snippets:    []string{"cloudflare_dns", "security_headers"},
			ExtraConfig: `header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
    }
    request_body {
        max_size 10GB
    }`,
		},
		{
			ID:          "wordpress",
			Name:        "WordPress",
			Description: "WordPress blog/CMS",
			Icon:        "📝",
			Category:    "web",
			TargetPort:  "80",
			Snippets:    []string{"cloudflare_dns", "security_headers", "compression"},
		},

		// Media Servers
		{
			ID:              "jellyfin",
			Name:            "Jellyfin",
			Description:     "Jellyfin media server",
			Icon:            "🎬",
			Category:        "media",
			TargetPort:      "8096",
			Snippets:        []string{"cloudflare_dns"},
			EnableWebSocket: true,
		},
		{
			ID:              "plex",
			Name:            "Plex",
			Description:     "Plex Media Server",
			Icon:            "🎥",
			Category:        "media",
			TargetPort:      "32400",
			Snippets:        []string{"cloudflare_dns"},
			EnableWebSocket: true,
			ExtraConfig: `request_body {
        max_size 100MB
    }`,
		},

		// Docker & Containers
		{
			ID:              "portainer",
			Name:            "Portainer",
			Description:     "Portainer Docker management",
			Icon:            "🐳",
			Category:        "docker",
			TargetPort:      "9000",
			IsInternal:      true,
			Snippets:        []string{"cloudflare_dns", "internal_only"},
			EnableWebSocket: true,
		},
		{
			ID:         "traefik_dashboard",
			Name:       "Traefik Dashboard",
			Description: "Traefik reverse proxy dashboard",
			Icon:       "🚦",
			Category:   "docker",
			TargetPort: "8080",
			IsInternal: true,
			Snippets:   []string{"cloudflare_dns", "internal_only"},
		},

		// Development
		{
			ID:          "gitea",
			Name:        "Gitea",
			Description: "Git server (Gitea/Forgejo)",
			Icon:        "🦊",
			Category:    "dev",
			TargetPort:  "3000",
			Snippets:    []string{"cloudflare_dns", "security_headers"},
			ExtraConfig: `request_body {
        max_size 1GB
    }`,
		},
		{
			ID:              "code_server",
			Name:            "Code Server",
			Description:     "VS Code in browser",
			Icon:            "💻",
			Category:        "dev",
			TargetPort:      "8443",
			IsInternal:      true,
			Snippets:        []string{"cloudflare_dns", "internal_only"},
			EnableWebSocket: true,
		},

		// Monitoring
		{
			ID:         "grafana",
			Name:       "Grafana",
			Description: "Grafana dashboards",
			Icon:       "📊",
			Category:   "monitoring",
			TargetPort: "3000",
			IsInternal: true,
			Snippets:   []string{"cloudflare_dns", "internal_only"},
		},
		{
			ID:         "prometheus",
			Name:       "Prometheus",
			Description: "Prometheus metrics",
			Icon:       "🔥",
			Category:   "monitoring",
			TargetPort: "9090",
			IsInternal: true,
			Snippets:   []string{"cloudflare_dns", "internal_only"},
		},

		// Home Automation
		{
			ID:              "home_assistant",
			Name:            "Home Assistant",
			Description:     "Home Assistant automation",
			Icon:            "🏠",
			Category:        "home",
			TargetPort:      "8123",
			Snippets:        []string{"cloudflare_dns"},
			EnableWebSocket: true,
		},
		{
			ID:         "homebridge",
			Name:       "Homebridge",
			Description: "Homebridge for Apple HomeKit",
			Icon:       "🍎",
			Category:   "home",
			TargetPort: "8581",
			IsInternal: true,
			Snippets:   []string{"cloudflare_dns", "internal_only"},
		},

		// NAS & Storage
		{
			ID:             "synology_dsm",
			Name:           "Synology DSM",
			Description:    "Synology DiskStation Manager",
			Icon:           "💾",
			Category:       "nas",
			TargetPort:     "5000",
			IsHTTPSBackend: true,
			IsInternal:     true,
			Snippets:       []string{"cloudflare_dns", "internal_only"},
		},
		{
			ID:          "synology_photos",
			Name:        "Synology Photos",
			Description: "Synology Photos application",
			Icon:        "📷",
			Category:    "nas",
			TargetPort:  "80",
			Snippets:    []string{"cloudflare_dns"},
		},

		// API & Services
		{
			ID:              "api_service",
			Name:            "REST API",
			Description:     "REST API service",
			Icon:            "⚡",
			Category:        "api",
			TargetPort:      "8080",
			Snippets:        []string{"cloudflare_dns", "security_headers", "compression", "rate_limit"},
			HealthCheckPath: "/health",
		},
		{
			ID:              "websocket_service",
			Name:            "WebSocket Service",
			Description:     "WebSocket server",
			Icon:            "🔌",
			Category:        "api",
			TargetPort:      "8080",
			Snippets:        []string{"cloudflare_dns"},
			EnableWebSocket: true,
		},
	}
}

// GetTemplatesByCategory returns templates grouped by category
func GetTemplatesByCategory() map[string][]ServiceTemplate {
	result := make(map[string][]ServiceTemplate)
	for _, t := range GetServiceTemplates() {
		result[t.Category] = append(result[t.Category], t)
	}
	return result
}

// GetTemplateByID finds a template by ID
func GetTemplateByID(id string) *ServiceTemplate {
	for _, t := range GetServiceTemplates() {
		if t.ID == id {
			return &t
		}
	}
	return nil
}
