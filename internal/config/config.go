package config

import (
	"os"
	"strconv"
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

	// App info
	Version   string
	BuildDate string
}

// Load loads configuration from environment variables
func Load(version, buildDate string) (*Config, error) {
	cfg := &Config{
		Port:          getEnvInt("PORT", 8501),
		Host:          getEnv("HOST", "0.0.0.0"),
		ConfigDir:     getEnv("CADDY_CONFIG_PATH", "/caddy-config"),
		DataDir:       getEnv("CADDY_DATA_PATH", "/caddy-data"),
		ContainerName: getEnv("CONTAINER_NAME", "caddy"),
		DefaultIP:     getEnv("DEFAULT_IP", "192.168.1.1"),
		Theme:         getEnv("THEME", "classic"),
		Version:       version,
		BuildDate:     buildDate,
	}

	// Derived paths
	cfg.SitesDir = cfg.ConfigDir + "/sites"

	return cfg, nil
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
