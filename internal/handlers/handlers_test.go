package handlers

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSafeRedirectTarget(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString(safeRedirectTarget(c)) })

	cases := map[string]string{
		"":                               "/",
		"http://example.com/sites?tag=a": "/sites?tag=a",
		"https://evil.com/phish":         "/",
		"//evil.com/x":                   "/",
		"/settings":                      "/settings",
	}
	for ref, want := range cases {
		req := httptest.NewRequest("GET", "http://example.com/", nil)
		if ref != "" {
			req.Header.Set("Referer", ref)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != want {
			t.Errorf("Referer %q: got %q, want %q", ref, body, want)
		}
	}
}
