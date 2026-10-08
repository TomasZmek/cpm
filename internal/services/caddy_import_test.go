package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const handWrittenCaddyfile = `{
    email admin@example.com
}

(my_headers) {
    header X-Test "1"
}

# Jellyfin
media.example.com {
    import my_headers
    reverse_proxy jellyfin:8096
    @blocked path /admin*
    respond @blocked 403
}

files.example.com {
    root * /srv
    file_server browse
}
`

func setupImport(t *testing.T) (*CaddyService, string) {
	t.Helper()
	cfg := newTestConfig(t)
	if err := os.MkdirAll(filepath.Join(cfg.SitesDir, "standard"), 0755); err != nil {
		t.Fatal(err)
	}
	cs := NewCaddyService(cfg, nil)
	return cs, cfg.ConfigDir
}

func TestImportFromMainCaddyfileMovesBlocks(t *testing.T) {
	cs, dir := setupImport(t)
	mainPath := filepath.Join(dir, "Caddyfile")
	os.WriteFile(mainPath, []byte(handWrittenCaddyfile), 0644)

	res, err := cs.ImportFromMainCaddyfile()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imported) != 2 {
		t.Fatalf("expected 2 imported sites, got %v (failed %v)", res.Imported, res.Failed)
	}

	// Site files are stored verbatim (no directive lost, #31)
	media, _ := os.ReadFile(filepath.Join(dir, "sites", "standard", "media.example.com.caddy"))
	for _, want := range []string{"import my_headers", "@blocked path /admin*", "respond @blocked 403", "reverse_proxy jellyfin:8096"} {
		if !strings.Contains(string(media), want) {
			t.Errorf("imported media site lost %q:\n%s", want, media)
		}
	}
	files, _ := os.ReadFile(filepath.Join(dir, "sites", "standard", "files.example.com.caddy"))
	if !strings.Contains(string(files), "file_server browse") || strings.Contains(string(files), "reverse_proxy") {
		t.Errorf("file_server site not stored verbatim:\n%s", files)
	}

	// Main Caddyfile keeps globals + snippets, loses the site blocks, imports standard sites (#30)
	main, _ := os.ReadFile(mainPath)
	m := string(main)
	if strings.Contains(m, "media.example.com {") || strings.Contains(m, "files.example.com {") {
		t.Errorf("site blocks still in main Caddyfile:\n%s", m)
	}
	for _, want := range []string{"email admin@example.com", "(my_headers) {", standardSitesImport} {
		if !strings.Contains(m, want) {
			t.Errorf("main Caddyfile is missing %q:\n%s", want, m)
		}
	}

	// Rollback restores everything
	if err := res.Rollback(); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(mainPath)
	if string(restored) != handWrittenCaddyfile {
		t.Errorf("rollback did not restore main Caddyfile")
	}
	if _, err := os.Stat(filepath.Join(dir, "sites", "standard", "media.example.com.caddy")); err == nil {
		t.Errorf("rollback did not remove imported site file")
	}
}

func TestImportContentCopiesCustomSnippetsAndSkipsExisting(t *testing.T) {
	cs, dir := setupImport(t)
	snippetsPath := filepath.Join(dir, "snippets.caddy")
	os.WriteFile(snippetsPath, []byte("(internal_only) {\n}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "sites", "standard", "files.example.com.caddy"), []byte("files.example.com {\n}\n"), 0644)

	res, err := cs.ImportContent(handWrittenCaddyfile)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imported) != 1 || res.Imported[0] != "media.example.com" {
		t.Errorf("expected only media.example.com imported, got %v", res.Imported)
	}
	if len(res.Failed) != 1 || !strings.Contains(res.Failed[0], "already exists") {
		t.Errorf("expected existing site to be skipped, got %v", res.Failed)
	}

	snippets, _ := os.ReadFile(snippetsPath)
	if !strings.Contains(string(snippets), "(my_headers) {") || !strings.Contains(string(snippets), "(internal_only) {") {
		t.Errorf("custom snippet not copied or existing lost:\n%s", snippets)
	}

	if err := res.Rollback(); err != nil {
		t.Fatal(err)
	}
	snippets, _ = os.ReadFile(snippetsPath)
	if strings.Contains(string(snippets), "my_headers") {
		t.Errorf("rollback did not restore snippets.caddy")
	}
}

func TestImportRejectsUnsafeAddress(t *testing.T) {
	if isSafeImportedAddress([]string{"a.com{"}) || isSafeImportedAddress(nil) || !isSafeImportedAddress([]string{"a.com", "http://b.com:8080"}) {
		t.Error("unexpected address validation result")
	}
}
