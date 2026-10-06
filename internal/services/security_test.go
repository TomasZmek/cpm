package services

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/TomasZmek/cpm/internal/config"
	"github.com/TomasZmek/cpm/internal/models"
)

func TestValidateSiteFilename(t *testing.T) {
	for _, ok := range []string{"app.example.com", "app.example.com.caddy", "my-site_1"} {
		if err := ValidateSiteFilename(ok); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
	for _, bad := range []string{"", "..", "../users", "a/b", `a\b`, "..caddy", "x\x00y"} {
		if err := ValidateSiteFilename(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func TestValidateDomainName(t *testing.T) {
	for _, ok := range []string{"example.com", "sub.example.co.uk", "xn--d1acufc.xn--p1ai"} {
		if err := ValidateDomainName(ok); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
	for _, bad := range []string{"", "localhost", "*.example.com", "example.com {", "a.com\nb.com", "a..com", "-a.com"} {
		if err := ValidateDomainName(bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

func newTestConfig(t *testing.T) *config.Config {
	dir := t.TempDir()
	return &config.Config{ConfigDir: dir, SitesDir: filepath.Join(dir, "sites"), DataDir: t.TempDir()}
}

func TestRestoreBackupRejectsUnsafePaths(t *testing.T) {
	cfg := newTestConfig(t)
	b := NewBackupService(cfg)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for _, name := range []string{
		"sites/standard/ok.caddy",
		"sites/../.auth_config.json",
		"pages/../.auth_config.json",
		".auth_config.json",
		"../outside.txt",
	} {
		w, _ := zw.Create(name)
		w.Write([]byte("data"))
	}
	zw.Close()

	b.RestoreBackup(buf.Bytes())

	if _, err := os.Stat(filepath.Join(cfg.SitesDir, "standard", "ok.caddy")); err != nil {
		t.Errorf("expected legit site file to be restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.ConfigDir, ".auth_config.json")); err == nil {
		t.Errorf("auth config must not be restorable from a backup")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfg.ConfigDir), "outside.txt")); err == nil {
		t.Errorf("file outside config dir was written")
	}
}

func TestBackupIncludesSiteSubdirectories(t *testing.T) {
	cfg := newTestConfig(t)
	for _, p := range []string{"wildcard/a.example.com.caddy", "standard/b.com.caddy"} {
		full := filepath.Join(cfg.SitesDir, p)
		os.MkdirAll(filepath.Dir(full), 0755)
		os.WriteFile(full, []byte("x"), 0644)
	}

	data, _, err := NewBackupService(cfg).CreateBackup()
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	found := map[string]bool{}
	for _, f := range zr.File {
		found[f.Name] = true
	}
	for _, want := range []string{"sites/wildcard/a.example.com.caddy", "sites/standard/b.com.caddy"} {
		if !found[want] {
			t.Errorf("backup is missing %s", want)
		}
	}
}

func TestImportRulesRejectsTraversal(t *testing.T) {
	cfg := newTestConfig(t)
	cs := NewCaddyService(cfg, nil)
	b := NewBackupService(cfg)

	rules := `[{"filename":"../../evil","raw_content":"evil"}]`
	imported, _, err := b.ImportRules([]byte(rules), cs, false)
	if err != nil {
		t.Fatal(err)
	}
	if imported != 0 {
		t.Errorf("expected traversal rule to be rejected, imported=%d", imported)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfg.ConfigDir), "evil.caddy")); err == nil {
		t.Errorf("file written outside sites dir")
	}
}

func TestAuthLastAdminProtection(t *testing.T) {
	a := NewAuthService(t.TempDir())
	if err := a.CreateInitialAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	if err := a.CreateInitialAdmin("other", "password123"); err == nil {
		t.Error("initial admin must only be creatable once")
	}
	if err := a.DeleteUser("admin"); err == nil {
		t.Error("expected error deleting last admin")
	}
	if err := a.UpdateRole("admin", models.RoleViewer); err == nil {
		t.Error("expected error demoting last admin")
	}
	if err := a.CreateUser("bob", "password123", models.Role("superuser")); err == nil {
		t.Error("expected error for invalid role")
	}
	if err := a.CreateUser("bob", "short", models.RoleEditor); err == nil {
		t.Error("expected error for short password")
	}
}

func TestPasswordChangeInvalidatesSessions(t *testing.T) {
	a := NewAuthService(t.TempDir())
	if err := a.CreateInitialAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	token, err := a.Authenticate("admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if a.ValidateSession(token) == nil {
		t.Fatal("expected valid session")
	}
	if err := a.UpdatePassword("admin", "newpassword123"); err != nil {
		t.Fatal(err)
	}
	if a.ValidateSession(token) != nil {
		t.Error("session should be invalidated after password change")
	}
}

func TestDemuxDockerStream(t *testing.T) {
	frame := func(stream byte, payload string) []byte {
		h := []byte{stream, 0, 0, 0, 0, 0, 0, byte(len(payload))}
		return append(h, payload...)
	}
	// One line split across two frames, plus a second line
	data := append(frame(1, "Valid conf"), frame(1, "iguration\nsecond line\n")...)
	if got, want := demuxDockerStream(data), "Valid configuration\nsecond line\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := demuxDockerStream([]byte("raw tty output\n")); got != "raw tty output\n" {
		t.Errorf("raw stream altered: %q", got)
	}
}
