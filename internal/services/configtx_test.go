package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/TomasZmek/cpm/internal/models"
)

func TestWriteConfigFileKeepsPermAndLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"wildcard.json", "Caddyfile", "site.caddy"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := writeConfigFile(path, []byte("new"), 0600); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != "new" {
			t.Errorf("%s: content %q", name, got)
		}
		info, _ := os.Stat(path)
		// os.WriteFile keeps the mode of an existing file, the rename path
		// uses the new one; both must not widen it.
		if info.Mode().Perm()&0077 != 0 && name == "wildcard.json" {
			t.Errorf("%s: mode %v, want 0600", name, info.Mode().Perm())
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if isTempFile(e.Name()) {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteConfigFileInPlaceKeepsInode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Caddyfile")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	if err := writeConfigFile(path, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) {
		t.Error("Caddyfile was replaced by a new file; a single-file bind mount would keep the old one")
	}
}

func failingReload() *ReloadResult {
	return &ReloadResult{Success: false, Error: "Validation failed: boom"}
}

func TestApplyChangeRollsBackRejectedChange(t *testing.T) {
	cs, dir := newManagedCaddyService(t)
	ss := NewSnippetsService(cs.config)

	existing := &models.Site{Domains: []string{"old.example.com"}, TargetIP: "10.0.0.1", TargetPort: "80", TLSMode: "auto"}
	if err := cs.CreateSite(existing); err != nil {
		t.Fatal(err)
	}
	sitePath := existing.Filepath
	siteBefore, _ := os.ReadFile(sitePath)
	caddyfileBefore, _ := os.ReadFile(filepath.Join(dir, "Caddyfile"))
	snippetsBefore, _ := os.ReadFile(filepath.Join(dir, "snippets.caddy"))

	cs.reloadFn = failingReload
	var newPath string
	result, err := cs.ApplyChange(func() error {
		// Change the existing rule, add a new one and a snippet setting
		existing.TargetPort = "8080"
		if err := cs.UpdateSite(existing); err != nil {
			return err
		}
		added := &models.Site{Domains: []string{"new.example.com"}, TargetIP: "10.0.0.2", TargetPort: "80", TLSMode: "auto"}
		if err := cs.CreateSite(added); err != nil {
			return err
		}
		newPath = added.Filepath
		cfg, _ := ss.GetConfig()
		cfg.RateLimit.Enabled = true
		return ss.SaveConfig(cfg)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || !result.RolledBack || result.RollbackError != "" {
		t.Fatalf("expected a rolled back failure, got %+v", result)
	}

	if got, _ := os.ReadFile(sitePath); string(got) != string(siteBefore) {
		t.Errorf("rule not restored:\n%s", got)
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Errorf("rule created by the rejected change still exists: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "Caddyfile")); string(got) != string(caddyfileBefore) {
		t.Error("Caddyfile not restored")
	}
	if _, err := os.Stat(filepath.Join(dir, ".snippets_config.json")); !os.IsNotExist(err) {
		t.Errorf("snippet config created by the rejected change still exists: %v", err)
	}
	if snippets, _ := os.ReadFile(filepath.Join(dir, "snippets.caddy")); string(snippets) != string(snippetsBefore) {
		t.Errorf("snippets.caddy not restored:\n%s", snippets)
	}
}

func TestApplyChangeRestoresDeletedRule(t *testing.T) {
	cs, _ := newManagedCaddyService(t)
	site := &models.Site{Domains: []string{"keep.example.com"}, TargetIP: "10.0.0.1", TargetPort: "80", TLSMode: "auto"}
	if err := cs.CreateSite(site); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(site.Filepath)

	cs.reloadFn = failingReload
	result, err := cs.ApplyChange(func() error { return cs.DeleteSite(site.Filename) })
	if err != nil || !result.RolledBack {
		t.Fatalf("expected rollback, got %+v, %v", result, err)
	}
	if got, err := os.ReadFile(site.Filepath); err != nil || string(got) != string(before) {
		t.Errorf("deleted rule not restored: %v\n%s", err, got)
	}
}

func TestApplyChangeKeepsAcceptedChange(t *testing.T) {
	cs, _ := newManagedCaddyService(t)
	cs.reloadFn = func() *ReloadResult { return &ReloadResult{Success: true} }
	site := &models.Site{Domains: []string{"ok.example.com"}, TargetIP: "10.0.0.1", TargetPort: "80", TLSMode: "auto"}
	result, err := cs.ApplyChange(func() error { return cs.CreateSite(site) })
	if err != nil || !result.Success || result.RolledBack {
		t.Fatalf("unexpected result %+v, %v", result, err)
	}
	if _, err := os.Stat(site.Filepath); err != nil {
		t.Errorf("accepted rule missing: %v", err)
	}
}

func TestApplyChangeKeepsChangeWhenDockerUnreachable(t *testing.T) {
	cs, _ := newManagedCaddyService(t)
	cs.reloadFn = func() *ReloadResult { return &ReloadResult{DockerUnreachable: true, Error: "unreachable"} }
	site := &models.Site{Domains: []string{"later.example.com"}, TargetIP: "10.0.0.1", TargetPort: "80", TLSMode: "auto"}
	result, err := cs.ApplyChange(func() error { return cs.CreateSite(site) })
	if err != nil || result.RolledBack {
		t.Fatalf("unexpected result %+v, %v", result, err)
	}
	if _, err := os.Stat(site.Filepath); err != nil {
		t.Errorf("rule saved while Docker was unreachable is missing: %v", err)
	}
}

func TestApplyChangeRestoresOnChangeError(t *testing.T) {
	cs, _ := newManagedCaddyService(t)
	called := false
	cs.reloadFn = func() *ReloadResult { called = true; return &ReloadResult{Success: true} }
	var path string
	boom := errors.New("boom")
	_, err := cs.ApplyChange(func() error {
		site := &models.Site{Domains: []string{"half.example.com"}, TargetIP: "10.0.0.1", TargetPort: "80", TLSMode: "auto"}
		if err := cs.CreateSite(site); err != nil {
			return err
		}
		path = site.Filepath
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected the change error, got %v", err)
	}
	if called {
		t.Error("Caddy must not be reloaded after a failed change")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("partial change left on disk: %v", err)
	}
}
