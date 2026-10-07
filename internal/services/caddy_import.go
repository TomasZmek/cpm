package services

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/TomasZmek/cpm/internal/models"
)

// MainCaddyfilePath returns the path to the main Caddyfile.
func (c *CaddyService) MainCaddyfilePath() string {
	return filepath.Join(c.config.ConfigDir, "Caddyfile")
}

// PreviewImportFromMainCaddyfile parses the site blocks written directly in the
// main Caddyfile and returns them WITHOUT writing anything to disk.
func (c *CaddyService) PreviewImportFromMainCaddyfile() ([]*models.Site, error) {
	return c.PreviewImportFromFile(c.MainCaddyfilePath())
}

// PreviewImportFromFile parses site blocks from the Caddyfile at the given path
// WITHOUT writing anything to disk.
func (c *CaddyService) PreviewImportFromFile(path string) ([]*models.Site, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Caddyfile %q: %w", path, err)
	}
	return c.PreviewImportContent(string(content))
}

// PreviewImportContent parses site blocks directly from Caddyfile text
// (e.g. an uploaded file) WITHOUT writing anything to disk.
func (c *CaddyService) PreviewImportContent(content string) ([]*models.Site, error) {
	sites, err := c.parser.ParseMainCaddyfile(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Caddyfile: %w", err)
	}
	return sites, nil
}

// ImportFromMainCaddyfile parses the site blocks written directly in the main
// Caddyfile and saves each one as sites/standard/{domain}.caddy.
//
// It returns the primary domains that were imported, the ones that failed
// (already-existing files or write errors, with a reason appended), and a fatal
// error only if the Caddyfile could not be read or parsed at all.
func (c *CaddyService) ImportFromMainCaddyfile() (imported []string, failed []string, err error) {
	return c.ImportFromFile(c.MainCaddyfilePath())
}

// ImportFromFile parses the Caddyfile at path and saves each site block to
// sites/standard/{domain}.caddy.
func (c *CaddyService) ImportFromFile(path string) (imported []string, failed []string, err error) {
	sites, err := c.PreviewImportFromFile(path)
	if err != nil {
		return nil, nil, err
	}
	return c.saveImportedSites(sites)
}

// ImportContent parses Caddyfile text and saves each site block to sites/standard.
func (c *CaddyService) ImportContent(content string) (imported []string, failed []string, err error) {
	sites, err := c.PreviewImportContent(content)
	if err != nil {
		return nil, nil, err
	}
	return c.saveImportedSites(sites)
}

// saveImportedSites writes each parsed site to sites/standard/{domain}.caddy.
func (c *CaddyService) saveImportedSites(sites []*models.Site) (imported []string, failed []string, err error) {
	standardDir := filepath.Join(c.config.SitesDir, "standard")
	if mkErr := os.MkdirAll(standardDir, 0755); mkErr != nil {
		return nil, nil, fmt.Errorf("failed to create standard sites directory: %w", mkErr)
	}

	for _, site := range sites {
		domain := site.PrimaryDomain()
		site.Filename = sanitizeFilename(domain)
		dest := filepath.Join(standardDir, site.Filename+".caddy")

		// Do not overwrite an existing managed site.
		if _, statErr := os.Stat(dest); statErr == nil {
			failed = append(failed, fmt.Sprintf("%s (already exists)", domain))
			continue
		}

		site.Filepath = dest
		if writeErr := os.WriteFile(dest, []byte(site.ToCaddyfile()), 0644); writeErr != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", domain, writeErr))
			continue
		}

		imported = append(imported, domain)
	}

	return imported, failed, nil
}
