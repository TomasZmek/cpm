package services

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/TomasZmek/cpm/internal/models"
)

// Access log location. Caddy writes into its data directory (/data in the
// Caddy container), which CPM has mounted as cfg.DataDir, so no extra volume
// is needed.
const (
	AccessLogCaddyDir = "/data/cpm-logs"
	AccessLogDirName  = "cpm-logs"
	AccessLogFileName = "access.log"
)

// AccessLogSnippetBlock returns the definition of the access log snippet that
// site blocks import while access logging is enabled.
func AccessLogSnippetBlock(a models.AccessLogSettings) string {
	a = a.Normalized()
	lines := []string{
		"(" + models.AccessLogSnippet + ") {",
		"    log {",
		"        output file " + AccessLogCaddyDir + "/" + AccessLogFileName + " {",
		fmt.Sprintf("            roll_size %dMiB", a.RollSizeMB),
		fmt.Sprintf("            roll_keep %d", a.RollKeep),
		"        }",
	}
	if a.MaskIPs {
		lines = append(lines,
			"        format filter {",
			"            wrap json",
			"            fields {",
			"                request>remote_ip ip_mask 24 48",
			"                request>client_ip ip_mask 24 48",
			"            }",
			"        }",
		)
	} else {
		lines = append(lines, "        format json")
	}
	lines = append(lines, "    }", "}")
	return strings.Join(lines, "\n")
}

var accessLogImportRe = regexp.MustCompile(`^\s*import\s+` + models.AccessLogSnippet + `\s*$`)

// setAccessLogImport adds (enabled) or removes the access log snippet import
// in the content of a standard site file. The import goes right after the
// first site address line. It returns the new content and whether it changed.
func setAccessLogImport(content string, enabled bool) (string, bool) {
	lines := strings.Split(content, "\n")
	has := false
	for _, l := range lines {
		if accessLogImportRe.MatchString(l) {
			has = true
			break
		}
	}
	if has == enabled {
		return content, false
	}
	if !enabled {
		out := lines[:0:0]
		for _, l := range lines {
			if !accessLogImportRe.MatchString(l) {
				out = append(out, l)
			}
		}
		return strings.Join(out, "\n"), true
	}
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if !strings.HasSuffix(t, "{") || strings.HasPrefix(t, "(") {
			return content, false // not a plain site block (e.g. a snippet file)
		}
		out := make([]string, 0, len(lines)+1)
		out = append(out, lines[:i+1]...)
		out = append(out, "    import "+models.AccessLogSnippet)
		out = append(out, lines[i+1:]...)
		return strings.Join(out, "\n"), true
	}
	return content, false
}

// SyncAccessLogImports makes every standard site file import the access log
// snippet while access logging is enabled, and removes the import otherwise.
// Wildcard rules are covered by their *.domain block in the Caddyfile. It
// returns a function that restores the files it changed.
func (c *CaddyService) SyncAccessLogImports() (restore func(), err error) {
	enabled := c.caddyfileManager.AccessLogSettings().Enabled
	dir := filepath.Join(c.config.SitesDir, "standard")
	files, _ := filepath.Glob(filepath.Join(dir, "*.caddy"))

	type saved struct {
		path    string
		content []byte
	}
	var changed []saved
	restore = func() {
		for _, s := range changed {
			_ = os.WriteFile(s.path, s.content, 0644)
		}
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return restore, fmt.Errorf("read %s: %w", filepath.Base(f), err)
		}
		if c.parser.isWildcardHandleFormat(string(data)) {
			continue
		}
		updated, ok := setAccessLogImport(string(data), enabled)
		if !ok {
			continue
		}
		if err := os.WriteFile(f, []byte(updated), 0644); err != nil {
			return restore, fmt.Errorf("write %s: %w", filepath.Base(f), err)
		}
		changed = append(changed, saved{f, data})
	}
	return restore, nil
}

// AccessLogEntry is one request from Caddy's JSON access log.
type AccessLogEntry struct {
	Time      time.Time
	Host      string
	Method    string
	URI       string
	Proto     string
	RemoteIP  string
	UserAgent string
	Status    int
	Size      int64
	Duration  time.Duration
}

// StatusClass returns "2xx".."5xx" (or "other").
func (e AccessLogEntry) StatusClass() string {
	if e.Status >= 200 && e.Status < 600 {
		return fmt.Sprintf("%dxx", e.Status/100)
	}
	return "other"
}

// DurationText formats the request duration for display.
func (e AccessLogEntry) DurationText() string { return formatDuration(e.Duration) }

// P50Text and P95Text format the response time percentiles for display.
func (s AccessLogStats) P50Text() string { return formatDuration(s.P50) }
func (s AccessLogStats) P95Text() string { return formatDuration(s.P95) }

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%d µs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2f s", d.Seconds())
	}
}

