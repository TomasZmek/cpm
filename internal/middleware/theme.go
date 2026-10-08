package middleware

import (
	"github.com/TomasZmek/cpm/internal/config"
	"github.com/TomasZmek/cpm/internal/views"
	"github.com/gofiber/fiber/v2"
)

// ThemeConfig holds theme configuration
type ThemeConfig struct {
	Name        string
	DisplayName string
	CSSPath     string
}

// AvailableThemes lists all available themes
var AvailableThemes = map[string]ThemeConfig{
	"classic": {
		Name:        "classic",
		DisplayName: "Classic",
		CSSPath:     "/static/css/themes/classic.css",
	},
	"modern": {
		Name:        "modern",
		DisplayName: "Modern",
		CSSPath:     "/static/css/themes/modern.css",
	},
}

// themeOrder is the order themes are offered in Settings.
var themeOrder = []string{"classic", "modern"}

// ThemeNames returns the available theme names in display order.
func ThemeNames() []string {
	return append([]string(nil), themeOrder...)
}

// ThemeList returns the available themes in display order.
func ThemeList() []ThemeConfig {
	list := make([]ThemeConfig, 0, len(themeOrder))
	for _, name := range themeOrder {
		list = append(list, AvailableThemes[name])
	}
	return list
}

// Theme middleware sets the current theme
func Theme(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get theme from cookie or config
		themeName := c.Cookies("cpm_theme", cfg.Theme)

		// Validate theme exists
		theme, ok := AvailableThemes[themeName]
		if !ok {
			theme = AvailableThemes["classic"]
			themeName = "classic"
		}

		// Store theme in locals for templates and select the template set
		c.Locals("theme", themeName)
		c.Bind(fiber.Map{views.ThemeKey: themeName})
		c.Locals("themeCSS", theme.CSSPath)
		c.Locals("themeName", theme.DisplayName)

		return c.Next()
	}
}

// SetTheme sets the theme cookie
func SetTheme(c *fiber.Ctx, themeName string) {
	c.Cookie(&fiber.Cookie{
		Name:     "cpm_theme",
		Value:    themeName,
		MaxAge:   86400 * 365, // 1 year
		HTTPOnly: true,
		SameSite: "Lax",
	})
}
