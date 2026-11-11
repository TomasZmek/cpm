package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/TomasZmek/cpm/internal/config"
	"github.com/TomasZmek/cpm/internal/handlers"
	"github.com/TomasZmek/cpm/internal/i18n"
	"github.com/TomasZmek/cpm/internal/inventory"
	"github.com/TomasZmek/cpm/internal/middleware"
	"github.com/TomasZmek/cpm/internal/models"
	"github.com/TomasZmek/cpm/internal/modules"
	"github.com/TomasZmek/cpm/internal/modules/dockerlocal"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/TomasZmek/cpm/internal/views"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
)

const (
	Version   = "3.7.0"
	BuildDate = "2026-11-10"
)

func main() {
	// Banner
	printBanner()

	// Load configuration
	cfg, err := config.Load(Version, BuildDate)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize i18n
	if err := i18n.Init(); err != nil {
		log.Fatalf("Failed to initialize i18n: %v", err)
	}

	// Initialize services
	dockerService := services.NewDockerService(cfg.ContainerName)
	caddyService := services.NewCaddyService(cfg, dockerService)
	certService := services.NewCertificateService(cfg.DataDir)
	snippetsService := services.NewSnippetsService(cfg)
	authService := services.NewAuthService(cfg.ConfigDir)
	backupService := services.NewBackupService(cfg)
	wildcardService := services.NewWildcardService(cfg.ConfigDir)
	settingsService := services.NewSettingsService(cfg.ConfigDir)

	// Link wildcard service to snippets service for combined config generation
	snippetsService.SetWildcardService(wildcardService)

	// Initialize CaddyfileManager for wildcard block management
	caddyfileManager := services.NewCaddyfileManager(cfg, wildcardService, snippetsService)
	caddyService.SetCaddyfileManager(caddyfileManager)
	caddyfileManager.SetAccessLogSettings(func() models.AccessLogSettings {
		st, _ := settingsService.Get()
		return st.AccessLog
	})

	// Ensure directory structure exists
	if err := caddyfileManager.EnsureDirectoryStructure(); err != nil {
		log.Printf("Warning: Failed to create directory structure: %v", err)
	}

	// Auto-register the local machine as a Docker discovery host when a local
	// Docker daemon is reachable. Runs in the background so a slow/unreachable
	// Docker socket never delays startup.
	go func() {
		if added, err := settingsService.EnsureLocalDockerHost(dockerService); err != nil {
			log.Printf("Local Docker auto-detect skipped: %v", err)
		} else if added {
			log.Printf("Local Docker host auto-registered for discovery")
		}
	}()

	// Modules: built-in container source for the local Docker daemon; modules
	// compiled in via modules.go register themselves from init(). Which ones
	// are active is decided at runtime in Settings → Modules (modules.json).
	dockerlocal.Register(dockerService, func() (string, string) {
		if st, err := settingsService.Get(); err == nil {
			for _, dh := range st.DiscoveryHosts {
				if dh.IsLocalDocker {
					return dh.IP, dh.Label
				}
			}
		}
		return "", ""
	})
	moduleManager, err := modules.NewManager(cfg.ConfigDir)
	if err != nil {
		log.Printf("Warning: %v (modules disabled until fixed in Settings → Modules)", err)
	}
	inv := inventory.New(moduleManager, func() []*models.Site {
		sites, _ := caddyService.GetAllSites()
		return sites
	})
	inv.Start(context.Background())

	// Initialize template engines, one per UI theme (templates/themes/<name>)
	engine := views.New("./templates/themes", middleware.ThemeNames(), "classic", map[string]interface{}{
		"t":       i18n.T,
		"tn":      i18n.TN,
		"timeAgo": services.TimeAgo,
		"contains": func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		},
		"join":    strings.Join,
		"replace": strings.ReplaceAll,
		"sub":     func(a, b int) int { return a - b },
		"eq":      func(a, b interface{}) bool { return a == b },
	}, true) // reload templates from disk (development convenience)

	app := newApp(cfg, engine)

	// Basic hardening headers for the management UI itself
	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "same-origin")
		return c.Next()
	})

	// Static files
	app.Get("/static*", static.New("./web/static"))

	// Custom middleware (theme and language are set up in newApp)
	app.Use(func(c fiber.Ctx) error {
		c.Locals("version", cfg.Version)
		return c.Next()
	})

	// Initialize handlers
	h := handlers.New(
		cfg,
		caddyService,
		certService,
		snippetsService,
		authService,
		backupService,
		dockerService,
		wildcardService,
		settingsService,
	)

	h.SetModules(moduleManager, inv)
	h.SetAccessLog(services.NewAccessLogService(cfg.DataDir))

	// Setup routes
	setupRoutes(app, h, authService)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("CPM v%s starting on http://0.0.0.0%s", Version, addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func setupRoutes(app *fiber.App, h *handlers.Handler, authService *services.AuthService) {
	// Public routes
	app.Get("/login", h.LoginPage)
	app.Post("/login", h.Login)
	app.Post("/logout", h.Logout)

	// Health check (no auth)
	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "version": Version})
	})

	// Protected routes
	protected := app.Group("", middleware.Auth(authService))

	// Permission levels (see models.User.HasPermission):
	//   view  - every authenticated user (read-only pages)
	//   edit  - Editor and Admin (proxy rules, snippets, certificates, reload)
	//   admin - Admin only (users, auth, backups, import/export, wildcard SSL)
	edit := middleware.RequirePermission(authService, "edit")
	admin := middleware.RequirePermission(authService, "admin")

	// Dashboard
	protected.Get("/", h.Dashboard)

	// Sites
	protected.Get("/sites", h.SitesList)
	protected.Get("/sites/new", edit, h.SiteNew)
	protected.Post("/sites/import-preview", admin, h.SitesImportPreview)
	protected.Post("/sites/import", admin, h.SitesImport)
	protected.Post("/sites", edit, h.SiteCreate)
	protected.Get("/sites/:id", h.SiteDetail)
	protected.Get("/sites/:id/edit", edit, h.SiteEdit)
	protected.Post("/sites/:id", edit, h.SiteUpdate)
	protected.Post("/sites/:id/delete", edit, h.SiteDelete)

	// HTMX partials for sites
	protected.Get("/htmx/sites/list", h.HTMXSitesList)
	protected.Get("/htmx/sites/status", h.SitesStatus)
	protected.Get("/htmx/sites/:id/preview", h.HTMXSitePreview)

	// Snippets
	protected.Get("/snippets", h.SnippetsList)
	protected.Post("/snippets/:name", edit, h.SnippetUpdate)

	// Certificates
	protected.Get("/certificates", h.CertificatesList)
	protected.Post("/certificates/:domain/delete", edit, h.CertificateDelete)
	protected.Post("/certificates/renew-all", edit, h.CertificatesRenewAll)
	protected.Post("/certificates/:domain/renew", edit, h.CertificateRenew)
	protected.Post("/certificates/:domain/renew-step", edit, h.CertificateRenewStep)
	protected.Post("/certificates/restart-caddy", edit, h.CertificatesRestartCaddy)
	protected.Post("/certificates/backups/:id/restore", edit, h.CertificateBackupRestore)
	protected.Post("/certificates/backups/:id/delete", edit, h.CertificateBackupDelete)
	protected.Get("/certificates/count", h.CertificatesCount)

	// Logs
	protected.Get("/logs", h.LogsPage)
	protected.Get("/htmx/logs/stream", h.HTMXLogsStream)

	// Settings
	protected.Get("/settings", h.SettingsPage)
	protected.Get("/settings/general", h.SettingsGeneral)
	protected.Get("/settings/backup", admin, h.SettingsBackup)
	protected.Get("/settings/caddy", h.SettingsCaddy)
	protected.Get("/settings/users", admin, h.SettingsUsers)
	protected.Post("/settings/backup/create", admin, h.BackupCreate)
	protected.Post("/settings/backup/restore", admin, h.BackupRestore)
	protected.Post("/settings/import", admin, h.ImportRules)
	protected.Get("/settings/export", admin, h.ExportRules)
	protected.Post("/settings/users", admin, h.UserCreate)
	protected.Post("/settings/users/:username/delete", admin, h.UserDelete)
	protected.Post("/settings/users/:username/role", admin, h.UserUpdateRole)
	protected.Post("/settings/users/:username/password", admin, h.UserUpdatePassword)
	protected.Post("/settings/auth/toggle", admin, h.ToggleAuth)

	// Docker Auto-Discovery
	protected.Get("/discovery", h.DiscoveryPage)
	protected.Post("/discovery/create", edit, h.DiscoveryCreate)
	protected.Get("/settings/docker", admin, h.SettingsDocker)
	protected.Get("/settings/modules", admin, h.SettingsModules)
	protected.Get("/settings/logs", admin, h.SettingsLogs)
	protected.Post("/settings/logs", admin, h.AccessLogSave)
	protected.Post("/settings/modules/:id", admin, h.ModuleSave)
	protected.Post("/settings/modules/:id/disable", admin, h.ModuleDisable)
	protected.Post("/settings/fallback", admin, h.FallbackSave)
	protected.Post("/settings/fallback/create", admin, h.FallbackCreate)
	protected.Post("/settings/error-page/:code", admin, h.ErrorPageSave)
	protected.Post("/settings/discovery-hosts", admin, h.SettingsDiscoveryHostsSave)
	protected.Get("/settings/discovery-detect", admin, h.SettingsDiscoveryDetect)

	// Wildcard SSL
	protected.Get("/settings/wildcard", admin, h.WildcardSettings)
	protected.Post("/settings/wildcard", admin, h.WildcardAdd)
	protected.Get("/settings/wildcard/migrate/:domain", admin, h.WildcardMigratePage)
	protected.Post("/settings/wildcard/migrate/:domain", admin, h.WildcardMigrateExecute)
	protected.Post("/settings/wildcard/:domain/delete", admin, h.WildcardDelete)

	// Caddy actions
	protected.Post("/caddy/reload", edit, h.CaddyReload)
	protected.Post("/caddy/reload-force", edit, h.CaddyReloadForce)
	protected.Post("/caddy/restart", edit, h.CaddyRestart)
	protected.Post("/caddy/validate", edit, h.CaddyValidate)

	// API v1 — uses the same session authentication as the UI
	api := app.Group("/api/v1", middleware.Auth(authService))
	api.Get("/sites", h.APISites)
	api.Get("/status", h.APIStatus)
	api.Post("/reload", edit, h.APIReload)
}

