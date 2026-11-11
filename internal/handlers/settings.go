package handlers

import (
	"io"
	"strings"

	"github.com/TomasZmek/cpm/internal/i18n"
	"github.com/TomasZmek/cpm/internal/middleware"
	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v3"
)

// SettingsDocker renders the Docker Auto-Discovery settings tab.
func (h *Handler) SettingsDocker(c fiber.Ctx) error {
	return h.renderSettingsTab(c, "docker")
}

// SettingsDiscoveryDetect suggests this Docker host's LAN IP as JSON. Inside a
// container on a bridge network the host's address is not visible, so the
// address the browser used to reach CPM is the fallback.
func (h *Handler) SettingsDiscoveryDetect(c fiber.Ctx) error {
	if ip, err := services.DetectLocalIP(); err == nil {
		return c.JSON(fiber.Map{"ip": ip})
	}
	if ip := requestHostIP(c); ip != "" {
		return c.JSON(fiber.Map{"ip": ip, "source": "browser"})
	}
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": tl(c, "msg_host_ip_not_detected")})
}

// SettingsDiscoveryHostsSave saves the list of Docker discovery hosts.
func (h *Handler) SettingsDiscoveryHostsSave(c fiber.Ctx) error {
	// Collect parallel arrays ip[] and label[] from the form.
	var ips, labels []string
	c.RequestCtx().PostArgs().VisitAll(func(key, value []byte) {
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
		return redirectTo(c, "/settings/docker")
	}

	settings.DiscoveryHosts = hosts

	if err := h.settingsService.Save(settings); err != nil {
		setFlash(c, "error", tl(c, "msg_settings_save_failed")+": "+err.Error())
		return redirectTo(c, "/settings/docker")
	}

	setFlash(c, "success", tl(c, "msg_discovery_hosts_saved"))
	return redirectTo(c, "/settings/docker")
}

// SettingsPage renders the settings page
func (h *Handler) SettingsPage(c fiber.Ctx) error {
	// Without ?tab= this is the settings index: wide screens show the
	// General tab, phones show the section list (list–detail navigation).
	tab := c.Query("tab")
	if tab == "" {
		return h.renderSettings(c, "general", true)
	}
	return h.renderSettingsTab(c, tab)
}

// SettingsSection is one entry of the settings navigation (tabs on wide
// screens, section list on phones).
type SettingsSection struct {
	Key   string
	URL   string
	Icon  string
	Title string // i18n key
	Desc  string // i18n key
}

var settingsSections = []struct {
	SettingsSection
	Permission string
}{
	{SettingsSection{"general", "/settings/general", "⚙️", "settings_general", "settings_desc_general"}, "view"},
	{SettingsSection{"backup", "/settings/backup", "💾", "settings_backup", "settings_desc_backup"}, "admin"},
	{SettingsSection{"caddy", "/settings/caddy", "🔧", "settings_caddy", "settings_desc_caddy"}, "view"},
	{SettingsSection{"wildcard", "/settings/wildcard", "🔐", "settings_wildcard", "settings_desc_wildcard"}, "admin"},
	{SettingsSection{"logs", "/settings/logs", "📝", "settings_logs", "settings_desc_logs"}, "admin"},
	{SettingsSection{"docker", "/settings/docker", "🐳", "settings_docker", "settings_desc_docker"}, "admin"},
	{SettingsSection{"users", "/settings/users", "👥", "settings_users", "settings_desc_users"}, "admin"},
	{SettingsSection{"modules", "/settings/modules", "🧩", "settings_modules", "settings_desc_modules"}, "admin"},
}

// visibleSettingsSections returns the sections the current user may open.
func (h *Handler) visibleSettingsSections(c fiber.Ctx) []SettingsSection {
	var out []SettingsSection
	for _, s := range settingsSections {
		if s.Key == "docker" && !h.hasContainerSource() {
			continue
		}
		if h.hasPermission(c, s.Permission) {
			out = append(out, s.SettingsSection)
		}
	}
	return out
}

// SettingsGeneral renders the general settings tab
func (h *Handler) SettingsGeneral(c fiber.Ctx) error {
	return h.renderSettingsTab(c, "general")
}

// SettingsBackup renders the backup settings tab
func (h *Handler) SettingsBackup(c fiber.Ctx) error {
	return h.renderSettingsTab(c, "backup")
}

// SettingsCaddy renders the Caddy settings tab
func (h *Handler) SettingsCaddy(c fiber.Ctx) error {
	return h.renderSettingsTab(c, "caddy")
}

// SettingsUsers renders the users settings tab
func (h *Handler) SettingsUsers(c fiber.Ctx) error {
	return h.renderSettingsTab(c, "users")
}

func (h *Handler) renderSettingsTab(c fiber.Ctx, tab string) error {
	return h.renderSettings(c, tab, false)
}

func (h *Handler) renderSettings(c fiber.Ctx, tab string, index bool) error {
	// Users and backup tabs expose sensitive data (user list, configuration
	// with API tokens) and are reachable via ?tab=, so check here as well.
	if (tab == "users" || tab == "backup" || tab == "docker" || tab == "modules" || tab == "logs") && !h.hasPermission(c, "admin") {
		return fiber.NewError(fiber.StatusForbidden, "You do not have permission to view this page")
	}

	// Docker discovery hosts belong to the container-source modules.
	if tab == "docker" && !h.hasContainerSource() {
		return redirectTo(c, "/settings/modules")
	}

	flashType, flashMsg := getFlash(c)

	data := h.baseData(c, "Settings")
	data["ActiveTab"] = tab
	data["FlashType"] = flashType
	data["FlashMessage"] = flashMsg
	data["Config"] = h.config
	data["Active"] = "settings"
	data["Sections"] = h.visibleSettingsSections(c)
	data["IsSettingsIndex"] = index

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

	case "modules":
		data["Modules"] = h.moduleViews(c)

	case "logs":
		appSettings, _ := h.settingsService.Get()
		data["AccessLog"] = appSettings.AccessLog.Normalized()
		data["AccessLogPath"] = services.AccessLogCaddyDir + "/" + services.AccessLogFileName
		data["MaxRollSizeMB"] = models.MaxAccessLogRollSizeMB
		data["MaxRollKeep"] = models.MaxAccessLogRollKeep
	}

	return c.Render("pages/settings", data, "layouts/base")
}

