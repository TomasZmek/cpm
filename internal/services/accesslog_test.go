package services

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TomasZmek/cpm/internal/models"
)

// A line as written by Caddy 2.10 (format filter + ip_mask).
const caddyAccessLine = `{"level":"info","ts":1794341431.8711154,"logger":"http.log.access.log0","msg":"handled request","request":{"remote_ip":"127.0.0.0","remote_port":"36032","client_ip":"127.0.0.0","proto":"HTTP/1.1","method":"GET","host":"a.example.com","uri":"/x","headers":{"Accept":["*/*"],"User-Agent":["test-ua"]}},"bytes_read":0,"user_id":"","duration":0.000066025,"size":4,"status":200,"resp_headers":{"Server":["Caddy"]}}`

func TestParseAccessLogLine(t *testing.T) {
	e, ok := parseAccessLogLine([]byte(caddyAccessLine))
	if !ok {
		t.Fatal("line not parsed")
	}
	if e.Host != "a.example.com" || e.URI != "/x" || e.Method != "GET" || e.Status != 200 ||
		e.RemoteIP != "127.0.0.0" || e.UserAgent != "test-ua" || e.Size != 4 {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if e.Duration != 66025*time.Nanosecond {
		t.Errorf("duration = %v", e.Duration)
	}
	if e.Time.Unix() != 1794341431 {
		t.Errorf("time = %v", e.Time)
	}
	for _, bad := range []string{"", "not json", `{"level":"info","msg":"server running"}`} {
		if _, ok := parseAccessLogLine([]byte(bad)); ok {
			t.Errorf("%q parsed as access log entry", bad)
		}
	}
}

func TestHostMatcher(t *testing.T) {
	cases := []struct {
		addrs []string
		host  string
		want  bool
	}{
		{[]string{"a.example.com"}, "a.example.com", true},
		{[]string{"a.example.com"}, "A.Example.com:443", true},
		{[]string{"https://a.example.com:8443"}, "a.example.com", true},
		{[]string{"a.example.com"}, "b.example.com", false},
		{[]string{"*.example.com"}, "x.example.com", true},
		{[]string{"*.example.com"}, "x.y.example.com", false},
		{[]string{"*.example.com"}, "example.com", false},
		{[]string{":8080"}, "anything.test", true},
		{nil, "anything.test", true},
	}
	for _, c := range cases {
		if got := newHostMatcher(c.addrs).match(c.host); got != c.want {
			t.Errorf("match(%v, %q) = %v, want %v", c.addrs, c.host, got, c.want)
		}
	}
}

func logLine(ts time.Time, host, uri string, status int, dur time.Duration) string {
	return fmt.Sprintf(`{"level":"info","ts":%f,"logger":"http.log.access.log0","msg":"handled request","request":{"client_ip":"10.0.0.1","method":"GET","host":%q,"uri":%q,"headers":{}},"duration":%f,"size":10,"status":%d}`,
		float64(ts.UnixNano())/1e9, host, uri, dur.Seconds(), status)
}

func TestAccessLogQuery(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, AccessLogDirName)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	// Rotated, gzipped file: older entries, one of them out of the 24 h window.
	var old []string
	old = append(old, logLine(now.Add(-48*time.Hour), "a.example.com", "/ancient", 200, time.Millisecond))
	for i := 0; i < 5; i++ {
		old = append(old, logLine(now.Add(-time.Duration(10-i)*time.Hour), "a.example.com", fmt.Sprintf("/old%d", i), 200, time.Millisecond))
	}
	gzPath := filepath.Join(logDir, "access-2026-11-10T06-00-00.000.log.gz")
	f, _ := os.Create(gzPath)
	gz := gzip.NewWriter(f)
	gz.Write([]byte(strings.Join(old, "\n") + "\n"))
	gz.Close()
	f.Close()

	cur := []string{
		`{"level":"info","ts":1,"logger":"http","msg":"not an access entry"}`,
		logLine(now.Add(-2*time.Hour), "a.example.com", "/new1", 404, 10*time.Millisecond),
		logLine(now.Add(-1*time.Hour), "b.example.com", "/other", 200, time.Millisecond),
		logLine(now.Add(-30*time.Minute), "a.example.com", "/new2", 502, 100*time.Millisecond),
	}
	os.WriteFile(filepath.Join(logDir, AccessLogFileName), []byte(strings.Join(cur, "\n")+"\n"), 0600)

	s := NewAccessLogService(dir)
	s.now = func() time.Time { return now }

	res, err := s.Query(AccessLogQuery{Hosts: []string{"a.example.com"}, Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	var uris []string
	for _, e := range res.Entries {
		uris = append(uris, e.URI)
	}
	if got := strings.Join(uris, ","); got != "/new2,/new1,/old4,/old3" {
		t.Errorf("entries = %s", got)
	}
	// 2 current + 5 rotated within 24 h; /ancient is older.
	if res.Stats.Total != 7 || res.Stats.ByClass["5xx"] != 1 || res.Stats.ByClass["4xx"] != 1 {
		t.Errorf("stats = %+v", res.Stats)
	}
	if !res.Available {
		t.Error("log should be available")
	}

	res, _ = s.Query(AccessLogQuery{Status: "errors"})
	if len(res.Entries) != 2 {
		t.Errorf("errors filter: %d entries", len(res.Entries))
	}
	res, _ = s.Query(AccessLogQuery{Text: "OTHER"})
	if len(res.Entries) != 1 || res.Entries[0].Host != "b.example.com" {
		t.Errorf("text filter: %+v", res.Entries)
	}

	empty := NewAccessLogService(t.TempDir())
	if res, _ := empty.Query(AccessLogQuery{}); res.Available || len(res.Entries) != 0 {
		t.Errorf("missing log: %+v", res)
	}
}

func TestSetAccessLogImport(t *testing.T) {
	in := "# @tags: x\nexample.com, www.example.com {\n    reverse_proxy 1.2.3.4:80\n}\n"
	on, changed := setAccessLogImport(in, true)
	if !changed || !strings.Contains(on, "example.com {\n    import cpm_access_log\n    reverse_proxy") {
		t.Fatalf("import not added:\n%s", on)
	}
	if again, changed := setAccessLogImport(on, true); changed || again != on {
		t.Error("adding twice changed the file")
	}
	off, changed := setAccessLogImport(on, false)
	if !changed || off != in {
		t.Errorf("import not removed cleanly:\n%s", off)
	}
	if _, changed := setAccessLogImport("(snip) {\n    header X 1\n}\n", true); changed {
		t.Error("snippet file must not be changed")
	}
}

func TestAccessLogSnippetBlock(t *testing.T) {
	b := AccessLogSnippetBlock(models.AccessLogSettings{Enabled: true, RollSizeMB: 500, RollKeep: 3})
	for _, want := range []string{"(cpm_access_log) {", "output file /data/cpm-logs/access.log", "roll_size 100MiB", "roll_keep 3", "format json"} {
		if !strings.Contains(b, want) {
			t.Errorf("snippet lacks %q:\n%s", want, b)
		}
	}
	if b := AccessLogSnippetBlock(models.AccessLogSettings{MaskIPs: true}); !strings.Contains(b, "request>client_ip ip_mask 24 48") {
		t.Errorf("mask missing:\n%s", b)
	}
}

func TestParseLogSkipRoundTrip(t *testing.T) {
	p := NewParserService()
	for _, site := range []*models.Site{
		{Domains: []string{"a.example.com"}, TargetIP: "1.2.3.4", TargetPort: "80", AccessLog: true, LogSkipPaths: []string{"/health*", "/metrics"}},
		{Domains: []string{"b.example.com"}, TargetIP: "1.2.3.4", TargetPort: "80", NoAccessLog: true},
		{Domains: []string{"c.wild.test"}, TLSMode: "wildcard:wild.test", TargetIP: "1.2.3.4", TargetPort: "80", LogSkipPaths: []string{"/ping"}},
	} {
		got := p.Parse(site.ToCaddyfile(), site.Domains[0])
		if got.NoAccessLog != site.NoAccessLog || strings.Join(got.LogSkipPaths, " ") != strings.Join(site.LogSkipPaths, " ") {
			t.Errorf("%s: parsed no_log=%v paths=%v", site.Domains[0], got.NoAccessLog, got.LogSkipPaths)
		}
		if got.ExtraConfig != "" {
			t.Errorf("%s: log directives leaked into extra config: %q", site.Domains[0], got.ExtraConfig)
		}
	}
	bad := &models.Site{Domains: []string{"a.test"}, TargetIP: "1.2.3.4", TargetPort: "80", LogSkipPaths: []string{"/x {"}}
	if bad.Validate() == nil {
		t.Error("unsafe log skip path accepted")
	}
}