func printBanner() {
	banner := `
    ╔═══════════╗
◄───╢    CPM    ╟───►
    ╚═══════════╝
   PROXY MANAGER
   
Caddy Proxy Manager v%s
Build: %s
`
	fmt.Printf(banner, Version, BuildDate)
}

// newApp creates the Fiber app with the security-relevant configuration and
// global middleware: proxy trust, body limit, compression, logging and CSRF.
func newApp(cfg *config.Config, engine fiber.Views) *fiber.App {
	// Behind a reverse proxy (Caddy terminates TLS and forwards plain HTTP)
	// the scheme and host the browser used are only known from the
	// X-Forwarded-* headers. They are honoured for TRUSTED_PROXIES (default:
	// private and loopback addresses, where Caddy connects from); the CSRF
	// origin check compares the browser's Origin against them.
	trustProxy, proxies, trustPrivate, trustLoopback := cfg.ProxyTrust()

	app := fiber.New(fiber.Config{
		AppName:      "CPM - Caddy Proxy Manager",
		ServerHeader: "CPM",
		ErrorHandler: handlers.ErrorHandler,
		Views:        engine,
		// Values returned by Ctx (FormValue, Params, Cookies, ...) are copies,
		// not views into the request buffer. Services keep some of them beyond
		// the request (e.g. the username in a session); without this, a later
		// request reusing the buffer silently changed them and logged users out.
		Immutable: true,
		// Backup restore and rule import upload files; multipart bodies are
		// parsed within this limit (Fiber v3 no longer pre-parses them).
		BodyLimit:  64 << 20,
		TrustProxy: trustProxy,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies:  proxies,
			Private:  trustPrivate,
			Loopback: trustLoopback,
		},
		ProxyHeader: fiber.HeaderXForwardedFor,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(compress.New())

	// Theme and language before CSRF, so its error page is rendered in the
	// user's theme and language.
	app.Use(middleware.Theme(cfg))
	app.Use(middleware.I18n())

	// CSRF protection (double-submit cookie): the token comes from the form
	// field "_csrf" or the "X-CSRF-Token" header (HTMX/fetch). Applies to the
	// API too, since it is authenticated by the session cookie. Unsafe
	// requests must also come from the page's own origin (Origin/Referer).
	// The cookie name changed with Fiber v3 (new token format), so tokens of
	// older versions are simply ignored.
	app.Use(csrf.New(csrf.Config{
		IdleTimeout:    24 * time.Hour,
		CookieName:     "cpm_csrf_v3",
		CookieSameSite: "Lax",
		CookieHTTPOnly: false,
		Extractor:      extractors.Chain(extractors.FromForm("_csrf"), extractors.FromHeader("X-CSRF-Token")),
		TrustedOrigins: cfg.TrustedOrigins,
		ErrorHandler:   handlers.CSRFError,
	}))

	return app
}
