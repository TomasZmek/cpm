package handlers

import (
	"net"
	"net/url"
	"strings"

	"github.com/TomasZmek/cpm/internal/config"
	"github.com/TomasZmek/cpm/internal/i18n"
	"github.com/TomasZmek/cpm/internal/inventory"
	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/modules"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
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
	wildcardService *services.WildcardService
	settingsService *services.SettingsService
	modules         *modules.Manager
	inventory       *inventory.Service
}

// SetModules attaches the module manager and the container inventory.
func (h *Handler) SetModules(m *modules.Manager, inv *inventory.Service) {
	h.modules = m
	h.inventory = inv
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
	wildcardService *services.WildcardService,
	settingsService *services.SettingsService,
) *Handler {
	return &Handler{
		config:          cfg,
		caddyService:    caddyService,
		certService:     certService,
		snippetsService: snippetsService,
		authService:     authService,
		backupService:   backupService,
		dockerService:   dockerService,
		wildcardService: wildcardService,
		settingsService: settingsService,
	}
}

// ErrorHandler handles errors globally
func ErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	// For HTMX requests, return error as HTML
	if c.Get("HX-Request") == "true" {
		return c.Status(code).SendString(`<div class="alert alert-error">` + escapeHTML(err.Error()) + `</div>`)
	}

	// For API requests, return JSON
	if isAPIRequest(c) {
		return c.Status(code).JSON(fiber.Map{
			"error": err.Error(),
			"code":  code,
		})
	}

	// For regular requests, show error page
	lang := "en"
	if l, ok := c.Locals("lang").(string); ok && l != "" {
		lang = l
	}

	themeCSS := "/static/css/themes/classic.css"
	if css, ok := c.Locals("themeCSS").(string); ok && css != "" {
		themeCSS = css
	}

	if rerr := c.Status(code).Render("pages/error", fiber.Map{
		"Code":     code,
		"Message":  err.Error(),
		"Title":    "Error",
		"Lang":     lang,
		"ThemeCSS": themeCSS,
		"Version":  c.Locals("version"),
	}, "layouts/base"); rerr != nil {
		// Keep the status even when the error page cannot be rendered.
		return c.Status(code).SendString(err.Error())
	}
	return nil
}

// isAPIRequest checks if request is an API request
func isAPIRequest(c fiber.Ctx) bool {
	return c.Get("Accept") == "application/json" ||
		strings.HasPrefix(c.Path(), "/api")
}

// getCurrentUser returns the current user from context
func (h *Handler) getCurrentUser(c fiber.Ctx) interface{} {
	return c.Locals("user")
}

// hasPermission reports whether the current request may perform actions
// requiring the given permission. With authentication disabled everything is allowed.
func (h *Handler) hasPermission(c fiber.Ctx, permission string) bool {
	if !h.authService.IsEnabled() {
		return true
	}
	user, ok := c.Locals("user").(*models.User)
	return ok && user != nil && user.HasPermission(permission)
}

// safeRedirectTarget returns the Referer path if it points back into this
// application (relative path), otherwise "/". Prevents open redirects.
func safeRedirectTarget(c fiber.Ctx) string {
	ref := c.Get("Referer")
	if ref == "" {
		return "/"
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "/"
	}
	// c.Host() keeps the port (c.Hostname() drops it in Fiber v3), as the
	// Referer does when CPM is opened on e.g. :8501.
	if u.Host != "" && !strings.EqualFold(u.Host, c.Host()) {
		return "/"
	}
	path := u.EscapedPath()
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return path
}

// baseData returns common template data
func (h *Handler) baseData(c fiber.Ctx, title string) fiber.Map {
	lang := "en"
	if l, ok := c.Locals("lang").(string); ok && l != "" {
		lang = l
	}

	themeCSS := "/static/css/themes/classic.css"
	if css, ok := c.Locals("themeCSS").(string); ok && css != "" {
		themeCSS = css
	}

	return fiber.Map{
		"Title":              title,
		"Lang":               lang,
		"ThemeCSS":           themeCSS,
		"Version":            h.config.Version,
		"User":               c.Locals("user"),
		"CSRFToken":          csrf.TokenFromContext(c),
		"HasContainerSource": h.hasContainerSource(),
	}
}

// hasContainerSource reports whether a module that lists containers is
// enabled. Docker discovery and its settings are hidden otherwise.
func (h *Handler) hasContainerSource() bool {
	return h.modules != nil && len(h.modules.Active()) > 0
}

// requestHostIP returns the IP address the browser used to reach CPM (Host
// header) when it is a usable LAN address — inside a container that is the
// best hint for the Docker host's IP. Host names, loopback and Docker
// bridge addresses are ignored.
func requestHostIP(c fiber.Ctx) string {
	host := c.Get(fiber.HeaderHost)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || services.IsDockerBridgeIP(ip) {
		return ""
	}
	return ip.String()
}

// getLang extracts the current language code from the request context.
func getLang(c fiber.Ctx) string {
	if lang, ok := c.Locals("lang").(string); ok && lang != "" {
		return lang
	}
	return "en"
}

// tl is a shorthand for i18n.T using the request's language.
func tl(c fiber.Ctx, key string, args ...interface{}) string {
	return i18n.T(getLang(c), key, args...)
}

// Flash messages helper
// setFlash queues a message for the next page. It uses Fiber's flash
// messages (a hex-encoded cookie, so any language is safe) and is delivered
// by the redirect that follows — always end the handler with redirectTo.
func setFlash(c fiber.Ctx, msgType, message string) {
	c.Redirect().With("type", msgType).With("message", message)
}

// getFlash returns the flash message set by the previous request.
func getFlash(c fiber.Ctx) (string, string) {
	return c.Redirect().Message("type").Value, c.Redirect().Message("message").Value
}

// redirectTo redirects to a path and delivers pending flash messages. HTMX
// requests get a 200 with HX-Redirect instead of a 303: an XHR would follow a
// redirect itself and swap the whole page into the target element.
func redirectTo(c fiber.Ctx, location string) error {
	if err := c.Redirect().To(location); err != nil {
		return err
	}
	if c.Get("HX-Request") == "true" {
		c.Response().Header.Del(fiber.HeaderLocation)
		c.Set("HX-Redirect", location)
		return c.SendStatus(fiber.StatusOK)
	}
	return nil
}
