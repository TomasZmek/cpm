package handlers

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

// WildcardSettings renders the wildcard settings page
func (h *Handler) WildcardSettings(c *fiber.Ctx) error {
	var domains []models.WildcardDomain
	
	if h.wildcardService != nil {
		var err error
		domains, err = h.wildcardService.GetDomains()
		if err != nil {
			log.Printf("Error getting wildcard domains: %v", err)
			domains = []models.WildcardDomain{}
		}
	} else {
		log.Printf("Warning: wildcardService is nil")
		domains = []models.WildcardDomain{}
	}

	data := h.baseData(c, "Settings - Wildcard SSL")
	data["ActiveTab"] = "wildcard"
	data["WildcardDomains"] = domains

	return c.Render("pages/settings", data, "layouts/base")
}

// WildcardAdd adds a new wildcard domain
func (h *Handler) WildcardAdd(c *fiber.Ctx) error {
	if h.wildcardService == nil {
		setFlash(c, "error", "Wildcard service not available")
		return c.Redirect("/settings/wildcard")
	}

	domain := c.FormValue("domain")
	provider := c.FormValue("provider")
	useEnv := c.FormValue("use_env") == "on"
	apiToken := c.FormValue("api_token")

	log.Printf("WildcardAdd: domain=%s, provider=%s, useEnv=%v", domain, provider, useEnv)

	if domain == "" {
		setFlash(c, "error", "Domain is required")
		return c.Redirect("/settings/wildcard")
	}

	// Check if domain already exists
	existing, _ := h.wildcardService.GetDomains()
	for _, d := range existing {
		if d.Domain == domain {
			setFlash(c, "info", "Wildcard domain already exists")
			return c.Redirect("/settings/wildcard/migrate/" + domain)
		}
	}

	wildcard := models.WildcardDomain{
		Domain:   domain,
		Provider: provider,
		UseEnv:   useEnv,
		APIToken: apiToken,
	}

	if err := h.wildcardService.AddDomain(wildcard); err != nil {
		log.Printf("Error adding wildcard domain: %v", err)
		setFlash(c, "error", "Failed to add wildcard domain: "+err.Error())
		return c.Redirect("/settings/wildcard")
	}

	// Regenerate wildcard Caddy config
	if err := h.regenerateWildcardConfig(); err != nil {
		log.Printf("Error regenerating wildcard config: %v", err)
		setFlash(c, "warning", "Domain added but failed to update Caddy config: "+err.Error())
		return c.Redirect("/settings/wildcard")
	}

	// Redirect to migration page
	return c.Redirect("/settings/wildcard/migrate/" + domain)
}

// WildcardMigratePage shows the migration options for a wildcard domain
func (h *Handler) WildcardMigratePage(c *fiber.Ctx) error {
	domain := c.Params("domain")
	
	if h.wildcardService == nil {
		setFlash(c, "error", "Wildcard service not available")
		return c.Redirect("/settings/wildcard")
	}

	// Get migration info
	info, err := h.wildcardService.GetMigrationInfo(
		domain,
		h.config.SitesDir,
		h.config.DataDir,
	)
	if err != nil {
		log.Printf("Error getting migration info: %v", err)
		setFlash(c, "error", "Failed to get migration info")
		return c.Redirect("/settings/wildcard")
	}

	data := h.baseData(c, "Migrate to Wildcard SSL")
	data["ActiveTab"] = "wildcard"
	data["MigrationInfo"] = info
	data["Domain"] = domain

	return c.Render("pages/wildcard_migrate", data, "layouts/base")
}

