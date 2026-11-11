# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**CPM - Caddy Proxy Manager** — a lightweight Go web UI for managing a Caddy reverse proxy. Distributed as a ~6 MB Docker image (`perteus/caddy-ui` on Docker Hub). Current released version: **3.7.0**.

- GitHub: https://github.com/TomasZmek/cpm
- Docker Hub: `perteus/caddy-ui`

### Branches

- `main` — released code (what is on Docker Hub)
- `develop` — integration branch; all work and PRs should target `develop`, which is merged into `main` for a release. Keep `develop` up to date with `main` by merging (no rebase / force-push).

## Commands

```bash
# Build binary
go build -o bin/cpm ./cmd/cpm

# Run locally (requires caddy-config/ and caddy-data/ directories)
./bin/cpm

# Hot reload dev (requires air)
make dev

# Tests
go test -v ./...
go test -v ./internal/services/...   # single package

# Format
gofmt -s -w .

# Docker build & push for release — see Release process section for multi-platform buildx command
```

Version is authoritative in `cmd/cpm/main.go` constants `Version` and `BuildDate`; the Makefile reads `VERSION` from there.

## Tech stack

- **Go 1.26** (module: `github.com/TomasZmek/cpm`)
- **Fiber v3** — HTTP framework (`fiber.Ctx` is an interface: handlers take `c fiber.Ctx`; redirects are `c.Redirect().To()`, view bindings `c.ViewBind()`, the fasthttp context `c.RequestCtx()`)
- **gofiber/template/html** — HTML template engine (Go `html/template`-compatible)
- **HTMX** — frontend interactivity (no build step, CDN)
- **SweetAlert2** — dialogs
- **github.com/leonelquinteros/gotext** — i18n library
- Templates live in `templates/themes/<theme>/` (`classic` = default, `modern` = optional redesign with dark mode), stylesheets in `web/static/css/themes/<theme>.css`, other static assets in `web/static/`
- i18n: English, Czech & Korean (ja, zh, es, de, fr exist but are hidden until complete) — PO/MO format via `github.com/leonelquinteros/gotext`, files in `internal/i18n/locales/{en,cs,ko}/LC_MESSAGES/messages.po`, embedded via `embed.FS`
- **github.com/moby/moby/client** — Docker API client (`api/pkg/stdcopy` for demultiplexing exec/log streams)

## Architecture

### Request flow

```
main.go → Fiber middleware stack → handlers.Handler → services → file system / Docker API
```

All services are constructed in `main.go` and injected into `handlers.Handler` as a single struct. There is no DI framework.

### Key services

| Service | File | Responsibility |
|---|---|---|
| `CaddyService` | `services/caddy.go` | CRUD for site `.caddy` files, reload/validate |
| `CaddyfileManager` | `services/caddyfile.go` | Generates the main `Caddyfile` with wildcard blocks |
| `ParserService` | `services/parser.go` | Parses `.caddy` files into `models.Site` |
| `WildcardService` | `services/wildcard.go` | Manages wildcard domain config (DNS provider, API token) |
| `SnippetsService` | `services/snippets.go` | Manages shared snippets in `snippets.caddy` |
| `DockerService` | `services/docker.go` | Runs `caddy reload` / `caddy validate` inside the Caddy container, reads logs, lists containers |
| `AuthService` | `services/auth.go` | Users, roles, in-memory sessions |
| `BackupService` | `services/backup.go` | ZIP backup/restore, JSON import/export of rules |
| `CertificateService` | `services/certificates.go` | Reads/deletes certificates from Caddy data dir |
| `SettingsService` | `services/settings.go` | App settings (`settings.json`), Docker discovery hosts, access log settings |
| `AccessLogService` | `services/accesslog.go` | Reads Caddy's JSON access log (`<data>/cpm-logs/access.log` + rotated `.gz`), filters by rule hosts/status/text, 24 h stats; also the `cpm_access_log` snippet and `SyncAccessLogImports` |
| Discovery | `services/discovery.go` | Docker Auto-Discovery — turns inventory containers into rows paired with proxy rules |
| `modules.Manager` | `modules/manager.go` | Registered modules, enable/configure/disable at runtime, `caddy-config/modules.json` (0600) |
| `inventory.Service` | `inventory/inventory.go` | Background refresh (30 s) of containers from active modules + TCP probe; backend status of rules |

