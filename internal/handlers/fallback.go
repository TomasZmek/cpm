package handlers

import (
	"strconv"

	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v3"
)

// defaultFallback answers plain-HTTP requests for hosts without a rule.
// Verified with Caddy 2.10: automatic HTTP->HTTPS redirects for configured
// sites still take precedence over this catch-all.
const defaultFallback = "# Fallback — handles plain-HTTP requests that don't match any proxy rule.\n" +
	"# Automatic HTTP->HTTPS redirects of configured sites still apply.\n" +
	":80 {\n\trespond \"No matching site\" 404\n}\n"

// FallbackSave saves the fallback rule content.
func (h *Handler) FallbackSave(c fiber.Ctx) error {
	return h.saveFallback(c, c.FormValue("content"), "msg_fallback_saved")
}

// FallbackCreate creates a default fallback.caddy file.
func (h *Handler) FallbackCreate(c fiber.Ctx) error {
	return h.saveFallback(c, defaultFallback, "msg_fallback_created")
}

// saveFallback writes the fallback, validates and reloads Caddy, and restores
// the previous state when the new configuration is rejected (#32).
func (h *Handler) saveFallback(c fiber.Ctx, content, successKey string) error {
	result, err := h.caddyService.ApplyChange(func() error {
		_, err := h.caddyService.SaveFallback(content)
		return err
	})
	if err != nil {
		return redirectFlash(c, "error", err.Error(), "/settings/caddy")
	}
	applyFlash(c, result, tl(c, successKey))
	return redirectTo(c, "/settings/caddy")
}

// ErrorPageSave saves a custom error page (403 or 404).
func (h *Handler) ErrorPageSave(c fiber.Ctx) error {
	code, err := strconv.Atoi(c.Params("code"))
	if err != nil || !services.AllowedErrorPages[code] {
		return c.Status(fiber.StatusBadRequest).SendString("invalid error code")
	}
	if err := h.caddyService.SaveErrorPage(code, c.FormValue("content")); err != nil {
		return redirectFlash(c, "error", err.Error(), "/settings/caddy")
	}
	return redirectFlash(c, "success", tl(c, "msg_error_page_saved"), "/settings/caddy")
}
