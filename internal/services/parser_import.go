package services

import (
	"regexp"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
)

// rawBlock represents a single top-level block extracted from a Caddyfile.
type rawBlock struct {
	header string // text before the opening brace (may include leading comment lines)
	full   string // entire block text: header + "{" ... "}"
}

// ParseMainCaddyfile parses a hand-written main Caddyfile and returns one Site
// per standard "domain { ... }" block.
//
// Blocks that are NOT individual standard sites are skipped:
//   - snippet definitions:   (name) { ... }
//   - the global options block / any anonymous "{ ... }" block
//   - wildcard parent blocks: *.domain.com { ... }
//   - named matcher blocks:   @name { ... }
//
// Per-site handling notes:
//   - reverse_proxy targets may be container names (e.g. jellyfin:8096), not IPs.
//   - "import <name>" for an unknown (custom) snippet is preserved via ExtraConfig.
//   - an inline "tls { dns cloudflare {env.XXX} }" block is recognised and recorded
//     in TLSMode (and mapped to the native cloudflare_dns snippet so the regenerated
//     file keeps working).
func (p *ParserService) ParseMainCaddyfile(content string) ([]*models.Site, error) {
	blocks := splitTopLevelBlocks(content)

	var sites []*models.Site
	for _, b := range blocks {
		address := blockAddress(b.header)

		// Skip non-site blocks.
		switch {
		case address == "": // global options / anonymous block
			continue
		case strings.HasPrefix(address, "("): // snippet definition
			continue
		case strings.HasPrefix(address, "*."): // wildcard parent block
			continue
		case strings.HasPrefix(address, "@"): // named matcher
			continue
		}

		if site := p.parseMainSiteBlock(b.full, address); site != nil {
			sites = append(sites, site)
		}
	}

	return sites, nil
}

// parseMainSiteBlock builds a Site from a single "domain { ... }" block.
func (p *ParserService) parseMainSiteBlock(blockText, address string) *models.Site {
	// Fallback filename/domain derived from the address line.
	defaultDomain := address
	if fields := strings.Fields(strings.ReplaceAll(address, ",", " ")); len(fields) > 0 {
		defaultDomain = fields[0]
	}

	importedNames := parseImportNames(blockText)
	hadCloudflareImport := containsString(importedNames, "cloudflare_dns")
	hasInlineCF := hasCloudflareDNSTLS(blockText)

	// Strip the inline Cloudflare tls block from the raw text BEFORE parsing so
	// it never lands in ExtraConfig (and cannot swallow directives that follow
	// it). We re-represent it via the cloudflare_dns snippet + TLSMode below.
	parseText := blockText
	if hasInlineCF {
		parseText = removeCloudflareTLSBlock(blockText)
	}

	// Reuse the existing single-site parser for the heavy lifting:
	// domains, reverse_proxy (incl. container:port), basic_auth, websocket,
	// health checks, timeouts, load balancing, tags and generic extra config.
	site := p.Parse(parseText, defaultDomain)

	// Parse() defaults every site to the cloudflare_dns snippet. For a faithful
	// migration, only keep it when the original block actually used Cloudflare
	// DNS (either via "import cloudflare_dns" or an inline tls block).
	if !hadCloudflareImport && !hasInlineCF {
		site.Snippets = removeString(site.Snippets, "cloudflare_dns")
	}

	// Custom snippet imports -> ExtraConfig. Parse() keeps only known snippets
	// and drops every "import" line from ExtraConfig, so unknown imports would
	// otherwise be lost.
	for _, name := range importedNames {
		if !containsString(p.knownSnippets, name) {
			site.ExtraConfig = appendConfigLine(site.ExtraConfig, "import "+name)
		}
	}

	// Inline Cloudflare DNS TLS -> TLSMode + native snippet.
	if hasInlineCF {
		site.TLSMode = "dns-cloudflare"
		if !containsString(site.Snippets, "cloudflare_dns") {
			site.Snippets = append(site.Snippets, "cloudflare_dns")
		}
	}

	// Fallback: reverse_proxy target without an explicit port
	// (e.g. "reverse_proxy jellyfin" or "reverse_proxy https://app").
	if site.TargetIP == "" {
		if ip, port := parseReverseProxyLoose(parseText); ip != "" {
			site.TargetIP = ip
			site.TargetPort = port
		}
	}

	site.RawContent = blockText
	return site
}

