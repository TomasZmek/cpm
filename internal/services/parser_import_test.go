package services

import (
	"strings"
	"testing"
)

const sampleMainCaddyfile = `# Hand-written Caddyfile
{
    email admin@example.com
}

import /etc/caddy/snippets.caddy

(logging) {
    log {
        output file /var/log/caddy/access.log
    }
}

jelly.example.com {
    import logging
    reverse_proxy jellyfin:8096
}

app.example.com, www.app.example.com {
    tls {
        dns cloudflare {env.CF_API_TOKEN}
    }
    encode gzip
    reverse_proxy 192.168.1.50:3000
}

secure.example.com {
    import internal_only
    basic_auth {
        admin $2a$14$abcdefghijklmnopqrstuv
    }
    reverse_proxy backend:8080
}

plain.example.com {
    reverse_proxy 10.0.0.5:9000
}

noport.example.com {
    reverse_proxy myapp
}

*.wild.example.com {
    import wildcard-tls-wild-example-com
    handle { abort }
}
`

func TestParseMainCaddyfile(t *testing.T) {
	p := NewParserService()
	sites, err := p.ParseMainCaddyfile(sampleMainCaddyfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Global options, the (logging) snippet, and the *.wild parent block are skipped.
	if len(sites) != 5 {
		var got []string
		for _, s := range sites {
			got = append(got, s.PrimaryDomain())
		}
		t.Fatalf("expected 5 sites, got %d: %v", len(sites), got)
	}

	byDomain := map[string]int{}
	for i, s := range sites {
		byDomain[s.PrimaryDomain()] = i
	}

	// jelly: container target + custom import preserved, no spurious cloudflare_dns.
	jelly := sites[byDomain["jelly.example.com"]]
	if jelly.TargetIP != "jellyfin" || jelly.TargetPort != "8096" {
		t.Errorf("jelly target = %s:%s, want jellyfin:8096", jelly.TargetIP, jelly.TargetPort)
	}
	if containsString(jelly.Snippets, "cloudflare_dns") {
		t.Errorf("jelly should not have a defaulted cloudflare_dns snippet: %v", jelly.Snippets)
	}
	if !strings.Contains(jelly.ExtraConfig, "import logging") {
		t.Errorf("jelly should preserve custom import in ExtraConfig: %q", jelly.ExtraConfig)
	}

	// app: inline cloudflare tls -> TLSMode + snippet, other directives preserved.
	app := sites[byDomain["app.example.com"]]
	if app.TLSMode != "dns-cloudflare" {
		t.Errorf("app TLSMode = %q, want dns-cloudflare", app.TLSMode)
	}
	if !containsString(app.Snippets, "cloudflare_dns") {
		t.Errorf("app should have cloudflare_dns snippet: %v", app.Snippets)
	}
	if len(app.Domains) != 2 {
		t.Errorf("app should have 2 domains, got %v", app.Domains)
	}
	if !strings.Contains(app.ExtraConfig, "encode gzip") {
		t.Errorf("app should keep 'encode gzip' (must survive tls stripping): %q", app.ExtraConfig)
	}
	if strings.Contains(app.ExtraConfig, "tls") {
		t.Errorf("app inline tls block should not remain in ExtraConfig: %q", app.ExtraConfig)
	}

	// secure: internal + basic auth + container target.
	secure := sites[byDomain["secure.example.com"]]
	if !secure.IsInternal {
		t.Errorf("secure should be internal")
	}
	if !secure.BasicAuthEnabled || len(secure.BasicAuthUsers) != 1 {
		t.Errorf("secure should have basic auth, got %v", secure.BasicAuthUsers)
	}
	if secure.TargetIP != "backend" || secure.TargetPort != "8080" {
		t.Errorf("secure target = %s:%s, want backend:8080", secure.TargetIP, secure.TargetPort)
	}

	// noport: port-less target falls back to :80.
	noport := sites[byDomain["noport.example.com"]]
	if noport.TargetIP != "myapp" || noport.TargetPort != "80" {
		t.Errorf("noport target = %s:%s, want myapp:80", noport.TargetIP, noport.TargetPort)
	}

	// wildcard parent must be skipped.
	if _, ok := byDomain["*.wild.example.com"]; ok {
		t.Errorf("wildcard parent block should be skipped")
	}
}
