package i18n

import (
	"fmt"
	"strings"
)

// Translations holds all translations
var translations = map[string]map[string]string{
	"en": englishTranslations,
	"cs": czechTranslations,
}

// AvailableLanguages returns all available languages
var AvailableLanguages = map[string]string{
	"en": "English",
	"cs": "Čeština",
}

// Init initializes the i18n system
func Init() error {
	// Translations are already loaded as variables
	return nil
}

// T translates a key to the specified language
func T(lang, key string, args ...interface{}) string {
	// Get language translations
	langTranslations, ok := translations[lang]
	if !ok {
		langTranslations = translations["en"]
	}

	// Get translation
	text, ok := langTranslations[key]
	if !ok {
		// Fallback to English
		text, ok = translations["en"][key]
		if !ok {
			return key // Return key if not found
		}
	}

	// Replace placeholders
	if len(args) > 0 {
		// Support both {0}, {1} style and {name} style
		for i, arg := range args {
			placeholder := fmt.Sprintf("{%d}", i)
			text = strings.ReplaceAll(text, placeholder, fmt.Sprint(arg))
		}
	}

	return text
}

// IsValidLanguage checks if a language is supported
func IsValidLanguage(lang string) bool {
	_, ok := translations[lang]
	return ok
}

// GetLanguages returns available languages
func GetLanguages() map[string]string {
	return AvailableLanguages
}

