package middleware

import (
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/TomasZmek/cpm/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// Auth middleware checks if user is authenticated
func Auth(authService *services.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// If auth is not enabled, allow all requests
		if !authService.IsEnabled() {
			return c.Next()
		}

		// Get session token from cookie
		token := c.Cookies(utils.SessionCookieName)
		if token == "" {
			return redirectToLogin(c)
		}

		// Validate session
		user := authService.ValidateSession(token)
		if user == nil {
			return redirectToLogin(c)
		}

		// Store user in context
		c.Locals("user", user)

		return c.Next()
	}
}

// RequirePermission middleware checks that the current user has the given
// permission ("view", "edit" or "admin", see models.User.HasPermission).
// Must be mounted after Auth. When authentication is disabled every request
// is allowed, matching the behaviour of Auth.
func RequirePermission(authService *services.AuthService, permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !authService.IsEnabled() {
			return c.Next()
		}

		user, ok := c.Locals("user").(*models.User)
		if !ok || user == nil {
			return redirectToLogin(c)
		}

		if !user.HasPermission(permission) {
			return fiber.NewError(fiber.StatusForbidden, "You do not have permission to perform this action")
		}

		return c.Next()
	}
}

func redirectToLogin(c *fiber.Ctx) error {
	// For HTMX requests, return a redirect header
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/login")
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	// For API requests, return JSON error
	if c.Get("Accept") == "application/json" || strings.HasPrefix(c.Path(), "/api/") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// For regular requests, redirect to login
	return c.Redirect("/login")
}
