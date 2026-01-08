package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/TomasZmek/cpm/internal/config"
	"github.com/TomasZmek/cpm/internal/models"
)

// ReloadResult represents the result of a reload operation
type ReloadResult struct {
	Success bool
	Message string
	Error   string
}

// CaddyService handles Caddy configuration management
type CaddyService struct {
	config        *config.Config
	dockerService *DockerService
	parser        *ParserService
}

// NewCaddyService creates a new Caddy service
func NewCaddyService(cfg *config.Config, dockerService *DockerService) *CaddyService {
	return &CaddyService{
		config:        cfg,
		dockerService: dockerService,
		parser:        NewParserService(),
	}
}

// GetAllSites returns all proxy rules
func (c *CaddyService) GetAllSites() ([]*models.Site, error) {
	sitesDir := c.config.SitesDir

	// Ensure directory exists
	if _, err := os.Stat(sitesDir); os.IsNotExist(err) {
		return []*models.Site{}, nil
	}

	entries, err := os.ReadDir(sitesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sites directory: %w", err)
	}

	var sites []*models.Site

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".caddy") {
			continue
		}

		// Skip fallback
		if name == "fallback.caddy" {
			continue
		}

		filepath := filepath.Join(sitesDir, name)
		site, err := c.loadSite(filepath)
		if err != nil {
			// Log error but continue
			fmt.Printf("Warning: Could not load site %s: %v\n", name, err)
			continue
		}

		sites = append(sites, site)
	}

	// Sort by primary domain
	sort.Slice(sites, func(i, j int) bool {
		return sites[i].PrimaryDomain() < sites[j].PrimaryDomain()
	})

	return sites, nil
}

// GetSite returns a single site by filename
func (c *CaddyService) GetSite(filename string) (*models.Site, error) {
	filepath := filepath.Join(c.config.SitesDir, filename)
	if !strings.HasSuffix(filepath, ".caddy") {
		filepath += ".caddy"
	}
	return c.loadSite(filepath)
}

// loadSite loads a site from file
func (c *CaddyService) loadSite(filepath string) (*models.Site, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	filename := strings.TrimSuffix(filepath[strings.LastIndex(filepath, "/")+1:], ".caddy")

	site := c.parser.Parse(string(content), filename)
	site.Filepath = filepath
	site.Filename = filename

	// Get modification time
	info, err := os.Stat(filepath)
	if err == nil {
		site.ModifiedAt = info.ModTime()
	}

	return site, nil
}

// CreateSite creates a new proxy rule
func (c *CaddyService) CreateSite(site *models.Site) error {
	// Generate filename from primary domain
	if site.Filename == "" {
		site.Filename = sanitizeFilename(site.PrimaryDomain())
	}

	site.Filepath = filepath.Join(c.config.SitesDir, site.Filename+".caddy")

	// Check if file already exists
	if _, err := os.Stat(site.Filepath); err == nil {
		return fmt.Errorf("site already exists: %s", site.Filename)
	}

	// Ensure sites directory exists
	if err := os.MkdirAll(c.config.SitesDir, 0755); err != nil {
		return fmt.Errorf("failed to create sites directory: %w", err)
	}

	// Generate Caddyfile content
	content := site.ToCaddyfile()

	// Write file
	if err := os.WriteFile(site.Filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write site file: %w", err)
	}

	return nil
}

// UpdateSite updates an existing proxy rule
func (c *CaddyService) UpdateSite(site *models.Site) error {
	if site.Filepath == "" {
		site.Filepath = filepath.Join(c.config.SitesDir, site.Filename+".caddy")
	}

	// Check if file exists
	if _, err := os.Stat(site.Filepath); os.IsNotExist(err) {
		return fmt.Errorf("site not found: %s", site.Filename)
	}

	// Generate Caddyfile content
	content := site.ToCaddyfile()

	// Write file
	if err := os.WriteFile(site.Filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write site file: %w", err)
	}

	return nil
}

// UpdateSiteRaw updates a site with raw content
func (c *CaddyService) UpdateSiteRaw(filename, content string) error {
	filepath := filepath.Join(c.config.SitesDir, filename)
	if !strings.HasSuffix(filepath, ".caddy") {
		filepath += ".caddy"
	}

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write site file: %w", err)
	}

	return nil
}

// DeleteSite deletes a proxy rule
func (c *CaddyService) DeleteSite(filename string) error {
	filepath := filepath.Join(c.config.SitesDir, filename)
	if !strings.HasSuffix(filepath, ".caddy") {
		filepath += ".caddy"
	}

	if err := os.Remove(filepath); err != nil {
		return fmt.Errorf("failed to delete site file: %w", err)
	}

	return nil
}

