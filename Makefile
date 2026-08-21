NAME := sandbox-probe
GITHUB_ORG = controlplaneio
PKG := github.com/$(GITHUB_ORG)/$(NAME)

SHELL := /bin/bash
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git -c log.showSignature=false rev-parse HEAD)
GIT_SHA := $(GIT_COMMIT)
GIT_UNTRACKED_CHANGES := $(shell git -c log.showSignature=false status --porcelain)
GIT_TAG := $(shell bash -c 'TAG=$$(git -c log.showSignature=false describe --tags --exact-match --abbrev=0 $(GIT_SHA) 2>/dev/null); echo "$${TAG:-dev}"')

# Mark as dirty if repo has uncommitted changes
ifneq ($(GIT_UNTRACKED_CHANGES),)
  GIT_COMMIT := $(GIT_COMMIT)-dirty
  ifneq ($(GIT_TAG),dev)
    GIT_TAG := $(GIT_TAG)-dirty
  endif
endif

# Build-time variables for -ldflags
CTIMEVAR=-X $(PKG)/cmd.commit=$(GIT_COMMIT) -X $(PKG)/cmd.version=$(GIT_TAG) -X $(PKG)/cmd.date=$(BUILD_DATE)
GO_LDFLAGS=-ldflags "-w $(CTIMEVAR)"
GO_LDFLAGS_STATIC=-ldflags "-w $(CTIMEVAR) -extldflags -static"

.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

# Build targets
.PHONY: build
build: ## Build the sandbox-probe binary
	@mkdir -p bin
	@go build $(GO_LDFLAGS) -o bin/sandbox-probe .

# Code generation targets
.PHONY: gen
gen: ## Generate protobuf code from buf schemas
	@command -v buf >/dev/null 2>&1 || { echo "buf not found, installing..."; $(MAKE) install-buf; }
	@echo "Generating protobuf code..."
	cd api && buf generate

# Development targets
.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...

.PHONY: tests
tests: ## Run all Go tests
	go test -v ./...

.PHONY: test
test: tests ## Run all Go tests

# Testing targets
.PHONY: e2etests
e2etests: ## Run the probe's own sandbox-fingerprint e2e tests (needs docker/podman/bwrap; each script skips if absent)
	@echo "Building sandbox-probe binary..."
	@mkdir -p bin
	@go build -o bin/sandbox-probe .
	@echo "Running fingerprint e2e tests..."
	@for test in tests/fingerprint/*.sh; do \
		if [ -f "$$test" ]; then \
			case "$$test" in \
				*_interactive.sh) echo "Skipping interactive test $$test (run manually)"; continue ;; \
			esac; \
			echo "Running $$test..."; \
			bash "$$test" || exit 1; \
		fi \
	done
	@echo "All fingerprint e2e tests completed successfully!"

.PHONY: e2etest-gvisor-bind
e2etest-gvisor-bind: ## GATED: run the probe under real gVisor and assert a known bind mount is reported (needs root + runsc)
	@bash tests/gated/gvisor_bind_mount.sh

# Installation targets
PREFIX ?= $(HOME)/.local/bin

.PHONY: install
install: build ## Install sandbox-probe to PREFIX (default: ~/.local/bin)
	@mkdir -p $(PREFIX)
	@install -m 755 bin/$(NAME) $(PREFIX)/$(NAME)
	@echo "Installed $(NAME) to $(PREFIX)/$(NAME)"

.PHONY: docker-test
docker-test: ## Build a Linux binary and run the alice/bob boundary demo (requires Docker)
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux GOARCH=$$(go env GOARCH) go build $(GO_LDFLAGS_STATIC) -o bin/sandbox-probe .
	@bash tests/example/run.sh

.PHONY: install-buf
install-buf: ## Install buf tool
	BIN="/usr/local/bin" && \
	VERSION="1.65.0" && \
	curl -sSL \
	"https://github.com/bufbuild/buf/releases/download/v$${VERSION}/buf-$$(uname -s)-$$(uname -m)" \
	-o "$${BIN}/buf" && \
	chmod +x "$${BIN}/buf"