### Modules (`sdk/`, `internal/modules`, `internal/inventory`)

`sdk/` is a separate, dependency-free Go module (`github.com/TomasZmek/cpm/sdk`, `replace`d locally in `go.mod`) — the public interface for modules: `sdk.Register(sdk.Module{...})` from `init`, `Source.Containers(ctx)`, `Field` (settings form is generated from it), `Text` (per-module translations). Modules are compiled in (blank import in `cmd/cpm/modules.go` or `--build-arg CPM_MODULES=...`) and enabled at runtime in Settings → Modules. The built-in `docker` module (`internal/modules/dockerlocal`) wraps `DockerService.ListContainers`; `modules/dockhand/` (own `go.mod`, `replace`d in the root `go.mod`, copied in the `Dockerfile` before `go mod download`) reads Dockhand's REST API. `go test ./...` in the root does not cover separate modules: also run it in `sdk/` and `modules/*/`. Dashboard backend status, the rules list dots (`/htmx/sites/status`), Docker discovery and the rule form's container picker read **only** the inventory, never Docker directly — `DockerService` itself is for controlling Caddy (reload/validate/restart/logs). Matching: container name, or Docker host IP + **published** port. See `docs/modules.md`.

`CaddyfileManager` depends on both `WildcardService` and `SnippetsService`; to avoid circular imports it is created after them and injected into `CaddyService` via `SetCaddyfileManager()`.

### Wildcard architecture (v3.1.0+)

This is the most important architectural concept. Two site formats coexist:

**Standard sites** (`sites/standard/*.caddy`) — classic Caddyfile block:
```
domain.com {
    import cloudflare_dns
    reverse_proxy 192.168.1.1:8080
}
```

**Wildcard sites** (`sites/wildcard/*.domain.caddy`) — handle blocks only (no TLS, no `{}`):
```
# @tls: wildcard:domain.com
@matcher_name host sub.domain.com
handle @matcher_name {
    reverse_proxy 192.168.1.1:8080
}
```

The main `Caddyfile` is **auto-generated** by `CaddyfileManager.GenerateCaddyfile()` and contains the wildcard `*.domain.com { }` blocks that `import` these handle-block files. The `Caddyfile` should not be edited manually.

`Site.IsWildcard()` checks `TLSMode` prefix (`"wildcard:domain.com"`). `ParserService.isWildcardHandleFormat()` detects the format by looking for `@matcher host domain` pattern. Both formats are backward-compatible — legacy flat `sites/*.caddy` files are still scanned.

`internal_only` (network restriction) must be at the wildcard block level in `Caddyfile`, not per-site handle block, to avoid nested handle block errors. For standard sites it can be a per-site snippet import.

### Caddyfile directory structure (on disk, not in repo)

```
caddy-config/
├── Caddyfile              # auto-generated by CPM
├── snippets.caddy         # wildcard TLS snippets + shared snippets
├── .snippets_config.json  # snippet settings (may contain API token, 0600)
├── wildcard.json          # wildcard domains + DNS API tokens (0600)
├── .auth_config.json      # users (bcrypt hashes) + auth enabled flag (0600)
├── settings.json          # app settings, Docker discovery hosts
├── modules.json           # enabled modules + their settings (may contain tokens, 0600)
├── pages/                 # custom 403/404 error pages
└── sites/
    ├── wildcard/          # handle blocks for wildcard sites
    └── standard/          # standard domain blocks
```

**Access log** (Settings → Logs, off by default): while enabled, `GenerateCaddyfile` defines the `cpm_access_log` snippet (`log { output file /data/cpm-logs/access.log … }`) and imports it in every wildcard block; standard site files get `import cpm_access_log` after their address line (`Site.AccessLog`, set by `CaddyService` before writing, and `SyncAccessLogImports` when the setting changes). While disabled, neither the snippet nor the imports exist, so files stay as before. Per rule, `NoAccessLog` / `LogSkipPaths` generate `log_skip` (a `log` directive cannot be inside `handle`, `log_skip` can).

In the Caddy data dir CPM reads `cpm-logs/` (written by Caddy) and only touches `caddy/certificates/` and `cpm-cert-backups/`: renewing or deleting a certificate moves its files (incl. private key, 0600) into a backup there (`services/cert_backups.go`), and renewal/restore restart the Caddy container — a reload keeps certificates cached in memory.

### Authentication

