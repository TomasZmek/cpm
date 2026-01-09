package handlers

import (
	"github.com/TomasZmek/cpm/internal/models"
	"github.com/gofiber/fiber/v2"
)

const sessionCookieName = "cpm_session"

// LoginPage renders the login page
func (h *Handler) LoginPage(c *fiber.Ctx) error {
	// If already logged in, redirect to dashboard
	if token := c.Cookies(sessionCookieName); token != "" {
		if user := h.authService.ValidateSession(token); user != nil {
			return c.Redirect("/")
		}
	}

	// Check if no users exist (first setup)
	needsSetup := !h.authService.HasUsers()

	data := fiber.Map{
		"NeedsSetup": needsSetup,
		"Error":      c.Query("error"),
		"Version":    h.config.Version,
		"Lang":       "en",
	}

	// Login page has its own HTML structure, no layout needed
	return c.Render("pages/login", data)
}

// Login handles the login form submission
func (h *Handler) Login(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	// Check if this is first setup (creating admin)
	if !h.authService.HasUsers() {
		// Create first admin user
		if err := h.authService.CreateUser(username, password, models.RoleAdmin); err != nil {
			return c.Redirect("/login?error=Failed+to+create+user")
		}

		// Enable authentication
		if err := h.authService.Enable(); err != nil {
			return c.Redirect("/login?error=Failed+to+enable+auth")
		}
	}

	// Authenticate
	token, err := h.authService.Authenticate(username, password)
	if err != nil {
		return c.Redirect("/login?error=Invalid+credentials")
	}

	// Set session cookie
	c.Cookie(&fiber.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   86400 * 7, // 7 days
	})

	return c.Redirect("/")
}

// Logout handles logout
func (h *Handler) Logout(c *fiber.Ctx) error {
	token := c.Cookies(sessionCookieName)
	if token != "" {
		h.authService.Logout(token)
	}

	// Clear cookie
	c.Cookie(&fiber.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		MaxAge: -1,
	})

	return c.Redirect("/login")
}

// UserCreate creates a new user (settings page)
func (h *Handler) UserCreate(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	role := models.Role(c.FormValue("role"))

	if username == "" || password == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Username and password are required")
	}

	if len(password) < 6 {
		return c.Status(fiber.StatusBadRequest).SendString("Password must be at least 6 characters")
	}

	if err := h.authService.CreateUser(username, password, role); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	setFlash(c, "success", "User '"+username+"' created successfully")

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/settings/users")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/settings/users")
}

// UserDelete deletes a user
func (h *Handler) UserDelete(c *fiber.Ctx) error {
	username := c.Params("username")

	// Can't delete yourself
	currentUser := h.getCurrentUser(c)
	if user, ok := currentUser.(*models.User); ok && user.Username == username {
		return c.Status(fiber.StatusBadRequest).SendString("Cannot delete your own account")
	}

	if err := h.authService.DeleteUser(username); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	setFlash(c, "success", "User '"+username+"' deleted successfully")

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/settings/users")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/settings/users")
}

// UserUpdateRole updates a user's role
func (h *Handler) UserUpdateRole(c *fiber.Ctx) error {
	username := c.Params("username")
	role := models.Role(c.FormValue("role"))

	if err := h.authService.UpdateRole(username, role); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	setFlash(c, "success", "Role updated successfully")

	if c.Get("HX-Request") == "true" {
		return c.SendString("OK")
	}

	return c.Redirect("/settings/users")
}

// UserUpdatePassword updates a user's password
func (h *Handler) UserUpdatePassword(c *fiber.Ctx) error {
	username := c.Params("username")
	password := c.FormValue("password")

	if len(password) < 6 {
		return c.Status(fiber.StatusBadRequest).SendString("Password must be at least 6 characters")
	}

	if err := h.authService.UpdatePassword(username, password); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	setFlash(c, "success", "Password updated successfully")

	if c.Get("HX-Request") == "true" {
		return c.SendString("OK")
	}

	return c.Redirect("/settings/users")
}

// ToggleAuth enables or disables authentication
func (h *Handler) ToggleAuth(c *fiber.Ctx) error {
	enabled := c.FormValue("enabled") == "true" || c.FormValue("enabled") == "on"

	var err error
	if enabled {
		err = h.authService.Enable()
	} else {
		err = h.authService.Disable()
	}

	if err != nil {
		if c.Get("HX-Request") == "true" {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		setFlash(c, "error", err.Error())
		return c.Redirect("/settings/users")
	}

	if enabled {
		setFlash(c, "success", "Authentication enabled")
	} else {
		setFlash(c, "success", "Authentication disabled")
	}

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/settings/users")
		return c.SendStatus(fiber.StatusOK)
	}

	return c.Redirect("/settings/users")
}
