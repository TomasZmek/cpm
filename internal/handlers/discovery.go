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
	discoveryIP := appSettings.DiscoveryIP
	data["DiscoveryIP"] = discoveryIP

	if discoveryIP == "" {
		return c.Render("pages/discovery", data, "layouts/base")
	}

	results, err := services.DiscoverContainers(h.dockerService, h.caddyService, discoveryIP)
	if err != nil {
		data["DiscoveryError"] = err.Error()
		return c.Render("pages/discovery", data, "layouts/base")
	}

	data["Results"] = results
	return c.Render("pages/discovery", data, "layouts/base")
}

// DiscoveryCreate redirects to the new-site form pre-filled with discovery data.
func (h *Handler) DiscoveryCreate(c *fiber.Ctx) error {
	containerName := c.FormValue("container_name")
	port := c.FormValue("port")

	appSettings, _ := h.settingsService.Get()
	discoveryIP := appSettings.DiscoveryIP

	suggestedName := services.SuggestContainerName(containerName)

	return c.Redirect("/sites/new?" + url.Values{
		"ip":   {discoveryIP},
		"port": {port},
		"name": {suggestedName},
	}.Encode())
}
