# CPM v3.4.0 - Modern theme, Caddyfile import & security hardening

Released 2026-10-08. Includes PR #19 by [@redstar-programmer](https://github.com/redstar-programmer) together with a security review and follow-up fixes.

## ⚠️ Upgrade notes

- **Roles are now enforced.** Viewer can only browse, Editor manages rules, snippets, certificates and reloads, and only Admin can manage users, backups, import/export, Wildcard SSL and Docker settings.
- **The API `/api/v1/*` requires login** (session cookie) and a CSRF token for POST requests. Scripts that called the API without logging in will stop working.
- **New passwords need at least 8 characters.** Existing passwords keep working.
- **Cloudflare snippet:** with "Use CF_API_TOKEN environment variable" enabled, the Caddy container must define `CF_API_TOKEN`; CPM now warns when it does not. A token typed into the form always takes precedence.
- **Caddyfile import** saves a backup to `caddy-config/backups/` first and removes the imported blocks from the main Caddyfile.

## ✨ New Features

- **Modern theme** (optional, PR #19) — redesigned UI with automatic light/dark mode and selectable accent colour. Classic stays the default; switch in Settings → General (or `THEME=modern`). Every feature works in both themes.
- **Caddyfile import** — import site blocks from the main Caddyfile or an uploaded file; blocks are stored verbatim (no directive is lost), custom snippets are carried over, duplicates are skipped, and the change is rolled back if Caddy rejects it
- **Logs viewer** — parsed Caddy JSON logs with level colours, level filter, search and line wrapping
- **Bulk certificate renewal** — expiring / expired / all, wildcard certificates excluded, warning about Let's Encrypt rate limits
- **Backend status** — reachability of each rule's backend on the rules list and dashboard
- **Container picker** in the rule form — fills service name and port from running containers
- **Docker Auto-Discovery** — local Docker host registered automatically; rules targeting a service/container name are paired correctly
- **Fallback rule** and custom 403/404 error pages are now actually loaded by Caddy (the fallback file was written but never imported)
- **Docker resilience** — automatic reconnect with one retry; "Docker unreachable — change saved" instead of a misleading validation error

## 📱 Phones & tablets

- Sidebar becomes an off-canvas menu with a top bar (Modern below 1024 px, Classic below 768 px)
- Settings on phones use list–detail navigation: a list of sections, each opening full-width with a back button
- Settings show only the sections the user's role may open
- Tables scroll inside their card; pages no longer scroll sideways

## 🔒 Security Fixes

- **Arbitrary file read** (#29) — Caddyfile import accepted a file path and could read any file readable by CPM; only the main Caddyfile or an uploaded file can be imported now, preview is POST-only
- **Data race in Docker reconnect** (#33) — the Docker client is swapped atomically and the old client is closed
- **Unauthenticated API** (cmd/cpm/main.go) — `/api/v1/*` now requires a session (previously anyone could list all rules and trigger a reload); CSRF protection now applies to the API too
- **Role enforcement** (internal/middleware/auth.go, cmd/cpm/main.go) — `RequirePermission` is implemented and applied to routes: Viewer is read-only, Editor manages rules/snippets/certificates/reload, Admin manages users, auth, backups, import/export and wildcard SSL
- **Path traversal in rules import** (internal/services/backup.go, caddy.go) — site filenames are validated (`ValidateSiteFilename`) before any file is read, written or deleted
- **Backup restore hardening** (internal/services/backup.go) — only files that a backup produces may be restored (no more overwriting `.auth_config.json`), `pages/../` traversal fixed, 10 MB per-file limit against zip bombs
- **Caddyfile injection** (internal/models/site.go, services/wildcard.go, handlers/snippets.go) — site fields, wildcard domains, DNS API tokens, allowed networks and security header values are validated so they cannot inject directives
- **XSS in HTMX error responses** (internal/handlers/handlers.go) — error messages are HTML-escaped
- **Open redirect** (internal/handlers/api.go) — reload/validate redirect only back to a same-host path
- **Initial admin setup** (internal/services/auth.go) — atomic `CreateInitialAdmin`, username/password validation (min. 8 characters)
- **User management** — role values validated, last admin cannot be deleted or demoted, password change/user deletion invalidates sessions, constant-time-ish login for unknown users
- **Cookies & files** — session cookie gets `Secure` when served over HTTPS and its lifetime matches the server session; `wildcard.json` and `.snippets_config.json` (API tokens) written with `0600`
- **Security headers** — UI sends `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`

## 🐛 Bug Fixes

- Random logouts right after saving (the session username aliased Fiber's request buffer); no "saved" message was shown
- Tags were lost when editing a rule
- Cloudflare snippet used an empty `{env.CF_API_TOKEN}` instead of the entered token (#21)
- Fresh install: generated Caddyfile imported a missing `snippets.caddy`
- Wildcard migration discarded its backup; wildcard settings did not show flash messages
- Modern theme: missing notifications after reload/validate and role change, broken domain names on rule cards
- Only the first checked snippet was saved when creating/editing a rule
- Backups did not include `sites/wildcard/`, `sites/standard/` and `wildcard.json`
- Raw edit wrote the file to `sites/` instead of the rule's actual directory (duplicate, change not applied)
- Uploaded backup/import files could be read only partially (`io.ReadAll`)
- Docker exec/log output corrupted by stripping 8 bytes per line; now properly demultiplexed with `stdcopy`
- Log viewer `lines` parameter capped (1–5000); log SSE stream stops when the client disconnects
- 429 login page was missing the CSRF token
- Tags and snippets are trimmed; validation errors return 400 instead of 500

## 🌐 i18n

- New UI texts translated in English, Czech and Korean
- Japanese, Chinese, Spanish, German and French added by PR #19 are included but hidden until they translate at least 95 % of the texts
- Fixed plural-form headers

## ✅ Tests

- Added tests for Caddyfile import and rollback, fallback import, Docker reconnect (race detector), token source priority, translation coverage, theme rendering, session buffer aliasing, site validation, filename/domain validation, backup restore/import traversal, backup contents, auth (last admin, sessions), Docker stream demux and safe redirects

## 🙏 Credits

UI redesign, Caddyfile import, logs viewer, Docker discovery improvements and new locales by [@redstar-programmer](https://github.com/redstar-programmer) (PR #19).

---

# CPM v3.3.1 - Security Patch

## 🔒 Security Fixes
- **Go 1.26.4** — fixes CVE-2026-42504 (High), CVE-2026-27145 (Medium), CVE-2026-42507 (Medium)

---

# CPM v3.3.0 - i18n Refactor & Korean Language

## 🌐 i18n System Refactor
- **PO/MO format** — translations migrated from monolithic Go maps to standard gettext PO files
- **Per-language files** — locales/en, locales/cs, locales/ko (easy to add new languages)
- **Korean language** — 한국어 added as third supported language
- **Plural support infrastructure** — ngettext/TN() ready for future use
- **Flash messages translated** — 35 UI messages now translated in all languages
- **Login page translated** — previously hardcoded English strings now localized

## 🙏 Credits
Korean translation by [@redstar-programmer](https://github.com/redstar-programmer)

---

# CPM v3.2.0 - Docker Auto-Discovery

## ✨ New Features

### Docker Auto-Discovery
- **Auto-Discovery** — automatic detection of running Docker containers on the host
- **Multi-host support** — configure multiple Docker hosts (Settings → Docker)
- **Local host flag** — mark one host as "Local Docker" for discovery; auto-detect its IP with one click
- **Smart pairing** — existing proxy rules are automatically matched to running containers (by IP + port)
- **One-click rule creation** — create proxy rules pre-filled with container name, IP and port
- **Quick host selector** — new/edit site form includes a host dropdown to fill target IP instantly

## 🐛 Bug Fixes

- **Duplicate containers** — Docker API returns one entry per network interface; deduplication added
- **Pairing detection** — pairing now checks both private port and host-mapped (public) port

---

# CPM v3.1.3 - Security patch

## 🔒 Security Fixes

- chore: bump golang.org/x/crypto 0.45.0 → 0.52.0 (CVE-2026-39831..39834, CVE-2026-42508, CVE-2026-39829, CVE-2026-46597, CVE-2026-46595)
- chore: migrate github.com/docker/docker → github.com/moby/moby/client v0.4.1 (CVE-2026-34040)

---

# CPM v3.1.2 - Security Fixes & Code Quality

## 🔒 Security Fixes

- **Race condition** (internal/services/auth.go) — ValidateSession was deleting from session map under RLock; fixed with double-checked locking
- **Path traversal / zip-slip** (internal/services/backup.go) — ZIP restore now validates all extracted paths stay within target directory using filepath.Abs + prefix check
- **CSRF protection** — Fiber CSRF middleware added, all HTML forms include _csrf hidden field, HTMX requests inject token via configRequest header
- **Brute-force login protection** — in-memory rate limiter, max 5 failed attempts per IP per 15-minute sliding window, returns HTTP 429

## 🐛 Bug Fixes

- **Dashboard certificate expiry** (internal/handlers/dashboard.go) — formatDaysLeft() was returning a Unicode control character instead of a number; fixed with fmt.Sprintf

## ♻️ Refactoring

- **Deduplicated contains()** — 3 identical copies replaced with generic utils.Contains[T comparable] in new internal/utils package
- **Centralized session cookie name** — "cpm_session" literal centralized as utils.SessionCookieName
- **Consistent logging** — 7 fmt.Printf calls in services replaced with log.Printf
- **JSON unmarshal errors** — 5 ignored json.Unmarshal errors in snippets.go now propagate properly
- **Wildcard domain matching** — strings.Contains replaced with per-token strings.HasSuffix to eliminate false positives

---

# CPM v3.1.1 - Security Update & Multi-platform

## 🔒 Security Updates

- **Go 1.26** - builder upgraded from Go 1.25, includes security fixes in net/http, html/template, crypto/tls and other packages
- **Fiber v2.52.13** - fixes CVE-2025-66630 (9.2 critical) and CVE-2026-25882
- **Docker SDK v28.5.2** - updated from v27.4.1

## 🏗️ Multi-platform Support

Docker image is now built for both amd64 and arm64:
- Intel/AMD servers and NAS devices (Synology DS220+, etc.)
- Apple Silicon (M1/M2/M3/M4)
- Raspberry Pi and other ARM devices

```bash
docker pull perteus/caddy-ui:3.1.1
```

## ✨ New Features

### Internal-only Restrictions
Sites can now be restricted to internal network access only.
For wildcard sites, the restriction is applied at the wildcard block level.

## 📝 Version History

| Version | Date | Notes |
|---------|------|-------|
| **3.1.1** | 2026-05-16 | 🔒 Security update, Go 1.26, multi-platform |
| **3.1.0** | 2026-01-15 | 🔐 Wildcard refactor, new architecture |
| 3.0.2 | 2026-01-11 | 🐛 Wildcard TLS fix, parser fix |
| 3.0.1 | 2026-01-09 | 🔐 Wildcard SSL, migration tools |
| 3.0.0 | 2026-01-07 | 🎉 Complete Go rewrite |

---

# CPM v3.1.0 - Wildcard Refactor

## 🚀 Major Changes

### Wildcard Architecture Refactor
The wildcard certificate handling has been completely rewritten to work correctly with Caddy.

**Previous (broken) approach:**
```
# Each site file - caused individual certificate requests
home.perteus.cz {
    import wildcard-tls-perteus-cz
    reverse_proxy ...
}
```

**New (correct) approach:**
```
# Caddyfile - single wildcard block
*.perteus.cz {
    import wildcard-tls-perteus-cz
    import /etc/caddy/sites/wildcard/*.perteus.cz.caddy
    handle_errors { ... }
    handle { abort }
}

# Site file - handle block only
@home_perteus_cz host home.perteus.cz
handle @home_perteus_cz {
    reverse_proxy http://192.168.50.159:8123
}
```

### New Directory Structure
```
sites/
├── wildcard/           # Handle blocks for wildcard sites
│   └── *.domain.caddy
├── standard/           # Classic domain {} blocks
│   └── domain.caddy
└── *.caddy             # Legacy (still supported)
```

### Automatic Caddyfile Management
CPM now generates and manages the main Caddyfile with:
- Wildcard blocks for each configured wildcard domain
- Internal network restrictions at wildcard level
- Error pages (403, 404) at wildcard level
- Proper snippet imports

## 🔧 Improvements

### Better Error Reporting
- Reload and validate operations now return detailed output
- `ReloadResult` includes `ValidationLog` and `ReloadLog` fields
- Error messages from Caddy are properly captured and displayed

### Internal-Only Handling
- For wildcard sites: Handled at wildcard block level (not per-site)
- For standard sites: Still uses `internal_only` snippet
- Prevents nested handle block issues

### Site File Format Detection
- Parser automatically detects wildcard vs standard format
- Supports both `@matcher host domain.com` and `domain.com { }` formats
- Backward compatible with existing site files

## ⚠️ Migration Notes

### Automatic Migration
When adding a wildcard domain, CPM will:
1. Create the wildcard block in Caddyfile
2. Offer to migrate existing sites to new format
3. Move site files to `sites/wildcard/` directory

### Manual Migration
For existing installations:
1. Go to Settings → Wildcard SSL
2. Remove and re-add your wildcard domains
3. Use "Migrate" button for each domain

### Backup First!
Always create a backup before migrating:
- Settings → Backup → Create Backup
- Or manually: `cp -r caddy-config caddy-config.backup`

## 🐛 Bug Fixes

- Fixed: Wildcard sites were requesting individual certificates
- Fixed: Internal-only caused nested handle block errors
- Fixed: handle_errors not working in wildcard sites
- Fixed: Parser corruption when editing wildcard sites
- Fixed: Reload not returning detailed error information

## 📝 Version History

| Version | Date | Notes |
|---------|------|-------|
| **3.1.0** | 2026-01-15 | 🔐 Wildcard refactor, new architecture |
| 3.0.2 | 2026-01-11 | 🐛 Wildcard TLS fix, parser fix |
| 3.0.1 | 2026-01-09 | 🔐 Wildcard SSL, migration tools |
| 3.0.0 | 2026-01-07 | 🎉 Complete Go rewrite |
