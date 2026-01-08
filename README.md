# CPM - Caddy Proxy Manager

<p align="center">
  <img src="web/static/img/logo.svg" alt="CPM Logo" width="200">
</p>

<p align="center">
  <strong>Lightweight web UI for managing Caddy reverse proxy</strong>
</p>

<p align="center">
  <a href="https://github.com/TomasZmek/cpm/releases"><img src="https://img.shields.io/github/v/release/TomasZmek/cpm" alt="Release"></a>
  <a href="https://github.com/TomasZmek/cpm/blob/main/LICENSE"><img src="https://img.shields.io/github/license/TomasZmek/cpm" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/TomasZmek/cpm"><img src="https://goreportcard.com/badge/github.com/TomasZmek/cpm" alt="Go Report Card"></a>
</p>

---

## ✨ Features

### 📊 Dashboard
- System overview with stats and alerts
- Certificate expiration warnings
- Recent changes tracking
- Quick actions (reload, validate, backup)

### 🔀 Proxy Rules Management
- Create, edit, delete reverse proxy rules
- Visual form editor with HTMX interactivity
- Raw Caddyfile editing for advanced users
- Duplicate rules with one click
- Tag-based organization
- Pre-configured templates for popular services

### 📋 Service Templates
- 17+ pre-configured templates
- Categories: Web, Media, Docker, Dev, Monitoring, Home, NAS, API
- Quick setup for Nextcloud, Jellyfin, Portainer, Home Assistant, and more

### ⚙️ Snippets Manager
- Visual configuration for shared snippets
- Cloudflare DNS challenge
- Internal network restrictions
- Security headers
- Compression settings
- Rate limiting
- Basic authentication

### 📜 SSL Certificates
- Certificate overview with status
- Expiration alerts (30/7 days)
- Force renewal by deleting certificates

### 👥 Multi-User Authentication
- Role-based access control (Admin, Editor, Viewer)
- Session management
- Bcrypt password hashing

### 💾 Backup & Restore
- Full configuration backup (ZIP)
- Import/Export rules as JSON

### 🌐 Internationalization
- English
- Czech (Čeština)

---

## 🚀 Quick Start

### Docker Compose

```yaml
services:
  caddy:
    image: caddy:2-alpine
    container_name: caddy
    ports:
      - "80:80"
      - "443:443/tcp"
      - "443:443/udp"
    volumes:
      - ./caddy-config/Caddyfile:/etc/caddy/Caddyfile
      - ./caddy-config/snippets.caddy:/etc/caddy/snippets.caddy
      - ./caddy-config/sites:/etc/caddy/sites
      - ./caddy-data:/data

  cpm:
    image: ghcr.io/tomaszmek/cpm:latest
    container_name: cpm
    ports:
      - "8080:8080"
    environment:
      - CONTAINER_NAME=caddy
    volumes:
      - ./caddy-config:/caddy-config
      - ./caddy-data:/caddy-data
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP port | `8080` |
| `CONTAINER_NAME` | Caddy container name | `caddy` |
| `CADDY_CONFIG_PATH` | Path to Caddy config | `/caddy-config` |
| `CADDY_DATA_PATH` | Path to Caddy data | `/caddy-data` |
| `DEFAULT_IP` | Default target IP | `192.168.1.1` |
| `THEME` | UI theme | `classic` |

---

## 📁 Required Folder Structure

```
caddy-config/
├── Caddyfile              # Main Caddy configuration
├── snippets.caddy         # Shared snippets (managed by CPM)
├── .snippets_config.json  # Snippets configuration
├── sites/                 # Proxy rules
│   ├── example.com.caddy
│   └── fallback.caddy
└── pages/                 # Custom error pages
    ├── 403.html
    └── 404.html

caddy-data/
└── caddy/
    └── certificates/      # SSL certificates
```

---

## 🏗️ Building from Source

### Prerequisites

- Go 1.22+
- Make (optional)

### Build

```bash
# Clone repository
git clone https://github.com/TomasZmek/cpm.git
cd cpm

# Build
make build
# or
go build -o bin/cpm ./cmd/cpm

# Run
./bin/cpm
```

### Docker Build

```bash
make docker-build
# or
docker build -t cpm:latest .
```

---

## 📚 API

CPM provides a REST API for automation:

```bash
# Get all sites
GET /api/v1/sites

# Get status
GET /api/v1/status

# Reload Caddy
POST /api/v1/reload
```

---

## 🎨 Theming

CPM supports multiple themes:

- **Classic** - Default theme
- **Modern** - Coming soon

Themes can be changed in Settings or via the `THEME` environment variable.

---

## 📝 Changelog

### v3.0.0 (2026-01-07)

- Complete rewrite in Go
- Lightweight Docker image (~20MB vs 800MB)
- HTMX-powered interactive UI
- Service templates (17+ services)
- Multi-user authentication
- Improved performance

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

---

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

---

## 🙏 Acknowledgments

- Built with [Go](https://golang.org/)
- Web framework: [Fiber](https://gofiber.io/)
- Interactivity: [HTMX](https://htmx.org/)
- Styling: [Tailwind CSS](https://tailwindcss.com/)
- Developed with assistance from [Claude AI](https://claude.ai)

---

<p align="center">
  <strong>CPM - Caddy Proxy Manager</strong><br>
  Made with ❤️ for home labs
</p>
