
ifndef GOLANGCI_LINT_MK_INCLUDED

TOOLS_DIR := $(CURDIR)/.tools
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.60.1

export GOLANGCI_LINT

$(GOLANGCI_LINT):
	@echo "==> Installing golangci-lint into $(TOOLS_DIR)..."
	@mkdir -p $(TOOLS_DIR)
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TOOLS_DIR) $(GOLANGCI_LINT_VERSION)

.PHONY: lint-tools
lint-tools: $(GOLANGCI_LINT)

.PHONY: lint
lint: lint-tools ## Run static analysis via golangci-lint
	@echo "==> Checking source code against linters..."
	$(GOLANGCI_LINT) run -v ./...

gofmt: ## Format Go source code in 'internal/provider' using gofmt.
	@echo "==> Format code using gofmt..."
	gofmt -s -w internal/provider

GOLANGCI_LINT_MK_INCLUDED := 1
endif
