package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TomasZmek/cpm/internal/models"
)

// A site migrated to a wildcard domain with a hyphen must keep that domain:
// the snippet name wildcard-tls-my-home-cz cannot be turned back into it.
func TestMigratedSiteHyphenatedWildcardDomain(t *testing.T) {
	cs, _ := newManagedCaddyService(t)
	ws := cs.caddyfileManager.wildcardService
	if err := ws.AddDomain(models.WildcardDomain{Domain: "my-home.cz", Provider: "cloudflare", UseEnv: true}); err != nil {
		t.Fatal(err)
	}
	std := filepath.Join(cs.config.SitesDir, "standard")

	// Legacy file migrated by an older version: import only, no @tls comment.
	legacy := "app.my-home.cz {\n    import wildcard-tls-my-home-cz\n    reverse_proxy 10.0.0.1:80\n}\n"
	if err := os.WriteFile(filepath.Join(std, "app.my-home.cz.caddy"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	site, err := cs.GetSite("app.my-home.cz")
	if err != nil {
		t.Fatal(err)
	}
	if site.TLSMode != "wildcard:my-home.cz" {
		t.Errorf("legacy TLSMode = %q, want wildcard:my-home.cz", site.TLSMode)
	}

	// Migration now records the domain in a @tls comment.
	path := filepath.Join(std, "web.my-home.cz.caddy")
	if err := os.WriteFile(path, []byte("web.my-home.cz {\n    reverse_proxy 10.0.0.2:80\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ws.MigrateSiteConfig(path, "my-home.cz"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "# @tls: wildcard:my-home.cz") || !strings.Contains(string(data), "import wildcard-tls-my-home-cz") {
		t.Errorf("migrated file:\n%s", data)
	}
	site, err = cs.GetSite("web.my-home.cz")
	if err != nil {
		t.Fatal(err)
	}
	if site.TLSMode != "wildcard:my-home.cz" {
		t.Errorf("migrated TLSMode = %q, want wildcard:my-home.cz", site.TLSMode)
	}
}
