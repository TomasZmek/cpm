package models

import "testing"

func TestSiteValidate(t *testing.T) {
	valid := func() *Site {
		return &Site{
			Domains:    []string{"app.example.com"},
			TargetIP:   "192.168.1.10",
			TargetPort: "8080",
			TLSMode:    "wildcard:example.com",
			Snippets:   []string{"internal_only", "wildcard-tls-example-com"},
			Tags:       []string{"prod"},
		}
	}

	if err := valid().Validate(); err != nil {
		t.Fatalf("expected valid site, got %v", err)
	}

	cases := map[string]func(s *Site){
		"no domains":         func(s *Site) { s.Domains = nil },
		"domain with brace":  func(s *Site) { s.Domains = []string{"a.com{"} },
		"ip with newline":    func(s *Site) { s.TargetIP = "1.2.3.4\n}\nevil.com {" },
		"port with space":    func(s *Site) { s.TargetPort = "80 {" },
		"empty port":         func(s *Site) { s.TargetPort = "" },
		"health no slash":    func(s *Site) { s.HealthCheckPath = "health" },
		"snippet injection":  func(s *Site) { s.Snippets = []string{"x\nrespond 200"} },
		"tag with newline":   func(s *Site) { s.Tags = []string{"a\n}\nevil {"} },
		"tls mode injection": func(s *Site) { s.TLSMode = "auto\nfoo" },
	}
	for name, mutate := range cases {
		s := valid()
		mutate(s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestCertificateIsWildcard(t *testing.T) {
	for domain, want := range map[string]bool{
		"wildcard_.example.com": true,
		"*.example.com":         true,
		"app.example.com":       false,
	} {
		if got := (&Certificate{Domain: domain}).IsWildcard(); got != want {
			t.Errorf("%s: got %v, want %v", domain, got, want)
		}
	}
}