Session-based, managed by `AuthService` (`services/auth.go`). Users stored in `caddy-config/.auth_config.json`; sessions are in memory only (lost on restart). When auth is disabled, everything is open. The first POST to `/login` with no users creates the initial admin (`CreateInitialAdmin`) and enables auth.

`middleware.Auth` protects all routes except `/login`, `/logout`, `/health` and `/static`. The API (`/api/v1/*`) uses the same session cookie and is also subject to CSRF.

Roles are enforced per route in `setupRoutes` (`cmd/cpm/main.go`) via `middleware.RequirePermission(authService, perm)`, with permissions from `models.User.HasPermission`:

| Permission | Roles | Used for |
|---|---|---|
| `view` | Viewer, Editor, Admin | all read-only pages |
| `edit` | Editor, Admin | create/edit/delete rules, snippets, certificates, reload/validate, discovery create |
| `admin` | Admin | users, auth toggle, backup/restore, import/export, wildcard SSL, Docker settings |

**Every new route must be given the right permission.** Settings tabs reachable via `?tab=` are additionally checked in `renderSettingsTab` (`h.hasPermission`). The last admin cannot be deleted or demoted; password change and user deletion invalidate that user's sessions.

### Security conventions

- **Generated Caddyfile content**: any user value written into a `.caddy` file must be validated so it cannot inject directives — `Site.Validate()` (`models/site.go`), `ValidateDomainName` / `ValidateAPIToken` (`services/wildcard.go`), network and header checks in `handlers/snippets.go`. `ExtraConfig` and raw mode are intentionally free-form (Editor+).
- **File paths**: site filenames from URL params, forms or imports go through `ValidateSiteFilename` (`services/caddy.go`). Backup restore accepts only paths that `CreateBackup` produces.
- **Output**: HTML built by hand in handlers must use `escapeHTML`; redirects to the Referer go through `safeRedirectTarget`.
- **Secrets**: files containing tokens or password hashes are written with `0600`.
- **CSRF**: forms include `<input type="hidden" name="_csrf" value="{{.CSRFToken}}">`; HTMX/fetch requests send the `X-CSRF-Token` header (set globally in `layouts/base.html`). The token is `csrf.TokenFromContext(c)`. Fiber v3 also checks that unsafe requests come from the page's own origin (`Origin`/`Referer` against scheme + host). Behind a reverse proxy that terminates TLS (Caddy) the scheme/host come from `X-Forwarded-*`, honoured only for `TRUSTED_PROXIES` (env, default `private,loopback`); `TRUSTED_ORIGINS` adds allowed origins. App and middleware setup live in `newApp` (`cmd/cpm/main.go`), tested in `cmd/cpm/app_test.go` incl. the Caddy cases. A rejected token (e.g. after a restart, tokens are in memory) redirects back with a flash (`handlers.CSRFError`).

### UI themes

