package handlers

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

// SnippetsList renders the snippets management page
func (h *Handler) SnippetsList(c *fiber.Ctx) error {
	cfg, err := h.snippetsService.GetConfig()
	if err != nil {
		return err
	}

	knownSnippets := models.KnownSnippets()

	flashType, flashMsg := getFlash(c)

	data := h.baseData(c, "Snippets Manager")
	data["Config"] = cfg
	data["KnownSnippets"] = knownSnippets
	data["FlashType"] = flashType
	data["FlashMessage"] = flashMsg
	data["Active"] = "snippets"

	return c.Render("pages/snippets", data, "layouts/base")
}

// SnippetUpdate updates a specific snippet configuration
func (h *Handler) SnippetUpdate(c *fiber.Ctx) error {
	snippetName := c.Params("name")

	cfg, err := h.snippetsService.GetConfig()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	switch snippetName {
	case "cloudflare_dns":
		cfg.CloudflareDNS.Enabled = c.FormValue("enabled") == "on"
		cfg.CloudflareDNS.UseEnv = c.FormValue("use_env") == "on"
		cfg.CloudflareDNS.APIToken = strings.TrimSpace(c.FormValue("api_token"))
		if !cfg.CloudflareDNS.UseEnv && cfg.CloudflareDNS.APIToken != "" {
			if err := services.ValidateAPIToken(cfg.CloudflareDNS.APIToken); err != nil {
				return c.Status(fiber.StatusBadRequest).SendString(err.Error())
			}
		}

	case "internal_only":
		cfg.InternalOnly.Enabled = c.FormValue("enabled") == "on"
		networks := c.FormValue("allowed_networks")
		cfg.InternalOnly.AllowedNetworks = parseNetworks(networks)
		for _, n := range cfg.InternalOnly.AllowedNetworks {
			if !isValidNetwork(n) {
				return c.Status(fiber.StatusBadRequest).SendString("Invalid IP address or CIDR: " + n)
			}
		}

	case "security_headers":
		cfg.SecurityHeaders.Enabled = c.FormValue("enabled") == "on"
		cfg.SecurityHeaders.HSTSMaxAge = formInt(c, "hsts_max_age", 31536000)
		cfg.SecurityHeaders.HSTSIncludeSubdomains = c.FormValue("hsts_include_subdomains") == "on"
		cfg.SecurityHeaders.XContentTypeOptions = c.FormValue("x_content_type_options") == "on"
		cfg.SecurityHeaders.XFrameOptions = c.FormValue("x_frame_options")
		cfg.SecurityHeaders.ReferrerPolicy = c.FormValue("referrer_policy")
		if !isSafeHeaderValue(cfg.SecurityHeaders.XFrameOptions) || !isSafeHeaderValue(cfg.SecurityHeaders.ReferrerPolicy) {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid header value")
		}
		cfg.SecurityHeaders.HideServer = c.FormValue("hide_server") == "on"

	case "compression":
		cfg.Compression.Enabled = c.FormValue("enabled") == "on"
		cfg.Compression.Zstd = c.FormValue("zstd") == "on"
		cfg.Compression.Gzip = c.FormValue("gzip") == "on"

	case "rate_limit":
		cfg.RateLimit.Enabled = c.FormValue("enabled") == "on"
		cfg.RateLimit.Requests = formInt(c, "requests", 100)
		cfg.RateLimit.WindowSecs = formInt(c, "window_secs", 60)

	case "basic_auth":
		cfg.BasicAuth.Enabled = c.FormValue("enabled") == "on"
		// User management is done separately

	default:
		return c.Status(fiber.StatusBadRequest).SendString("Unknown snippet: " + snippetName)
	}

	if err := h.snippetsService.SaveConfig(cfg); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Reload Caddy
	result := h.caddyService.ReloadWithValidation()
	if !result.Success {
		setFlash(c, "warning", "Snippet updated but reload failed: "+result.Error)
	} else {
		setFlash(c, "success", "Snippet '"+snippetName+"' updated successfully")
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/snippets")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/snippets")
}

// HTMXSnippetForm returns a snippet form as HTML partial
func (h *Handler) HTMXSnippetForm(c *fiber.Ctx) error {
	snippetName := c.Params("name")
	if !snippetFormNameRegex.MatchString(snippetName) {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid snippet name")
	}

	cfg, err := h.snippetsService.GetConfig()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.Render("partials/snippet_form_"+snippetName, fiber.Map{
		"Config": cfg,
	})
}

var snippetFormNameRegex = regexp.MustCompile(`^[a-z_]+$`)

// parseNetworks parses networks from form input
func parseNetworks(input string) []string {
	var networks []string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			networks = append(networks, line)
		}
	}
	return networks
}

// isValidNetwork reports whether v is an IP address, a CIDR range or Caddy's
// "private_ranges" shortcut
func isValidNetwork(v string) bool {
	if v == "private_ranges" {
		return true
	}
	if _, err := netip.ParsePrefix(v); err == nil {
		return true
	}
	_, err := netip.ParseAddr(v)
	return err == nil
}

// isSafeHeaderValue rejects values that would break out of the quoted
// header value in the generated snippets file
func isSafeHeaderValue(v string) bool {
	return !strings.ContainsAny(v, "\"\\{}\r\n")
}

// formInt parses form value as int with default
func formInt(c *fiber.Ctx, key string, defaultVal int) int {
	if v := c.FormValue(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
