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
