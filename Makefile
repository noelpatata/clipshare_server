BIN      := clipshare
BIN_WIN  := clipshare.exe
BIN_LNX  := clipshare-linux-amd64
INSTALL_DIR ?= $(HOME)/.local/bin

.PHONY: all build build-linux build-windows install uninstall clean test

all: build

build: ## Build for the current platform
	go build -o $(BIN) ./cmd/clipshare

build-linux: ## Cross-compile a static Linux amd64 binary
	GOOS=linux GOARCH=amd64 go build -o $(BIN_LNX) ./cmd/clipshare

build-windows: ## Cross-compile a Windows amd64 binary
	GOOS=windows GOARCH=amd64 go build -o $(BIN_WIN) ./cmd/clipshare

install: build ## Symlink $(BIN) into $(INSTALL_DIR) and print PATH hint
	@mkdir -p $(INSTALL_DIR)
	@ln -sf "$(CURDIR)/$(BIN)" "$(INSTALL_DIR)/$(BIN)"
	@echo "Installed $(INSTALL_DIR)/$(BIN) -> $(CURDIR)/$(BIN)"
	@echo "Make sure $(INSTALL_DIR) is on your PATH (e.g. export PATH=\$$HOME/.local/bin:\$$PATH)"

uninstall: ## Remove the symlink from $(INSTALL_DIR)
	@rm -f "$(INSTALL_DIR)/$(BIN)"
	@echo "Removed $(INSTALL_DIR)/$(BIN)"

clean: ## Remove built binaries
	rm -f $(BIN) $(BIN_WIN) $(BIN_LNX)

test: ## Run go vet
	go vet ./...

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
