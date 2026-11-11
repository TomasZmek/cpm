package handlers

import (
	"errors"
	"sort"
	"strings"

	"github.com/TomasZmek/cpm/internal/inventory"
	"github.com/TomasZmek/cpm/sdk"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/TomasZmek/cpm/internal/utils"
	"github.com/gofiber/fiber/v3"
)

// SitesList renders the sites list page
func (h *Handler) SitesList(c fiber.Ctx) error {
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

	data := h.baseData(c, "Proxy Rules")
	data["Sites"] = filteredSites
	data["TotalSites"] = len(sites)
	data["AllTags"] = allTags
	data["SelectedTag"] = tag
	data["Search"] = search
	data["AvailableSnippets"] = availableSnippets
	data["FlashType"] = flashType
	data["FlashMessage"] = flashMsg
	data["Active"] = "sites"

	return c.Render("pages/sites", data, "layouts/base")
}

// SiteNew renders the new site form
func (h *Handler) SiteNew(c fiber.Ctx) error {
	site := &models.Site{TLSMode: "auto"}
	if qIP := c.Query("ip"); qIP != "" {
		site.TargetIP = qIP
	}
	if qPort := c.Query("port"); qPort != "" {
		site.TargetPort = qPort
	}
	if qName := c.Query("name"); qName != "" {
		site.Domains = []string{qName}
	}
	return h.renderSiteForm(c, site, true, "")
}

// renderSiteForm renders the rule form for a new (isNew) or existing rule.
// With formError it is shown again after a failed submit: the submitted
// values stay in the form and the error is shown as a flash message.
func (h *Handler) renderSiteForm(c fiber.Ctx, site *models.Site, isNew bool, formError string) error {
	availableSnippets, _ := h.snippetsService.GetAvailableSnippets()

	// Get wildcard domains for TLS selection
	var wildcardDomains []models.WildcardDomain
	if h.wildcardService != nil {
		wildcardDomains, _ = h.wildcardService.GetDomains()
	}

	appSettings, _ := h.settingsService.Get()

	title := "New Proxy Rule"
	if !isNew {
		title = "Edit: " + site.PrimaryDomain()
	}
	data := h.baseData(c, title)
	data["IsNew"] = isNew
	data["Site"] = site
	data["DefaultIP"] = h.config.DefaultIP
	data["AvailableSnippets"] = availableSnippets
	data["WildcardDomains"] = wildcardDomains
	data["Containers"] = h.discoverableContainers()
	data["DiscoveryHosts"] = appSettings.DiscoveryHosts
	data["Active"] = "sites"
	if isNew {
		data["Templates"] = models.GetServiceTemplates()
		data["Categories"] = models.GetTemplateCategories()
	}
	if formError != "" {
		data["FlashType"] = "error"
		data["FlashMessage"] = formError
		c.Status(fiber.StatusBadRequest)
	}

	return c.Render("pages/site_form", data, "layouts/base")
}

// siteFormError returns a translated message for an error of CreateSite /
// UpdateSite.
func siteFormError(c fiber.Ctx, site *models.Site, err error) string {
	if errors.Is(err, services.ErrSiteExists) {
		return tl(c, "msg_rule_exists", site.PrimaryDomain())
	}
	return tl(c, "msg_rule_save_failed") + ": " + err.Error()
}

