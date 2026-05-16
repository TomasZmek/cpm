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
