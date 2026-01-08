package handlers

import (
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

// SitesList renders the sites list page
func (h *Handler) SitesList(c *fiber.Ctx) error {
	sites, err := h.caddyService.GetAllSites()
	if err != nil {
		return err
	}

	// Get filter parameters
	search := c.Query("search")
	tag := c.Query("tag")

	// Filter sites
	filteredSites := filterSites(sites, search, tag)

	// Get all tags for filter dropdown
	allTags, _ := h.caddyService.GetAllTags()

	// Get available snippets
	availableSnippets, _ := h.snippetsService.GetAvailableSnippets()

	flashType, flashMsg := getFlash(c)

	return c.Render("pages/sites", fiber.Map{
		"Title":             "Proxy Rules",
		"Sites":             filteredSites,
		"TotalSites":        len(sites),
		"AllTags":           allTags,
		"SelectedTag":       tag,
		"Search":            search,
		"AvailableSnippets": availableSnippets,
		"FlashType":         flashType,
		"FlashMessage":      flashMsg,
	}, "layouts/base")
}

// SiteNew renders the new site form
func (h *Handler) SiteNew(c *fiber.Ctx) error {
	availableSnippets, _ := h.snippetsService.GetAvailableSnippets()
	templates := models.GetServiceTemplates()
	categories := models.GetTemplateCategories()

	return c.Render("pages/site_form", fiber.Map{
		"Title":             "New Proxy Rule",
		"IsNew":             true,
		"Site":              &models.Site{},
		"DefaultIP":         h.config.DefaultIP,
		"AvailableSnippets": availableSnippets,
		"Templates":         templates,
		"Categories":        categories,
	}, "layouts/base")
}

// SiteCreate creates a new site
func (h *Handler) SiteCreate(c *fiber.Ctx) error {
	site := &models.Site{}

	// Parse form
	site.Domains = services.CleanDomains(c.FormValue("domains"))
	site.TargetIP = c.FormValue("target_ip")
	site.TargetPort = c.FormValue("target_port")
	site.IsHTTPSBackend = c.FormValue("is_https_backend") == "on"
	site.IsInternal = c.FormValue("is_internal") == "on"
	site.EnableWebSocket = c.FormValue("enable_websocket") == "on"
	site.HealthCheckPath = c.FormValue("health_check_path")
	site.ExtraConfig = c.FormValue("extra_config")

	// Parse snippets
	if snippets := c.FormValue("snippets"); snippets != "" {
		site.Snippets = strings.Split(snippets, ",")
	}

	// Parse tags
	if tags := c.FormValue("tags"); tags != "" {
		site.Tags = strings.Split(tags, ",")
	}

	// Validation
	if len(site.Domains) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("At least one domain is required")
	}
	if site.TargetPort == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Port is required")
	}

	// Create site
	if err := h.caddyService.CreateSite(site); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Reload Caddy
	result := h.caddyService.ReloadWithValidation()
	if !result.Success {
		// Site was created but reload failed
		setFlash(c, "warning", "Rule created but reload failed: "+result.Error)
	} else {
		setFlash(c, "success", "Rule '"+site.PrimaryDomain()+"' created successfully")
	}

	// HTMX redirect
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/sites")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/sites")
}

// SiteDetail shows a single site
func (h *Handler) SiteDetail(c *fiber.Ctx) error {
	filename := c.Params("id")

	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Site not found")
	}

	return c.Render("pages/site_detail", fiber.Map{
		"Title": site.PrimaryDomain(),
		"Site":  site,
	}, "layouts/base")
}

// SiteEdit renders the edit form
func (h *Handler) SiteEdit(c *fiber.Ctx) error {
	filename := c.Params("id")

	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Site not found")
	}

	availableSnippets, _ := h.snippetsService.GetAvailableSnippets()

	return c.Render("pages/site_form", fiber.Map{
		"Title":             "Edit: " + site.PrimaryDomain(),
		"IsNew":             false,
		"Site":              site,
		"DefaultIP":         h.config.DefaultIP,
		"AvailableSnippets": availableSnippets,
	}, "layouts/base")
}