// WildcardMigrateExecute performs the migration
func (h *Handler) WildcardMigrateExecute(c *fiber.Ctx) error {
	domain := c.Params("domain")
	migrateSites := c.FormValue("migrate_sites") == "on"
	deleteCerts := c.FormValue("delete_certs") == "on"

	log.Printf("WildcardMigrateExecute: domain=%s, migrateSites=%v, deleteCerts=%v", domain, migrateSites, deleteCerts)

	// 1. Create backup first
	_, backupName, err := h.backupService.CreateBackup()
	if err != nil {
		log.Printf("Error creating backup: %v", err)
		setFlash(c, "error", "Failed to create backup before migration: "+err.Error())
		return c.Redirect("/settings/wildcard/migrate/" + domain)
	}
	log.Printf("Backup created: %s", backupName)

	snippetName := services.GetSnippetName(domain)
	var migratedCount, deletedCount int
	var errors []string

	// 2. Migrate site configs
	if migrateSites {
		info, _ := h.wildcardService.GetMigrationInfo(domain, h.config.SitesDir, h.config.DataDir)
		for _, siteFile := range info.MatchingSites {
			sitePath := filepath.Join(h.config.SitesDir, siteFile)
			if err := h.wildcardService.MigrateSiteConfig(sitePath, snippetName); err != nil {
				log.Printf("Error migrating site %s: %v", siteFile, err)
				errors = append(errors, "Site "+siteFile+": "+err.Error())
			} else {
				migratedCount++
			}
		}
	}

	// 3. Delete old certificates
	if deleteCerts {
		info, _ := h.wildcardService.GetMigrationInfo(domain, h.config.SitesDir, h.config.DataDir)
		for _, certDomain := range info.Certificates {
			if err := h.certService.DeleteCertificate(certDomain); err != nil {
				log.Printf("Error deleting cert %s: %v", certDomain, err)
				errors = append(errors, "Cert "+certDomain+": "+err.Error())
			} else {
				deletedCount++
			}
		}
	}

	// 4. Reload Caddy
	result := h.caddyService.Reload()
	if !result.Success {
		errors = append(errors, "Caddy reload: "+result.Error)
	}

	// Build result message
	if len(errors) > 0 {
		setFlash(c, "warning", "Migration completed with errors. Check logs.")
	} else {
		msg := "Migration completed successfully!"
		if migratedCount > 0 {
			msg += fmt.Sprintf(" %d sites migrated.", migratedCount)
		}
		if deletedCount > 0 {
			msg += fmt.Sprintf(" %d certificates deleted.", deletedCount)
		}
		setFlash(c, "success", msg)
	}

	return c.Redirect("/settings/wildcard")
}

// WildcardDelete removes a wildcard domain
func (h *Handler) WildcardDelete(c *fiber.Ctx) error {
	if h.wildcardService == nil {
		setFlash(c, "error", "Wildcard service not available")
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/settings/wildcard")
			return c.SendStatus(fiber.StatusOK)
		}
		return c.Redirect("/settings/wildcard")
	}

	domain := c.Params("domain")
	log.Printf("WildcardDelete: domain=%s", domain)

	if err := h.wildcardService.DeleteDomain(domain); err != nil {
		log.Printf("Error deleting wildcard domain: %v", err)
		setFlash(c, "error", "Failed to delete wildcard domain: "+err.Error())
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/settings/wildcard")
			return c.SendStatus(fiber.StatusOK)
		}
		return c.Redirect("/settings/wildcard")
	}

	// Regenerate wildcard Caddy config
	if err := h.regenerateWildcardConfig(); err != nil {
		log.Printf("Error regenerating wildcard config after delete: %v", err)
		setFlash(c, "warning", "Domain removed but failed to update Caddy config")
	} else {
		// Reload Caddy
		result := h.caddyService.Reload()
		if !result.Success {
			setFlash(c, "warning", "Domain removed but Caddy reload failed")
		} else {
			setFlash(c, "success", "Wildcard domain removed successfully")
		}
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/settings/wildcard")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/settings/wildcard")
}

// regenerateWildcardConfig generates and saves the wildcard Caddy configuration
func (h *Handler) regenerateWildcardConfig() error {
	if h.wildcardService == nil {
		return nil
	}
	
	config, err := h.wildcardService.GenerateCaddyConfig()
	if err != nil {
		return err
	}

	return h.caddyService.SaveWildcardConfig(config)
}
