package services

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
)

// ParserService handles parsing of Caddyfile configurations
type ParserService struct {
	knownSnippets []string
}

// NewParserService creates a new parser service
func NewParserService() *ParserService {
	return &ParserService{
		knownSnippets: []string{
			"cloudflare_dns",
			"internal_only",
			"security_headers",
			"compression",
			"rate_limit",
			"basic_auth",
		},
	}
}

// Parse parses Caddyfile content into a Site model
func (p *ParserService) Parse(content, filenameDomain string) *models.Site {
	site := &models.Site{
		Filename:   filenameDomain,
		Domains:    []string{filenameDomain},
		RawContent: content,
		Snippets:   []string{},
		Tags:       []string{},
		TLSMode:    "auto",
	}

	// Parse tags from comment
	site.Tags = p.parseTags(content)

	// Parse TLS mode from comment or import
	site.TLSMode = p.parseTLSMode(content)

	// Parse domains from header
	site.Domains = p.parseDomains(content, filenameDomain)

	// Parse snippets
	site.Snippets = p.parseSnippets(content)

	// Detect internal access
	site.IsInternal = contains(site.Snippets, "internal_only")

	// Parse reverse proxy settings
	p.parseReverseProxy(content, site)

	// Parse load balancing
	p.parseLoadBalancing(content, site)

	// Detect WebSocket
	site.EnableWebSocket = strings.Contains(content, "header_up") && strings.Contains(content, "X-Real-IP")

	// Parse health check
	if match := regexp.MustCompile(`health_uri\s+(\S+)`).FindStringSubmatch(content); len(match) > 1 {
		site.HealthCheckPath = match[1]
	}

	// Parse timeout
	if match := regexp.MustCompile(`dial_timeout\s+(\d+)s`).FindStringSubmatch(content); len(match) > 1 {
		if timeout, err := strconv.Atoi(match[1]); err == nil {
			site.TimeoutSeconds = timeout
		}
	}

	// Parse basic auth
	site.BasicAuthEnabled, site.BasicAuthUsers = p.parseBasicAuth(content)

	// Parse extra config
	site.ExtraConfig = p.parseExtraConfig(content)

	return site
}

// parseTags extracts tags from # @tags: comment
func (p *ParserService) parseTags(content string) []string {
	re := regexp.MustCompile(`#\s*@tags:\s*(.+)$`)
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return []string{}
	}

	var tags []string
	for _, tag := range strings.Split(match[1], ",") {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// parseDomains extracts domains from the header
func (p *ParserService) parseDomains(content, defaultDomain string) []string {
	re := regexp.MustCompile(`^([^{]+)\{`)
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return []string{defaultDomain}
	}

	// Split by newlines and filter out comment lines
	var domainParts []string
	for _, line := range strings.Split(match[1], "\n") {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domainParts = append(domainParts, line)
	}
	
	domainsStr := strings.Join(domainParts, " ")
	domainsStr = strings.TrimSpace(domainsStr)

	var domains []string
	for _, d := range strings.Split(domainsStr, ",") {
		d = strings.TrimSpace(d)
		for _, part := range strings.Fields(d) {
			if part != "" {
				domains = append(domains, part)
			}
		}
	}

	if len(domains) == 0 {
		return []string{defaultDomain}
	}
	return domains
}