// SiteCreate creates a new site
func (h *Handler) SiteCreate(c fiber.Ctx) error {
	site := &models.Site{}

	// Parse form
	site.Domains = services.CleanDomains(c.FormValue("domains"))
	site.TargetIP = c.FormValue("target_ip")
	site.TargetPort = c.FormValue("target_port")
	site.IsHTTPSBackend = c.FormValue("is_https_backend") == "on"
	site.EnableWebSocket = c.FormValue("enable_websocket") == "on"
	site.HealthCheckPath = c.FormValue("health_check_path")
	site.ExtraConfig = c.FormValue("extra_config")
	site.TLSMode = c.FormValue("tls_mode")
	if site.TLSMode == "" {
		site.TLSMode = "auto"
	}

	// Parse snippets
	site.Snippets = formList(c, "snippets")

	// Derive IsInternal from snippets (internal_only snippet = internal site)
	site.IsInternal = utils.Contains(site.Snippets, "internal_only")

	// Parse tags
	site.Tags = splitList(c.FormValue("tags"))
	readLogSkipForm(c, site)

	// Validation
	if len(site.Domains) == 0 {
		return h.renderSiteForm(c, site, true, tl(c, "msg_rule_domain_required"))
	}
	if site.TargetPort == "" {
		return h.renderSiteForm(c, site, true, tl(c, "msg_rule_port_required"))
	}

	// Create site
	if err := h.caddyService.CreateSite(site); err != nil {
		// CreateSite may have set a filename derived from the domain.
		site.Filename, site.Filepath = "", ""
		return h.renderSiteForm(c, site, true, siteFormError(c, site, err))
	}

	// Reload Caddy
	result := h.caddyService.ReloadWithValidation()
	if !result.Success {
		setFlash(c, "warning", tl(c, "msg_rule_create_reload_failed")+": "+result.Error)
	} else {
		setFlash(c, "success", tl(c, "msg_rule_created_name", site.PrimaryDomain()))
	}

	// HTMX redirect
	return redirectTo(c, "/sites")
}

// SiteDetail shows a single site
func (h *Handler) SiteDetail(c fiber.Ctx) error {
	filename := c.Params("id")

	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Site not found")
	}

	data := h.baseData(c, site.PrimaryDomain())
	data["Site"] = site
	data["Active"] = "sites"

	return c.Render("pages/site_detail", data, "layouts/base")
}

// SiteEdit renders the edit form
func (h *Handler) SiteEdit(c fiber.Ctx) error {
	site, err := h.caddyService.GetSite(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Site not found")
	}
	return h.renderSiteForm(c, site, false, "")
}

// SiteUpdate updates an existing site
func (h *Handler) SiteUpdate(c fiber.Ctx) error {
	filename := c.Params("id")

	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Site not found")
	}

	// Check for raw mode
	if rawContent := c.FormValue("raw_content"); rawContent != "" {
		if err := h.caddyService.UpdateSiteRaw(filename, rawContent); err != nil {
			site.RawContent = rawContent
			return h.renderSiteForm(c, site, false, siteFormError(c, site, err))
		}
	} else {
		// Update from form
		site.Domains = services.CleanDomains(c.FormValue("domains"))
		site.TargetIP = c.FormValue("target_ip")
		site.TargetPort = c.FormValue("target_port")
		site.IsHTTPSBackend = c.FormValue("is_https_backend") == "on"
		site.EnableWebSocket = c.FormValue("enable_websocket") == "on"
		site.HealthCheckPath = c.FormValue("health_check_path")
		site.ExtraConfig = c.FormValue("extra_config")
		site.TLSMode = c.FormValue("tls_mode")
		if site.TLSMode == "" {
			site.TLSMode = "auto"
		}

		// Parse snippets
		site.Snippets = formList(c, "snippets")

		// Derive IsInternal from snippets
		site.IsInternal = utils.Contains(site.Snippets, "internal_only")

		site.Tags = splitList(c.FormValue("tags"))
		readLogSkipForm(c, site)

		if len(site.Domains) == 0 {
			return h.renderSiteForm(c, site, false, tl(c, "msg_rule_domain_required"))
		}
		if err := h.caddyService.UpdateSite(site); err != nil {
			return h.renderSiteForm(c, site, false, siteFormError(c, site, err))
		}
	}

	// Reload Caddy
	result := h.caddyService.ReloadWithValidation()
	if !result.Success {
		setFlash(c, "warning", tl(c, "msg_rule_update_reload_failed")+": "+result.Error)
	} else {
		setFlash(c, "success", tl(c, "msg_rule_updated_name", site.PrimaryDomain()))
	}

	return redirectTo(c, "/sites")
}

