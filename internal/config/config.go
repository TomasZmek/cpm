package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration
type Config struct {
	// Server settings
	Port int
	Host string

	// Paths
	ConfigDir string // /caddy-config
	DataDir   string // /caddy-data
	SitesDir  string // /caddy-config/sites

	// Docker
	ContainerName string

	// Default values
	DefaultIP string

	// Theme
	Theme string

	// Reverse proxy in front of CPM (e.g. Caddy). TrustedProxies lists who may
	// set X-Forwarded-* headers: "private", "loopback", IPs or CIDRs, comma
	// separated; "none" disables it. TrustedOrigins are extra origins
	// accepted by the CSRF origin check (e.g. https://cpm.example.com).
	TrustedProxies string
	TrustedOrigins []string

	// App info
	Version   string
	BuildDate string
}

// Load loads configuration from environment variables
func Load(version, buildDate string) (*Config, error) {
	cfg := &Config{
		Port:           getEnvInt("PORT", 8501),
		Host:           getEnv("HOST", "0.0.0.0"),
		ConfigDir:      getEnv("CADDY_CONFIG_PATH", "/caddy-config"),
		DataDir:        getEnv("CADDY_DATA_PATH", "/caddy-data"),
		ContainerName:  getEnv("CONTAINER_NAME", "caddy"),
		DefaultIP:      getEnv("DEFAULT_IP", "192.168.1.1"),
		Theme:          getEnv("THEME", "classic"),
		TrustedProxies: getEnv("TRUSTED_PROXIES", "private,loopback"),
		TrustedOrigins: splitList(os.Getenv("TRUSTED_ORIGINS")),
		Version:        version,
		BuildDate:      buildDate,
	}

	// Derived paths
	cfg.SitesDir = cfg.ConfigDir + "/sites"

	return cfg, nil
}

// ProxyTrust parses TrustedProxies: whether to trust a proxy at all, the
// explicit IPs/CIDRs, and whether all private / loopback ranges are trusted.
func (c *Config) ProxyTrust() (enabled bool, proxies []string, private, loopback bool) {
	for _, v := range splitList(c.TrustedProxies) {
		switch strings.ToLower(v) {
		case "none", "off", "false":
			return false, nil, false, false
		case "private":
			private = true
		case "loopback":
			loopback = true
		default:
			proxies = append(proxies, v)
		}
	}
	return private || loopback || len(proxies) > 0, proxies, private, loopback
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// getEnv returns environment variable or default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns environment variable as int or default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool returns environment variable as bool or default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
