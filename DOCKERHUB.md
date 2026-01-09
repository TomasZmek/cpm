# CPM - Caddy Proxy Manager

<p align="center">
  <strong>🚀 v3.0.0 - Complete Go Rewrite!</strong><br>
  Lightweight web UI for managing Caddy reverse proxy
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-3.0.0-blue" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go" alt="Go">
  <img src="https://img.shields.io/badge/image_size-~25MB-green" alt="Image Size">
</p>

---

## 🆕 What's New in v3.0.0

- **Complete Go rewrite** - From Python to Go
- **~25MB Docker image** - Down from 800MB!
- **Modern UI** - Fresh, clean design
- **Persistent auth** - Sessions survive restarts
- **Lightning fast** - Go + Fiber framework

---

## ✨ Features

| Feature | Description |
|---------|-------------|
| 📊 **Dashboard** | System overview, stats, alerts |
| 🔀 **Proxy Rules** | Visual editor for reverse proxy |
| ⚙️ **Snippets** | Cloudflare DNS, security headers |
| 📜 **Certificates** | SSL overview with expiration alerts |
| 👥 **Multi-User** | Role-based access control |
| 💾 **Backup** | Full config backup & restore |
| 🌐 **i18n** | English & Czech |

---

## 🚀 Quick Start

```bash
docker pull perteus/cpm:latest
docker pull perteus/cpm:3.0.0
```

### Docker Compose

```yaml
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    container_name: caddy_proxy
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./caddy-config:/etc/caddy
      - ./caddy-data:/data

  cpm:
    image: perteus/cpm:3.0.0
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

---

## 🔗 Links

- 📦 **Source Code**: [github.com/TomasZmek/cpm](https://github.com/TomasZmek/cpm)
- 🐛 **Report Bugs**: [GitHub Issues](https://github.com/TomasZmek/cpm/issues)
- 💡 **Feature Requests**: [GitHub Discussions](https://github.com/TomasZmek/cpm/discussions)

---

## 📝 Version History

| Version | Notes |
|---------|-------|
| **3.0.0** | 🎉 Complete Go rewrite, new UI |
| 2.x | Python version (deprecated) |

---

## 📄 License

MIT License

---

<p align="center">
  <strong>CPM v3.0.0 - Caddy Proxy Manager</strong><br>
  Made with ❤️ for home labs
</p>
