package handlers

import (
	"net/url"

	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

// DiscoveryPage renders the Docker Auto-Discovery page.
func (h *Handler) DiscoveryPage(c *fiber.Ctx) error {
	flashType, flashMsg := getFlash(c)

	data := h.baseData(c, "Docker Discovery")
	data["Active"] = "discovery"
	data["FlashType"] = flashType
	data["FlashMessage"] = flashMsg

	appSettings, _ := h.settingsService.Get()
	hosts := appSettings.DiscoveryHosts
	data["Hosts"] = hosts

	if len(hosts) == 0 {
		return c.Render("pages/discovery", data, "layouts/base")
	}

	results, err := services.DiscoverAllHosts(h.dockerService, h.caddyService, hosts)
	if err != nil {
		data["DiscoveryError"] = err.Error()
		return c.Render("pages/discovery", data, "layouts/base")
	}

	data["Results"] = results
	return c.Render("pages/discovery", data, "layouts/base")
}

// DiscoveryCreate redirects to the new-site form pre-filled with discovery data.
// The host IP is taken from the form (submitted as a hidden field from the discovery result).
func (h *Handler) DiscoveryCreate(c *fiber.Ctx) error {
	containerName := c.FormValue("container_name")
	port := c.FormValue("port")
	ip := c.FormValue("ip")

	suggestedName := services.SuggestContainerName(containerName)

	return c.Redirect("/sites/new?" + url.Values{
		"ip":   {ip},
		"port": {port},
		"name": {suggestedName},
	}.Encode())
}
