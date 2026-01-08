package middleware

import (
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
)

const sessionCookieName = "cpm_session"

// Auth middleware checks if user is authenticated
func Auth(authService *services.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// If auth is not enabled, allow all requests
		if !authService.IsEnabled() {
			return c.Next()
		}

		// Get session token from cookie
		token := c.Cookies(sessionCookieName)
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

// RequireRole middleware checks if user has required role
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := c.Locals("user")
		if user == nil {
			return redirectToLogin(c)
		}

		// Check if user has any of the required roles
		// This is a simplified check - you might want to implement proper role checking
		return c.Next()
	}
}

// RequirePermission middleware checks if user has required permission
func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := c.Locals("user")
		if user == nil {
			return redirectToLogin(c)
		}

		// For now, allow all authenticated users
		// You can implement proper permission checking based on your User model
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
	if c.Get("Accept") == "application/json" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// For regular requests, redirect to login
	return c.Redirect("/login")
}
