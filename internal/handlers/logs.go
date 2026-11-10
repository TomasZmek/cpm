package handlers

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"

	"github.com/gofiber/fiber/v3"
)

// LogsPage renders the logs viewer: the access log (requests, filterable by
// rule) or Caddy's own log (container output).
func (h *Handler) LogsPage(c fiber.Ctx) error {
	appSettings, _ := h.settingsService.Get()
	accessEnabled := appSettings.AccessLog.Enabled

	view := c.Query("view")
	if view != "access" && view != "caddy" {
		view = "caddy"
		if accessEnabled || c.Query("site") != "" {
			view = "access"
		}
	}

	data := h.baseData(c, "Caddy Logs")
	data["Active"] = "logs"
	data["View"] = view
	data["AccessLogEnabled"] = accessEnabled
	data["CanAdmin"] = h.hasPermission(c, "admin")

	if view == "access" {
		h.accessLogData(c, data)
		return c.Render("pages/logs", data, "layouts/base")
	}

	lines := fiber.Query[int](c, "lines", 100)
	if lines < 1 {
		lines = 100
	} else if lines > 5000 {
		lines = 5000
	}

	logs, err := h.dockerService.GetLogs(lines)
	if err != nil {
		logs = []string{"Error fetching logs: " + err.Error()}
	}

	data["Logs"] = logs
	data["Lines"] = lines
	data["LogCount"] = len(logs)

	return c.Render("pages/logs", data, "layouts/base")
}

// accessLogLimits are the selectable numbers of shown requests.
var accessLogLimits = []int{100, 200, 500, 1000}

// accessLogData fills the template data of the access log view.
func (h *Handler) accessLogData(c fiber.Ctx, data fiber.Map) {
	q := services.AccessLogQuery{
		Status: c.Query("status"),
		Text:   strings.TrimSpace(c.Query("q")),
		Limit:  fiber.Query[int](c, "limit", 200),
	}
	switch q.Status {
	case "", "2xx", "3xx", "4xx", "5xx", "errors":
	default:
		q.Status = ""
	}
	if q.Limit < 1 || q.Limit > 1000 {
		q.Limit = 200
	}

	sites, _ := h.caddyService.GetAllSites()
	sort.Slice(sites, func(i, j int) bool { return sites[i].PrimaryDomain() < sites[j].PrimaryDomain() })
	data["Sites"] = sites

	siteParam := c.Query("site")
	if siteParam != "" {
		if site, err := h.caddyService.GetSite(siteParam); err == nil {
			q.Hosts = site.Domains
			data["Site"] = site
		} else {
			data["SiteNotFound"] = true
		}
	}

	data["SiteParam"] = siteParam
	data["Status"] = q.Status
	data["Query"] = q.Text
	data["Limit"] = q.Limit
	data["Limits"] = accessLogLimits

	if h.accessLog == nil || data["SiteNotFound"] == true {
		data["AccessLog"] = &services.AccessLogResult{}
		return
	}
	res, err := h.accessLog.Query(q)
	if err != nil {
		data["AccessLogError"] = err.Error()
	}
	data["AccessLog"] = res
}

// SettingsLogs renders the access log settings tab.
func (h *Handler) SettingsLogs(c fiber.Ctx) error {
	return h.renderSettingsTab(c, "logs")
}

// AccessLogSave saves the access log settings, regenerates the Caddyfile and
// the rule files and reloads Caddy. When Caddy rejects the configuration,
// everything is restored.
func (h *Handler) AccessLogSave(c fiber.Ctx) error {
	appSettings, err := h.settingsService.Get()
	if err != nil {
		setFlash(c, "error", tl(c, "msg_settings_load_failed")+": "+err.Error())
		return redirectTo(c, "/settings/logs")
	}
	previous := *appSettings

	size, _ := strconv.Atoi(c.FormValue("roll_size_mb"))
	keep, _ := strconv.Atoi(c.FormValue("roll_keep"))
	appSettings.AccessLog = models.AccessLogSettings{
		Enabled:    c.FormValue("enabled") == "on",
		RollSizeMB: size,
		RollKeep:   keep,
		MaskIPs:    c.FormValue("mask_ips") == "on",
	}.Normalized()

	if err := h.settingsService.Save(appSettings); err != nil {
		setFlash(c, "error", tl(c, "msg_settings_save_failed")+": "+err.Error())
		return redirectTo(c, "/settings/logs")
	}

	restoreFiles, err := h.caddyService.SyncAccessLogImports()
	if err == nil {
		err = h.caddyService.RegenerateCaddyfile()
	}
	if err != nil {
		h.restoreAccessLog(&previous, restoreFiles)
		setFlash(c, "error", tl(c, "msg_settings_save_failed")+": "+err.Error())
		return redirectTo(c, "/settings/logs")
	}

	result := h.caddyService.ReloadWithValidation()
	switch {
	case result.Success:
		setFlash(c, "success", tl(c, "msg_access_log_saved"))
	case result.DockerUnreachable:
		setFlash(c, "warning", tl(c, "msg_access_log_saved_no_reload")+": "+result.Error)
	default:
		h.restoreAccessLog(&previous, restoreFiles)
		setFlash(c, "error", tl(c, "msg_access_log_reverted")+": "+result.Error)
	}
	return redirectTo(c, "/settings/logs")
}

// restoreAccessLog puts back the previous settings, rule files and Caddyfile.
func (h *Handler) restoreAccessLog(previous *models.AppSettings, restoreFiles func()) {
	_ = h.settingsService.Save(previous)
	if restoreFiles != nil {
		restoreFiles()
	}
	_ = h.caddyService.RegenerateCaddyfile()
}

// HTMXLogsStream streams logs via Server-Sent Events
func (h *Handler) HTMXLogsStream(c fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	return c.SendStreamWriter(func(w *bufio.Writer) {
		lastLines := make(map[string]bool)

		for i := 0; i < 60; i++ { // Stream for 60 seconds max
			logs, err := h.dockerService.GetLogs(50)
			if err != nil {
				fmt.Fprintf(w, "data: Error: %s\n\n", err.Error())
				if w.Flush() != nil {
					return // client disconnected
				}
				time.Sleep(5 * time.Second)
				continue
			}

			// Send only new lines
			for _, line := range logs {
				line = strings.TrimSpace(line)
				if line == "" || lastLines[line] {
					continue
				}
				lastLines[line] = true

				// Escape for SSE
				escaped := strings.ReplaceAll(line, "\n", "\\n")
				fmt.Fprintf(w, "data: %s\n\n", escaped)
			}

			if w.Flush() != nil {
				return // client disconnected
			}
			time.Sleep(2 * time.Second)
		}
	})
}