// SizeText formats the response size for display.
func (e AccessLogEntry) SizeText() string {
	switch {
	case e.Size < 1024:
		return fmt.Sprintf("%d B", e.Size)
	case e.Size < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(e.Size)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(e.Size)/(1024*1024))
	}
}

// AccessLogQuery selects entries. Empty fields match everything.
type AccessLogQuery struct {
	// Hosts are site addresses (as in Site.Domains); "*.example.com" matches
	// any subdomain.
	Hosts  []string
	Status string // "2xx", "3xx", "4xx", "5xx" or "errors" (>= 400)
	Text   string // substring of URI, host, method, client IP or user agent
	Limit  int    // newest entries to return (default 200)
}

// AccessLogStats summarises the matching requests of the last 24 hours.
type AccessLogStats struct {
	Total      int
	ByClass    map[string]int
	ErrorRate  float64 // share of 5xx, in percent
	P50, P95   time.Duration
	OldestSeen time.Time // oldest entry read (tells how far back the log goes)
}

// AccessLogResult is the answer to a query.
type AccessLogResult struct {
	Entries   []AccessLogEntry // newest first
	Stats     AccessLogStats
	Available bool // the log file exists
}

// AccessLogService reads Caddy's access log (including rotated files).
type AccessLogService struct {
	dir string
	now func() time.Time
}

// NewAccessLogService creates the reader for <dataDir>/cpm-logs.
func NewAccessLogService(dataDir string) *AccessLogService {
	return &AccessLogService{dir: filepath.Join(dataDir, AccessLogDirName), now: time.Now}
}

// maxScanBytes bounds how much log data one query reads.
const maxScanBytes = 256 << 20

// files returns the current log file and its rotated backups, newest first.
func (s *AccessLogService) files() []string {
	var out []string
	current := filepath.Join(s.dir, AccessLogFileName)
	if _, err := os.Stat(current); err == nil {
		out = append(out, current)
	}
	// Rotated files are named access-<timestamp>.log[.gz]; the timestamp sorts.
	backups, _ := filepath.Glob(filepath.Join(s.dir, "access-*.log*"))
	sort.Sort(sort.Reverse(sort.StringSlice(backups)))
	return append(out, backups...)
}

// Query returns the newest matching entries and statistics for the last
// 24 hours.
func (s *AccessLogService) Query(q AccessLogQuery) (*AccessLogResult, error) {
	if q.Limit <= 0 {
		q.Limit = 200
	}
	m := newHostMatcher(q.Hosts)
	text := strings.ToLower(strings.TrimSpace(q.Text))
	since := s.now().Add(-24 * time.Hour)

	res := &AccessLogResult{Stats: AccessLogStats{ByClass: map[string]int{}}}
	var durations []time.Duration
	var scanned int64

	for _, path := range s.files() {
		res.Available = true
		var fileMatches []AccessLogEntry
		oldest := time.Time{}
		n, err := readAccessLogFile(path, func(e AccessLogEntry) {
			if oldest.IsZero() || e.Time.Before(oldest) {
				oldest = e.Time
			}
			if !m.match(e.Host) || !matchText(e, text) {
				return
			}
			if !e.Time.Before(since) {
				res.Stats.Total++
				res.Stats.ByClass[e.StatusClass()]++
				if len(durations) < 200000 {
					durations = append(durations, e.Duration)
				}
			}
			if !matchStatus(e, q.Status) {
				return
			}
			// Keep only the newest entries this file can still contribute.
			keep := q.Limit - len(res.Entries)
			if keep <= 0 {
				return
			}
			fileMatches = append(fileMatches, e)
			if len(fileMatches) > 2*keep {
				fileMatches = append(fileMatches[:0], fileMatches[len(fileMatches)-keep:]...)
			}
		})
		if err != nil {
			return res, err
		}
		scanned += n
		if !oldest.IsZero() && (res.Stats.OldestSeen.IsZero() || oldest.Before(res.Stats.OldestSeen)) {
			res.Stats.OldestSeen = oldest
		}
		for i := len(fileMatches) - 1; i >= 0 && len(res.Entries) < q.Limit; i-- {
			res.Entries = append(res.Entries, fileMatches[i])
		}
		statsDone := !oldest.IsZero() && oldest.Before(since)
		if (len(res.Entries) >= q.Limit && statsDone) || scanned > maxScanBytes {
			break
		}
	}

	if res.Stats.Total > 0 {
		res.Stats.ErrorRate = float64(res.Stats.ByClass["5xx"]) * 100 / float64(res.Stats.Total)
	}
	res.Stats.P50, res.Stats.P95 = percentiles(durations)
	return res, nil
}