Two complete template sets: **Classic** (default) and **Modern** (optional, from PR #19: light/dark mode, accent colours). `internal/views.Themed` holds one template engine per theme; `middleware.Theme` reads the `cpm_theme` cookie (default: `THEME` env) and binds it via `c.Bind`, so every `c.Render(...)` picks the right set. Users switch in Settings → General.

On narrow screens both layouts switch the sidebar to an off-canvas drawer opened from `.mobile-topbar` (Modern below 1024 px, Classic below 768 px; behaviour in `web/static/js/app.js`). Check new pages at phone width (~390 px) — the page must not scroll horizontally. Settings uses list–detail navigation on phones (≤767 px): `/settings` shows the section list, `/settings/<section>` shows one section with a back button and the section title in the header; wider screens show tabs. Sections come from `settingsSections` in `handlers/settings.go` (with the permission each needs), so a new settings section is added there, not in the templates.

**A UI feature must be implemented in both themes.** Handlers pass the same data to both; only the markup differs. Themes are registered in `middleware/theme.go` (`AvailableThemes`, `themeOrder`).

### Flash messages

Handlers call `setFlash(c, type, msg)` (`handlers/handlers.go`), which queues Fiber's built-in flash messages (`c.Redirect().With`, hex-encoded cookie, safe for any language), and **must then return `redirectTo(c, url)`** — the flash cookie is only written by the redirect; `redirectTo` answers HTMX requests with 200 + `HX-Redirect`. Page handlers read them with `getFlash(c)`, pass them as `FlashType` / `FlashMessage`, and `layouts/base.html` renders them. Use `tl(c, key, args...)` for translated messages.

## Tests

Tests live next to the code (`*_test.go`): site validation (`models`), filename/domain validation, backup/import path traversal, auth rules and Docker stream demux (`services/security_test.go`), safe redirects (`handlers`). Run `go test ./...` and `go vet ./...` before committing; there is no CI yet. Before a release, scan for known vulnerabilities: `go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...` (and `govulncheck -mode=binary <binary>` built with the Go version from the `Dockerfile`), and keep the Go version in `Dockerfile` / `Dockerfile.alpine` on the latest patch release.

## Commit convention

```
type: description
```
Types: `feat`, `fix`, `docs`, `refactor`, `chore`

## Release process

1. Update `Version` and `BuildDate` constants in `cmd/cpm/main.go`
2. Update `RELEASE_NOTES.md`: rename the `# CPM vVERSION - Unreleased` heading to `# CPM vVERSION - <title>` and add the release date
3. Update version badges in `README.md` and `DOCKERHUB.md`
4. Build and push Docker image (multi-platform amd64 + arm64):
   ```bash
   # Multi-platform build a push — jediný správný způsob
   docker buildx create --name multibuilder --use --bootstrap 2>/dev/null || \
     docker buildx use multibuilder
   docker buildx build \
     --platform linux/amd64,linux/arm64 \
     --push \
     --no-cache \
     -t perteus/caddy-ui:VERSION \
     -t perteus/caddy-ui:latest \
     .
   ```
   Nebo přes Makefile: `make docker-build`

   POZNÁMKA: `buildx --push` pushuje přímo během buildu. `make docker-push` není potřeba volat zvlášť.
5. Commit with `chore: bump version to vVERSION`, merge into `main`, then `main` back into `develop`
6. Run the **Release** workflow (GitHub → Actions → Release → Run workflow, version without `v`, on `main`). It checks that `cmd/cpm/main.go` has that version, pushes the multi-arch image to `ghcr.io/tomaszmek/cpm:VERSION` and `:latest`, and creates the GitHub release `vVERSION` (tag included) with the notes from `RELEASE_NOTES.md` and offline packages `cpm-VERSION-amd64.tar` / `-arm64.tar` (`docker load`, tagged `perteus/caddy-ui:VERSION`). Pushing a `v*` tag runs it too. With the repository secrets `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` it also pushes `perteus/caddy-ui:VERSION` + `:latest` (multi-arch) and updates the Docker Hub description from `DOCKERHUB.md` — then step 4 is not needed.

### Test builds of `develop`

Every push to `develop` runs the **Develop build** workflow (`.github/workflows/develop.yml`): multi-arch images `perteus/caddy-ui:develop`, `:NEXT-dev` and `:develop-<short sha>` (Docker Hub) and the same tags on `ghcr.io/tomaszmek/cpm`, never `latest`, plus a GitHub pre-release `vNEXT-dev` (replaced on every run, not marked latest; older `-dev` pre-releases are removed) with offline packages and the Unreleased section of `RELEASE_NOTES.md`. NEXT is the version being prepared, taken from the `# CPM vNEXT - Unreleased` heading at the top of `RELEASE_NOTES.md` (falls back to the next patch version); that section ends with `---` like the others. Never create a tag named `develop` — it makes `git push origin develop` ambiguous. Use these for testing instead of hand-built images.

## i18n

Translations use gettext PO format via `github.com/leonelquinteros/gotext`.

**Adding a new language:**
1. Create `locales/{lang}/LC_MESSAGES/messages.po` with correct plural forms header
2. Copy all msgid keys from `locales/en/LC_MESSAGES/messages.po`
3. Add `"{lang}": "Language Name"` to `AvailableLanguages`, the code to `languageOrder` and a plural rule to `pluralRules` in `internal/i18n/i18n.go`
4. Run `go test ./internal/i18n/` — it lists missing keys and checks that the `Plural-Forms` header matches `pluralRules`

A language is only offered in the UI (and accepted from cookie / `Accept-Language`) once it translates at least `MinCoverage` (95 %) of the English keys; incomplete languages stay hidden. Every new UI key must be added to en, cs and ko.

**Plural strings** use `i18n.TN(lang, singular, plural, n)` in Go and `{{tn .Lang "singular" "plural" .Count}}` in templates.
