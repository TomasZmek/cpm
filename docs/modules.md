# CPM modules

Modules extend CPM without changes to its code. Today a module is a
**container source**: it tells CPM which containers run on which Docker host,
so CPM can show the backend status of proxy rules and offer containers in
Docker discovery and in the rule form.

- Modules are **compiled in** (a statically linked Go binary cannot load code
  at runtime). Adding a module needs a new build, but no code edit.
- Which modules are **active** is decided at runtime in **Settings → Modules**
  (admin only). Enabling a module writes its settings to
  `caddy-config/modules.json` (mode 0600, it may contain tokens); disabling
  it removes the entry again. No restart is needed.
- Without `modules.json`, modules marked `DefaultEnabled` (the built-in
  *Docker (local)*) are active.

## How CPM uses the containers

Every 30 s CPM asks all enabled modules for their containers (in parallel,
10 s timeout each) and keeps the result in memory. Pages never wait for a
module. A proxy rule is matched to a container

1. by **container name** when the rule targets a service name
   (`reverse_proxy anki-sync:8080`), or
2. by **Docker host IP + published port** (`reverse_proxy 192.168.1.10:8080`
   and the container publishes host port 8080 on that host).

Rules no container matches are checked with a TCP connect to `host:port`
(1 s timeout, at most 8 at a time): *reachable* / *unreachable*. Single-label
names (Docker service names that only resolve inside Caddy's network) stay
*unknown*.

## Writing a module

A module is a Go module that depends only on `github.com/TomasZmek/cpm/sdk`
(no other dependencies) and registers itself in `init`:

```go
package dockhand

import (
	"context"

	"github.com/TomasZmek/cpm/sdk"
)

func init() {
	sdk.Register(sdk.Module{
		ID:   "dockhand", // key in modules.json: [a-z][a-z0-9_-]{1,31}
		Icon: "⚓",
		Name: sdk.Text{"en": "Dockhand", "cs": "Dockhand"},
		Description: sdk.Text{"en": "Containers of all environments managed by Dockhand."},
		Fields: []sdk.Field{
			{Key: "url", Type: sdk.FieldURL, Required: true,
				Label: sdk.Text{"en": "Dockhand URL"}, Placeholder: "http://192.168.1.10:3000"},
			{Key: "token", Type: sdk.FieldPassword, Required: true,
				Label: sdk.Text{"en": "API token"}},
		},
		New: func(cfg sdk.Config) (sdk.Source, error) {
			return &source{url: cfg["url"], token: cfg["token"]}, nil
		},
	})
}

type source struct{ url, token string }

// Containers is called from a background goroutine; honour ctx.
func (s *source) Containers(ctx context.Context) ([]sdk.Container, error) {
	// …query the API and return containers with HostIP (the address used
	// in proxy rules) and published ports.
	return nil, nil
}
```

Rules for modules:

- `New` validates the configuration and must not do network I/O; errors are
  shown when the administrator saves the settings.
- `Containers` returns *all* containers (running and stopped) with
  `HostIP` set to the Docker host address that proxy rules use, and `Ports`
  with `Public` = the port published on that host.
- Fields of type `FieldHostIP` take a host's IP; the settings page suggests
  the address the browser opened CPM on (inside a container the host's LAN
  IP is not visible otherwise).
- Fields of type `FieldPassword` are secrets: CPM never shows them again
  and keeps the stored value when the field is left empty.
- Texts are `sdk.Text` maps (language → text, `en` is the fallback), so a
  module ships its own translations.
- The SDK follows semantic versioning; `sdk.APIVersion` changes on
  incompatible changes.

## Building CPM with extra modules

Pass the modules (space-separated, with versions) as a build argument; the
Dockerfile fetches them and blank-imports them into the binary:

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg CPM_MODULES="github.com/you/cpm-dockhand@v0.1.0" \
  -t you/caddy-ui:custom --push .
```

Without Docker, add a blank import to `cmd/cpm/modules.go`
(`import _ "github.com/you/cpm-dockhand"`), run `go get` and build as usual.
A module adds only its own code to the binary (a few kB).

## Modules in this repository

| ID | Module | Settings |
|---|---|---|
| `docker` | Docker (local): the Docker daemon behind the mounted socket | `host_ip`: IP of this host as used in proxy rules (default: the host marked as local in Settings → Docker) |
| `dockhand` | [Dockhand](https://dockhand.pro): containers of all its environments (local socket, remote hosts via Hawser incl. Edge mode) through its REST API — `modules/dockhand/` | `url`; `token` (Dockhand → Profile → API tokens, `dh_…`; not needed with Dockhand auth off); `host_ips` (optional `ID=IP` per environment); `environments` (optional ID filter) |

### Dockhand

The host IP of each environment (needed to match `IP:port` rules) is, in this
order: the `host_ips` mapping, the environment's **Public IP** set in
Dockhand (environment settings), or the address Dockhand connects to (direct
/ Hawser standard). Environments in Hawser Edge mode or on a local socket
need a Public IP in Dockhand or a `host_ips` entry. After Dockhand rejects the
token CPM pauses for 5 minutes, because Dockhand blocks an IP after 10 failed
token attempts.

When both *Docker (local)* and *Dockhand* are enabled, containers reported by
both are listed once (matched by container ID).