// splitTopLevelBlocks walks the Caddyfile and returns every brace-balanced
// top-level block. Comments (#...) and quoted strings are ignored when matching
// braces. Standalone top-level directives (e.g. "import snippets.caddy") and
// blank lines are discarded so they do not bleed into the next block's address;
// consecutive comment lines are kept as block metadata.
func splitTopLevelBlocks(content string) []rawBlock {
	var blocks []rawBlock

	depth := 0
	inString := false
	headerStart := 0
	headerEnd := -1
	lineStart := 0

	resetHeader := func(pos int) {
		headerStart = pos
		headerEnd = -1
	}

	n := len(content)
	for i := 0; i < n; i++ {
		ch := content[i]

		// Line comment: skip to end of line (never inside a string).
		if ch == '#' && !inString {
			j := i
			for j < n && content[j] != '\n' {
				j++
			}
			i = j - 1 // outer loop's ++ lands on the newline
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}

		switch ch {
		case '{':
			if depth == 0 {
				headerEnd = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
			if depth == 0 {
				header := ""
				if headerEnd >= 0 {
					header = content[headerStart:headerEnd]
				}
				blocks = append(blocks, rawBlock{
					header: header,
					full:   content[headerStart : i+1],
				})
				resetHeader(i + 1)
				lineStart = i + 1
			}
		case '\n':
			if depth == 0 {
				line := strings.TrimSpace(content[lineStart:i])
				if line == "" || !strings.HasPrefix(line, "#") {
					resetHeader(i + 1)
				}
			}
			lineStart = i + 1
		}
	}

	return blocks
}

// blockAddress returns the address portion of a block header with comment lines
// and surrounding whitespace removed.
func blockAddress(header string) string {
	var parts []string
	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts = append(parts, line)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// parseImportNames returns the names of every "import <name>" directive in the block.
func parseImportNames(content string) []string {
	re := regexp.MustCompile(`(?m)^\s*import\s+(\S+)`)
	var names []string
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			names = append(names, m[1])
		}
	}
	return names
}

// hasCloudflareDNSTLS reports whether the block contains an inline
// "tls { ... dns cloudflare ... }" directive.
func hasCloudflareDNSTLS(content string) bool {
	re := regexp.MustCompile(`(?s)tls\s*\{[^{}]*dns\s+cloudflare`)
	return re.MatchString(content)
}

// removeCloudflareTLSBlock removes the first brace-balanced "tls { ... }" block
// whose body references "dns cloudflare". Nested braces (e.g. {env.CF_API_TOKEN})
// are handled correctly.
func removeCloudflareTLSBlock(content string) string {
	re := regexp.MustCompile(`tls\s*\{`)
	for _, loc := range re.FindAllStringIndex(content, -1) {
		openIdx := loc[1] - 1 // index of '{'
		endIdx := matchBrace(content, openIdx)
		if endIdx < 0 {
			continue
		}
		inner := content[openIdx : endIdx+1]
		if strings.Contains(inner, "dns") && strings.Contains(inner, "cloudflare") {
			return content[:loc[0]] + content[endIdx+1:]
		}
	}
	return content
}

// matchBrace returns the index of the '}' matching the '{' at openIdx, or -1.
func matchBrace(s string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseReverseProxyLoose extracts a reverse_proxy target that may omit the port
// (e.g. "reverse_proxy jellyfin" or "reverse_proxy https://app"). Port defaults to "80".
func parseReverseProxyLoose(content string) (string, string) {
	re := regexp.MustCompile(`reverse_proxy\s+(?:https?://)?([^\s{:]+)(?::(\d+))?`)
	m := re.FindStringSubmatch(content)
	if len(m) < 2 || m[1] == "" {
		return "", ""
	}
	port := m[2]
	if port == "" {
		port = "80"
	}
	return m[1], port
}

// appendConfigLine appends a line to a config string, keeping it newline-separated.
func appendConfigLine(existing, line string) string {
	if strings.TrimSpace(existing) == "" {
		return line
	}
	return existing + "\n" + line
}

// containsString reports whether slice contains item.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// removeString returns slice with all occurrences of item removed.
func removeString(slice []string, item string) []string {
	out := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			out = append(out, s)
		}
	}
	return out
}
