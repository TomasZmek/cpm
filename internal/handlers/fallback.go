package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// FallbackSave saves the fallback rule content.
func (h *Handler) FallbackSave(c *fiber.Ctx) error {
	if err := h.caddyService.SaveFallback(c.FormValue("content")); err != nil {
		setFlash(c, "error", err.Error())
		return c.Redirect("/settings/caddy")
	}
	h.caddyService.ReloadWithValidation()
	setFlash(c, "success", tl(c, "msg_fallback_saved"))
	return c.Redirect("/settings/caddy")
}

// FallbackCreate creates a default fallback.caddy file.
func (h *Handler) FallbackCreate(c *fiber.Ctx) error {
	def := "# Fallback — handles requests that don't match any proxy rule.\n" +
		"# Edit and save below.\n" +
		":80 {\n\trespond \"No matching site\" 404\n}\n"
	if err := h.caddyService.SaveFallback(def); err != nil {
		setFlash(c, "error", err.Error())
		return c.Redirect("/settings/caddy")
	}
	setFlash(c, "success", tl(c, "msg_fallback_created"))
	return c.Redirect("/settings/caddy")
}

// ErrorPageSave saves a custom error page (e.g. 403/404).
func (h *Handler) ErrorPageSave(c *fiber.Ctx) error {
	code, err := strconv.Atoi(c.Params("code"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid error code")
	}
	if err := h.caddyService.SaveErrorPage(code, c.FormValue("content")); err != nil {
		setFlash(c, "error", err.Error())
		return c.Redirect("/settings/caddy")
	}
	setFlash(c, "success", tl(c, "msg_error_page_saved"))
	return c.Redirect("/settings/caddy")
}
