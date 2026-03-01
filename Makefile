# DERO Wallet TUI Build Configuration
# Optimized for Go 1.26 with minimal binary size

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Linker flags: strip debug info (-s -w), embed version info
LDFLAGS := -s -w \
  -X main.version=$(VERSION) \
  -X main.commit=$(COMMIT) \
  -X main.date=$(DATE)

# Go build flags
GOFLAGS := -trimpath

# Binary name
BINARY := derotui

.PHONY: all build build-debug clean test fmt vet release install

all: build

# Standard optimized build (14MB)
build:
	go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/

# Debug build with symbols (18MB)
build-debug:
	go build -o $(BINARY)-debug ./cmd/

# Run all tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Static analysis
vet:
	go vet ./...

# Release build with UPX compression (~6MB)
release: build
	@if command -v upx >/dev/null 2>&1; then \
		upx --best $(BINARY) -o $(BINARY)-release; \
		echo "Release binary created: $(BINARY)-release"; \
	else \
		echo "UPX not installed, skipping compression"; \
		cp $(BINARY) $(BINARY)-release; \
	fi

# Install to GOPATH/bin
install:
	go install $(GOFLAGS) -ldflags="$(LDFLAGS)" ./cmd/

# Clean build artifacts
clean:
	rm -f $(BINARY) $(BINARY)-debug $(BINARY)-release
	go clean ./...

# Show build info
info:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"
	@echo "Binary:  $(BINARY)"

# Development workflow
dev: fmt vet test build