// parseSnippets extracts imported snippets
func (p *ParserService) parseSnippets(content string) []string {
	re := regexp.MustCompile(`import\s+(\S+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	var snippets []string
	for _, match := range matches {
		if len(match) > 1 {
			snippet := match[1]
			if contains(p.knownSnippets, snippet) {
				snippets = append(snippets, snippet)
			}
		}
	}

	if len(snippets) == 0 {
		return []string{"cloudflare_dns"}
	}
	return snippets
}

// parseReverseProxy extracts reverse proxy settings
func (p *ParserService) parseReverseProxy(content string, site *models.Site) {
	re := regexp.MustCompile(`reverse_proxy\s+(https?://)?([^:\s{]+):(\d+)`)
	match := re.FindStringSubmatch(content)

	if len(match) > 3 {
		if match[1] != "" {
			site.IsHTTPSBackend = strings.Contains(strings.ToLower(match[1]), "https")
		}
		site.TargetIP = match[2]
		site.TargetPort = match[3]
	}
}

// parseLoadBalancing extracts load balancing settings
func (p *ParserService) parseLoadBalancing(content string, site *models.Site) {
	// Find all backends on reverse_proxy line
	re := regexp.MustCompile(`reverse_proxy\s+([^{]+?)(?:\{|$)`)
	match := re.FindStringSubmatch(content)

	if len(match) > 1 {
		backendStr := strings.TrimSpace(match[1])
		backendRe := regexp.MustCompile(`(https?://[^:\s]+:\d+)`)
		backends := backendRe.FindAllString(backendStr, -1)

		if len(backends) > 1 {
			// First is main, rest are additional
			site.AdditionalBackends = backends[1:]
		}
	}

	// Find lb_policy
	lbRe := regexp.MustCompile(`lb_policy\s+(\S+)`)
	lbMatch := lbRe.FindStringSubmatch(content)
	if len(lbMatch) > 1 {
		site.LBPolicy = lbMatch[1]
	}
}

// parseBasicAuth extracts basic auth settings
func (p *ParserService) parseBasicAuth(content string) (bool, []string) {
	// Check for inline basic_auth (not import)
	if strings.Contains(content, "import basic_auth") {
		return false, nil
	}

	re := regexp.MustCompile(`basic_auth\s*\{([^}]+)\}`)
	match := re.FindStringSubmatch(content)

	if len(match) < 2 {
		return false, nil
	}

	var users []string
	userRe := regexp.MustCompile(`^\s*(\S+)\s+(\$\S+)\s*$`)

	for _, line := range strings.Split(match[1], "\n") {
		userMatch := userRe.FindStringSubmatch(line)
		if len(userMatch) > 2 {
			users = append(users, userMatch[1]+" "+userMatch[2])
		}
	}

	if len(users) > 0 {
		return true, users
	}
	return false, nil
}

// parseExtraConfig extracts non-standard configuration
func (p *ParserService) parseExtraConfig(content string) string {
	lines := strings.Split(content, "\n")
	var extraLines []string
	startReading := false
	inSkipBlock := false
	braceDepth := 0

	skipPatterns := []string{
		"import ",
		"reverse_proxy",
		"lb_policy",
		"health_uri",
		"health_interval",
		"header_up",
		"transport http",
		"tls_insecure_skip_verify",
		"dial_timeout",
		"response_header_timeout",
		"basic_auth",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Start of main block
		if strings.HasSuffix(trimmed, "{") && !startReading {
			startReading = true
			continue
		}

		if !startReading {
			continue
		}

		// End of main block
		if trimmed == "}" && braceDepth == 0 {
			break
		}

		// Track nested blocks
		if strings.Contains(trimmed, "{") {
			braceDepth += strings.Count(trimmed, "{")
			// Check if entering skip block
			for _, pattern := range []string{"transport http", "reverse_proxy", "basic_auth"} {
				if strings.Contains(trimmed, pattern) {
					inSkipBlock = true
					break
				}
			}
		}

		if strings.Contains(trimmed, "}") {
			braceDepth -= strings.Count(trimmed, "}")
			if braceDepth == 0 {
				inSkipBlock = false
				continue
			}
		}

		if inSkipBlock {
			continue
		}

		// Check if standard line
		isStandard := false
		for _, pattern := range skipPatterns {
			if strings.Contains(trimmed, pattern) {
				isStandard = true
				break
			}
		}

		if !isStandard && trimmed != "" && trimmed != "}" {
			extraLines = append(extraLines, trimmed)
		}
	}

	return strings.Join(extraLines, "\n")
}

// CleanDomains normalizes domain list
func CleanDomains(domainsStr string) []string {
	var domains []string
	domainsStr = strings.ReplaceAll(domainsStr, ",", " ")
	for _, d := range strings.Fields(domainsStr) {
		if d = strings.TrimSpace(d); d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// parseTLSMode extracts TLS mode from config
func (p *ParserService) parseTLSMode(content string) string {
	// First try to find @tls comment
	tlsRegex := regexp.MustCompile(`#\s*@tls:\s*(.+)`)
	if match := tlsRegex.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// Try to find wildcard-tls import
	wildcardRegex := regexp.MustCompile(`import\s+wildcard-tls-([a-zA-Z0-9-]+)`)
	if match := wildcardRegex.FindStringSubmatch(content); len(match) > 1 {
		// Convert snippet name back to domain (e.g., "zrnek-cz" -> "zrnek.cz")
		domain := strings.ReplaceAll(match[1], "-", ".")
		return "wildcard:" + domain
	}

	return "auto"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
