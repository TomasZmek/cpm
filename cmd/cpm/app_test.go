package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/TomasZmek/cpm/internal/config"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
)

// testApp builds the real middleware stack (proxy trust + CSRF) with a form
// page and a POST target. app.Test connects from 0.0.0.0, so that address
// stands in for the reverse proxy in TRUSTED_PROXIES.
func testApp(t *testing.T, trustedProxies string, origins ...string) *fiber.App {
	t.Helper()
	cfg := &config.Config{TrustedProxies: trustedProxies, TrustedOrigins: origins}
	app := newApp(cfg, nil)
	app.Get("/form", func(c fiber.Ctx) error { return c.SendString(csrf.TokenFromContext(c)) })
	app.Post("/save", func(c fiber.Ctx) error { return c.SendString("saved") })
	return app
}

// formToken loads the form page as the browser would and returns the token
// and the CSRF cookie.
func formToken(t *testing.T, app *fiber.App, host string, headers map[string]string) (string, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/form", nil)
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	for _, ck := range resp.Cookies() {
		if ck.Name == "cpm_csrf_v3" {
			return string(body), ck
		}
	}
	t.Fatal("no CSRF cookie issued")
	return "", nil
}

func post(t *testing.T, app *fiber.App, host, origin, token string, cookie *http.Cookie, headers map[string]string) *http.Response {
	t.Helper()
	form := url.Values{"_csrf": {token}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/save", strings.NewReader(form))
	req.Host = host
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// Caddy in front of CPM: TLS ends at Caddy, the request to CPM is plain HTTP
// with the original Host (Caddy keeps it) and X-Forwarded-Proto: https.
var caddyHeaders = map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-For": "192.168.50.10"}

func TestCSRFDirectAccess(t *testing.T) {
	app := testApp(t, "private,loopback")
	token, ck := formToken(t, app, "192.168.50.222:8501", nil)
	if resp := post(t, app, "192.168.50.222:8501", "http://192.168.50.222:8501", token, ck, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("direct access: status %d", resp.StatusCode)
	}
}

func TestCSRFBehindTrustedCaddy(t *testing.T) {
	app := testApp(t, "0.0.0.0")
	token, ck := formToken(t, app, "cpm.example.com", caddyHeaders)
	if resp := post(t, app, "cpm.example.com", "https://cpm.example.com", token, ck, caddyHeaders); resp.StatusCode != http.StatusOK {
		t.Fatalf("behind trusted Caddy: status %d", resp.StatusCode)
	}
}

func TestCSRFBehindUntrustedProxyNeedsTrustedOrigin(t *testing.T) {
	// The proxy is not trusted: CPM sees http while the browser says https.
	app := testApp(t, "none")
	token, ck := formToken(t, app, "cpm.example.com", caddyHeaders)
	if resp := post(t, app, "cpm.example.com", "https://cpm.example.com", token, ck, caddyHeaders); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("untrusted proxy: status %d, want 403", resp.StatusCode)
	}
	// TRUSTED_ORIGINS fixes it without trusting the proxy.
	app = testApp(t, "none", "https://cpm.example.com")
	token, ck = formToken(t, app, "cpm.example.com", caddyHeaders)
	if resp := post(t, app, "cpm.example.com", "https://cpm.example.com", token, ck, caddyHeaders); resp.StatusCode != http.StatusOK {
		t.Fatalf("with TRUSTED_ORIGINS: status %d", resp.StatusCode)
	}
}

func TestCSRFRejectsCrossSite(t *testing.T) {
	app := testApp(t, "0.0.0.0")
	token, ck := formToken(t, app, "cpm.example.com", caddyHeaders)
	if resp := post(t, app, "cpm.example.com", "https://evil.example.org", token, ck, caddyHeaders); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site origin: status %d, want 403", resp.StatusCode)
	}
	// A forwarded header from an untrusted client does not make it https.
	app = testApp(t, "10.9.9.9")
	token, ck = formToken(t, app, "cpm.example.com", caddyHeaders)
	if resp := post(t, app, "cpm.example.com", "https://cpm.example.com", token, ck, caddyHeaders); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("spoofed X-Forwarded-Proto: status %d, want 403", resp.StatusCode)
	}
}

func TestCSRFExpiredTokenRedirectsBack(t *testing.T) {
	app := testApp(t, "private,loopback")
	// A form opened before a restart/upgrade: its token is not known any more.
	resp := post(t, app, "192.168.50.222:8501", "http://192.168.50.222:8501", "stale-token",
		&http.Cookie{Name: "cpm_csrf_v3", Value: "stale-token"},
		map[string]string{"Referer": "http://192.168.50.222:8501/sites/new"})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/sites/new" {
		t.Fatalf("expired token: status %d location %q, want 303 back to the form", resp.StatusCode, resp.Header.Get("Location"))
	}
	flash := false
	for _, ck := range resp.Cookies() {
		if ck.Name == fiber.FlashCookieName && ck.Value != "" {
			flash = true
		}
	}
	if !flash {
		t.Fatal("no flash message explaining the expired page")
	}
}