// DuplicateSite creates a copy of a site with new domains
func (c *CaddyService) DuplicateSite(sourceFilename string, newDomains []string) (*models.Site, error) {
	source, err := c.GetSite(sourceFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to load source site: %w", err)
	}

	// Create new site with same settings
	newSite := &models.Site{
		Domains:            newDomains,
		TargetIP:           source.TargetIP,
		TargetPort:         source.TargetPort,
		IsHTTPSBackend:     source.IsHTTPSBackend,
		IsInternal:         source.IsInternal,
		Snippets:           source.Snippets,
		Tags:               source.Tags,
		AdditionalBackends: source.AdditionalBackends,
		LBPolicy:           source.LBPolicy,
		EnableWebSocket:    source.EnableWebSocket,
		HealthCheckPath:    source.HealthCheckPath,
		TimeoutSeconds:     source.TimeoutSeconds,
		BasicAuthEnabled:   source.BasicAuthEnabled,
		BasicAuthUsers:     source.BasicAuthUsers,
		ExtraConfig:        source.ExtraConfig,
	}

	if err := c.CreateSite(newSite); err != nil {
		return nil, err
	}

	return newSite, nil
}

// Reload reloads Caddy configuration
func (c *CaddyService) Reload() *ReloadResult {
	if err := c.dockerService.ReloadCaddy(); err != nil {
		return &ReloadResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return &ReloadResult{
		Success: true,
		Message: "Configuration reloaded successfully",
	}
}

// Validate validates Caddy configuration
func (c *CaddyService) Validate() *ReloadResult {
	if err := c.dockerService.ValidateConfig(); err != nil {
		return &ReloadResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	return &ReloadResult{
		Success: true,
		Message: "Configuration is valid",
	}
}

// ReloadWithValidation validates and then reloads
func (c *CaddyService) ReloadWithValidation() *ReloadResult {
	// First validate
	if result := c.Validate(); !result.Success {
		return &ReloadResult{
			Success: false,
			Error:   fmt.Sprintf("Validation failed: %s", result.Error),
		}
	}

	// Then reload
	return c.Reload()
}

// GetFallback returns the fallback rule content
func (c *CaddyService) GetFallback() (string, error) {
	filepath := filepath.Join(c.config.SitesDir, "fallback.caddy")
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SaveFallback saves the fallback rule
func (c *CaddyService) SaveFallback(content string) error {
	filepath := filepath.Join(c.config.SitesDir, "fallback.caddy")
	return os.WriteFile(filepath, []byte(content), 0644)
}

// FallbackExists checks if fallback.caddy exists
func (c *CaddyService) FallbackExists() bool {
	filepath := filepath.Join(c.config.SitesDir, "fallback.caddy")
	_, err := os.Stat(filepath)
	return err == nil
}

// GetErrorPage returns an error page content
func (c *CaddyService) GetErrorPage(code int) (string, error) {
	filepath := filepath.Join(c.config.ConfigDir, "pages", fmt.Sprintf("%d.html", code))
	content, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SaveErrorPage saves an error page
func (c *CaddyService) SaveErrorPage(code int, content string) error {
	dir := filepath.Join(c.config.ConfigDir, "pages")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	filepath := filepath.Join(dir, fmt.Sprintf("%d.html", code))
	return os.WriteFile(filepath, []byte(content), 0644)
}

// GetAllTags returns all unique tags from all sites
func (c *CaddyService) GetAllTags() ([]string, error) {
	sites, err := c.GetAllSites()
	if err != nil {
		return nil, err
	}

	tagSet := make(map[string]bool)
	for _, site := range sites {
		for _, tag := range site.Tags {
			tagSet[tag] = true
		}
	}

	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	return tags, nil
}

// GetRecentChanges returns recently modified sites
func (c *CaddyService) GetRecentChanges(limit int) ([]*models.Site, error) {
	sites, err := c.GetAllSites()
	if err != nil {
		return nil, err
	}

	// Sort by modification time (newest first)
	sort.Slice(sites, func(i, j int) bool {
		return sites[i].ModifiedAt.After(sites[j].ModifiedAt)
	})

	if len(sites) > limit {
		sites = sites[:limit]
	}

	return sites, nil
}

// GetStats returns statistics about the configuration
func (c *CaddyService) GetStats() map[string]interface{} {
	sites, _ := c.GetAllSites()
	tags, _ := c.GetAllTags()

	internal := 0
	public := 0
	withAuth := 0

	for _, site := range sites {
		if site.IsInternal {
			internal++
		} else {
			public++
		}
		if site.BasicAuthEnabled {
			withAuth++
		}
	}

	return map[string]interface{}{
		"total":     len(sites),
		"internal":  internal,
		"public":    public,
		"with_auth": withAuth,
		"tags":      len(tags),
	}
}

// sanitizeFilename creates a safe filename from domain
func sanitizeFilename(domain string) string {
	// Remove wildcards
	domain = strings.TrimPrefix(domain, "*.")
	// Replace invalid characters
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(domain)
}

// TimeAgo returns human-readable time difference
func TimeAgo(t time.Time) string {
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