// English translations
var englishTranslations = map[string]string{
	// General
	"app_name":    "CPM - Caddy Proxy Manager",
	"app_version": "Version",
	"save":        "Save",
	"cancel":      "Cancel",
	"delete":      "Delete",
	"edit":        "Edit",
	"create":      "Create",
	"back":        "Back",
	"search":      "Search",
	"filter":      "Filter",
	"loading":     "Loading...",
	"success":     "Success",
	"error":       "Error",
	"warning":     "Warning",
	"confirm":     "Confirm",
	"yes":         "Yes",
	"no":          "No",
	"actions":     "Actions",
	"settings":    "Settings",
	"logout":      "Logout",

	// Navigation
	"nav_dashboard":    "Dashboard",
	"nav_sites":        "Proxy Rules",
	"nav_snippets":     "Snippets",
	"nav_certificates": "Certificates",
	"nav_logs":         "Logs",
	"nav_settings":     "Settings",

	// Dashboard
	"dashboard_title":          "Dashboard",
	"dashboard_rules":          "Rules",
	"dashboard_certificates":   "Certificates",
	"dashboard_snippets":       "Snippets",
	"dashboard_caddy_status":   "Caddy Status",
	"dashboard_running":        "Running",
	"dashboard_stopped":        "Stopped",
	"dashboard_recent_changes": "Recent Changes",
	"dashboard_no_changes":     "No recent changes",
	"dashboard_alerts":         "Alerts",
	"dashboard_no_alerts":      "All systems operational",
	"dashboard_quick_actions":  "Quick Actions",
	"dashboard_reload":         "Reload Caddy",
	"dashboard_validate":       "Validate Config",
	"dashboard_new_rule":       "New Rule",
	"dashboard_backup":         "Backup",

	// Sites
	"sites_title":            "Proxy Rules",
	"sites_new":              "New Rule",
	"sites_edit":             "Edit Rule",
	"sites_delete":           "Delete Rule",
	"sites_duplicate":        "Duplicate Rule",
	"sites_empty":            "No proxy rules found",
	"sites_search":           "Search by domain, IP, or port...",
	"sites_filter_tag":       "Filter by tag",
	"sites_all_tags":         "All tags",
	"sites_domain":           "Domain(s)",
	"sites_target":           "Target",
	"sites_port":             "Port",
	"sites_ip":               "IP Address",
	"sites_https_backend":    "HTTPS Backend",
	"sites_internal":         "Internal Only",
	"sites_websocket":        "WebSocket Support",
	"sites_health_check":     "Health Check Path",
	"sites_timeout":          "Timeout (seconds)",
	"sites_snippets":         "Snippets",
	"sites_tags":             "Tags",
	"sites_extra_config":     "Extra Configuration",
	"sites_raw_edit":         "Raw Edit",
	"sites_form_edit":        "Form Edit",
	"sites_preview":          "Preview",
	"sites_confirm_delete":   "Are you sure you want to delete this rule?",
	"sites_created":          "Rule created successfully",
	"sites_updated":          "Rule updated successfully",
	"sites_deleted":          "Rule deleted successfully",
	"sites_duplicated":       "Rule duplicated successfully",
	"sites_from_template":    "From Template",
	"sites_from_scratch":     "From Scratch",
	"sites_select_template":  "Select Template",
	"sites_template_category": "Category",

	// Snippets
	"snippets_title":            "Snippets Manager",
	"snippets_description":      "Configure shared snippets used across your proxy rules",
	"snippets_cloudflare":       "Cloudflare DNS",
	"snippets_internal":         "Internal Only",
	"snippets_security":         "Security Headers",
	"snippets_compression":      "Compression",
	"snippets_rate_limit":       "Rate Limiting",
	"snippets_basic_auth":       "Basic Authentication",
	"snippets_enabled":          "Enabled",
	"snippets_disabled":         "Disabled",
	"snippets_use_env":          "Use Environment Variable",
	"snippets_api_token":        "API Token",
	"snippets_allowed_networks": "Allowed Networks (CIDR)",
	"snippets_hsts_max_age":     "HSTS Max Age",
	"snippets_x_frame_options":  "X-Frame-Options",
	"snippets_referrer_policy":  "Referrer Policy",
	"snippets_hide_server":      "Hide Server Header",
	"snippets_zstd":             "Zstd",
	"snippets_gzip":             "Gzip",
	"snippets_requests":         "Requests",
	"snippets_window":           "Window (seconds)",
	"snippets_users":            "Users",
	"snippets_add_user":         "Add User",
	"snippets_username":         "Username",
	"snippets_password":         "Password",

	// Certificates
	"certs_title":          "SSL Certificates",
	"certs_domain":         "Domain",
	"certs_issuer":         "Issuer",
	"certs_expires":        "Expires",
	"certs_status":         "Status",
	"certs_days_left":      "Days Left",
	"certs_valid":          "Valid",
	"certs_expiring":       "Expiring Soon",
	"certs_critical":       "Critical",
	"certs_expired":        "Expired",
	"certs_delete":         "Force Renewal",
	"certs_confirm_delete": "Delete this certificate to force renewal?",
	"certs_empty":          "No certificates found",

	// Logs
	"logs_title":   "Caddy Logs",
	"logs_lines":   "Lines to show",
	"logs_refresh": "Refresh",
	"logs_stream":  "Live Stream",
	"logs_stop":    "Stop Stream",
	"logs_empty":   "No logs available",

	// Settings
	"settings_title":         "Settings",
	"settings_general":       "General",
	"settings_backup":        "Backup",
	"settings_caddy":         "Caddy",
	"settings_users":         "Users",
	"settings_language":      "Language",
	"settings_theme":         "Theme",
	"settings_backup_create": "Create Backup",
	"settings_backup_restore": "Restore Backup",
	"settings_import":        "Import Rules",
	"settings_export":        "Export Rules",
	"settings_fallback":      "Fallback Rule",
	"settings_error_pages":   "Error Pages",
	"settings_auth_enabled":  "Authentication Enabled",
	"settings_auth_disabled": "Authentication Disabled",
	"settings_add_user":      "Add User",
	"settings_role":          "Role",
	"settings_role_admin":    "Admin",
	"settings_role_editor":   "Editor",
	"settings_role_viewer":   "Viewer",

	// Login
	"login_title":    "Login",
	"login_username": "Username",
	"login_password": "Password",
	"login_submit":   "Login",
	"login_setup":    "Create Admin Account",
	"login_error":    "Invalid credentials",

	// Messages
	"msg_reload_success":   "Configuration reloaded successfully",
	"msg_reload_error":     "Failed to reload configuration",
	"msg_validate_success": "Configuration is valid",
	"msg_validate_error":   "Configuration validation failed",
	"msg_backup_created":   "Backup created successfully",
	"msg_backup_restored":  "Backup restored successfully",
	"msg_import_success":   "Rules imported successfully",
	"msg_user_created":     "User created successfully",
	"msg_user_deleted":     "User deleted successfully",
}

