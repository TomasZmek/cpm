package handlers

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRequestHostIP(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(requestHostIP(c)) })
	for host, want := range map[string]string{
		"192.168.50.222:8501": "192.168.50.222",
		"192.168.50.222":      "192.168.50.222",
		"cpm.example.com":     "",
		"localhost:8501":      "",
		"127.0.0.1:8501":      "",
		"172.23.0.2:8501":     "", // container address
		"[fd00::5]:8501":      "fd00::5",
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Host = host
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != want {
			t.Errorf("Host %q: got %q, want %q", host, body, want)
		}
	}
}
