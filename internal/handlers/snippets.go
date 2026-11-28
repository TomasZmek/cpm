package handlers

import (
	"errors"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v3"
)

// SnippetsList renders the snippets management page
func (h *Handler) SnippetsList(c fiber.Ctx) error {
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
func (h *Handler) SnippetUpdate(c fiber.Ctx) error {
	snippetName := c.Params("name")

	cfg, err := h.snippetsService.GetConfig()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	var envWarning string

	switch snippetName {
	case "cloudflare_dns":
		cfg.CloudflareDNS.Enabled = c.FormValue("enabled") == "on"
		if !cfg.CloudflareDNS.Enabled {
			// The form disables (and therefore omits) all fields of a disabled
			// card; keep the stored token/env choice for when it is re-enabled.
			break
		}
		cfg.CloudflareDNS.UseEnv = c.FormValue("use_env") == "on"
		cfg.CloudflareDNS.APIToken = strings.TrimSpace(c.FormValue("api_token"))
		// A token typed into the form is an explicit choice: use it even if the
		// "use env" toggle was left on (its default). Otherwise Caddy would get
		// an empty {env.CF_API_TOKEN} and fail with "API token '' appears
		// invalid" (TomasZmek/cpm#21). The form disables the token field while
		// the toggle is on, so switching to env does not submit the old token.
		if cfg.CloudflareDNS.APIToken != "" {
			cfg.CloudflareDNS.UseEnv = false
			if err := services.ValidateAPIToken(cfg.CloudflareDNS.APIToken); err != nil {
				return c.Status(fiber.StatusBadRequest).SendString(err.Error())
			}
		}
		if cfg.CloudflareDNS.Enabled && !cfg.CloudflareDNS.UseEnv && cfg.CloudflareDNS.APIToken == "" {
			return c.Status(fiber.StatusBadRequest).SendString(tl(c, "msg_cf_token_required"))
		}
		if cfg.CloudflareDNS.Enabled && cfg.CloudflareDNS.UseEnv {
			envWarning = h.cloudflareEnvWarning(c)
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
		// Users are managed with BasicAuthUserAdd / BasicAuthUserDelete

	default:
		return c.Status(fiber.StatusBadRequest).SendString("Unknown snippet: " + snippetName)
	}

	// Save, validate and reload; a change Caddy rejects is undone (#23)
	result, err := h.caddyService.ApplyChange(func() error { return h.snippetsService.SaveConfig(cfg) })
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	switch {
	case !result.Success && !result.DockerUnreachable && envWarning != "":
		setFlash(c, "error", envWarning+" "+applyErrorMessage(c, result))
	case result.Success && envWarning != "":
		setFlash(c, "warning", envWarning)
	default:
		applyFlash(c, result, tl(c, "msg_snippet_updated", snippetName))
	}

	return redirectTo(c, "/snippets")
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

// cloudflareEnvWarning returns a translated warning when the Caddy container
// does not define CF_API_TOKEN, which makes "{env.CF_API_TOKEN}" empty.
// Returns "" when the variable is set or Docker cannot be queried.
func (h *Handler) cloudflareEnvWarning(c fiber.Ctx) string {
	if h.dockerService == nil || !h.dockerService.IsAvailable() {
		return ""
	}
	has, err := h.dockerService.ContainerHasEnv("CF_API_TOKEN")
	if err != nil || has {
		return ""
	}
	return tl(c, "msg_cf_env_missing", h.config.ContainerName)
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
func formInt(c fiber.Ctx, key string, defaultVal int) int {
	if v := c.FormValue(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// BasicAuthUserAdd adds a user to the basic_auth snippet, or sets a new
// password for an existing one (TomasZmek/cpm#41).
func (h *Handler) BasicAuthUserAdd(c fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")

	if err := services.ValidateBasicAuthUsername(username); err != nil {
		setFlash(c, "error", tl(c, "msg_basic_auth_username_invalid"))
		return redirectTo(c, "/snippets")
	}
	if err := services.ValidateBasicAuthPassword(password); err != nil {
		setFlash(c, "error", tl(c, "msg_basic_auth_password_length", services.BasicAuthMinPassword, services.BasicAuthMaxPassword))
		return redirectTo(c, "/snippets")
	}

	var created bool
	result, err := h.caddyService.ApplyChange(func() error {
		var err error
		created, err = h.snippetsService.AddBasicAuthUser(username, password)
		return err
	})
	if err != nil {
		setFlash(c, "error", tl(c, "msg_basic_auth_save_failed")+": "+err.Error())
		return redirectTo(c, "/snippets")
	}
	key := "msg_basic_auth_user_updated"
	if created {
		key = "msg_basic_auth_user_added"
	}
	applyFlash(c, result, tl(c, key, username))
	return redirectTo(c, "/snippets")
}

// BasicAuthUserDelete removes a user of the basic_auth snippet.
func (h *Handler) BasicAuthUserDelete(c fiber.Ctx) error {
	username := c.FormValue("username")

	result, err := h.caddyService.ApplyChange(func() error {
		return h.snippetsService.RemoveBasicAuthUser(username)
	})
	switch {
	case errors.Is(err, services.ErrBasicAuthLastUser):
		setFlash(c, "error", tl(c, "msg_basic_auth_last_user"))
		return redirectTo(c, "/snippets")
	case errors.Is(err, services.ErrBasicAuthUserNotFound):
		setFlash(c, "error", tl(c, "msg_basic_auth_user_not_found"))
		return redirectTo(c, "/snippets")
	case err != nil:
		setFlash(c, "error", tl(c, "msg_basic_auth_save_failed")+": "+err.Error())
		return redirectTo(c, "/snippets")
	}
	applyFlash(c, result, tl(c, "msg_basic_auth_user_removed", username))
	return redirectTo(c, "/snippets")
}
