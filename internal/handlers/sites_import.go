package handlers

import (
	"fmt"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/gofiber/fiber/v2"
)

// SitesImportPreview parses the site blocks written directly in the main
// Caddyfile and returns them as JSON WITHOUT saving anything. Used to populate
// the import modal on the proxy-rules page.
func (h *Handler) SitesImportPreview(c *fiber.Ctx) error {
	content := c.FormValue("content")
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		path = strings.TrimSpace(c.FormValue("path"))
	}
	var sites []*models.Site
	var err error
	switch {
	case content != "":
		sites, err = h.caddyService.PreviewImportContent(content)
	case path != "":
		sites, err = h.caddyService.PreviewImportFromFile(path)
	default:
		sites, err = h.caddyService.PreviewImportFromMainCaddyfile()
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	type previewSite struct {
		Domain      string   `json:"domain"`
		Domains     []string `json:"domains"`
		Target      string   `json:"target"`
		TLSMode     string   `json:"tls_mode"`
		Snippets    []string `json:"snippets"`
		ExtraConfig string   `json:"extra_config"`
		Filename    string   `json:"filename"`
	}

	preview := make([]previewSite, 0, len(sites))
	for _, s := range sites {
		target := s.TargetIP
		if s.TargetPort != "" {
			target = s.TargetIP + ":" + s.TargetPort
		}
		preview = append(preview, previewSite{
			Domain:      s.PrimaryDomain(),
			Domains:     s.Domains,
			Target:      target,
			TLSMode:     s.TLSMode,
			Snippets:    s.Snippets,
			ExtraConfig: s.ExtraConfig,
			Filename:    s.PrimaryDomain(),
		})
	}

	return c.JSON(fiber.Map{
		"count": len(preview),
		"sites": preview,
	})
}

// SitesImport parses the site blocks in the main Caddyfile and saves each one as
// sites/standard/{domain}.caddy, then validates and reloads Caddy.
func (h *Handler) SitesImport(c *fiber.Ctx) error {
	content := c.FormValue("content")
	path := strings.TrimSpace(c.FormValue("path"))
	var imported, failed []string
	var err error
	switch {
	case content != "":
		imported, failed, err = h.caddyService.ImportContent(content)
	case path != "":
		imported, failed, err = h.caddyService.ImportFromFile(path)
	default:
		imported, failed, err = h.caddyService.ImportFromMainCaddyfile()
	}
	if err != nil {
		setFlash(c, "error", "Import failed: "+err.Error())
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/sites")
			return c.SendStatus(fiber.StatusOK)
		}
		return c.Redirect("/sites")
	}

	if len(imported) == 0 && len(failed) == 0 {
		setFlash(c, "info", "No site blocks found in the main Caddyfile to import.")
		if c.Get("HX-Request") == "true" {
			c.Set("HX-Redirect", "/sites")
			return c.SendStatus(fiber.StatusOK)
		}
		return c.Redirect("/sites")
	}

	// Reload Caddy only if at least one site was imported.
	if len(imported) > 0 {
		result := h.caddyService.ReloadWithValidation()
		if !result.Success {
			setFlash(c, "warning", fmt.Sprintf("Imported %d rule(s) but reload failed: %s", len(imported), result.Error))
			if c.Get("HX-Request") == "true" {
				c.Set("HX-Redirect", "/sites")
				return c.SendStatus(fiber.StatusOK)
			}
			return c.Redirect("/sites")
		}
	}

	msg := fmt.Sprintf("Imported %d rule(s).", len(imported))
	if len(failed) > 0 {
		msg += fmt.Sprintf(" Skipped %d: %s", len(failed), strings.Join(failed, ", "))
		setFlash(c, "warning", msg)
	} else {
		setFlash(c, "success", msg)
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/sites")
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect("/sites")
}
