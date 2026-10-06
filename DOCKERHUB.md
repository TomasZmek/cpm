# CPM - Caddy Proxy Manager

<p align="center">
  <img src="https://raw.githubusercontent.com/TomasZmek/cpm/main/web/static/img/logo.svg" alt="CPM Logo" width="180">
</p>

<p align="center">
  <strong>🚀 Lightweight web UI for managing Caddy reverse proxy</strong><br>
  Wildcard SSL • Docker Auto-Discovery • One-click rule creation
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-3.3.1-blue" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/image_size-~6MB-green" alt="Image Size">
  <img src="https://img.shields.io/badge/platforms-amd64%20%7C%20arm64-lightgrey" alt="Platforms">
</p>

---

## 📸 Screenshots

<p align="center">
  <img src="https://raw.githubusercontent.com/TomasZmek/cpm/main/img/dashboard.png" alt="CPM Dashboard" width="800">
  <br><em>Dashboard — system overview, alerts, quick actions</em>
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/TomasZmek/cpm/main/img/edit-rules.png" alt="CPM Proxy Rules" width="800">
  <br><em>Proxy Rules — visual editor for reverse proxy configuration</em>
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/TomasZmek/cpm/main/img/settings.png" alt="CPM Settings" width="800">
  <br><em>Settings — language, theme, wildcard SSL, Docker discovery, users</em>
</p>

<p align="center">
  <img src="https://raw.githubusercontent.com/TomasZmek/cpm/main/img/certificates.png" alt="CPM Certificates" width="800">
  <br><em>Certificates — SSL overview with expiration status</em>
</p>

---

## ✨ Features

| Feature | Description |
|---------|-------------|
| 📊 **Dashboard** | System overview, stats, alerts |
| 🔀 **Proxy Rules** | Visual editor for reverse proxy |
| 🔐 **Wildcard SSL** | Manage wildcard certificates with DNS challenge |
| ⚙️ **Snippets** | Cloudflare DNS, security headers, rate limiting |
| 📜 **Certificates** | SSL overview with expiration alerts |
| 👥 **Multi-User** | Role-based access control |
| 💾 **Backup** | Full config backup & restore |
| 🌐 **i18n** | English, Czech & Korean |
| 🏠 **Internal-only** | Restrict sites to internal network only |
| 🐳 **Docker Auto-Discovery** | Automatic container detection with one-click rule creation |

---

## 🚀 Quick Start

```bash
docker pull perteus/caddy-ui:latest
docker pull perteus/caddy-ui:3.2.0
```

### Docker Compose

```yaml
version: '3.8'
services:
  caddy:
    image: serfriz/caddy-cloudflare:latest
    container_name: caddy_proxy
    environment:
      - CF_API_TOKEN=${CF_API_TOKEN}
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./caddy-config:/etc/caddy
      - ./caddy-data:/data

  cpm:
    image: perteus/caddy-ui:3.2.0
    container_name: cpm
    ports:
      - "8501:8501"
    environment:
      - CONTAINER_NAME=caddy_proxy
      - DEFAULT_IP=192.168.1.100
    volumes:
      - ./caddy-config:/caddy-config
      - ./caddy-data:/caddy-data
      - /var/run/docker.sock:/var/run/docker.sock
```

### Synology NAS

Pro Synology přidej `privileged: true` pro přístup k Docker socketu.

```yaml
cpm:
  image: perteus/caddy-ui:3.2.0
  privileged: true
  volumes:
    - /volume1/docker/caddy-config:/caddy-config
    - /volume1/docker/caddy-data:/caddy-data
    - /var/run/docker.sock:/var/run/docker.sock
```

---

## ⚙️ Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8501` | HTTP port |
| `CONTAINER_NAME` | `caddy` | Caddy container name |
| `DEFAULT_IP` | `192.168.1.1` | Default target IP |
| `CF_API_TOKEN` | - | Cloudflare API token (for wildcard SSL) |

---

## 📝 Version History

| Version | Notes |
|---------|-------|
| **3.3.1** | 🔒 Security patch — Go 1.26.4 (CVE-2026-42504, CVE-2026-27145, CVE-2026-42507) |
| **3.3.0** | 🌐 i18n refactor — PO/MO format, Korean language, plural support infrastructure |
| **3.1.0** | 🔐 Wildcard refactor - new architecture, handle blocks |
| **3.0.2** | 🐛 Wildcard TLS fix, parser fix, 405 fix |
| **3.0.1** | 🔐 Wildcard SSL, migration tools, UI improvements |
| **3.2.0** | 🐳 Docker Auto-Discovery — automatic container detection, multi-host support |
| **3.1.3** | 🔒 Security patch — CVE fixes, migrace na moby/moby/client |
| **3.1.2** | 🔒 Security fixes, CSRF, race condition, path traversal |
| **3.1.1** | 🔐 Internal-only restrictions, Go 1.26, multi-platform (amd64/arm64) |
| **3.1.0** | 🔐 Wildcard refactor, new architecture |
| **3.0.0** | 🎉 Complete Go rewrite (794MB → 6MB) |
| 2.x | Python version (deprecated) |

### v3.2.0 - Docker Auto-Discovery
- ✅ **Auto-Discovery** — automatic detection of running containers
- ✅ **Multi-host support** — monitor multiple Docker hosts
- ✅ **Smart pairing** — match existing proxy rules with containers
- ✅ **One-click rule creation** — pre-filled from discovered containers
- ✅ **Local IP autodetection** — detects host IP automatically

---

### v3.1.3 - Security Patch
- ✅ **CVE fixes** — bump golang.org/x/crypto 0.45.0 → 0.52.0
- ✅ **Docker SDK migration** — github.com/docker/docker → github.com/moby/moby/client (trvalé řešení)
- ✅ **Multi-platform** — native amd64 + arm64

---

## 🔗 Links

- 📦 **Source Code**: [github.com/TomasZmek/cpm](https://github.com/TomasZmek/cpm)
- 🐛 **Report Bugs**: [GitHub Issues](https://github.com/TomasZmek/cpm/issues)
- 💡 **Feature Requests**: [GitHub Discussions](https://github.com/TomasZmek/cpm/discussions)

---

## 🙏 Acknowledgments

- 🇰🇷 Korean translation: [@redstar-programmer](https://github.com/redstar-programmer)

---

## 📄 License

MIT License

---

<p align="center">
  <strong>CPM - Caddy Proxy Manager</strong><br>
  Made with ❤️ for home labs
</p>
