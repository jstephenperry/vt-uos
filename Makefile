# VT-UOS Makefile
# Build automation for Vault-Tec Unified Operating System

.PHONY: all build build-pi build-pi-zero test test-integration lint clean run migrate seed help

# Build variables
BINARY_NAME := vtuos
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Go build flags for static binary (no CGO)
GO_BUILD_FLAGS := CGO_ENABLED=0

# Default target
all: lint test build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) for current platform..."
	$(GO_BUILD_FLAGS) go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/vtuos

# Build for Raspberry Pi 4/5 (ARM64)
build-pi:
	@echo "Building $(BINARY_NAME) for Raspberry Pi (ARM64)..."
	$(GO_BUILD_FLAGS) GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/vtuos

# Build for Raspberry Pi Zero 2W (ARM64, minimal)
build-pi-zero:
	@echo "Building $(BINARY_NAME) for Raspberry Pi Zero 2W (ARM64 minimal)..."
	$(GO_BUILD_FLAGS) GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -tags "sqlite_omit_load_extension" -o bin/$(BINARY_NAME)-linux-arm64-minimal ./cmd/vtuos

# Build all platforms
build-all: build build-pi build-pi-zero
	@echo "All builds complete."

# Run unit tests
test:
	@echo "Running unit tests..."
	go test -v -race -cover ./internal/...

# Run unit tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | grep total
	@echo "Coverage report: coverage.html"

# Run tests with coverage for specific package
test-pkg:
	@echo "Usage: make test-pkg PKG=./internal/models"
	@if [ -z "$(PKG)" ]; then echo "Error: PKG not set"; exit 1; fi
	go test -v -race -coverprofile=coverage.out $(PKG)
	go tool cover -func=coverage.out

# Run tests and check coverage threshold (80%)
test-coverage-check:
	@echo "Running tests with 80% coverage threshold..."
	go test -v -race -coverprofile=coverage.out ./internal/...
	@coverage=$$(go tool cover -func=coverage.out | grep total | awk '{print substr($$3, 1, length($$3)-1)}'); \
	echo "Total coverage: $$coverage%"; \
	if [ $$(echo "$$coverage < 80" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage ($$coverage%) is below 80% threshold"; \
		exit 1; \
	else \
		echo "✅ Coverage ($$coverage%) meets 80% threshold"; \
	fi

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./internal/...

# Run all tests
test-all: test test-integration

# Run linter
lint:
	@echo "Running golangci-lint..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./bin/$(BINARY_NAME)

# Run database migrations
migrate:
	@echo "Running database migrations..."
	./bin/$(BINARY_NAME) --migrate-only

# Generate seed data
seed:
	@echo "Generating seed data..."
	./bin/$(BINARY_NAME) --seed

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f *.db *.db-wal *.db-shm

# Verify no CGO dependencies
verify-static:
	@echo "Verifying static binary (no CGO)..."
	$(GO_BUILD_FLAGS) go build -o /tmp/vtuos-check ./cmd/vtuos
	@if ldd /tmp/vtuos-check 2>&1 | grep -q "not a dynamic"; then \
		echo "✓ Binary is static (no CGO dependencies)"; \
	else \
		echo "✗ Binary has dynamic dependencies:"; \
		ldd /tmp/vtuos-check; \
		exit 1; \
	fi
	@rm /tmp/vtuos-check

# Install development tools
dev-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Create backup of database
backup:
	@echo "Creating database backup..."
	@mkdir -p backups
	@if [ -f vault.db ]; then \
		cp vault.db backups/vault-$(shell date +%Y%m%d-%H%M%S).db; \
		echo "Backup created: backups/vault-$(shell date +%Y%m%d-%H%M%S).db"; \
	else \
		echo "No database file found."; \
	fi

# Show help
help:
	@echo "VT-UOS Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build targets:"
	@echo "  build          Build for current platform"
	@echo "  build-pi       Build for Raspberry Pi 4/5 (ARM64)"
	@echo "  build-pi-zero  Build for Raspberry Pi Zero 2W (ARM64 minimal)"
	@echo "  build-all      Build for all platforms"
	@echo ""
	@echo "Test targets:"
	@echo "  test                  Run unit tests"
	@echo "  test-coverage         Run tests with coverage report"
	@echo "  test-coverage-check   Run tests and verify 80% coverage"
	@echo "  test-pkg PKG=<path>   Run tests for specific package"
	@echo "  test-integration      Run integration tests"
	@echo "  test-all              Run all tests"
	@echo ""
	@echo "Other targets:"
	@echo "  lint           Run golangci-lint"
	@echo "  fmt            Format code with gofmt and goimports"
	@echo "  run            Build and run the application"
	@echo "  migrate        Run database migrations"
	@echo "  seed           Generate seed data"
	@echo "  clean          Remove build artifacts"
	@echo "  verify-static  Verify binary has no CGO dependencies"
	@echo "  dev-tools      Install development tools"
	@echo "  backup         Create database backup"
	@echo "  help           Show this help message"
