package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newManagedCaddyService(t *testing.T) (*CaddyService, string) {
	t.Helper()
	cfg := newTestConfig(t)
	cfg.Version = "test"
	ws := NewWildcardService(cfg.ConfigDir)
	ss := NewSnippetsService(cfg)
	cm := NewCaddyfileManager(cfg, ws, ss)
	if err := cm.EnsureDirectoryStructure(); err != nil {
		t.Fatal(err)
	}
	cs := NewCaddyService(cfg, nil)
	cs.SetCaddyfileManager(cm)
	if err := cm.SaveCaddyfile(); err != nil {
		t.Fatal(err)
	}
	return cs, cfg.ConfigDir
}

func TestSaveFallbackAddsImportAndRollsBack(t *testing.T) {
	cs, dir := newManagedCaddyService(t)
	mainPath := filepath.Join(dir, "Caddyfile")
	before, _ := os.ReadFile(mainPath)
	if strings.Contains(string(before), FallbackImport) {
		t.Fatal("fallback import must not be generated without fallback.caddy")
	}

	rollback, err := cs.SaveFallback(":80 {\n\trespond 404\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(mainPath)
	if strings.Count(string(after), FallbackImport) != 1 {
		t.Errorf("expected exactly one fallback import:\n%s", after)
	}

	// Saving again must not duplicate the import
	if _, err := cs.SaveFallback(":80 {\n\trespond 410\n}\n"); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(mainPath)
	if strings.Count(string(again), FallbackImport) != 1 {
		t.Errorf("fallback import duplicated:\n%s", again)
	}

	// Regenerating the Caddyfile keeps the import while fallback.caddy exists
	if err := cs.RegenerateCaddyfile(); err != nil {
		t.Fatal(err)
	}
	regen, _ := os.ReadFile(mainPath)
	if !strings.Contains(string(regen), FallbackImport) {
		t.Errorf("regenerated Caddyfile lost the fallback import:\n%s", regen)
	}

	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sites", "fallback.caddy")); err == nil {
		t.Error("rollback should remove a fallback that did not exist before")
	}
	restored, _ := os.ReadFile(mainPath)
	if string(restored) != string(before) {
		t.Error("rollback did not restore the main Caddyfile")
	}
}

func TestSaveErrorPageOnlyKnownCodes(t *testing.T) {
	cs, _ := newManagedCaddyService(t)
	if err := cs.SaveErrorPage(404, "<h1>404</h1>"); err != nil {
		t.Errorf("404 should be allowed: %v", err)
	}
	if err := cs.SaveErrorPage(-1, "x"); err == nil {
		t.Error("expected error for unsupported code")
	}
}
