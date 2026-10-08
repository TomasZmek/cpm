package services

import (
	"testing"

	"github.com/TomasZmek/cpm/internal/models"
)

// Regression tests for TomasZmek/cpm#21: a token entered in the snippet form
// must never be replaced by an (empty) {env.CF_API_TOKEN} placeholder.
func TestCloudflareDNSDirective(t *testing.T) {
	const tok = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUV_-" // > 50 chars
	env := "dns cloudflare " + CloudflareDNSEnvPlaceholder
	wcEnv := []models.WildcardDomain{{Domain: "example.com", Provider: "cloudflare", UseEnv: true}}
	wcTok := []models.WildcardDomain{{Domain: "example.com", Provider: "cloudflare", APIToken: "wildcardtoken"}}

	cases := []struct {
		name   string
		cf     models.CloudflareDNSConfig
		wc     []models.WildcardDomain
		want   string
		wantOK bool
	}{
		{"disabled, no wildcard", models.CloudflareDNSConfig{}, nil, "", false},
		{"explicit token", models.CloudflareDNSConfig{Enabled: true, APIToken: tok}, nil, "dns cloudflare " + tok, true},
		{"env", models.CloudflareDNSConfig{Enabled: true, UseEnv: true}, nil, env, true},
		{"explicit token wins over wildcard env", models.CloudflareDNSConfig{Enabled: true, APIToken: tok}, wcEnv, "dns cloudflare " + tok, true},
		{"snippet disabled, wildcard token", models.CloudflareDNSConfig{}, wcTok, "dns cloudflare wildcardtoken", true},
		{"snippet disabled, wildcard env", models.CloudflareDNSConfig{}, wcEnv, env, true},
		{"enabled without token falls back to wildcard token", models.CloudflareDNSConfig{Enabled: true}, wcTok, "dns cloudflare wildcardtoken", true},
	}
	for _, tc := range cases {
		got, ok := CloudflareDNSDirective(tc.cf, tc.wc)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestEnvListHas(t *testing.T) {
	env := []string{"PATH=/usr/bin", "CF_API_TOKEN=secret", "EMPTY="}
	if !envListHas(env, "CF_API_TOKEN") {
		t.Error("expected CF_API_TOKEN to be found")
	}
	if envListHas(env, "EMPTY") {
		t.Error("empty value must count as missing")
	}
	if envListHas(env, "CF_API") {
		t.Error("prefix of another variable must not match")
	}
}
