package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestFlashRoundTripAnyLanguage(t *testing.T) {
	msg := `Pravidlo "anki" uloženo; 규칙이 저장되었습니다 ✓`
	app := fiber.New()
	app.Post("/save", func(c fiber.Ctx) error {
		setFlash(c, "success", msg)
		return redirectTo(c, "/page")
	})
	app.Get("/page", func(c fiber.Ctx) error {
		typ, m := getFlash(c)
		return c.SendString(typ + "|" + m)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/save", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/page" {
		t.Fatalf("status %d, location %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	for _, ck := range resp.Cookies() {
		req.AddCookie(ck)
	}
	resp2, _ := app.Test(req)
	body, _ := io.ReadAll(resp2.Body)
	if string(body) != "success|"+msg {
		t.Fatalf("flash = %q", body)
	}
}

func TestRedirectToHTMX(t *testing.T) {
	app := fiber.New()
	app.Post("/save", func(c fiber.Ctx) error {
		setFlash(c, "success", "ok")
		return redirectTo(c, "/sites")
	})
	req := httptest.NewRequest(http.MethodPost, "/save", nil)
	req.Header.Set("HX-Request", "true")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("HX-Redirect") != "/sites" || resp.Header.Get("Location") != "" {
		t.Fatalf("status %d, HX-Redirect %q, Location %q", resp.StatusCode, resp.Header.Get("HX-Redirect"), resp.Header.Get("Location"))
	}
	found := false
	for _, ck := range resp.Cookies() {
		found = found || (ck.Name == fiber.FlashCookieName && ck.Value != "")
	}
	if !found {
		t.Fatal("flash cookie not set on the HTMX response")
	}
}
