package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/TomasZmek/cpm/internal/models"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled             bool           `json:"enabled"`
	Users               []*models.User `json:"users"`
	SessionTimeoutHours int            `json:"session_timeout_hours"`
}

// Session represents a user session
type Session struct {
	Username  string
	ExpiresAt time.Time
}

// AuthService handles authentication
type AuthService struct {
	configPath string
	config     *AuthConfig
	sessions   map[string]*Session
	mu         sync.RWMutex
}

// NewAuthService creates a new auth service
func NewAuthService(configDir string) *AuthService {
	s := &AuthService{
		configPath: filepath.Join(configDir, ".auth_config.json"),
		sessions:   make(map[string]*Session),
	}
	s.loadConfig()
	return s
}

// loadConfig loads configuration from file
func (a *AuthService) loadConfig() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config = &AuthConfig{
		Enabled:             false,
		Users:               []*models.User{},
		SessionTimeoutHours: 24,
	}

	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		return
	}

	content, err := os.ReadFile(a.configPath)
	if err != nil {
		fmt.Printf("Warning: Could not read auth config: %v\n", err)
		return
	}

	if err := json.Unmarshal(content, a.config); err != nil {
		fmt.Printf("Warning: Could not parse auth config: %v\n", err)
	}
}

// saveConfig saves configuration to file
func (a *AuthService) saveConfig() error {
	content, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(a.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(a.configPath, content, 0600)
}

// IsEnabled returns whether authentication is enabled
func (a *AuthService) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config.Enabled
}

// Enable enables authentication
func (a *AuthService) Enable() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.config.Users) == 0 {
		return fmt.Errorf("cannot enable authentication without users")
	}

	a.config.Enabled = true
	return a.saveConfig()
}

// Disable disables authentication
func (a *AuthService) Disable() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config.Enabled = false
	return a.saveConfig()
}

// GetUsers returns all users
func (a *AuthService) GetUsers() []*models.User {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config.Users
}

// GetUser returns a user by username
func (a *AuthService) GetUser(username string) *models.User {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, user := range a.config.Users {
		if user.Username == username {
			return user
		}
	}
	return nil
}

// CreateUser creates a new user
func (a *AuthService) CreateUser(username, password string, role models.Role) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if user exists
	for _, user := range a.config.Users {
		if user.Username == username {
			return fmt.Errorf("user already exists: %s", username)
		}
	}

	user, err := models.NewUser(username, password, role)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	a.config.Users = append(a.config.Users, user)
	return a.saveConfig()
}

// DeleteUser deletes a user
func (a *AuthService) DeleteUser(username string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, user := range a.config.Users {
		if user.Username == username {
			a.config.Users = append(a.config.Users[:i], a.config.Users[i+1:]...)
			return a.saveConfig()
		}
	}

	return fmt.Errorf("user not found: %s", username)
}

// UpdatePassword updates a user's password
func (a *AuthService) UpdatePassword(username, newPassword string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, user := range a.config.Users {
		if user.Username == username {
			if err := user.SetPassword(newPassword); err != nil {
				return err
			}
			return a.saveConfig()
		}
	}

	return fmt.Errorf("user not found: %s", username)
}

// UpdateRole updates a user's role
func (a *AuthService) UpdateRole(username string, role models.Role) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, user := range a.config.Users {
		if user.Username == username {
			user.Role = role
			return a.saveConfig()
		}
	}

	return fmt.Errorf("user not found: %s", username)
}

// Authenticate verifies credentials and returns a session token
func (a *AuthService) Authenticate(username, password string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, user := range a.config.Users {
		if user.Username == username {
			if !user.CheckPassword(password) {
				return "", fmt.Errorf("invalid password")
			}

			// Update last login
			user.LastLogin = time.Now()
			a.saveConfig()

			// Create session
			token := generateToken()
			a.sessions[token] = &Session{
				Username:  username,
				ExpiresAt: time.Now().Add(time.Duration(a.config.SessionTimeoutHours) * time.Hour),
			}

			return token, nil
		}
	}

	return "", fmt.Errorf("user not found: %s", username)
}

// ValidateSession validates a session token
func (a *AuthService) ValidateSession(token string) *models.User {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session, ok := a.sessions[token]
	if !ok {
		return nil
	}

	if time.Now().After(session.ExpiresAt) {
		// Session expired
		delete(a.sessions, token)
		return nil
	}

	return a.GetUser(session.Username)
}

// Logout invalidates a session
func (a *AuthService) Logout(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, token)
}

// HasUsers returns whether any users exist
func (a *AuthService) HasUsers() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.config.Users) > 0
}

// generateToken generates a secure random token
func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
