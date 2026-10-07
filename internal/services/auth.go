package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/TomasZmek/cpm/internal/models"
	"golang.org/x/crypto/bcrypt"
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
		log.Printf("Warning: Could not read auth config: %v", err)
		return
	}

	if err := json.Unmarshal(content, a.config); err != nil {
		log.Printf("Warning: Could not parse auth config: %v", err)
	}
	if a.config.SessionTimeoutHours <= 0 {
		a.config.SessionTimeoutHours = 24
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
	users := make([]*models.User, len(a.config.Users))
	copy(users, a.config.Users)
	return users
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

// ErrLastAdmin is returned when an operation would remove the last admin
var ErrLastAdmin = errors.New("the last admin user cannot be deleted or demoted")

// MinPasswordLength is the minimum accepted password length
const MinPasswordLength = 8

var usernameRegex = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// dummyHash is compared against when a username does not exist, so that
// login timing does not reveal whether a user exists.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("cpm-dummy-password"), 12)

// ValidateCredentials checks username format and password strength
func ValidateCredentials(username, password string) error {
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("username may contain only letters, digits, '.', '_' and '-' (max 64 characters)")
	}
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	return nil
}

// countAdmins returns the number of admin users. Caller must hold the lock.
func (a *AuthService) countAdmins() int {
	n := 0
	for _, user := range a.config.Users {
		if user.Role == models.RoleAdmin {
			n++
		}
	}
	return n
}

// invalidateUserSessions removes all sessions of a user. Caller must hold the write lock.
func (a *AuthService) invalidateUserSessions(username string) {
	for token, session := range a.sessions {
		if session.Username == username {
			delete(a.sessions, token)
		}
	}
}

// CreateUser creates a new user
func (a *AuthService) CreateUser(username, password string, role models.Role) error {
	if err := ValidateCredentials(username, password); err != nil {
		return err
	}
	if !role.IsValid() {
		return fmt.Errorf("invalid role: %s", role)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if user exists
	for _, user := range a.config.Users {
		if user.Username == username {
			return fmt.Errorf("user already exists: %s", username)
		}
	}

	user, err := models.NewUser(strings.Clone(username), password, role)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	a.config.Users = append(a.config.Users, user)
	return a.saveConfig()
}

// CreateInitialAdmin creates the first admin user and enables authentication.
// It fails if any user already exists, so it cannot be raced or replayed.
func (a *AuthService) CreateInitialAdmin(username, password string) error {
	if err := ValidateCredentials(username, password); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.config.Users) > 0 {
		return fmt.Errorf("initial setup already completed")
	}

	user, err := models.NewUser(strings.Clone(username), password, models.RoleAdmin)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	a.config.Users = append(a.config.Users, user)
	a.config.Enabled = true
	return a.saveConfig()
}

// DeleteUser deletes a user
func (a *AuthService) DeleteUser(username string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, user := range a.config.Users {
		if user.Username == username {
			if user.Role == models.RoleAdmin && a.countAdmins() <= 1 {
				return ErrLastAdmin
			}
			a.config.Users = append(a.config.Users[:i], a.config.Users[i+1:]...)
			a.invalidateUserSessions(username)
			return a.saveConfig()
		}
	}

	return fmt.Errorf("user not found: %s", username)
}

// UpdatePassword updates a user's password and logs out all of the user's sessions
func (a *AuthService) UpdatePassword(username, newPassword string) error {
	if len(newPassword) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, user := range a.config.Users {
		if user.Username == username {
			if err := user.SetPassword(newPassword); err != nil {
				return err
			}
			a.invalidateUserSessions(username)
			return a.saveConfig()
		}
	}

	return fmt.Errorf("user not found: %s", username)
}

// UpdateRole updates a user's role
func (a *AuthService) UpdateRole(username string, role models.Role) error {
	if !role.IsValid() {
		return fmt.Errorf("invalid role: %s", role)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, user := range a.config.Users {
		if user.Username == username {
			if user.Role == models.RoleAdmin && role != models.RoleAdmin && a.countAdmins() <= 1 {
				return ErrLastAdmin
			}
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
				return "", fmt.Errorf("invalid credentials")
			}

			// Update last login
			user.LastLogin = time.Now()
			a.saveConfig()

			// Create session (store the user's own name, never a string that
			// may alias a request buffer)
			token := generateToken()
			a.sessions[token] = &Session{
				Username:  user.Username,
				ExpiresAt: time.Now().Add(time.Duration(a.config.SessionTimeoutHours) * time.Hour),
			}

			return token, nil
		}
	}

	// Spend comparable time for unknown users to avoid username enumeration
	bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
	return "", fmt.Errorf("invalid credentials")
}

// SessionTimeout returns the configured session lifetime
func (a *AuthService) SessionTimeout() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return time.Duration(a.config.SessionTimeoutHours) * time.Hour
}

// ValidateSession validates a session token
func (a *AuthService) ValidateSession(token string) *models.User {
	a.mu.RLock()
	session, ok := a.sessions[token]
	a.mu.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().After(session.ExpiresAt) {
		// Session expired — acquire write lock to safely delete.
		// Re-check under write lock: another goroutine may have already removed it.
		a.mu.Lock()
		if s, exists := a.sessions[token]; exists && time.Now().After(s.ExpiresAt) {
			delete(a.sessions, token)
		}
		a.mu.Unlock()
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
