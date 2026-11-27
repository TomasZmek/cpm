package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestValidateBasicAuthUsername(t *testing.T) {
	for _, ok := range []string{"alice", "bob.smith", "ops-team_1", "me@example.com"} {
		if err := ValidateBasicAuthUsername(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"", "two words", "evil\n}\nimport x", "a{b}", "#comment", "{env.X}",
		"quote\"", "tab\tname", strings.Repeat("a", 65),
	} {
		if err := ValidateBasicAuthUsername(bad); !errors.Is(err, ErrBasicAuthUsername) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidateBasicAuthPassword(t *testing.T) {
	if ValidateBasicAuthPassword("short") == nil {
		t.Error("password under the minimum accepted")
	}
	if ValidateBasicAuthPassword(strings.Repeat("x", BasicAuthMaxPassword+1)) == nil {
		t.Error("password over bcrypt's 72 bytes accepted")
	}
	if err := ValidateBasicAuthPassword("long enough"); err != nil {
		t.Error(err)
	}
}

func TestBasicAuthUsersInSnippet(t *testing.T) {
	cfg := newTestConfig(t)
	ss := NewSnippetsService(cfg)

	if created, err := ss.AddBasicAuthUser("zed", "password-1"); err != nil || !created {
		t.Fatalf("add zed: %v %v", created, err)
	}
	if _, err := ss.AddBasicAuthUser("anna", "password-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.AddBasicAuthUser("bad name", "password-3"); !errors.Is(err, ErrBasicAuthUsername) {
		t.Errorf("invalid username accepted: %v", err)
	}

	// Not enabled yet: no snippet, not offered to rules
	snippets, _ := os.ReadFile(filepath.Join(cfg.ConfigDir, "snippets.caddy"))
	if strings.Contains(string(snippets), "(basic_auth)") {
		t.Error("disabled basic_auth snippet generated")
	}

	c, _ := ss.GetConfig()
	c.BasicAuth.Enabled = true
	if err := ss.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	snippets, _ = os.ReadFile(filepath.Join(cfg.ConfigDir, "snippets.caddy"))
	s := string(snippets)
	ia, iz := strings.Index(s, "        anna $2"), strings.Index(s, "        zed $2")
	if ia < 0 || iz < 0 || ia > iz {
		t.Errorf("users missing or not sorted:\n%s", s)
	}
	if strings.Contains(s, "password-1") {
		t.Error("plain-text password in snippets.caddy")
	}
	available, _ := ss.GetAvailableSnippets()
	if !strings.Contains(strings.Join(available, ","), "basic_auth") {
		t.Error("enabled basic_auth with users not offered to rules")
	}

	// Same username again sets a new password
	c, _ = ss.GetConfig()
	oldHash := c.BasicAuth.Users["anna"]
	if created, err := ss.AddBasicAuthUser("anna", "new-password"); err != nil || created {
		t.Fatalf("password change: %v %v", created, err)
	}
	c, _ = ss.GetConfig()
	if c.BasicAuth.Users["anna"] == oldHash {
		t.Error("password not changed")
	}
	if bcrypt.CompareHashAndPassword([]byte(c.BasicAuth.Users["anna"]), []byte("new-password")) != nil {
		t.Error("stored hash does not match the new password")
	}

	// Remove: unknown user, last user while enabled
	if err := ss.RemoveBasicAuthUser("nobody"); !errors.Is(err, ErrBasicAuthUserNotFound) {
		t.Errorf("unknown user: %v", err)
	}
	if err := ss.RemoveBasicAuthUser("zed"); err != nil {
		t.Fatal(err)
	}
	if err := ss.RemoveBasicAuthUser("anna"); !errors.Is(err, ErrBasicAuthLastUser) {
		t.Errorf("last user removed while enabled: %v", err)
	}
	c, _ = ss.GetConfig()
	c.BasicAuth.Enabled = false
	if err := ss.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	if err := ss.RemoveBasicAuthUser("anna"); err != nil {
		t.Errorf("last user of a disabled snippet: %v", err)
	}
}

func TestBasicAuthSnippetSkipsUnsafeEntries(t *testing.T) {
	cfg := newTestConfig(t)
	ss := NewSnippetsService(cfg)
	c, _ := ss.GetConfig()
	c.BasicAuth.Enabled = true
	c.BasicAuth.Users = map[string]string{
		"good":            "$2a$12$abcdefghijklmnopqrstuuvwxyzABCDEFGHIJKLMNOPQRSTUV",
		"evil\n}\nimport": "$2a$12$abc",
		"plain":           "not-a-hash } import x",
	}
	if err := ss.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	snippets, _ := os.ReadFile(filepath.Join(cfg.ConfigDir, "snippets.caddy"))
	s := string(snippets)
	if !strings.Contains(s, "        good $2a$12$") {
		t.Errorf("valid entry missing:\n%s", s)
	}
	if strings.Contains(s, "evil") || strings.Contains(s, "plain") {
		t.Errorf("unsafe entry written:\n%s", s)
	}
}
