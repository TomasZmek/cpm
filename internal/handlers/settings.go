package handlers

import (
	"io"
	"strings"

	"github.com/TomasZmek/cpm/internal/i18n"
	"github.com/TomasZmek/cpm/internal/middleware"
	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

// SettingsDocker renders the Docker Auto-Discovery settings tab.
func (h *Handler) SettingsDocker(c *fiber.Ctx) error {
	return h.renderSettingsTab(c, "docker")
}

// SettingsDiscoveryDetect detects the local machine's IP and returns it as JSON.
func (h *Handler) SettingsDiscoveryDetect(c *fiber.Ctx) error {
	ip, err := services.DetectLocalIP()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ip": ip})
}

// SettingsDiscoveryHostsSave saves the list of Docker discovery hosts.
func (h *Handler) SettingsDiscoveryHostsSave(c *fiber.Ctx) error {
	// Collect parallel arrays ip[] and label[] from the form.
	var ips, labels []string
	c.Context().PostArgs().VisitAll(func(key, value []byte) {
		switch string(key) {
		case "ip[]":
			ips = append(ips, strings.TrimSpace(string(value)))
		case "label[]":
			labels = append(labels, strings.TrimSpace(string(value)))
		}
	})

	// The hidden field local_docker_ip identifies which host is the local Docker host.
	localDockerIP := strings.TrimSpace(c.FormValue("local_docker_ip"))

	// Build hosts list, skipping entries with empty IP.
	var hosts []models.DiscoveryHost
	for i, ip := range ips {
		if ip == "" {
			continue
		}
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		hosts = append(hosts, models.DiscoveryHost{
			IP:            ip,
			Label:         label,
			IsLocalDocker: ip == localDockerIP,
		})
	}

	settings, err := h.settingsService.Get()
	if err != nil {
		setFlash(c, "error", tl(c, "msg_settings_load_failed")+": "+err.Error())
		return c.Redirect("/settings/docker")
	}

	settings.DiscoveryHosts = hosts

	if err := h.settingsService.Save(settings); err != nil {
		setFlash(c, "error", tl(c, "msg_settings_save_failed")+": "+err.Error())
		return c.Redirect("/settings/docker")
	}

	setFlash(c, "success", tl(c, "msg_discovery_hosts_saved"))
	return c.Redirect("/settings/docker")
}

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
	// Users and backup tabs expose sensitive data (user list, configuration
	// with API tokens) and are reachable via ?tab=, so check here as well.
	if (tab == "users" || tab == "backup" || tab == "docker") && !h.hasPermission(c, "admin") {
		return fiber.NewError(fiber.StatusForbidden, "You do not have permission to view this page")
	}

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
		data["Languages"] = i18n.SelectableLanguages()
		data["Themes"] = middleware.ThemeList()

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

	case "docker":
		appSettings, _ := h.settingsService.Get()
		data["DiscoveryHosts"] = appSettings.DiscoveryHosts
		for _, dh := range appSettings.DiscoveryHosts {
			if dh.IsLocalDocker {
				data["LocalDockerIP"] = dh.IP
				break
			}
		}
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

	data, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read file")
	}

	result := h.backupService.RestoreBackup(data)

	if !result.Success {
		setFlash(c, "error", result.Message)
	} else {
		// Reload Caddy
		reloadResult := h.caddyService.Reload()
		if reloadResult.Success {
			setFlash(c, "success", tl(c, "msg_backup_reloaded"))
		} else {
			setFlash(c, "warning", tl(c, "msg_backup_restore_reload_failed")+": "+reloadResult.Error)
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

	data, err := io.ReadAll(f)
	if err != nil {
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

	if skipped > 0 {
		setFlash(c, "success", tl(c, "msg_import_result_with_skip", imported, skipped))
	} else {
		setFlash(c, "success", tl(c, "msg_import_result", imported))
	}

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
