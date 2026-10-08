package handlers

import (
	"path/filepath"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

// SitesImportPreview parses the site blocks written directly in the main
// Caddyfile and returns them as JSON WITHOUT saving anything. Used to populate
// the import modal on the proxy-rules page.
func (h *Handler) SitesImportPreview(c *fiber.Ctx) error {
	// Sources are limited to an uploaded file's content or the main Caddyfile.
	// Arbitrary server-side paths are deliberately not accepted (#29).
	content := c.FormValue("content")
	var sites []*models.Site
	var err error
	if content != "" {
		sites, err = h.caddyService.PreviewImportContent(content)
	} else {
		sites, err = h.caddyService.PreviewImportFromMainCaddyfile()
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
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

// SitesImport saves the site blocks of the main Caddyfile (or an uploaded
// Caddyfile) as sites/standard/{domain}.caddy, then validates and reloads
// Caddy. A backup is stored first, and all file changes are rolled back if the
// resulting configuration does not validate (#30, #31).
func (h *Handler) SitesImport(c *fiber.Ctx) error {
	backupPath, err := h.backupService.SaveBackupToDisk()
	if err != nil {
		return redirectFlash(c, "error", tl(c, "msg_import_backup_failed")+": "+err.Error(), "/sites")
	}

	content := c.FormValue("content")
	var res *services.ImportResult
	if content != "" {
		res, err = h.caddyService.ImportContent(content)
	} else {
		res, err = h.caddyService.ImportFromMainCaddyfile()
	}
	if err != nil {
		return redirectFlash(c, "error", tl(c, "msg_import_failed")+": "+err.Error(), "/sites")
	}

	if len(res.Imported) == 0 && len(res.Failed) == 0 {
		return redirectFlash(c, "info", tl(c, "msg_import_none"), "/sites")
	}

	if len(res.Imported) > 0 {
		result := h.caddyService.ReloadWithValidation()
		if !result.Success && result.DockerUnreachable {
			// Nothing was validated; keep the import (consistent with other
			// saves) and tell the user to reload once Docker is reachable.
			return redirectFlash(c, "warning", tl(c, "msg_import_done", len(res.Imported))+" "+result.Error, "/sites")
		}
		if !result.Success {
			if rbErr := res.Rollback(); rbErr != nil {
				return redirectFlash(c, "error", tl(c, "msg_import_rollback_failed", filepath.Base(backupPath))+": "+rbErr.Error(), "/sites")
			}
			return redirectFlash(c, "warning", tl(c, "msg_import_rolled_back")+": "+result.Error, "/sites")
		}
	}

	msg := tl(c, "msg_import_done", len(res.Imported)) + " " + tl(c, "msg_import_backup_saved", filepath.Base(backupPath))
	if len(res.Failed) > 0 {
		msg += " " + tl(c, "msg_import_skipped", len(res.Failed), strings.Join(res.Failed, ", "))
		return redirectFlash(c, "warning", msg, "/sites")
	}
	return redirectFlash(c, "success", msg, "/sites")
}
