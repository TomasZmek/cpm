package handlers

import (
	"fmt"
	"log"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v3"
)

// CertificatesList renders the certificates page
func (h *Handler) CertificatesList(c fiber.Ctx) error {
	certs, err := h.certService.GetAllCertificates()
	if err != nil {
		return err
	}

	stats := h.certService.GetStats()
	flashType, flashMsg := getFlash(c)

	data := h.baseData(c, "SSL Certificates")
	data["Certificates"] = certs
	data["Stats"] = stats
	backups, err := h.certService.ListBackups()
	if err != nil {
		log.Printf("Error listing certificate backups: %v", err)
	}
	data["CertBackups"] = backups
	data["FlashType"] = flashType
	data["FlashMessage"] = flashMsg
	data["Active"] = "certificates"

	return c.Render("pages/certificates", data, "layouts/base")
}

// CertificateDelete moves a certificate to the backups without restarting
// Caddy (Caddy obtains a new one after its next restart if still needed).
func (h *Handler) CertificateDelete(c fiber.Ctx) error {
	domain := c.Params("domain")

	if _, err := h.certService.DeleteCertificate(domain, services.CertBackupReasonDelete); err != nil {
		setFlash(c, "error", tl(c, "msg_cert_delete_failed")+": "+err.Error())
		return certsRedirect(c)
	}

	setFlash(c, "success", tl(c, "msg_cert_deleted", domain))
	return certsRedirect(c)
}

// CertificateRenew moves a certificate to the backups and restarts Caddy so
// it obtains a new one. A reload is not enough: Caddy keeps serving the
// certificate from memory and never notices it is gone.
func (h *Handler) CertificateRenew(c fiber.Ctx) error {
	domain := c.Params("domain")

	if _, err := h.certService.DeleteCertificate(domain, services.CertBackupReasonRenew); err != nil {
		setFlash(c, "error", tl(c, "msg_cert_renew_failed")+": "+err.Error())
		return certsRedirect(c)
	}

	if failed := h.restartCaddyForCertificates(c); failed != "" {
		setFlash(c, "warning", tl(c, "msg_cert_deleted", domain)+" "+failed)
	} else {
		setFlash(c, "success", tl(c, "msg_cert_renewed", domain))
	}
	return certsRedirect(c)
}

// CertificateBackupRestore puts a backed-up certificate back into Caddy's
// storage and restarts Caddy to load it.
func (h *Handler) CertificateBackupRestore(c fiber.Ctx) error {
	b, err := h.certService.RestoreBackup(c.Params("id"))
	if err != nil {
		setFlash(c, "error", tl(c, "msg_cert_restore_failed")+": "+err.Error())
		return certsRedirect(c)
	}

	if failed := h.restartCaddyForCertificates(c); failed != "" {
		setFlash(c, "warning", tl(c, "msg_cert_restored", b.Domain)+" "+failed)
	} else {
		setFlash(c, "success", tl(c, "msg_cert_restored", b.Domain)+" "+tl(c, "msg_caddy_restarted"))
	}
	return certsRedirect(c)
}

// restartCaddyForCertificates restarts Caddy and returns a translated error
// message, or "" on success.
func (h *Handler) restartCaddyForCertificates(c fiber.Ctx) string {
	result := h.caddyService.RestartForCertificates()
	switch {
	case result.Success:
		return ""
	case result.DockerUnreachable:
		return tl(c, "msg_cert_restart_failed")
	default:
		return tl(c, "msg_cert_restart_failed") + ": " + result.Error
	}
}

// CertificateBackupDelete permanently removes a certificate backup.
func (h *Handler) CertificateBackupDelete(c fiber.Ctx) error {
	id := c.Params("id")
	b, err := h.certService.GetBackup(id)
	if err == nil {
		err = h.certService.DeleteBackup(id)
	}
	if err != nil {
		setFlash(c, "error", err.Error())
		return certsRedirect(c)
	}
	setFlash(c, "success", tl(c, "msg_cert_backup_deleted", b.Domain))
	return certsRedirect(c)
}

// CertificatesRestartCaddy restarts Caddy after the bulk-renew progress UI
// has moved certificates to the backups.
func (h *Handler) CertificatesRestartCaddy(c fiber.Ctx) error {
	result := h.caddyService.RestartForCertificates()
	if result.Success {
		return c.JSON(fiber.Map{"ok": true})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"ok": false, "error": result.Error})
}

// CertificatesRenewAll renews certificates in bulk based on a scope:
//
//	expiring (default) -> expiring, critical or already-expired certificates
//	expired            -> only already-expired certificates
//	all                -> every certificate
//
// Wildcard certificates are always excluded; renewing many certificates at
// once can hit Let's Encrypt rate limits (#34). Renewal = move the cert to the
// backups so Caddy obtains a new one after a restart.
func (h *Handler) CertificatesRenewAll(c fiber.Ctx) error {
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
		if _, delErr := h.certService.DeleteCertificate(cert.Domain, services.CertBackupReasonRenew); delErr != nil {
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

	result := h.caddyService.RestartForCertificates()
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
func certsRedirect(c fiber.Ctx) error {
	return redirectTo(c, "/certificates")
}

// CertificateRenewStep moves a single certificate to the backups WITHOUT
// restarting Caddy. Used by the bulk-renew progress UI, which restarts once
// at the end.
func (h *Handler) CertificateRenewStep(c fiber.Ctx) error {
	domain := c.Params("domain")
	if _, err := h.certService.DeleteCertificate(domain, services.CertBackupReasonRenew); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"ok": false, "error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// CertificatesCount returns the current number of certificates as JSON.
// Used by the bulk-renew progress UI to wait until Caddy re-issues certs.
func (h *Handler) CertificatesCount(c fiber.Ctx) error {
	certs, _ := h.certService.GetAllCertificates()
	return c.JSON(fiber.Map{"count": len(certs)})
}
