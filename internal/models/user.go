package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role represents user role
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// User represents an application user
type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    time.Time `json:"last_login"`
}

// NewUser creates a new user with hashed password
func NewUser(username, password string, role Role) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}

	return &User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now(),
	}, nil
}

// CheckPassword verifies the password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// SetPassword sets a new password
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// HasPermission checks if user has the required permission
func (u *User) HasPermission(permission string) bool {
	permissions := map[Role][]string{
		RoleViewer: {"view"},
		RoleEditor: {"view", "edit"},
		RoleAdmin:  {"view", "edit", "admin"},
	}

	for _, p := range permissions[u.Role] {
		if p == permission {
			return true
		}
	}
	return false
}

// RoleIcon returns icon for the role
func (u *User) RoleIcon() string {
	switch u.Role {
	case RoleAdmin:
		return "👑"
	case RoleEditor:
		return "✏️"
	case RoleViewer:
		return "👁️"
	default:
		return "👤"
	}
}

// RoleDisplayName returns display name for the role
func (u *User) RoleDisplayName() string {
	switch u.Role {
	case RoleAdmin:
		return "Admin"
	case RoleEditor:
		return "Editor"
	case RoleViewer:
		return "Viewer"
	default:
		return string(u.Role)
	}
}

// AllRoles returns all available roles
func AllRoles() []Role {
	return []Role{RoleAdmin, RoleEditor, RoleViewer}
}
