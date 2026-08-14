BIN      := clipshare
BIN_WIN  := clipshare.exe
BIN_LNX  := clipshare-linux-amd64
INSTALL_DIR ?= $(HOME)/.local/bin

VERSION := $(shell grep 'Version = ' internal/version/version.go | sed -e 's/.*"\([^"]*\)".*/\1/')
DIST    := dist

GO_LDFLAGS := -s -w

.PHONY: all build build-linux build-windows build-all release install uninstall clean test

all: build

build: ## Build for the current platform
	go build -o $(BIN) ./cmd/clipshare

build-linux: ## Cross-compile a static Linux amd64 binary
	GOOS=linux GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o $(BIN_LNX) ./cmd/clipshare

build-windows: ## Cross-compile a Windows amd64 binary
	GOOS=windows GOARCH=amd64 go build -ldflags "$(GO_LDFLAGS)" -o $(BIN_WIN) ./cmd/clipshare

build-all: build-linux build-windows ## Build release binaries for all platforms into $(DIST)
	@mkdir -p $(DIST)
	cp $(BIN_LNX) $(DIST)/clipshare-$(VERSION)-linux-amd64
	cp $(BIN_WIN) $(DIST)/clipshare-$(VERSION)-windows-amd64.exe
	cd $(DIST) && sha256sum clipshare-$(VERSION)-linux-amd64 clipshare-$(VERSION)-windows-amd64.exe > checksums.txt
	@echo "Release artifacts in $(DIST)/"

release: build-all ## Alias for build-all (used by CI)

install: build ## Symlink $(BIN) into $(INSTALL_DIR) and print PATH hint
	@mkdir -p $(INSTALL_DIR)
	@ln -sf "$(CURDIR)/$(BIN)" "$(INSTALL_DIR)/$(BIN)"
	@echo "Installed $(INSTALL_DIR)/$(BIN) -> $(CURDIR)/$(BIN)"
	@echo "Make sure $(INSTALL_DIR) is on your PATH (e.g. export PATH=\$$HOME/.local/bin:\$$PATH)"

uninstall: ## Remove the symlink from $(INSTALL_DIR)
	@rm -f "$(INSTALL_DIR)/$(BIN)"
	@echo "Removed $(INSTALL_DIR)/$(BIN)"

clean: ## Remove built binaries
	rm -rf $(BIN) $(BIN_WIN) $(BIN_LNX) $(DIST)

test: ## Run go vet
	go vet ./...

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
