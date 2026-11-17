package services

import (
	"strings"
	"testing"

	"github.com/TomasZmek/cpm/internal/models"
)

// A note (several lines, an empty line, characters that would mean something
// in a Caddyfile) survives generate → parse in both formats.
func TestNoteRoundTrip(t *testing.T) {
	p := NewParserService()
	note := "Home Assistant on the NAS\n\nask Peter {before} changing # really"
	for _, tc := range []struct {
		name string
		site *models.Site
	}{
		{"standard", &models.Site{Domains: []string{"ha.example.net"}, TargetIP: "10.0.0.5", TargetPort: "8123", TLSMode: "auto", Tags: []string{"home"}, Note: note}},
		{"wildcard", &models.Site{Domains: []string{"ha.example.com"}, TargetIP: "10.0.0.5", TargetPort: "8123", TLSMode: "wildcard:example.com", Note: note}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.site.Validate(); err != nil {
				t.Fatal(err)
			}
			out := tc.site.ToCaddyfile()
			for _, l := range strings.Split(out, "\n") {
				if strings.Contains(l, "{before}") && !strings.HasPrefix(l, "# @note: ") {
					t.Errorf("note text outside a comment line: %q", l)
				}
			}
			got := p.Parse(out, tc.site.Domains[0])
			if got.Note != note {
				t.Errorf("note = %q, want %q\n%s", got.Note, note, out)
			}
			if got.ExtraConfig != "" {
				t.Errorf("note leaked into ExtraConfig: %q", got.ExtraConfig)
			}
			if again := got.ToCaddyfile(); again != out {
				t.Errorf("second round trip changed the file:\n%s\n---\n%s", out, again)
			}
		})
	}
}

func TestNoNoteNoCommentLine(t *testing.T) {
	s := &models.Site{Domains: []string{"a.example.net"}, TargetIP: "10.0.0.1", TargetPort: "80", TLSMode: "auto"}
	if out := s.ToCaddyfile(); strings.Contains(out, "@note") {
		t.Errorf("empty note must not write a comment:\n%s", out)
	}
}

func TestNormalizeAndValidateNote(t *testing.T) {
	if got := models.NormalizeNote("\r\n  first  \r\nsecond\t\r\n\r\n"); got != "  first\nsecond" {
		t.Errorf("NormalizeNote = %q", got)
	}
	s := &models.Site{Domains: []string{"a.example.net"}, TargetIP: "10.0.0.1", TargetPort: "80"}
	s.Note = "bad\x00char"
	if s.Validate() == nil {
		t.Error("control character in note must be rejected")
	}
	s.Note = strings.Repeat("x", models.MaxNoteLength+1)
	if s.Validate() == nil {
		t.Error("too long note must be rejected")
	}
}
