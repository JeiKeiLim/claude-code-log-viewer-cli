# Makefile for cclv - Claude Code Log Viewer CLI

# Binary name
BINARY_NAME=cclv
BINARY_DIR=dist

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=gofmt
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get

# Build flags
LDFLAGS=-s -w
BUILD_FLAGS=-ldflags "$(LDFLAGS)"

# Version info (can be overridden)
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Platforms for cross-compilation
PLATFORMS=darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

# Source files
SRC=$(shell find . -name "*.go" -type f)

.PHONY: all build clean test lint fmt vet tidy install uninstall help
.PHONY: build-all build-darwin build-linux ci check

# Default target
all: clean lint test build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) ./cmd/cclv

# Build for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(BINARY_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/} ./cmd/cclv; \
		echo "Built $(BINARY_DIR)/$(BINARY_NAME)-$${platform%/*}-$${platform#*/}"; \
	done

# Build for Darwin (macOS) only
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BINARY_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/cclv
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/cclv

# Build for Linux only
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BINARY_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/cclv
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/cclv

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run tests with short flag (skip long-running tests)
test-short:
	@echo "Running short tests..."
	$(GOTEST) -v -short ./...

# Run tests and generate coverage report
coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w $(SRC)

# Check formatting (for CI)
fmt-check:
	@echo "Checking code formatting..."
	@if [ -n "$$($(GOFMT) -l $(SRC))" ]; then \
		echo "The following files need formatting:"; \
		$(GOFMT) -l $(SRC); \
		exit 1; \
	fi
	@echo "All files are properly formatted."

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, running go vet instead..."; \
		$(GOVET) ./...; \
	fi

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Verify dependencies
verify:
	@echo "Verifying dependencies..."
	$(GOMOD) verify

# CI target - runs all checks
ci: deps fmt-check vet test build
	@echo "CI checks passed!"

# Quick check - fast validation
check: fmt-check vet
	@echo "Quick checks passed!"

# Install to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install ./cmd/cclv

# Install to /usr/local/bin (requires sudo)
install-global: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"

# Uninstall from GOPATH/bin
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	rm -f $(shell go env GOPATH)/bin/$(BINARY_NAME)

# Uninstall from /usr/local/bin (requires sudo)
uninstall-global:
	@echo "Uninstalling $(BINARY_NAME) from /usr/local/bin..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

# Run the application
run: build
	./$(BINARY_NAME)

# Run with a specific file
run-file: build
	@if [ -z "$(FILE)" ]; then \
		echo "Usage: make run-file FILE=path/to/file.jsonl"; \
		exit 1; \
	fi
	./$(BINARY_NAME) $(FILE)

# Development mode - build and run with hot reload (requires air)
dev:
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not installed. Install with: go install github.com/air-verse/air@latest"; \
		echo "Running without hot reload..."; \
		$(MAKE) run; \
	fi

# Show binary size
size: build
	@echo "Binary size:"
	@ls -lh $(BINARY_NAME) | awk '{print $$5, $$9}'

# Show version info
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Generate release artifacts
release: clean build-all
	@echo "Creating release artifacts..."
	@cd $(BINARY_DIR) && \
	for f in $(BINARY_NAME)-*; do \
		tar -czf $$f.tar.gz $$f; \
		echo "Created $$f.tar.gz"; \
	done
	@echo "Release artifacts created in $(BINARY_DIR)/"

# Help
help:
	@echo "Claude Code Log Viewer CLI (cclv) - Makefile targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  make build        - Build for current platform"
	@echo "  make build-all    - Build for all platforms (darwin/linux, amd64/arm64)"
	@echo "  make build-darwin - Build for macOS only"
	@echo "  make build-linux  - Build for Linux only"
	@echo ""
	@echo "Test targets:"
	@echo "  make test         - Run all tests with coverage"
	@echo "  make test-short   - Run short tests only"
	@echo "  make coverage     - Generate HTML coverage report"
	@echo ""
	@echo "Code quality:"
	@echo "  make fmt          - Format code"
	@echo "  make fmt-check    - Check code formatting (for CI)"
	@echo "  make vet          - Run go vet"
	@echo "  make lint         - Run linter (golangci-lint or go vet)"
	@echo "  make check        - Quick validation (fmt-check + vet)"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps         - Download dependencies"
	@echo "  make tidy         - Tidy go.mod"
	@echo "  make verify       - Verify dependencies"
	@echo ""
	@echo "Installation:"
	@echo "  make install        - Install to GOPATH/bin"
	@echo "  make install-global - Install to /usr/local/bin (requires sudo)"
	@echo "  make uninstall      - Uninstall from GOPATH/bin"
	@echo "  make uninstall-global - Uninstall from /usr/local/bin"
	@echo ""
	@echo "Other:"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make run          - Build and run"
	@echo "  make run-file FILE=path/to/file.jsonl - Run with a specific file"
	@echo "  make dev          - Development mode with hot reload (requires air)"
	@echo "  make size         - Show binary size"
	@echo "  make version      - Show version info"
	@echo "  make release      - Create release artifacts"
	@echo "  make ci           - Run all CI checks"
	@echo "  make all          - Clean, lint, test, and build"
	@echo "  make help         - Show this help"
