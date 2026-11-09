package handlers

import (
	"net/url"
	"strings"

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

	// Containers come from the inventory (enabled module sources). Each
	// container belongs to the Docker host its source reports, so hosts are
	// never mixed up.
	hasSource := false
	if h.modules != nil {
		hasSource = len(h.modules.Active()) > 0
	}
	data["HasLocalHost"] = hasSource
	if !hasSource || h.inventory == nil {
		return c.Render("pages/discovery", data, "layouts/base")
	}

	var errs []string
	for _, st := range h.inventory.Sources() {
		if st.Err != "" {
			errs = append(errs, st.Name.In(getLang(c))+": "+st.Err)
		}
	}
	if len(errs) > 0 {
		data["DiscoveryError"] = strings.Join(errs, "; ")
	}

	sites, _ := h.caddyService.GetAllSites()
	data["Results"] = services.Discover(h.inventory.Containers(), sites, appSettings.DiscoveryHosts, h.dockerService.IsSelfOrCaddy)
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
