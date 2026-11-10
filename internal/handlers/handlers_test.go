package handlers

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestSafeRedirectTarget(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(safeRedirectTarget(c)) })

	cases := map[string]string{
		"":                               "/",
		"http://example.com/sites?tag=a": "/sites?tag=a",
		"https://evil.com/phish":         "/",
		"//evil.com/x":                   "/",
		"/settings":                      "/settings",
		"http://example.com:8501/x":      "/", // other port = other origin
	}
	run := func(host string, cases map[string]string) {
		for ref, want := range cases {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = host
			if ref != "" {
				req.Header.Set("Referer", ref)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			if string(body) != want {
				t.Errorf("Host %q, Referer %q: got %q, want %q", host, ref, body, want)
			}
		}
	}
	// CPM opened with a port, e.g. http://192.168.50.222:8501
	run("192.168.50.222:8501", map[string]string{
		"http://192.168.50.222:8501/sites/new": "/sites/new",
		"http://192.168.50.222/sites/new":      "/",
	})

	for ref, want := range cases {
		// Origin-form request with an explicit Host header: fasthttp >= 1.70
		// rejects requests without one.
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "example.com"
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
