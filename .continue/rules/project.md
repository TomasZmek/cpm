# CPM - Caddy Proxy Manager

## O projektu
Go aplikace (Fiber framework) pro správu Caddy reverse proxy. Viz README.md pro full dokumentaci.

## Tech stack
- Go 1.23+
- Fiber (HTTP framework)
- HTMX (frontend interaktivita)
- Docker (distribuce, ~6MB image)

## Konvence kódu
- Go standard formatting (gofmt)
- Komentáře v angličtině
- Chybové hlášky lowercase

## Struktura
- `cmd/` - entrypoint
- `internal/` - business logika
- `web/` - frontend assets
- `templates/` - HTML šablony

## Git workflow
- Remote: github.com/TomasZmek/cpm
- Docker Hub: perteus/caddy-ui
- Commit formát: `typ: popis` (feat, fix, docs, refactor, chore)
- Aktuální verze: 3.1.0

## Důležité poznámky
- Wildcard SSL bloky patří do Caddyfile, handle bloky pro sites
- internal_only snippet musí být na úrovni wildcard bloku
- Synology vyžaduje privileged: true
EOF
```

Pak v Continue Agent módu dej:
```
Projdi projekt CPM, zkontroluj git status a připrav commit pro aktuální změny.
