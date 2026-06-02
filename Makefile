# wau-cli Makefile
# https://github.com/wau/wau-cli

.PHONY: all build test lint clean run install uninstall

# Variables
BINARY_NAME = wau
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
LDFLAGS = -ldflags "-X github.com/wau/wau-cli/internal/cmd.Version=$(VERSION)"

# Default target
all: build

# Build binary
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/wau
	@echo "Build complete: ./$(BINARY_NAME)"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	go clean

# Run in development mode
run:
	go run ./cmd/wau $(ARGS)

# Install to /usr/local/bin
install: build
	@echo "Installing to /usr/local/bin/$(BINARY_NAME)..."
	sudo mv $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed: wau --version"

# Uninstall
uninstall:
	@echo "Uninstalling..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstalled"

# Show help
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  test       - Run tests"
	@echo "  lint       - Run linter"
	@echo "  clean      - Remove build artifacts"
	@echo "  run        - Run in development mode (use ARGS=...)"
	@echo "  install    - Install to /usr/local/bin"
	@echo "  uninstall  - Remove from /usr/local/bin"
