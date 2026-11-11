package services

import (
	"strings"
	"testing"
)

// A rule saved from the form is regenerated from the parsed Site, so parsing
// must keep what is in the file: no added cloudflare_dns, no lost imports.
func TestParseKeepsImportsOnRoundTrip(t *testing.T) {
	p := NewParserService()
	content := "# @tags: web\nshop.example.net {\n    import cpm_access_log\n    import logging\n    import security_headers\n    encode gzip\n    reverse_proxy 10.0.0.1:8080\n}\n"
	site := p.Parse(content, "shop.example.net")

	if containsString(site.Snippets, "cloudflare_dns") {
		t.Errorf("cloudflare_dns must not be defaulted: %v", site.Snippets)
	}
	if !containsString(site.Snippets, "security_headers") {
		t.Errorf("known snippet lost: %v", site.Snippets)
	}
	if !strings.Contains(site.ExtraConfig, "import logging") {
		t.Errorf("custom import lost from ExtraConfig: %q", site.ExtraConfig)
	}
	if strings.Contains(site.ExtraConfig, "cpm_access_log") || strings.Contains(site.ExtraConfig, "security_headers") {
		t.Errorf("generated imports must not be in ExtraConfig: %q", site.ExtraConfig)
	}

	out := site.ToCaddyfile()
	if strings.Contains(out, "cloudflare_dns") {
		t.Errorf("regenerated file gained cloudflare_dns:\n%s", out)
	}
	if strings.Count(out, "import logging") != 1 {
		t.Errorf("regenerated file should keep import logging once:\n%s", out)
	}
	// Parsing the regenerated file again is stable.
	again := p.Parse(out, "shop.example.net")
	if again.ExtraConfig != site.ExtraConfig {
		t.Errorf("ExtraConfig changed on second round trip: %q -> %q", site.ExtraConfig, again.ExtraConfig)
	}
}

func TestParseWildcardHandleKeepsCustomImport(t *testing.T) {
	p := NewParserService()
	content := "# @tls: wildcard:example.com\n@app_example_com host app.example.com\nhandle @app_example_com {\n    import my_auth\n    reverse_proxy 10.0.0.1:80\n}\n"
	site := p.Parse(content, "app.example.com")
	if len(site.Snippets) != 0 {
		t.Errorf("no snippets expected, got %v", site.Snippets)
	}
	if site.ExtraConfig != "import my_auth" {
		t.Errorf("ExtraConfig = %q, want import my_auth", site.ExtraConfig)
	}
}

func TestImportCaddyfileCustomImportNotDuplicated(t *testing.T) {
	p := NewParserService()
	sites, err := p.ParseMainCaddyfile("a.example.com {\n    import logging\n    reverse_proxy 10.0.0.1:80\n}\n")
	if err != nil || len(sites) != 1 {
		t.Fatalf("sites=%v err=%v", sites, err)
	}
	if n := strings.Count(sites[0].ExtraConfig, "import logging"); n != 1 {
		t.Errorf("import logging appears %d times in %q", n, sites[0].ExtraConfig)
	}
}
