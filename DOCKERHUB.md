# CPM - Caddy Proxy Manager

<p align="center">
  <strong>🚀 Lightweight web UI for managing Caddy reverse proxy</strong><br>
  Wildcard SSL • Auto-detection • One-click migration
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-3.1.3-blue" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/image_size-~6MB-green" alt="Image Size">
  <img src="https://img.shields.io/badge/platforms-amd64%20%7C%20arm64-lightgrey" alt="Platforms">
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
| 🏠 **Internal-only** | Restrict sites to internal network only |

---

## 🚀 Quick Start

```bash
docker pull perteus/caddy-ui:latest
docker pull perteus/caddy-ui:3.1.3
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
    image: perteus/caddy-ui:3.1.3
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
  image: perteus/caddy-ui:3.1.3
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
| **3.1.3** | 🔒 Security patch — CVE fixes, migrace na moby/moby/client |
| **3.1.2** | 🔒 Security fixes, CSRF, race condition, path traversal |
| **3.1.1** | 🔐 Internal-only restrictions, Go 1.26, multi-platform (amd64/arm64) |
| **3.1.0** | 🔐 Wildcard refactor, new architecture |
| **3.0.0** | 🎉 Complete Go rewrite (794MB → 6MB) |
| 2.x | Python version (deprecated) |

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

## 📄 License

MIT License

---

<p align="center">
  <strong>CPM - Caddy Proxy Manager</strong><br>
  Made with ❤️ for home labs
</p>
