package handlers

import (
	"fmt"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/gofiber/fiber/v2"
)

// SettingsPage renders the settings page
func (h *Handler) SettingsPage(c *fiber.Ctx) error {
	tab := c.Query("tab", "general")
	return h.renderSettingsTab(c, tab)
}

// SettingsGeneral renders the general settings tab
func (h *Handler) SettingsGeneral(c *fiber.Ctx) error {
	return h.renderSettingsTab(c, "general")
}

// SettingsBackup renders the backup settings tab
func (h *Handler) SettingsBackup(c *fiber.Ctx) error {
	return h.renderSettingsTab(c, "backup")
}

// SettingsCaddy renders the Caddy settings tab
func (h *Handler) SettingsCaddy(c *fiber.Ctx) error {
	return h.renderSettingsTab(c, "caddy")
}

// SettingsUsers renders the users settings tab
func (h *Handler) SettingsUsers(c *fiber.Ctx) error {
	return h.renderSettingsTab(c, "users")
}

func (h *Handler) renderSettingsTab(c *fiber.Ctx, tab string) error {
	flashType, flashMsg := getFlash(c)

	data := h.baseData(c, "Settings")
	data["ActiveTab"] = tab
	data["FlashType"] = flashType
	data["FlashMessage"] = flashMsg
	data["Config"] = h.config
	data["Active"] = "settings"

	// Tab-specific data
	switch tab {
	case "general":
		// Language and theme settings
		data["Languages"] = []map[string]string{
			{"code": "en", "name": "English"},
			{"code": "cs", "name": "Čeština"},
		}
		data["Themes"] = []map[string]string{
			{"code": "classic", "name": "Classic"},
			{"code": "modern", "name": "Modern (Coming Soon)"},
		}

	case "backup":
		sites, _ := h.caddyService.GetAllSites()
		data["SitesCount"] = len(sites)

	case "caddy":
		fallback, _ := h.caddyService.GetFallback()
		data["Fallback"] = fallback
		data["FallbackExists"] = h.caddyService.FallbackExists()

		// Error pages
		page403, _ := h.caddyService.GetErrorPage(403)
		page404, _ := h.caddyService.GetErrorPage(404)
		data["ErrorPage403"] = page403
		data["ErrorPage404"] = page404

	case "users":
		data["Users"] = h.authService.GetUsers()
		data["AuthEnabled"] = h.authService.IsEnabled()
		data["Roles"] = models.AllRoles()
	}

	return c.Render("pages/settings", data, "layouts/base")
}

// BackupCreate creates a backup
func (h *Handler) BackupCreate(c *fiber.Ctx) error {
	data, filename, err := h.backupService.CreateBackup()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set("Content-Disposition", "attachment; filename="+filename)
	c.Set("Content-Type", "application/zip")
	return c.Send(data)
}

// BackupRestore restores from a backup
func (h *Handler) BackupRestore(c *fiber.Ctx) error {
	file, err := c.FormFile("backup")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("No file uploaded")
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open file")
	}
	defer f.Close()

	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read file")
	}

	result := h.backupService.RestoreBackup(data)

	if !result.Success {
		setFlash(c, "error", result.Message)
	} else {
		// Reload Caddy
		reloadResult := h.caddyService.Reload()
		if reloadResult.Success {
			setFlash(c, "success", "Backup restored and Caddy reloaded")
		} else {
			setFlash(c, "warning", "Backup restored but reload failed: "+reloadResult.Error)
		}
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/settings?tab=backup")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/settings?tab=backup")
}

// ImportRules imports rules from JSON
func (h *Handler) ImportRules(c *fiber.Ctx) error {
	file, err := c.FormFile("rules")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("No file uploaded")
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open file")
	}
	defer f.Close()

	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read file")
	}

	skipExisting := c.FormValue("skip_existing") == "on"

	imported, skipped, err := h.backupService.ImportRules(data, h.caddyService, skipExisting)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	// Reload Caddy
	if imported > 0 {
		h.caddyService.ReloadWithValidation()
	}

	setFlash(c, "success", formatImportResult(imported, skipped))

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/settings?tab=backup")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/settings?tab=backup")
}

// ExportRules exports rules as JSON
func (h *Handler) ExportRules(c *fiber.Ctx) error {
	sites, err := h.caddyService.GetAllSites()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	data, err := h.backupService.ExportRules(sites)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set("Content-Disposition", "attachment; filename=cpm_rules_export.json")
	c.Set("Content-Type", "application/json")
	return c.Send(data)
}

func formatImportResult(imported, skipped int) string {
	if skipped > 0 {
		return fmt.Sprintf("Imported %d rules, skipped %d existing", imported, skipped)
	}
	return fmt.Sprintf("Imported %d rules", imported)
}
