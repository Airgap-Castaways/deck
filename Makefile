GO ?= go
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/deck
GOLANGCI_LINT ?= $(BIN_DIR)/golangci-lint

.PHONY: build test lint

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) ./cmd/deck

test:
	$(GO) test ./...

lint:
	$(GOLANGCI_LINT) run