// BackupCreate creates a backup
func (h *Handler) BackupCreate(c fiber.Ctx) error {
	data, filename, err := h.backupService.CreateBackup()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set("Content-Disposition", "attachment; filename="+filename)
	c.Set("Content-Type", "application/zip")
	return c.Send(data)
}

// BackupRestore restores from a backup
func (h *Handler) BackupRestore(c fiber.Ctx) error {
	data, msg := readUploadedFile(c, "backup")
	if msg != "" {
		return redirectFlash(c, "error", msg, "/settings?tab=backup")
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

	return redirectTo(c, "/settings?tab=backup")
}

// ImportRules imports rules from JSON
func (h *Handler) ImportRules(c fiber.Ctx) error {
	data, msg := readUploadedFile(c, "rules")
	if msg != "" {
		return redirectFlash(c, "error", msg, "/settings?tab=backup")
	}

	skipExisting := c.FormValue("skip_existing") == "on"

	imported, skipped, err := h.backupService.ImportRules(data, h.caddyService, skipExisting)
	if err != nil {
		return redirectFlash(c, "error", tl(c, "msg_rules_import_failed")+": "+err.Error(), "/settings?tab=backup")
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

	return redirectTo(c, "/settings?tab=backup")
}

// readUploadedFile reads an uploaded form file. On failure it returns a
// translated message for a flash instead of the data.
func readUploadedFile(c fiber.Ctx, field string) ([]byte, string) {
	file, err := c.FormFile(field)
	if err != nil {
		return nil, tl(c, "msg_no_file_selected")
	}
	f, err := file.Open()
	if err != nil {
		return nil, tl(c, "msg_file_read_failed")
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, tl(c, "msg_file_read_failed")
	}
	return data, ""
}

// ExportRules exports rules as JSON
func (h *Handler) ExportRules(c fiber.Ctx) error {
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
