# CPM - Caddy Proxy Manager

<p align="center">
  <strong>🚀 Lightweight web UI for managing Caddy reverse proxy</strong><br>
  Wildcard SSL • Auto-detection • One-click migration
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-3.1.0-blue" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/image_size-~6MB-green" alt="Image Size">
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
| 🌐 **i18n** | English & Czech |

---

## 🚀 Quick Start

```bash
docker pull perteus/caddy-ui:latest
docker pull perteus/caddy-ui:3.1.0
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
    image: perteus/caddy-ui:3.1.0
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

For Synology, add `privileged: true` for Docker socket access.

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
| **3.1.0** | 🐛 Wildcard TLS fix, parser fix, 405 fix |
| **3.0.1** | 🔐 Wildcard SSL, migration tools, UI improvements |
| **3.0.0** | 🎉 Complete Go rewrite (794MB → 6MB) |
| 2.x | Python version (deprecated) |

### v3.1.0 Bug Fixes
- ✅ **Wildcard TLS snippets** now correctly generated in `snippets.caddy`
- ✅ **Parser fix** - comments no longer parsed as domains
- ✅ **405 Method Not Allowed** - fixed site/snippet update forms

---

## 🔗 Links

- 📦 **Source Code**: [github.com/TomasZmek/cpm](https://github.com/TomasZmek/cpm)
- 🐛 **Report Bugs**: [GitHub Issues](https://github.com/TomasZmek/cpm/issues)
- 💡 **Feature Requests**: [GitHub Discussions](https://github.com/TomasZmek/cpm/discussions)

---

## 📄 License

MIT License

---

<p align="center">
  <strong>CPM - Caddy Proxy Manager</strong><br>
  Made with ❤️ for home labs
</p>
