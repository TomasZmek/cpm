package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// DashboardData contains data for the dashboard page
type DashboardData struct {
	Stats         map[string]interface{}
	CertStats     map[string]int
	RecentChanges interface{}
	Alerts        []Alert
	CaddyStatus   string
}

// Alert represents a dashboard alert
type Alert struct {
	Type    string // success, warning, error, info
	Icon    string
	Title   string
	Message string
}

// Dashboard renders the dashboard page
func (h *Handler) Dashboard(c *fiber.Ctx) error {
	// Get stats
	stats := h.caddyService.GetStats()
	certStats := h.certService.GetStats()

	// Get recent changes
	recentChanges, _ := h.caddyService.GetRecentChanges(5)

	// Build alerts
	var alerts []Alert

	// Check for expiring certificates
	expiringCerts, _ := h.certService.GetExpiringCertificates(30)
	for _, cert := range expiringCerts {
		alertType := "warning"
		icon := "⚠️"
		title := "Certificate Expiring"

		if cert.DaysLeft <= 0 {
			alertType = "error"
			icon = "❌"
			title = "Certificate Expired"
		} else if cert.DaysLeft <= 7 {
			alertType = "error"
			icon = "🔴"
			title = "Certificate Critical"
		}

		alerts = append(alerts, Alert{
			Type:    alertType,
			Icon:    icon,
			Title:   title,
			Message: cert.Domain + " - " + formatDaysLeft(cert.DaysLeft),
		})
	}

	// Check Caddy status
	caddyStatus := h.dockerService.GetContainerStatus()
	if caddyStatus != "running" {
		alerts = append(alerts, Alert{
			Type:    "error",
			Icon:    "🔴",
			Title:   "Caddy Not Running",
			Message: "Container status: " + caddyStatus,
		})
	}

	// Get available snippets count
	availableSnippets, _ := h.snippetsService.GetAvailableSnippets()
	stats["snippets"] = len(availableSnippets)
	stats["caddy_status"] = caddyStatus

	data := h.baseData(c, "Dashboard")
	data["Stats"] = stats
	data["CertStats"] = certStats
	data["RecentChanges"] = recentChanges
	data["Alerts"] = alerts
	data["CaddyStatus"] = caddyStatus
	data["Active"] = "dashboard"

	return c.Render("pages/dashboard", data, "layouts/base")
}

func formatDaysLeft(days int) string {
	if days < 0 {
		return "expired"
	}
	if days == 0 {
		return "today"
	}
	if days == 1 {
		return "1 day left"
	}
	return string(rune(days)) + " days left"
}
