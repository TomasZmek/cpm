package middleware

import (
	"github.com/TomasZmek/cpm/internal/i18n"
	"github.com/gofiber/fiber/v2"
)

// I18n middleware detects the user's language and stores it in c.Locals("lang").
func I18n() fiber.Handler {
	return func(c *fiber.Ctx) error {
		lang := c.Cookies("cpm_lang", "")

		if lang == "" {
			acceptLang := c.Get("Accept-Language")
			if len(acceptLang) >= 2 {
				lang = acceptLang[:2]
			}
		}

		if !i18n.IsValidLanguage(lang) {
			lang = "en"
		}

		c.Locals("lang", lang)
		return c.Next()
	}
}

// SetLanguage sets the language cookie for the given request context.
func SetLanguage(c *fiber.Ctx, lang string) {
	if !i18n.IsValidLanguage(lang) {
		lang = "en"
	}

	c.Cookie(&fiber.Cookie{
		Name:     "cpm_lang",
		Value:    lang,
		MaxAge:   86400 * 365,
		HTTPOnly: true,
		SameSite: "Lax",
	})
}
