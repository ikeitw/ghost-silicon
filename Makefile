# Makefile
# Ghost-Silicon build system.
# Targets work on both Windows (via Git Bash / MSYS2) and Linux/macOS.

BINARY        = ghost-silicon
CMD_DIR       = ./cmd/ghost-silicon
OUT_DIR       = bin
DIST_DIR      = dist
VERSION      ?= 0.1.0
COMMIT       ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME   ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)

LDFLAGS = -X ghost-silicon/pkg/version.CommitHash=$(COMMIT) \
          -X ghost-silicon/pkg/version.BuildTime=$(BUILD_TIME) \
          -s -w

# ── Primary targets ────────────────────────────────────────────────────────────

.PHONY: all build test lint clean install-tools

all: build

## build: Compile ghost-silicon for the current platform
build:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY) $(CMD_DIR)
	@echo "→ $(OUT_DIR)/$(BINARY)"

## build-windows: Cross-compile for Windows amd64
build-windows:
	@mkdir -p $(OUT_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY).exe $(CMD_DIR)
	@echo "→ $(OUT_DIR)/$(BINARY).exe"

## test: Run the full test suite
test:
	CGO_ENABLED=0 go test ./... -timeout=120s

## test-verbose: Run tests with verbose output
test-verbose:
	CGO_ENABLED=0 go test ./... -timeout=120s -v

## test-cover: Run tests with coverage report
test-cover:
	CGO_ENABLED=0 go test ./... -timeout=120s -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## test-race: Run tests with race detector
test-race:
	go test ./... -timeout=120s -race

## lint: Run go vet (and staticcheck if available)
lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || \
		echo "staticcheck not installed — run: go install honnef.co/go/tools/cmd/staticcheck@latest"

## fmt: Format all Go source files
fmt:
	gofmt -w -s .

## tidy: Tidy go.mod and go.sum
tidy:
	go mod tidy

## clean: Remove build artifacts
clean:
	rm -rf $(OUT_DIR) $(DIST_DIR) coverage.out coverage.html

# ── Tool targets ───────────────────────────────────────────────────────────────

## profile-inspector: Build the profile inspector tool
profile-inspector:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=0 go build -o $(OUT_DIR)/profile-inspector ./tools/profile-inspector
	@echo "→ $(OUT_DIR)/profile-inspector"

## sandbox-checker: Build the sandbox checker tool
sandbox-checker:
	@mkdir -p $(OUT_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build -o $(OUT_DIR)/sandbox-checker.exe ./tools/sandbox-checker
	@echo "→ $(OUT_DIR)/sandbox-checker.exe"

## network-debugger: Build the network debugger tool
network-debugger:
	@mkdir -p $(OUT_DIR)
	CGO_ENABLED=0 go build -o $(OUT_DIR)/network-debugger ./tools/network-debugger
	@echo "→ $(OUT_DIR)/network-debugger"

## windows-env-checker: Build the Windows environment checker
windows-env-checker:
	@mkdir -p $(OUT_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build -o $(OUT_DIR)/windows-env-checker.exe ./tools/windows-env-checker
	@echo "→ $(OUT_DIR)/windows-env-checker.exe"

## tools: Build all developer tools
tools: profile-inspector network-debugger sandbox-checker windows-env-checker

# ── Distribution ───────────────────────────────────────────────────────────────

## dist: Package a Windows release zip
dist: build-windows
	@mkdir -p $(DIST_DIR)
	zip -j $(DIST_DIR)/ghost-silicon-v$(VERSION)-windows-amd64.zip \
		$(OUT_DIR)/$(BINARY).exe \
		configs/ghost-silicon.yaml \
		configs/network-policy.yaml \
		configs/sandbox-policy.yaml \
		README.md \
		LICENSE
	@echo "→ $(DIST_DIR)/ghost-silicon-v$(VERSION)-windows-amd64.zip"

# ── Helpers ────────────────────────────────────────────────────────────────────

## install-tools: Install recommended dev tools
install-tools:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/tools/cmd/goimports@latest

## version: Print the version string
version:
	@go run $(CMD_DIR) -version

## help: List available make targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'