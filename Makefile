GO ?= go
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/deck
GOLANGCI_LINT ?= $(BIN_DIR)/golangci-lint
GOLANGCI_LINT_PKG ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.11.3
GOVULNCHECK ?= $(BIN_DIR)/govulncheck
GOVULNCHECK_PKG ?= golang.org/x/vuln/cmd/govulncheck
GOVULNCHECK_VERSION ?= v1.1.4
GOVULNCHECK_GOTOOLCHAIN ?= go1.26.6
GORELEASER ?= $(BIN_DIR)/goreleaser
GORELEASER_PKG ?= github.com/goreleaser/goreleaser/v2
GORELEASER_VERSION ?= v2.14.3
GORELEASER_VERSION_NUMBER := $(patsubst v%,%,$(GORELEASER_VERSION))
BUILDINFO_PKG ?= github.com/Airgap-Castaways/deck/internal/buildinfo
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DIRTY ?= $(shell if [ -n "$$(git status --short 2>/dev/null)" ]; then printf true; else printf false; fi)
LDFLAGS ?= -X $(BUILDINFO_PKG).Version=$(VERSION) -X $(BUILDINFO_PKG).Commit=$(COMMIT) -X $(BUILDINFO_PKG).Date=$(DATE) -X $(BUILDINFO_PKG).Dirty=$(DIRTY)

.PHONY: build test lint vuln generate verify-generated print-build-meta ensure-goreleaser release-check release-snapshot release-publish docs-install docs-serve docs-build docs-deploy docs-check docs-translate

GENERATED_PATHS := \
	docs/contributing/tool-definition-schema.md \
	docs/step-kinds \
	docs/step-kinds.md \
	docs/workflow-model.md \
	docs/workspace-layout.md \
	docs/cli \
	schemas \
	':(exclude)schemas/embed.go' \
	':(exclude)schemas/embed_test.go'

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/deck

test:
	$(GO) test ./...

generate:
	$(GO) run ./cmd/schema-gen
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/deck
	$(BIN) __gendocs docs/cli

verify-generated: generate
	git diff --exit-code -- $(GENERATED_PATHS)

lint:
	@if [ ! -x "$(GOLANGCI_LINT)" ]; then \
		mkdir -p "$(BIN_DIR)"; \
		GOBIN="$(abspath $(BIN_DIR))" $(GO) install $(GOLANGCI_LINT_PKG)@$(GOLANGCI_LINT_VERSION); \
	fi
	$(GOLANGCI_LINT) run


$(GOVULNCHECK):
	@mkdir -p "$(BIN_DIR)"
	GOBIN="$(abspath $(BIN_DIR))" GOTOOLCHAIN="$(GOVULNCHECK_GOTOOLCHAIN)" $(GO) install $(GOVULNCHECK_PKG)@$(GOVULNCHECK_VERSION)

vuln: $(GOVULNCHECK)
	$(GOVULNCHECK) ./...

print-build-meta:
	@printf 'VERSION=%s\nCOMMIT=%s\nDATE=%s\nDIRTY=%s\n' "$(VERSION)" "$(COMMIT)" "$(DATE)" "$(DIRTY)"

docs-install:
	cd website && npm ci

docs-serve:
	cd website && npm run start

docs-build:
	cd website && npm run build

docs-deploy:
	cd website && GIT_USER=$${GIT_USER:?set GIT_USER to your GitHub username} npm run deploy

docs-check:
	$(GO) test ./internal/doccheck/ -run 'TestI18nKoDrift|TestDocsRelativeLinksResolve|TestAllErrorCodesDocumented'

docs-translate:
	node website/scripts/translate-docs.mjs $(ARGS)

ensure-goreleaser:
	@mkdir -p "$(BIN_DIR)"
	@if [ ! -x "$(GORELEASER)" ] || ! "$(GORELEASER)" --version 2>/dev/null | grep -q -E "(GitVersion|version):[[:space:]]*v?$(GORELEASER_VERSION_NUMBER)"; then \
		GOBIN="$(abspath $(BIN_DIR))" $(GO) install $(GORELEASER_PKG)@$(GORELEASER_VERSION); \
	fi

release-check: ensure-goreleaser
	$(GORELEASER) check

release-snapshot: ensure-goreleaser
	$(GORELEASER) release --snapshot --clean

release-publish: ensure-goreleaser
	$(GORELEASER) release --clean
