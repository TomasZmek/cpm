.PHONY: build run dev test clean docker docker-build docker-push

# Variables
APP_NAME := cpm
VERSION := 3.0.0
BUILD_DATE := $(shell date -u +%Y-%m-%d)
DOCKER_REPO := ghcr.io/tomaszmek/cpm
GO_FILES := $(shell find . -name '*.go' -type f)

# Build flags
LDFLAGS := -ldflags "-w -s -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)"

# Default target
all: build

# Build binary
build:
	@echo "Building $(APP_NAME) v$(VERSION)..."
	@go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/cpm

# Run locally
run: build
	@echo "Running $(APP_NAME)..."
	@./bin/$(APP_NAME)

# Development mode with hot reload (requires air)
dev:
	@air

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -rf tmp/

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w .

# Lint code
lint:
	@echo "Linting..."
	@golangci-lint run

# Generate templ templates
templ:
	@echo "Generating templates..."
	@templ generate

# Docker build
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_REPO):$(VERSION) -t $(DOCKER_REPO):latest .

# Docker push
docker-push: docker-build
	@echo "Pushing Docker image..."
	@docker push $(DOCKER_REPO):$(VERSION)
	@docker push $(DOCKER_REPO):latest

# Docker run locally
docker-run:
	@echo "Running in Docker..."
	@docker compose up -d

# Docker stop
docker-stop:
	@docker compose down

# Docker logs
docker-logs:
	@docker compose logs -f cpm

# Install development dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go install github.com/cosmtrek/air@latest
	@go install github.com/a-h/templ/cmd/templ@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Create release
release: test build docker-build
	@echo "Creating release v$(VERSION)..."
	@git tag -a v$(VERSION) -m "Release v$(VERSION)"
	@git push origin v$(VERSION)

# Help
help:
	@echo "CPM - Caddy Proxy Manager"
	@echo ""
	@echo "Usage:"
	@echo "  make build        - Build binary"
	@echo "  make run          - Build and run"
	@echo "  make dev          - Run with hot reload"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-push  - Push Docker image"
	@echo "  make docker-run   - Run with Docker Compose"
	@echo "  make deps         - Install dev dependencies"
	@echo "  make help         - Show this help"
