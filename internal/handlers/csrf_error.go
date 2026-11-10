package handlers

import (
	"errors"
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
)

// CSRFError answers a request the CSRF middleware rejected.
//
//   - Token missing or invalid: tokens are kept in memory, so every CPM
//     restart or upgrade invalidates the forms open in browsers. Instead of a
//     bare 403 the user is sent back to the page (which issues a new token)
//     with a hint to submit again.
//   - Origin/Referer mismatch: either a real cross-site request or CPM sits
//     behind a reverse proxy it does not trust (the browser's https://domain
//     does not match the plain http:// request CPM sees). Answered 403 with a
//     hint at TRUSTED_PROXIES / TRUSTED_ORIGINS, never redirected.
func CSRFError(c fiber.Ctx, err error) error {
	log.Printf("CSRF: rejected %s %s from %s: %v (origin %q, scheme %s, host %s)",
		c.Method(), c.Path(), c.IP(), err, c.Get(fiber.HeaderOrigin), c.Scheme(), c.Host())

	origin := errors.Is(err, csrf.ErrOriginNoMatch) || errors.Is(err, csrf.ErrOriginInvalid) ||
		errors.Is(err, csrf.ErrRefererNoMatch) || errors.Is(err, csrf.ErrRefererInvalid) ||
		errors.Is(err, csrf.ErrRefererNotFound) || errors.Is(err, csrf.ErrFetchSiteInvalid)

	if strings.HasPrefix(c.Path(), "/api/") || strings.Contains(c.Get(fiber.HeaderAccept), fiber.MIMEApplicationJSON) {
		msg := "invalid or expired CSRF token, reload the page"
		if origin {
			msg = "request origin does not match this site"
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": msg})
	}
	if origin {
		return fiber.NewError(fiber.StatusForbidden, tl(c, "msg_csrf_origin"))
	}

	setFlash(c, "warning", tl(c, "msg_csrf_expired"))
	return redirectTo(c, safeRedirectTarget(c))
}
