# Build stage
FROM golang:1.26.11-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files (the module SDK and in-repo modules are separate Go modules, replaced locally)
COPY go.mod go.sum ./
COPY sdk/go.mod ./sdk/
COPY modules/dockhand/go.mod ./modules/dockhand/
RUN go mod download

# Copy source code
COPY . .

# Optional extra modules, e.g. --build-arg CPM_MODULES="github.com/you/cpm-module@v0.1.0"
# (space-separated). Each is fetched and blank-imported into the binary.
ARG CPM_MODULES=""
RUN if [ -n "$CPM_MODULES" ]; then \
      printf 'package main\n\nimport (\n' > cmd/cpm/modules_extra.go && \
      for m in $CPM_MODULES; do \
        go get "$m" && printf '\t_ "%s"\n' "${m%@*}" >> cmd/cpm/modules_extra.go || exit 1; \
      done && \
      printf ')\n' >> cmd/cpm/modules_extra.go; \
    fi

# Build binary - static linking
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -extldflags '-static'" -o cpm ./cmd/cpm

# Final stage - scratch (minimal, no CVEs)
FROM scratch

WORKDIR /app

# Copy certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /app/cpm .

# Copy static files and templates
COPY --from=builder /app/web ./web
COPY --from=builder /app/templates ./templates

# Expose port
EXPOSE 8501

# Note: scratch image has no shell, so no HEALTHCHECK with wget
# Use external health checks (Docker Compose, Kubernetes, etc.)

# Run
ENTRYPOINT ["./cpm"]
