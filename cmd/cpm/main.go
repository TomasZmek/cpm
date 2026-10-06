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
	Version   = "3.1.3"
	BuildDate = "2026-05-30"
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

	// Link wildcard service to snippets service for combined config generation
	snippetsService.SetWildcardService(wildcardService)

	// Initialize CaddyfileManager for wildcard block management
	caddyfileManager := services.NewCaddyfileManager(cfg, wildcardService, snippetsService)
	caddyService.SetCaddyfileManager(caddyfileManager)

	// Ensure directory structure exists
	if err := caddyfileManager.EnsureDirectoryStructure(); err != nil {
		log.Printf("Warning: Failed to create directory structure: %v", err)
	}

	// Initialize template engine
	engine := html.New("./templates/themes/classic", ".html")
	engine.AddFunc("t", i18n.T)
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
	// Applies to API routes as well, since they are authenticated by the session cookie.
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
	}))

	// Basic hardening headers for the management UI itself
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "same-origin")
		return c.Next()
	})

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
	protected.Post("/sites", edit, h.SiteCreate)
	protected.Get("/sites/:id", h.SiteDetail)
	protected.Get("/sites/:id/edit", edit, h.SiteEdit)
	protected.Post("/sites/:id", edit, h.SiteUpdate)
	protected.Post("/sites/:id/delete", edit, h.SiteDelete)
	protected.Post("/sites/:id/duplicate", edit, h.SiteDuplicate)

	// HTMX partials for sites
	protected.Get("/htmx/sites/list", h.HTMXSitesList)
	protected.Get("/htmx/sites/:id/card", h.HTMXSiteCard)
	protected.Get("/htmx/sites/:id/preview", h.HTMXSitePreview)

	// Snippets
	protected.Get("/snippets", h.SnippetsList)
	protected.Post("/snippets/:name", edit, h.SnippetUpdate)
	protected.Get("/htmx/snippets/:name/form", edit, h.HTMXSnippetForm)

	// Certificates
	protected.Get("/certificates", h.CertificatesList)
	protected.Post("/certificates/:domain/delete", edit, h.CertificateDelete)
	protected.Post("/certificates/:domain/renew", edit, h.CertificateRenew)
	protected.Get("/htmx/certificates/list", h.HTMXCertificatesList)

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

	// Wildcard SSL
	protected.Get("/settings/wildcard", admin, h.WildcardSettings)
	protected.Post("/settings/wildcard", admin, h.WildcardAdd)
	protected.Get("/settings/wildcard/migrate/:domain", admin, h.WildcardMigratePage)
	protected.Post("/settings/wildcard/migrate/:domain", admin, h.WildcardMigrateExecute)
	protected.Post("/settings/wildcard/:domain/delete", admin, h.WildcardDelete)

	// Caddy actions
	protected.Post("/caddy/reload", edit, h.CaddyReload)
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