// readAccessLogFile calls fn for every access log entry in the file, oldest
// first, and returns the number of bytes read.
func readAccessLogFile(path string, fn func(AccessLogEntry)) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // rotated away meanwhile
		}
		return 0, err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return 0, nil // incomplete or foreign file; skip
		}
		defer gz.Close()
		r = gz
	}

	var n int64
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		line := sc.Bytes()
		n += int64(len(line)) + 1
		if e, ok := parseAccessLogLine(line); ok {
			fn(e)
		}
	}
	return n, sc.Err()
}

type rawAccessLog struct {
	TS      float64 `json:"ts"`
	Logger  string  `json:"logger"`
	Request struct {
		RemoteIP string              `json:"remote_ip"`
		ClientIP string              `json:"client_ip"`
		Proto    string              `json:"proto"`
		Method   string              `json:"method"`
		Host     string              `json:"host"`
		URI      string              `json:"uri"`
		Headers  map[string][]string `json:"headers"`
	} `json:"request"`
	Duration float64 `json:"duration"`
	Size     int64   `json:"size"`
	Status   int     `json:"status"`
}

// parseAccessLogLine decodes one line of Caddy's default JSON access log
// (ts in unix seconds, duration in seconds).
func parseAccessLogLine(line []byte) (AccessLogEntry, bool) {
	var r rawAccessLog
	if len(line) == 0 || line[0] != '{' || json.Unmarshal(line, &r) != nil {
		return AccessLogEntry{}, false
	}
	if r.Request.Host == "" && r.Request.Method == "" {
		return AccessLogEntry{}, false // not an access log entry
	}
	sec, frac := math.Modf(r.TS)
	e := AccessLogEntry{
		Time:     time.Unix(int64(sec), int64(frac*1e9)),
		Host:     r.Request.Host,
		Method:   r.Request.Method,
		URI:      r.Request.URI,
		Proto:    r.Request.Proto,
		RemoteIP: r.Request.ClientIP,
		Status:   r.Status,
		Size:     r.Size,
		Duration: time.Duration(r.Duration * float64(time.Second)),
	}
	if e.RemoteIP == "" {
		e.RemoteIP = r.Request.RemoteIP
	}
	for k, v := range r.Request.Headers {
		if strings.EqualFold(k, "User-Agent") && len(v) > 0 {
			e.UserAgent = v[0]
		}
	}
	return e, true
}

func matchStatus(e AccessLogEntry, status string) bool {
	switch status {
	case "":
		return true
	case "errors":
		return e.Status >= 400
	default:
		return e.StatusClass() == status
	}
}

func matchText(e AccessLogEntry, text string) bool {
	if text == "" {
		return true
	}
	for _, f := range []string{e.URI, e.Host, e.Method, e.RemoteIP, e.UserAgent} {
		if strings.Contains(strings.ToLower(f), text) {
			return true
		}
	}
	return false
}

func percentiles(d []time.Duration) (p50, p95 time.Duration) {
	if len(d) == 0 {
		return 0, 0
	}
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	at := func(p float64) time.Duration { return d[int(math.Ceil(p*float64(len(d))))-1] }
	return at(0.50), at(0.95)
}

// hostMatcher matches request hosts against site addresses.
type hostMatcher struct {
	exact    map[string]bool
	suffixes []string // ".example.com" for "*.example.com"
	any      bool
}

func newHostMatcher(addresses []string) hostMatcher {
	m := hostMatcher{exact: map[string]bool{}}
	for _, a := range addresses {
		h := normalizeHost(a)
		switch {
		case h == "":
			// ":8080" and similar catch-all addresses match every host
			if strings.TrimSpace(a) != "" {
				m.any = true
			}
		case strings.HasPrefix(h, "*."):
			m.suffixes = append(m.suffixes, h[1:])
		default:
			m.exact[h] = true
		}
	}
	if len(addresses) == 0 {
		m.any = true
	}
	return m
}

func (m hostMatcher) match(host string) bool {
	if m.any {
		return true
	}
	h := normalizeHost(host)
	if m.exact[h] {
		return true
	}
	for _, s := range m.suffixes {
		// *.example.com matches exactly one label, like Caddy
		if strings.HasSuffix(h, s) && !strings.Contains(strings.TrimSuffix(h, s), ".") && len(h) > len(s) {
			return true
		}
	}
	return false
}

// normalizeHost turns a site address or Host header into a bare lowercase
// host name: no scheme, port, path or trailing dot.
func normalizeHost(a string) string {
	a = strings.ToLower(strings.TrimSpace(a))
	if i := strings.Index(a, "://"); i >= 0 {
		a = a[i+3:]
	}
	if i := strings.IndexByte(a, '/'); i >= 0 {
		a = a[:i]
	}
	if h, _, err := net.SplitHostPort(a); err == nil {
		a = h
	}
	a = strings.TrimPrefix(strings.TrimSuffix(a, "]"), "[")
	return strings.TrimSuffix(a, ".")
}