// SiteUpdate updates an existing site
func (h *Handler) SiteUpdate(c *fiber.Ctx) error {
	filename := c.Params("id")

	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Site not found")
	}

	// Check for raw mode
	if rawContent := c.FormValue("raw_content"); rawContent != "" {
		if err := h.caddyService.UpdateSiteRaw(filename, rawContent); err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
	} else {
		// Update from form
		site.Domains = services.CleanDomains(c.FormValue("domains"))
		site.TargetIP = c.FormValue("target_ip")
		site.TargetPort = c.FormValue("target_port")
		site.IsHTTPSBackend = c.FormValue("is_https_backend") == "on"
		site.IsInternal = c.FormValue("is_internal") == "on"
		site.EnableWebSocket = c.FormValue("enable_websocket") == "on"
		site.HealthCheckPath = c.FormValue("health_check_path")
		site.ExtraConfig = c.FormValue("extra_config")

		if snippets := c.FormValue("snippets"); snippets != "" {
			site.Snippets = strings.Split(snippets, ",")
		}
		if tags := c.FormValue("tags"); tags != "" {
			site.Tags = strings.Split(tags, ",")
		}

		if err := h.caddyService.UpdateSite(site); err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
	}

	// Reload Caddy
	result := h.caddyService.ReloadWithValidation()
	if !result.Success {
		setFlash(c, "warning", "Rule updated but reload failed: "+result.Error)
	} else {
		setFlash(c, "success", "Rule '"+site.PrimaryDomain()+"' updated successfully")
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/sites")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/sites")
}

// SiteDelete deletes a site
func (h *Handler) SiteDelete(c *fiber.Ctx) error {
	filename := c.Params("id")

	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Site not found")
	}

	if err := h.caddyService.DeleteSite(filename); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Reload Caddy
	result := h.caddyService.ReloadWithValidation()
	if !result.Success {
		setFlash(c, "warning", "Rule deleted but reload failed: "+result.Error)
	} else {
		setFlash(c, "success", "Rule '"+site.PrimaryDomain()+"' deleted successfully")
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/sites")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/sites")
}

// SiteDuplicate duplicates a site
func (h *Handler) SiteDuplicate(c *fiber.Ctx) error {
	filename := c.Params("id")
	newDomains := services.CleanDomains(c.FormValue("domains"))

	if len(newDomains) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("New domains are required")
	}

	newSite, err := h.caddyService.DuplicateSite(filename, newDomains)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Reload Caddy
	result := h.caddyService.ReloadWithValidation()
	if !result.Success {
		setFlash(c, "warning", "Rule duplicated but reload failed: "+result.Error)
	} else {
		setFlash(c, "success", "Rule '"+newSite.PrimaryDomain()+"' created successfully")
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/sites")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/sites")
}

// HTMXSitesList returns sites list as HTML partial
func (h *Handler) HTMXSitesList(c *fiber.Ctx) error {
	sites, _ := h.caddyService.GetAllSites()
	search := c.Query("search")
	tag := c.Query("tag")

	filteredSites := filterSites(sites, search, tag)

	return c.Render("partials/sites_list", fiber.Map{
		"Sites":      filteredSites,
		"TotalSites": len(sites),
	})
}

// HTMXSiteCard returns a single site card as HTML partial
func (h *Handler) HTMXSiteCard(c *fiber.Ctx) error {
	filename := c.Params("id")
	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Site not found")
	}

	return c.Render("partials/site_card", fiber.Map{
		"Site": site,
	})
}

// HTMXSitePreview returns site config preview
func (h *Handler) HTMXSitePreview(c *fiber.Ctx) error {
	filename := c.Params("id")
	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Site not found")
	}

	return c.SendString(site.RawContent)
}

// filterSites filters sites by search query and tag
func filterSites(sites []*models.Site, search, tag string) []*models.Site {
	if search == "" && tag == "" {
		return sites
	}

	var filtered []*models.Site
	search = strings.ToLower(search)

	for _, site := range sites {
		// Filter by tag
		if tag != "" {
			hasTag := false
			for _, t := range site.Tags {
				if t == tag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// Filter by search
		if search != "" {
			matches := false

			// Search in domains
			for _, d := range site.Domains {
				if strings.Contains(strings.ToLower(d), search) {
					matches = true
					break
				}
			}

			// Search in filename
			if !matches && strings.Contains(strings.ToLower(site.Filename), search) {
				matches = true
			}

			// Search in IP
			if !matches && strings.Contains(site.TargetIP, search) {
				matches = true
			}

			// Search in port
			if !matches && strings.Contains(site.TargetPort, search) {
				matches = true
			}

			if !matches {
				continue
			}
		}

		filtered = append(filtered, site)
	}

	return filtered
}
