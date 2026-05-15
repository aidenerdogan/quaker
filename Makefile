# Makefile for Quaker

.PHONY: all build clean release

# Output directory
BIN_DIR := bin

# Go toolchain
GO ?= go
GO_DOWNLOAD_RETRIES ?= 3

# Binary
QK := qk
QK_SRC := ./cmd/qk

# Build flags
LDFLAGS := -s -w

all: build

# Download modules with retries to mitigate transient proxy/network EOF errors.
mod-download:
	@attempt=1; \
	while [ $$attempt -le $(GO_DOWNLOAD_RETRIES) ]; do \
		echo "Downloading Go modules ($$attempt/$(GO_DOWNLOAD_RETRIES))..."; \
		if $(GO) mod download; then \
			exit 0; \
		fi; \
		sleep $$((attempt * 2)); \
		attempt=$$((attempt + 1)); \
	done; \
	echo "Go module download failed after $(GO_DOWNLOAD_RETRIES) attempts"; \
	exit 1

# Local build (current architecture)
build: mod-download
	@echo "Building for local architecture..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(QK)-go $(QK_SRC)

# Release build targets (run on native architectures for CGO support)
release-amd64: mod-download
	@echo "Building release binaries (amd64)..."
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(QK)-darwin-amd64 $(QK_SRC)

release-arm64: mod-download
	@echo "Building release binaries (arm64)..."
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(QK)-darwin-arm64 $(QK_SRC)

clean:
	@echo "Cleaning binaries..."
	rm -f $(BIN_DIR)/$(QK)-*
