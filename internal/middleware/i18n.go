package middleware

import (
	"github.com/TomasZmek/cpm/internal/i18n"
	"github.com/gofiber/fiber/v2"
)

// I18n middleware sets the current language
func I18n() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get language from cookie or Accept-Language header
		lang := c.Cookies("cpm_lang", "")

		if lang == "" {
			// Try to detect from Accept-Language header
			acceptLang := c.Get("Accept-Language")
			if len(acceptLang) >= 2 {
				lang = acceptLang[:2]
			}
		}

		// Validate language
		if !i18n.IsValidLanguage(lang) {
			lang = "en" // default
		}

		// Store language in locals
		c.Locals("lang", lang)

		// Create translation function for this request
		t := func(key string, args ...interface{}) string {
			return i18n.T(lang, key, args...)
		}
		c.Locals("t", t)

		return c.Next()
	}
}

// SetLanguage sets the language cookie
func SetLanguage(c *fiber.Ctx, lang string) {
	if !i18n.IsValidLanguage(lang) {
		lang = "en"
	}

	c.Cookie(&fiber.Cookie{
		Name:     "cpm_lang",
		Value:    lang,
		MaxAge:   86400 * 365, // 1 year
		HTTPOnly: true,
		SameSite: "Lax",
	})
}
