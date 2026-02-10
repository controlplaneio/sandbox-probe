.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

# Build targets
.PHONY: build
build: ## Build the sandbox-probe binary
	@mkdir -p bin
	@go build -o bin/sandbox-probe .

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

# Testing targets
.PHONY: e2etests
e2etests: ## Run end-to-end tests
	@echo "Building sandbox-probe binary..."
	@mkdir -p bin
	@go build -o bin/sandbox-probe .
	@echo "Running e2e tests..."
	@for test in tests/*.sh; do \
		if [ -f "$$test" ]; then \
			echo "Running $$test..."; \
			bash "$$test" || exit 1; \
		fi \
	done
	@echo "All e2e tests completed successfully!"

# Installation targets
.PHONY: install-buf
install-buf: ## Install buf tool
	BIN="/usr/local/bin" && \
	VERSION="1.65.0" && \
	curl -sSL \
	"https://github.com/bufbuild/buf/releases/download/v$${VERSION}/buf-$$(uname -s)-$$(uname -m)" \
	-o "$${BIN}/buf" && \
	chmod +x "$${BIN}/buf"
