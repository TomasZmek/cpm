package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// APISites returns all sites as JSON
func (h *Handler) APISites(c *fiber.Ctx) error {
	sites, err := h.caddyService.GetAllSites()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"sites": sites,
		"count": len(sites),
	})
}

// APIStatus returns system status as JSON
func (h *Handler) APIStatus(c *fiber.Ctx) error {
	stats := h.caddyService.GetStats()
	certStats := h.certService.GetStats()
	caddyStatus := h.dockerService.GetContainerStatus()

	return c.JSON(fiber.Map{
		"status": "ok",
		"caddy": fiber.Map{
			"container": h.config.ContainerName,
			"status":    caddyStatus,
			"running":   caddyStatus == "running",
		},
		"sites":        stats,
		"certificates": certStats,
		"version":      h.config.Version,
	})
}

// APIReload reloads Caddy configuration
func (h *Handler) APIReload(c *fiber.Ctx) error {
	validate := c.QueryBool("validate", true)

	var success bool
	var errMsg, message string

	if validate {
		result := h.caddyService.ReloadWithValidation()
		success = result.Success
		errMsg = result.Error
		message = result.Message
	} else {
		result := h.caddyService.Reload()
		success = result.Success
		errMsg = result.Error
		message = result.Message
	}

	if !success {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   errMsg,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": message,
	})
}

// CaddyReload handles the reload button from UI
func (h *Handler) CaddyReload(c *fiber.Ctx) error {
	result := h.caddyService.ReloadWithValidation()

	if c.Get("HX-Request") == "true" {
		if result.Success {
			return c.SendString(`<div class="alert alert-success">✅ Configuration reloaded successfully</div>`)
		}
		return c.SendString(`<div class="alert alert-error">❌ Reload failed: ` + result.Error + `</div>`)
	}

	if result.Success {
		setFlash(c, "success", "Configuration reloaded successfully")
	} else {
		setFlash(c, "error", "Reload failed: "+result.Error)
	}

	return c.Redirect(c.Get("Referer", "/"))
}

// CaddyValidate validates Caddy configuration
func (h *Handler) CaddyValidate(c *fiber.Ctx) error {
	result := h.caddyService.Validate()

	if c.Get("HX-Request") == "true" {
		if result.Success {
			return c.SendString(`<div class="alert alert-success">✅ Configuration is valid</div>`)
		}
		return c.SendString(`<div class="alert alert-error">❌ Validation failed: ` + result.Error + `</div>`)
	}

	if result.Success {
		setFlash(c, "success", "Configuration is valid")
	} else {
		setFlash(c, "error", "Validation failed: "+result.Error)
	}

	return c.Redirect(c.Get("Referer", "/"))
}
