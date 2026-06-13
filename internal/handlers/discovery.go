package handlers

import (
	"net/url"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

// DiscoveryPage renders the Docker Auto-Discovery page.
// It lists running containers on the host marked as IsLocalDocker.
func (h *Handler) DiscoveryPage(c *fiber.Ctx) error {
	flashType, flashMsg := getFlash(c)

	data := h.baseData(c, "Docker Discovery")
	data["Active"] = "discovery"
	data["FlashType"] = flashType
	data["FlashMessage"] = flashMsg

	appSettings, _ := h.settingsService.Get()

	// Find the host marked as the local Docker host.
	var localHost *models.DiscoveryHost
	for i := range appSettings.DiscoveryHosts {
		if appSettings.DiscoveryHosts[i].IsLocalDocker {
			localHost = &appSettings.DiscoveryHosts[i]
			break
		}
	}

	data["HasLocalHost"] = localHost != nil

	if localHost == nil {
		return c.Render("pages/discovery", data, "layouts/base")
	}

	results, err := services.DiscoverContainers(h.dockerService, h.caddyService, *localHost)
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
