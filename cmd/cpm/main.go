package main

import (
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
	"github.com/TomasZmek/cpm/internal/middleware"
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
)

const (
	Version   = "3.3.1"
	BuildDate = "2026-06-17"

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

	// Initialize template engine
	engine := html.New("./templates/themes/classic", ".html")
	engine.AddFunc("t", i18n.T)
	engine.AddFunc("tn", i18n.TN)
	engine.AddFunc("timeAgo", services.TimeAgo)
	engine.AddFunc("contains", func(slice []string, item string) bool {
		for _, s := range slice {
			if s == item {
				return true
			}
		}
		return false
	})
	engine.AddFunc("join", strings.Join)
	engine.AddFunc("replace", strings.ReplaceAll)
	engine.AddFunc("sub", func(a, b int) int { return a - b })
	engine.AddFunc("eq", func(a, b interface{}) bool { return a == b })

	// Reload templates in development
	engine.Reload(true)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "CPM - Caddy Proxy Manager",
		ServerHeader: "CPM",
		ErrorHandler: handlers.ErrorHandler,
		Views:        engine,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(compress.New())

	// CSRF protection — accepts token from form field "_csrf" or header "X-CSRF-Token".
	// API routes (/api/*) are excluded as they use token-based auth.
	app.Use(csrf.New(csrf.Config{
		Expiration:     24 * time.Hour,
		CookieName:     "cpm_csrf",
		CookieSameSite: "Lax",
		CookieHTTPOnly: false,
		ContextKey:     "csrf_token",
		Extractor: func(c *fiber.Ctx) (string, error) {
			if token := c.FormValue("_csrf"); token != "" {
				return token, nil
			}
			if token := c.Get("X-CSRF-Token"); token != "" {
				return token, nil
			}
			return "", csrf.ErrTokenNotFound
		},
		Next: func(c *fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/api/")
		},
	}))

	// Static files
	app.Static("/static", "./web/static")

	// Custom middleware
	app.Use(middleware.Theme(cfg))
	app.Use(middleware.I18n())
	app.Use(func(c *fiber.Ctx) error {
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
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "version": Version})
	})

	// Protected routes
	protected := app.Group("", middleware.Auth(authService))

	// Dashboard
	protected.Get("/", h.Dashboard)

	// Sites
	protected.Get("/sites", h.SitesList)
	protected.Get("/sites/new", h.SiteNew)
	protected.Get("/sites/import-preview", h.SitesImportPreview)
	protected.Post("/sites/import-preview", h.SitesImportPreview)
	protected.Post("/sites/import", h.SitesImport)
	protected.Post("/sites", h.SiteCreate)
	protected.Get("/sites/:id", h.SiteDetail)
	protected.Get("/sites/:id/edit", h.SiteEdit)
	protected.Post("/sites/:id", h.SiteUpdate)
	protected.Post("/sites/:id/delete", h.SiteDelete)
	protected.Post("/sites/:id/duplicate", h.SiteDuplicate)

	// HTMX partials for sites
	protected.Get("/htmx/sites/list", h.HTMXSitesList)
	protected.Get("/htmx/sites/status", h.SitesStatus)
	protected.Get("/htmx/sites/:id/card", h.HTMXSiteCard)
	protected.Get("/htmx/sites/:id/preview", h.HTMXSitePreview)

	// Snippets
	protected.Get("/snippets", h.SnippetsList)
	protected.Post("/snippets/:name", h.SnippetUpdate)
	protected.Get("/htmx/snippets/:name/form", h.HTMXSnippetForm)

	// Certificates
	protected.Get("/certificates", h.CertificatesList)
	protected.Post("/certificates/:domain/delete", h.CertificateDelete)
	protected.Post("/certificates/renew-all", h.CertificatesRenewAll)
	protected.Post("/certificates/:domain/renew", h.CertificateRenew)
	protected.Post("/certificates/:domain/renew-step", h.CertificateRenewStep)
	protected.Get("/certificates/count", h.CertificatesCount)
	protected.Get("/htmx/certificates/list", h.HTMXCertificatesList)

	// Logs
	protected.Get("/logs", h.LogsPage)
	protected.Get("/htmx/logs/stream", h.HTMXLogsStream)

	// Settings
	protected.Get("/settings", h.SettingsPage)
	protected.Get("/settings/general", h.SettingsGeneral)
	protected.Get("/settings/backup", h.SettingsBackup)
	protected.Get("/settings/caddy", h.SettingsCaddy)
	protected.Get("/settings/users", h.SettingsUsers)
	protected.Post("/settings/backup/create", h.BackupCreate)
	protected.Post("/settings/backup/restore", h.BackupRestore)
	protected.Post("/settings/import", h.ImportRules)
	protected.Get("/settings/export", h.ExportRules)
	protected.Post("/settings/users", h.UserCreate)
	protected.Post("/settings/users/:username/delete", h.UserDelete)
	protected.Post("/settings/users/:username/role", h.UserUpdateRole)
	protected.Post("/settings/users/:username/password", h.UserUpdatePassword)
	protected.Post("/settings/auth/toggle", h.ToggleAuth)

	// Docker Auto-Discovery
	protected.Get("/discovery", h.DiscoveryPage)
	protected.Post("/discovery/create", h.DiscoveryCreate)
	protected.Get("/settings/docker", h.SettingsDocker)
	protected.Post("/settings/fallback", h.FallbackSave)
	protected.Post("/settings/fallback/create", h.FallbackCreate)
	protected.Post("/settings/error-page/:code", h.ErrorPageSave)
	protected.Post("/settings/discovery-hosts", h.SettingsDiscoveryHostsSave)
	protected.Get("/settings/discovery-detect", h.SettingsDiscoveryDetect)

	// Wildcard SSL
	protected.Get("/settings/wildcard", h.WildcardSettings)
	protected.Post("/settings/wildcard", h.WildcardAdd)
	protected.Get("/settings/wildcard/migrate/:domain", h.WildcardMigratePage)
	protected.Post("/settings/wildcard/migrate/:domain", h.WildcardMigrateExecute)
	protected.Post("/settings/wildcard/:domain/delete", h.WildcardDelete)

	// Caddy actions
	protected.Post("/caddy/reload", h.CaddyReload)
	protected.Post("/caddy/reload-force", h.CaddyReloadForce)
	protected.Post("/caddy/validate", h.CaddyValidate)

	// API v1
	api := app.Group("/api/v1")
	api.Get("/sites", h.APISites)
	api.Get("/status", h.APIStatus)
	api.Post("/reload", h.APIReload)
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
