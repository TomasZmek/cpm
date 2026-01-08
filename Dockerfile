# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cpm ./cmd/cpm

# Final stage
FROM alpine:3.20

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/cpm .

# Copy static files and templates
COPY --from=builder /app/web ./web
COPY --from=builder /app/templates ./templates

# Create non-root user
RUN adduser -D -H -s /sbin/nologin cpm
USER cpm

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

# Run
ENTRYPOINT ["./cpm"]
