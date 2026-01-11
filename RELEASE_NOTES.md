# 🚀 CPM v3.0.3 Release Notes

## 🐛 Bug Fixes

### v3.0.2 - HTTP Method Fix
- **Fixed: 405 Method Not Allowed on Site/Snippet Update**
  - Issue: Editing existing proxy rules or snippets returned 405 error
  - Cause: HTML forms only support GET/POST methods, but routes expected PUT/DELETE
  - Solution: Converted PUT routes to POST for site and snippet updates
- **Fixed: Wildcard TLS Certificate Import Error**
  - Issue: Sites using wildcard TLS showed `File to import not found: wildcard-tls-*` error
  - Cause: Wildcard TLS snippets were saved to a separate `_wildcard.caddy` file that wasn't properly loaded
  - Solution: Wildcard TLS snippets are now integrated into the main `snippets.caddy` file
  - This ensures proper loading order and snippet availability

---

## 🔐 Wildcard SSL Management (v3.0.1)

The headline feature is comprehensive wildcard SSL certificate support:

### Wildcard Features
- **Settings → Wildcard SSL** - Dedicated section for managing wildcard certificates
- **DNS Challenge Configuration** - Easy setup with Cloudflare (more providers coming)
- **Auto-detection** - When creating new proxy rules, CPM automatically detects and suggests available wildcard certificates
- **Bulk Migration Tool** - Migrate all existing sites to use wildcard with one click
- **Certificate Cleanup** - Option to delete individual certificates after migration

### How It Works
1. Add a wildcard domain (e.g., `zrnek.cz` for `*.zrnek.cz`)
2. Configure DNS provider (Cloudflare API token)
3. CPM generates TLS snippet: `import wildcard-tls-zrnek-cz`
4. New sites automatically use wildcard when domain matches

---

## 🎨 UI Improvements (v3.0.1)

### SweetAlert2 Dialogs
- Beautiful confirmation dialogs replace ugly browser `confirm()` popups
- Localized buttons (English/Czech)
- Consistent styling across the app

### Certificate Management
- **Split Actions**: Separate "Renew" and "Delete" buttons with clear explanations
- **Info Box**: Explains the difference between renewing and deleting certificates

### Form Enhancements
- **TLS Mode Selector**: New dropdown in site form to choose between wildcard and automatic certificates
- **Smart Defaults**: Auto-selects wildcard when domain matches available wildcards

---

## 🔄 Migration from v3.0.x

No breaking changes. Simply update your Docker image:

```bash
docker pull perteus/caddy-ui:3.0.3
docker-compose up -d
```

After update, you may need to:
1. Go to **Settings → Snippets** and click Save (to regenerate snippets.caddy with wildcard TLS)
2. Or go to **Settings → Wildcard SSL** and re-add your wildcard domain

Old `_wildcard.caddy` files in `/etc/caddy/sites/` are no longer used and can be removed:
```bash
docker exec caddy rm -f /etc/caddy/sites/_wildcard.caddy
```

---

## 📊 Image Size

| Version | Size |
|---------|------|
| v3.0.3 | ~6 MB |
| v2.2.1 (Python) | ~800 MB |

---

**Full Changelog**: https://github.com/TomasZmek/cpm/compare/v3.0.1...v3.0.3
