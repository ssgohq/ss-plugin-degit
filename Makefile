.PHONY: all build clean install test lint snapshot release-local help install-plugin uninstall-plugin test-install

# Variables
BINARY_NAME := ss-plugin-degit
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# Default target
all: build

# Build using goreleaser for current platform (recommended - processes plugin.yaml.tpl)
build: snapshot
	@echo "Build complete. Binary is in dist/"
	@ls -la dist/$(BINARY_NAME)_$(GOOS)_$(GOARCH)*/$(BINARY_NAME)* 2>/dev/null || true

# Build binary directly with go build (quick build, no plugin.yaml processing)
build-quick:
	@echo "Building $(BINARY_NAME) (quick build without goreleaser)..."
	@LDFLAGS="-s -w -X main.version=$(VERSION) -X main.commit=$$(git rev-parse --short HEAD 2>/dev/null || echo none) -X main.date=$$(date -u +%Y-%m-%dT%H:%M:%SZ)"; \
	go build -ldflags "$$LDFLAGS" -o $(BINARY_NAME) .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf dist/

# Install to GOPATH/bin (quick install without goreleaser)
install:
	@echo "Installing $(BINARY_NAME)..."
	@LDFLAGS="-s -w -X main.version=$(VERSION) -X main.commit=$$(git rev-parse --short HEAD 2>/dev/null || echo none) -X main.date=$$(date -u +%Y-%m-%dT%H:%M:%SZ)"; \
	go install -ldflags "$$LDFLAGS" .

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Build snapshot with goreleaser (builds binary only, for development)
snapshot:
	@echo "Building snapshot with goreleaser..."
	goreleaser build --snapshot --clean --single-target

# Build full release locally (creates archives with plugin.yaml)
release-local:
	@echo "Building local release with goreleaser..."
	goreleaser release --snapshot --clean --skip=publish

# Install plugin to ss-cli using goreleaser output (recommended)
install-plugin: snapshot
	@echo "Installing plugin to ss-cli..."
	@mkdir -p ~/.ss/plugins/degit
	@BINARY=$$(find dist -name "$(BINARY_NAME)" -type f | head -1); \
	if [ -n "$$BINARY" ]; then \
		cp "$$BINARY" ~/.ss/plugins/degit/; \
	else \
		echo "Binary not found in dist/, run 'make snapshot' first"; \
		exit 1; \
	fi
	@VERSION=$(VERSION); \
	sed 's/__VERSION__/'"$$VERSION"'/g' plugin.yaml.tpl > ~/.ss/plugins/degit/plugin.yaml
	@echo "Plugin installed to ~/.ss/plugins/degit/"
	@echo "Test with: ss degit --help"

# Install plugin using release archive (processes plugin.yaml through goreleaser)
install-plugin-release: release-local
	@echo "Installing plugin from release archive..."
	@PLATFORM=$(GOOS)-$(GOARCH); \
	ARCHIVE="dist/$(BINARY_NAME)-$$PLATFORM.tar.gz"; \
	if [ ! -f "$$ARCHIVE" ]; then \
		ARCHIVE="dist/$(BINARY_NAME)-$$PLATFORM.zip"; \
	fi; \
	if [ -f "$$ARCHIVE" ]; then \
		echo "Found archive: $$ARCHIVE"; \
		rm -rf ~/.ss/plugins/degit; \
		mkdir -p ~/.ss/plugins/degit; \
		if echo "$$ARCHIVE" | grep -q ".tar.gz"; then \
			tar -xzf "$$ARCHIVE" -C ~/.ss/plugins/degit; \
		else \
			unzip -q "$$ARCHIVE" -d ~/.ss/plugins/degit; \
		fi; \
		echo "Plugin installed from archive"; \
		echo "Contents of ~/.ss/plugins/degit:"; \
		ls -la ~/.ss/plugins/degit/; \
		echo ""; \
		echo "Test with: ss degit --help"; \
	else \
		echo "Archive not found: $$ARCHIVE"; \
		exit 1; \
	fi

# Uninstall plugin from ss-cli
uninstall-plugin:
	@echo "Uninstalling plugin from ss-cli..."
	@rm -rf ~/.ss/plugins/degit
	@echo "Plugin uninstalled"

# Test plugin installation from dist archives (simulates ss plugin install)
test-install: install-plugin-release
	@echo "Testing plugin..."
	@ss degit --help || echo "ss-cli not in PATH or plugin not working"

# Show help
help:
	@echo "Available targets:"
	@echo "  build              - Build using goreleaser (recommended)"
	@echo "  build-quick        - Quick build without goreleaser"
	@echo "  clean              - Remove build artifacts"
	@echo "  install            - Install to GOPATH/bin"
	@echo "  test               - Run tests"
	@echo "  lint               - Run linter"
	@echo "  snapshot           - Build snapshot with goreleaser (single target)"
	@echo "  release-local      - Build full release locally (with archives)"
	@echo "  install-plugin     - Install plugin to ss-cli using goreleaser output"
	@echo "  install-plugin-release - Install plugin from release archive"
	@echo "  uninstall-plugin   - Remove plugin from ss-cli"
	@echo "  test-install       - Build release and test installation"
	@echo "  help               - Show this help"
