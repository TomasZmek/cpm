# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary - static linking
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s -extldflags '-static'" -o cpm ./cmd/cpm

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
