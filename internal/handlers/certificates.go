package handlers

import (
	"fmt"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/gofiber/fiber/v2"
)

// CertificatesList renders the certificates page
func (h *Handler) CertificatesList(c *fiber.Ctx) error {
	certs, err := h.certService.GetAllCertificates()
	if err != nil {
		return err
	}

	stats := h.certService.GetStats()
	flashType, flashMsg := getFlash(c)

	data := h.baseData(c, "SSL Certificates")
	data["Certificates"] = certs
	data["Stats"] = stats
	data["FlashType"] = flashType
	data["FlashMessage"] = flashMsg
	data["Active"] = "certificates"

	return c.Render("pages/certificates", data, "layouts/base")
}

// CertificateDelete deletes a certificate (no renewal)
func (h *Handler) CertificateDelete(c *fiber.Ctx) error {
	domain := c.Params("domain")

	if err := h.certService.DeleteCertificate(domain); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	setFlash(c, "success", tl(c, "msg_cert_deleted", domain))

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/certificates")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/certificates")
}

// CertificateRenew deletes a certificate and reloads Caddy to trigger renewal
func (h *Handler) CertificateRenew(c *fiber.Ctx) error {
	domain := c.Params("domain")

	if err := h.certService.DeleteCertificate(domain); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	result := h.caddyService.ReloadForce()
	if !result.Success {
		setFlash(c, "warning", tl(c, "msg_cert_reload_failed")+": "+result.Error)
	} else {
		setFlash(c, "success", tl(c, "msg_cert_renewed", domain))
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/certificates")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/certificates")
}

// HTMXCertificatesList returns certificates list as HTML partial
func (h *Handler) HTMXCertificatesList(c *fiber.Ctx) error {
	certs, _ := h.certService.GetAllCertificates()

	return c.Render("partials/certificates_list", fiber.Map{
		"Certificates": certs,
	})
}

// CertificatesRenewAll renews certificates in bulk based on a scope:
//
//	expiring (default) -> expiring, critical or already-expired certificates
//	expired            -> only already-expired certificates
//	all                -> every certificate
//
// Wildcard certificates are always excluded; renewing many certificates at
// once can hit Let's Encrypt rate limits (#34). Renewal = delete the cert so
// Caddy re-issues it after a forced reload.
func (h *Handler) CertificatesRenewAll(c *fiber.Ctx) error {
	scope := c.FormValue("scope", "expiring")
	if scope != "expiring" && scope != "expired" && scope != "all" {
		return c.Status(fiber.StatusBadRequest).SendString("invalid scope")
	}

	certs, err := h.certService.GetAllCertificates()
	if err != nil {
		setFlash(c, "error", err.Error())
		return certsRedirect(c)
	}

	renewed := 0
	var failed []string
	for _, cert := range certs {
		if cert.IsWildcard() {
			continue
		}
		match := false
		switch scope {
		case "expired":
			match = cert.Status == models.CertStatusExpired
		case "expiring":
			match = cert.Status == models.CertStatusExpiring ||
				cert.Status == models.CertStatusCritical ||
				cert.Status == models.CertStatusExpired
		case "all":
			match = true
		}
		if !match {
			continue
		}
		if delErr := h.certService.DeleteCertificate(cert.Domain); delErr != nil {
			failed = append(failed, cert.Domain)
			continue
		}
		renewed++
	}

	if renewed == 0 && len(failed) == 0 {
		setFlash(c, "info", tl(c, "msg_certs_bulk_none"))
		return certsRedirect(c)
	}

	failedMsg := ""
	if len(failed) > 0 {
		failedMsg = " " + tl(c, "msg_certs_bulk_failed", len(failed), strings.Join(failed, ", "))
	}

	result := h.caddyService.ReloadForce()
	switch {
	case !result.Success:
		setFlash(c, "warning", fmt.Sprintf("%s (%d): %s", tl(c, "msg_certs_bulk_reload_failed"), renewed, result.Error)+failedMsg)
	case len(failed) > 0:
		setFlash(c, "warning", tl(c, "msg_certs_bulk_done", renewed)+failedMsg)
	default:
		setFlash(c, "success", tl(c, "msg_certs_bulk_done", renewed))
	}
	return certsRedirect(c)
}

// certsRedirect returns the user to the certificates page (HX-aware).
func certsRedirect(c *fiber.Ctx) error {
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/certificates")
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect("/certificates")
}

// CertificateRenewStep deletes a single certificate WITHOUT reloading Caddy.
// Used by the bulk-renew progress UI, which reloads once at the end.
func (h *Handler) CertificateRenewStep(c *fiber.Ctx) error {
	domain := c.Params("domain")
	if err := h.certService.DeleteCertificate(domain); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"ok": false, "error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// CertificatesCount returns the current number of certificates as JSON.
// Used by the bulk-renew progress UI to wait until Caddy re-issues certs.
func (h *Handler) CertificatesCount(c *fiber.Ctx) error {
	certs, _ := h.certService.GetAllCertificates()
	return c.JSON(fiber.Map{"count": len(certs)})
}
