package services

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/TomasZmek/cpm/internal/models"
)

// standardSitesImport is the directive the main Caddyfile needs so that
// imported sites in sites/standard/ are loaded by Caddy.
const standardSitesImport = "import /etc/caddy/sites/standard/*.caddy"

// ImportResult describes the outcome of a Caddyfile import. Rollback undoes
// every file change made by the import; call it when the resulting
// configuration fails to validate.
type ImportResult struct {
	Imported []string
	Failed   []string
	Rollback func() error
}

// MainCaddyfilePath returns the path to the main Caddyfile.
func (c *CaddyService) MainCaddyfilePath() string {
	return filepath.Join(c.config.ConfigDir, "Caddyfile")
}

// PreviewImportFromMainCaddyfile parses the site blocks written directly in the
// main Caddyfile and returns them WITHOUT writing anything to disk.
func (c *CaddyService) PreviewImportFromMainCaddyfile() ([]*models.Site, error) {
	content, err := os.ReadFile(c.MainCaddyfilePath())
	if err != nil {
		return nil, fmt.Errorf("failed to read the main Caddyfile")
	}
	return c.PreviewImportContent(string(content))
}

// PreviewImportContent parses site blocks directly from Caddyfile text
// (e.g. an uploaded file) WITHOUT writing anything to disk.
func (c *CaddyService) PreviewImportContent(content string) ([]*models.Site, error) {
	sites, err := c.parser.ParseMainCaddyfile(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Caddyfile: %w", err)
	}
	return sites, nil
}

// ImportFromMainCaddyfile moves the site blocks written directly in the main
// Caddyfile to sites/standard/{domain}.caddy. The moved blocks are removed
// from the main Caddyfile (everything else — global options, snippets,
// wildcard blocks, comments — is kept) so Caddy does not see each site twice,
// and the main Caddyfile is made to import sites/standard/*.caddy.
func (c *CaddyService) ImportFromMainCaddyfile() (*ImportResult, error) {
	mainPath := c.MainCaddyfilePath()
	original, err := os.ReadFile(mainPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the main Caddyfile")
	}

	sites, err := c.PreviewImportContent(string(original))
	if err != nil {
		return nil, err
	}

	res, written := c.saveImportedSites(sites)
	rollbackSites := rollbackFiles(written)
	res.Rollback = rollbackSites

	if len(res.Imported) == 0 {
		return res, nil
	}

	// Remove every successfully imported block from the main Caddyfile.
	updated := string(original)
	for _, site := range sites {
		if containsString(res.Imported, site.PrimaryDomain()) && site.RawContent != "" {
			updated = strings.Replace(updated, site.RawContent, "", 1)
		}
	}
	updated = ensureStandardSitesImport(updated)

	if err := os.WriteFile(mainPath, []byte(updated), 0644); err != nil {
		_ = rollbackSites()
		return nil, fmt.Errorf("failed to update the main Caddyfile: %w", err)
	}

	res.Rollback = func() error {
		if err := os.WriteFile(mainPath, original, 0644); err != nil {
			return err
		}
		return rollbackSites()
	}
	return res, nil
}

// ImportContent saves the site blocks of uploaded Caddyfile text to
// sites/standard/ and copies custom snippet definitions it contains into
// snippets.caddy, so imported "import <snippet>" lines keep working.
func (c *CaddyService) ImportContent(content string) (*ImportResult, error) {
	sites, err := c.PreviewImportContent(content)
	if err != nil {
		return nil, err
	}

	snippetsPath := filepath.Join(c.config.ConfigDir, "snippets.caddy")
	originalSnippets, readErr := os.ReadFile(snippetsPath)
	snippetsExisted := readErr == nil

	if err := c.copyCustomSnippets(content, snippetsPath, string(originalSnippets)); err != nil {
		return nil, err
	}

	res, written := c.saveImportedSites(sites)
	rollbackSites := rollbackFiles(written)
	res.Rollback = func() error {
		if snippetsExisted {
			if err := os.WriteFile(snippetsPath, originalSnippets, 0644); err != nil {
				return err
			}
		} else {
			os.Remove(snippetsPath)
		}
		return rollbackSites()
	}
	return res, nil
}

