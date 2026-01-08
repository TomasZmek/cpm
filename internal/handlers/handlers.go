package handlers

import (
	"github.com/TomasZmek/cpm/internal/config"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

// Handler contains all HTTP handlers
type Handler struct {
	config          *config.Config
	caddyService    *services.CaddyService
	certService     *services.CertificateService
	snippetsService *services.SnippetsService
	authService     *services.AuthService
	backupService   *services.BackupService
	dockerService   *services.DockerService
}

// New creates a new Handler instance
func New(
	cfg *config.Config,
	caddyService *services.CaddyService,
	certService *services.CertificateService,
	snippetsService *services.SnippetsService,
	authService *services.AuthService,
	backupService *services.BackupService,
	dockerService *services.DockerService,
) *Handler {
	return &Handler{
		config:          cfg,
		caddyService:    caddyService,
		certService:     certService,
		snippetsService: snippetsService,
		authService:     authService,
		backupService:   backupService,
		dockerService:   dockerService,
	}
}

// ErrorHandler handles errors globally
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	// For HTMX requests, return error as HTML
	if c.Get("HX-Request") == "true" {
		return c.Status(code).SendString(`<div class="alert alert-error">` + err.Error() + `</div>`)
	}

	// For API requests, return JSON
	if isAPIRequest(c) {
		return c.Status(code).JSON(fiber.Map{
			"error": err.Error(),
			"code":  code,
		})
	}

	// For regular requests, show error page
	return c.Status(code).Render("error", fiber.Map{
		"Code":    code,
		"Message": err.Error(),
	})
}

// isAPIRequest checks if request is an API request
func isAPIRequest(c *fiber.Ctx) bool {
	return c.Get("Accept") == "application/json" ||
		c.Path()[:4] == "/api"
}

// getCurrentUser returns the current user from context
func (h *Handler) getCurrentUser(c *fiber.Ctx) interface{} {
	return c.Locals("user")
}

// Flash messages helper
func setFlash(c *fiber.Ctx, msgType, message string) {
	c.Cookie(&fiber.Cookie{
		Name:  "flash_type",
		Value: msgType,
	})
	c.Cookie(&fiber.Cookie{
		Name:  "flash_message",
		Value: message,
	})
}

func getFlash(c *fiber.Ctx) (string, string) {
	msgType := c.Cookies("flash_type")
	message := c.Cookies("flash_message")

	// Clear flash cookies
	c.Cookie(&fiber.Cookie{Name: "flash_type", Value: "", MaxAge: -1})
	c.Cookie(&fiber.Cookie{Name: "flash_message", Value: "", MaxAge: -1})

	return msgType, message
}
