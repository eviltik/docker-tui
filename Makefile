.PHONY: build build-static clean run install dist dist-linux dist-darwin dist-windows dist-static dist-static-all all test test-race test-cover test-bench test-one test-all

# Binary name
BINARY_NAME=docker-tui
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"
STATIC_LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -extldflags '-static'"

# Default target
all: clean build

# Build the binary (dynamic linking)
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) ./src
	@echo "Build complete: ./$(BINARY_NAME)"

# Build static binary (no GLIBC dependencies - portable across Linux distros)
build-static:
	@echo "Building static $(BINARY_NAME) version $(VERSION)..."
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY_NAME) ./src
	@echo "Static build complete: ./$(BINARY_NAME)"
	@echo "✓ This binary is portable and has no GLIBC dependencies"

# Run the application
run:
	go run ./src

# Watch and rebuild on changes (requires entr: apt install entr / brew install entr)
watch:
	@echo "Watching for changes... (Ctrl+C to stop)"
	@find src -name '*.go' | entr -r make build

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf dist/
	@echo "Clean complete"

# Install to system
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "Install complete"

# Create distribution directory
dist: clean
	@echo "Creating distribution binaries..."
	@mkdir -p dist

# Build for Linux (amd64)
dist-linux:
	@echo "Building for Linux amd64..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./src
	@echo "Linux binary: dist/$(BINARY_NAME)-linux-amd64"

# Build for Linux (arm64)
dist-linux-arm:
	@echo "Building for Linux arm64..."
	@mkdir -p dist
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./src
	@echo "Linux ARM binary: dist/$(BINARY_NAME)-linux-arm64"

# Build for macOS (amd64)
dist-darwin:
	@echo "Building for macOS amd64..."
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./src
	@echo "macOS binary: dist/$(BINARY_NAME)-darwin-amd64"

# Build for macOS (arm64 - Apple Silicon)
dist-darwin-arm:
	@echo "Building for macOS arm64 (Apple Silicon)..."
	@mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./src
	@echo "macOS ARM binary: dist/$(BINARY_NAME)-darwin-arm64"

# Build for Windows
dist-windows:
	@echo "Building for Windows amd64..."
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe ./src
	@echo "Windows binary: dist/$(BINARY_NAME)-windows-amd64.exe"

# Build static Linux binaries (portable - no GLIBC dependencies)
dist-static:
	@echo "Building static Linux binaries (portable)..."
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64-static ./src
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64-static ./src
	@echo "✓ Static Linux amd64 binary: dist/$(BINARY_NAME)-linux-amd64-static"
	@echo "✓ Static Linux arm64 binary: dist/$(BINARY_NAME)-linux-arm64-static"
	@echo "These binaries have no GLIBC dependencies and work on any Linux distro"

# Build all distribution binaries (dynamic linking)
dist-all: dist dist-linux dist-linux-arm dist-darwin dist-darwin-arm dist-windows
	@echo "All distribution binaries created in dist/"
	@ls -lh dist/

# Build all distribution binaries including static Linux versions
dist-static-all: dist-all dist-static
	@echo ""
	@echo "All distribution binaries (including static) created in dist/"
	@ls -lh dist/

# Test
test:
	@echo "Running tests..."
	go test -v ./src/...

# Test with race detection
test-race:
	@echo "Running tests with race detector..."
	go test -race -v ./src/...

# Test with coverage
test-cover:
	@echo "Running tests with coverage..."
	go test -cover -coverprofile=coverage.out ./src/...
	@echo "Coverage report saved to coverage.out"
	@echo "View detailed coverage: go tool cover -html=coverage.out"

# Test with coverage HTML report
test-cover-html: test-cover
	@echo "Opening coverage report in browser..."
	go tool cover -html=coverage.out

# Run benchmarks
test-bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./src/...

# Run specific test
# Usage: make test-one TEST=TestFormatCPU
test-one:
	@if [ -z "$(TEST)" ]; then \
		echo "Error: TEST variable not set"; \
		echo "Usage: make test-one TEST=TestFormatCPU"; \
		exit 1; \
	fi
	@echo "Running test: $(TEST)"
	go test -v -run $(TEST) ./src/...

# Run all tests (verbose, race, coverage)
test-all:
	@echo "Running complete test suite..."
	@echo "================================"
	@echo "1. Running tests with race detector..."
	@go test -race ./src/...
	@echo ""
	@echo "2. Running tests with coverage..."
	@go test -cover -coverprofile=coverage.out ./src/...
	@echo ""
	@echo "3. Running benchmarks..."
	@go test -bench=. -benchmem ./src/... 2>/dev/null || true
	@echo ""
	@echo "================================"
	@echo "Complete test suite finished!"
	@echo "Coverage report: coverage.out"

# Format code
fmt:
	go fmt ./src/...

# Lint
lint:
	golangci-lint run

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build:"
	@echo "  make build              - Build the binary (dynamic linking)"
	@echo "  make build-static       - Build static binary (no GLIBC deps, portable)"
	@echo "  make run                - Run the application"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make install            - Install to /usr/local/bin"
	@echo ""
	@echo "Distribution:"
	@echo "  make dist-linux         - Build Linux amd64 binary"
	@echo "  make dist-linux-arm     - Build Linux arm64 binary"
	@echo "  make dist-darwin        - Build macOS amd64 binary"
	@echo "  make dist-darwin-arm    - Build macOS arm64 binary"
	@echo "  make dist-windows       - Build Windows binary"
	@echo "  make dist-static        - Build static Linux binaries (portable)"
	@echo "  make dist-all           - Build all platform binaries"
	@echo "  make dist-static-all    - Build all platform + static binaries"
	@echo ""
	@echo "Testing:"
	@echo "  make test               - Run tests (verbose)"
	@echo "  make test-race          - Run tests with race detector"
	@echo "  make test-cover         - Run tests with coverage report"
	@echo "  make test-cover-html    - Run tests with coverage + HTML report"
	@echo "  make test-bench         - Run benchmarks"
	@echo "  make test-one TEST=Name - Run specific test"
	@echo "  make test-all           - Run complete test suite"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt                - Format code"
	@echo "  make lint               - Run linter"
	@echo ""
	@echo "Other:"
	@echo "  make help               - Show this help"