// saveImportedSites writes each parsed site to sites/standard/{domain}.caddy.
// The original block text is stored verbatim, so directives the parser does
// not model are not lost. Returns the result and the paths that were written.
func (c *CaddyService) saveImportedSites(sites []*models.Site) (*ImportResult, []string) {
	res := &ImportResult{}
	var written []string

	standardDir := filepath.Join(c.config.SitesDir, "standard")
	if err := os.MkdirAll(standardDir, 0755); err != nil {
		for _, site := range sites {
			res.Failed = append(res.Failed, fmt.Sprintf("%s (%v)", site.PrimaryDomain(), err))
		}
		return res, nil
	}

	for _, site := range sites {
		domain := site.PrimaryDomain()
		filename := sanitizeFilename(domain)

		if err := ValidateSiteFilename(filename); err != nil {
			res.Failed = append(res.Failed, fmt.Sprintf("%s (invalid name)", domain))
			continue
		}
		if !isSafeImportedAddress(site.Domains) {
			res.Failed = append(res.Failed, fmt.Sprintf("%s (invalid address)", domain))
			continue
		}

		// Do not overwrite an existing managed site (in any directory).
		if _, err := c.GetSite(filename); err == nil {
			res.Failed = append(res.Failed, fmt.Sprintf("%s (already exists)", domain))
			continue
		}

		content := strings.TrimSpace(site.RawContent)
		if content == "" {
			content = strings.TrimSpace(site.ToCaddyfile())
		}

		dest := filepath.Join(standardDir, filename+".caddy")
		if err := os.WriteFile(dest, []byte(content+"\n"), 0644); err != nil {
			res.Failed = append(res.Failed, fmt.Sprintf("%s (%v)", domain, err))
			continue
		}

		written = append(written, dest)
		res.Imported = append(res.Imported, domain)
	}

	return res, written
}

// isSafeImportedAddress checks the site addresses of an imported block. The
// block body is stored verbatim (import is an admin-only, raw operation), but
// the addresses must be single tokens so the filename and preview are sane.
func isSafeImportedAddress(domains []string) bool {
	if len(domains) == 0 {
		return false
	}
	for _, d := range domains {
		if d == "" || strings.ContainsAny(d, " \t\r\n{}\"'`#;\\") {
			return false
		}
	}
	return true
}

var snippetDefRegex = regexp.MustCompile(`(?m)^[ \t]*\(([A-Za-z0-9_.\-]+)\)\s*\{`)

// copyCustomSnippets appends snippet definitions "(name) { ... }" found in
// source to snippets.caddy, skipping names CPM manages and names that are
// already defined in snippets.caddy or the main Caddyfile.
func (c *CaddyService) copyCustomSnippets(source, snippetsPath, existingSnippets string) error {
	mainContent, _ := os.ReadFile(c.MainCaddyfilePath())
	defined := map[string]bool{}
	for _, text := range []string{existingSnippets, string(mainContent)} {
		for _, m := range snippetDefRegex.FindAllStringSubmatch(text, -1) {
			defined[m[1]] = true
		}
	}

	var blocks []string
	for _, loc := range snippetDefRegex.FindAllStringSubmatchIndex(source, -1) {
		name := source[loc[2]:loc[3]]
		if defined[name] || managedSnippetNames[name] || strings.HasPrefix(name, "wildcard-tls-") {
			continue
		}
		end := matchBrace(source, loc[1]-1)
		if end < 0 {
			continue
		}
		blocks = append(blocks, strings.TrimSpace(source[loc[0]:end+1]))
		defined[name] = true
	}
	if len(blocks) == 0 {
		return nil
	}

	updated := strings.TrimRight(existingSnippets, "\n") + "\n\n# --- CUSTOM SNIPPETS (preserved by CPM) ---\n" +
		strings.Join(blocks, "\n\n") + "\n"
	if err := os.WriteFile(snippetsPath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write snippets: %w", err)
	}
	return nil
}

// ensureStandardSitesImport appends the sites/standard import to a main
// Caddyfile that does not have it yet.
func ensureStandardSitesImport(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == standardSitesImport {
			return content
		}
	}
	return strings.TrimRight(content, "\n") + "\n\n# === STANDARD SITES (managed by CPM) ===\n" + standardSitesImport + "\n"
}

// rollbackFiles returns a function that removes the given files.
func rollbackFiles(paths []string) func() error {
	return func() error {
		var firstErr error
		for _, p := range paths {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
}
