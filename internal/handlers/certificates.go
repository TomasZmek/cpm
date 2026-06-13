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

	result := h.caddyService.Reload()
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