// SiteDelete deletes a site
func (h *Handler) SiteDelete(c fiber.Ctx) error {
	filename := c.Params("id")

	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		setFlash(c, "error", tl(c, "msg_rule_not_found"))
		return redirectTo(c, "/sites")
	}

	if err := h.caddyService.DeleteSite(filename); err != nil {
		setFlash(c, "error", tl(c, "msg_rule_delete_failed")+": "+err.Error())
		return redirectTo(c, "/sites")
	}

	// Reload Caddy
	result := h.caddyService.ReloadWithValidation()
	if !result.Success {
		setFlash(c, "warning", tl(c, "msg_rule_delete_reload_failed")+": "+result.Error)
	} else {
		setFlash(c, "success", tl(c, "msg_rule_deleted_name", site.PrimaryDomain()))
	}

	return redirectTo(c, "/sites")
}

// HTMXSitesList serves the live search and tag filter of the rules page.
func (h *Handler) HTMXSitesList(c fiber.Ctx) error {
	// The rules page already filters by ?search= and ?tag=; the search box
	// and tag filter pick its #sites-list with hx-select.
	return h.SitesList(c)
}

// HTMXSitePreview returns site config preview
func (h *Handler) HTMXSitePreview(c fiber.Ctx) error {
	filename := c.Params("id")
	site, err := h.caddyService.GetSite(filename)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Site not found")
	}

	return c.SendString(site.RawContent)
}

// formList collects all values of a (possibly repeated) form field, e.g.
// several checked checkboxes with the same name, also accepting a single
// comma-separated value.
func formList(c fiber.Ctx, key string) []string {
	var raw []string
	if form, err := c.MultipartForm(); err == nil && form != nil {
		raw = form.Value[key]
	} else {
		for _, v := range c.Request().PostArgs().PeekMulti(key) {
			raw = append(raw, string(v))
		}
	}

	items := []string{}
	for _, v := range raw {
		for _, item := range splitList(v) {
			if !utils.Contains(items, item) {
				items = append(items, item)
			}
		}
	}
	return items
}

// splitList splits a comma-separated form value into trimmed, non-empty items
// readLogSkipForm reads the access log exclusions of the rule form.
func readLogSkipForm(c fiber.Ctx, site *models.Site) {
	site.NoAccessLog = c.FormValue("no_access_log") == "on"
	site.LogSkipPaths = strings.Fields(strings.ReplaceAll(c.FormValue("log_skip_paths"), ",", " "))
}

func splitList(v string) []string {
	items := []string{}
	for _, item := range strings.Split(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

// SitesStatus returns each site's backend status as JSON: filename -> "up"|"down"|"unknown".
// "up"/"down" are based on whether a Docker container matching the target name is running.
// IP targets (or names without a matching container) are reported as "unknown".
func (h *Handler) SitesStatus(c fiber.Ctx) error {
	sites, _ := h.caddyService.GetAllSites()
	out := make(map[string]inventory.Status, len(sites))
	for _, s := range sites {
		out[s.Filename] = h.backendStatus(s)
	}
	return c.JSON(out)
}

// backendStatus returns a rule's backend status from the inventory.
func (h *Handler) backendStatus(s *models.Site) inventory.Status {
	if h.inventory == nil {
		return inventory.Status{State: inventory.Unknown}
	}
	return h.inventory.StatusFor(s)
}

// PickerContainer is a container offered by the rule form's picker.
type PickerContainer struct {
	Name   string
	HostIP string
	Host   string
	Ports  []sdk.Port
}

// discoverableContainers returns running containers from the inventory for
// the rule form's container picker (Caddy and CPM itself excluded).
func (h *Handler) discoverableContainers() []PickerContainer {
	if h.inventory == nil {
		return nil
	}
	labels := map[string]string{}
	if st, err := h.settingsService.Get(); err == nil {
		for _, dh := range st.DiscoveryHosts {
			labels[dh.IP] = dh.Label
		}
	}
	var out []PickerContainer
	for _, c := range h.inventory.Containers() {
		if !c.Running() || (h.dockerService != nil && h.dockerService.IsSelfOrCaddy(c)) {
			continue
		}
		host := labels[c.HostIP]
		if host == "" {
			host = c.Host
		}
		out = append(out, PickerContainer{Name: c.Name, HostIP: c.HostIP, Host: host, Ports: c.Ports})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].HostIP != out[j].HostIP {
			return out[i].HostIP < out[j].HostIP
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
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
