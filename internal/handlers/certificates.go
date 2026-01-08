package handlers

import (
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

	return c.Render("pages/certificates", fiber.Map{
		"Title":        "SSL Certificates",
		"Certificates": certs,
		"Stats":        stats,
		"FlashType":    flashType,
		"FlashMessage": flashMsg,
	}, "layouts/base")
}

// CertificateDelete deletes a certificate to force renewal
func (h *Handler) CertificateDelete(c *fiber.Ctx) error {
	domain := c.Params("domain")

	if err := h.certService.DeleteCertificate(domain); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Reload Caddy to trigger renewal
	result := h.caddyService.Reload()
	if !result.Success {
		setFlash(c, "warning", "Certificate deleted but reload failed: "+result.Error)
	} else {
		setFlash(c, "success", "Certificate for '"+domain+"' deleted. Renewal will be triggered.")
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