// Czech translations
var czechTranslations = map[string]string{
	// General
	"app_name":    "CPM - Caddy Proxy Manager",
	"app_version": "Verze",
	"save":        "Uložit",
	"cancel":      "Zrušit",
	"delete":      "Smazat",
	"edit":        "Upravit",
	"create":      "Vytvořit",
	"back":        "Zpět",
	"search":      "Hledat",
	"filter":      "Filtr",
	"loading":     "Načítání...",
	"success":     "Úspěch",
	"error":       "Chyba",
	"warning":     "Varování",
	"confirm":     "Potvrdit",
	"yes":         "Ano",
	"no":          "Ne",
	"actions":     "Akce",
	"settings":    "Nastavení",
	"logout":      "Odhlásit",

	// Navigation
	"nav_dashboard":    "Dashboard",
	"nav_sites":        "Proxy pravidla",
	"nav_snippets":     "Snippety",
	"nav_certificates": "Certifikáty",
	"nav_logs":         "Logy",
	"nav_settings":     "Nastavení",

	// Dashboard
	"dashboard_title":          "Dashboard",
	"dashboard_rules":          "Pravidel",
	"dashboard_certificates":   "Certifikátů",
	"dashboard_snippets":       "Snippetů",
	"dashboard_caddy_status":   "Stav Caddy",
	"dashboard_running":        "Běží",
	"dashboard_stopped":        "Zastaveno",
	"dashboard_recent_changes": "Poslední změny",
	"dashboard_no_changes":     "Žádné nedávné změny",
	"dashboard_alerts":         "Upozornění",
	"dashboard_no_alerts":      "Vše v pořádku",
	"dashboard_quick_actions":  "Rychlé akce",
	"dashboard_reload":         "Reload Caddy",
	"dashboard_validate":       "Validovat config",
	"dashboard_new_rule":       "Nové pravidlo",
	"dashboard_backup":         "Záloha",

	// Sites
	"sites_title":            "Proxy pravidla",
	"sites_new":              "Nové pravidlo",
	"sites_edit":             "Upravit pravidlo",
	"sites_delete":           "Smazat pravidlo",
	"sites_duplicate":        "Duplikovat pravidlo",
	"sites_empty":            "Žádná pravidla nenalezena",
	"sites_search":           "Hledat podle domény, IP nebo portu...",
	"sites_filter_tag":       "Filtrovat podle štítku",
	"sites_all_tags":         "Všechny štítky",
	"sites_domain":           "Doména(y)",
	"sites_target":           "Cíl",
	"sites_port":             "Port",
	"sites_ip":               "IP adresa",
	"sites_https_backend":    "HTTPS backend",
	"sites_internal":         "Pouze interní",
	"sites_websocket":        "WebSocket podpora",
	"sites_health_check":     "Health check cesta",
	"sites_timeout":          "Timeout (sekundy)",
	"sites_snippets":         "Snippety",
	"sites_tags":             "Štítky",
	"sites_extra_config":     "Extra konfigurace",
	"sites_raw_edit":         "Raw editace",
	"sites_form_edit":        "Formulář",
	"sites_preview":          "Náhled",
	"sites_confirm_delete":   "Opravdu chcete smazat toto pravidlo?",
	"sites_created":          "Pravidlo úspěšně vytvořeno",
	"sites_updated":          "Pravidlo úspěšně aktualizováno",
	"sites_deleted":          "Pravidlo úspěšně smazáno",
	"sites_duplicated":       "Pravidlo úspěšně zduplikováno",
	"sites_from_template":    "Ze šablony",
	"sites_from_scratch":     "Od začátku",
	"sites_select_template":  "Vybrat šablonu",
	"sites_template_category": "Kategorie",

	// Snippets
	"snippets_title":            "Správce snippetů",
	"snippets_description":      "Konfigurace sdílených snippetů používaných v proxy pravidlech",
	"snippets_cloudflare":       "Cloudflare DNS",
	"snippets_internal":         "Pouze interní",
	"snippets_security":         "Bezpečnostní hlavičky",
	"snippets_compression":      "Komprese",
	"snippets_rate_limit":       "Rate limiting",
	"snippets_basic_auth":       "Basic autentizace",
	"snippets_enabled":          "Zapnuto",
	"snippets_disabled":         "Vypnuto",
	"snippets_use_env":          "Použít proměnnou prostředí",
	"snippets_api_token":        "API token",
	"snippets_allowed_networks": "Povolené sítě (CIDR)",
	"snippets_hsts_max_age":     "HSTS Max Age",
	"snippets_x_frame_options":  "X-Frame-Options",
	"snippets_referrer_policy":  "Referrer Policy",
	"snippets_hide_server":      "Skrýt Server hlavičku",
	"snippets_zstd":             "Zstd",
	"snippets_gzip":             "Gzip",
	"snippets_requests":         "Požadavků",
	"snippets_window":           "Okno (sekundy)",
	"snippets_users":            "Uživatelé",
	"snippets_add_user":         "Přidat uživatele",
	"snippets_username":         "Uživatelské jméno",
	"snippets_password":         "Heslo",

	// Certificates
	"certs_title":          "SSL certifikáty",
	"certs_domain":         "Doména",
	"certs_issuer":         "Vydavatel",
	"certs_expires":        "Vyprší",
	"certs_status":         "Stav",
	"certs_days_left":      "Zbývá dní",
	"certs_valid":          "Platný",
	"certs_expiring":       "Brzy vyprší",
	"certs_critical":       "Kritické",
	"certs_expired":        "Vypršel",
	"certs_delete":         "Vynutit obnovu",
	"certs_confirm_delete": "Smazat certifikát pro vynucení obnovy?",
	"certs_empty":          "Žádné certifikáty nenalezeny",

	// Logs
	"logs_title":   "Caddy logy",
	"logs_lines":   "Počet řádků",
	"logs_refresh": "Obnovit",
	"logs_stream":  "Živě",
	"logs_stop":    "Zastavit",
	"logs_empty":   "Žádné logy k dispozici",

	// Settings
	"settings_title":         "Nastavení",
	"settings_general":       "Obecné",
	"settings_backup":        "Zálohy",
	"settings_caddy":         "Caddy",
	"settings_users":         "Uživatelé",
	"settings_language":      "Jazyk",
	"settings_theme":         "Téma",
	"settings_backup_create": "Vytvořit zálohu",
	"settings_backup_restore": "Obnovit zálohu",
	"settings_import":        "Import pravidel",
	"settings_export":        "Export pravidel",
	"settings_fallback":      "Fallback pravidlo",
	"settings_error_pages":   "Chybové stránky",
	"settings_auth_enabled":  "Autentizace zapnuta",
	"settings_auth_disabled": "Autentizace vypnuta",
	"settings_add_user":      "Přidat uživatele",
	"settings_role":          "Role",
	"settings_role_admin":    "Administrátor",
	"settings_role_editor":   "Editor",
	"settings_role_viewer":   "Čtenář",

	// Login
	"login_title":    "Přihlášení",
	"login_username": "Uživatelské jméno",
	"login_password": "Heslo",
	"login_submit":   "Přihlásit",
	"login_setup":    "Vytvořit administrátorský účet",
	"login_error":    "Neplatné přihlašovací údaje",

	// Messages
	"msg_reload_success":   "Konfigurace úspěšně načtena",
	"msg_reload_error":     "Nepodařilo se načíst konfiguraci",
	"msg_validate_success": "Konfigurace je platná",
	"msg_validate_error":   "Validace konfigurace selhala",
	"msg_backup_created":   "Záloha úspěšně vytvořena",
	"msg_backup_restored":  "Záloha úspěšně obnovena",
	"msg_import_success":   "Pravidla úspěšně importována",
	"msg_user_created":     "Uživatel úspěšně vytvořen",
	"msg_user_deleted":     "Uživatel úspěšně smazán",
}
