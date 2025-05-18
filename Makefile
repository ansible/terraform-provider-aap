include makefiles/golangci.mk

.PHONY: build test lint


default: build

build: ## Compile the Go package. This is the default.
	@echo "==> Building package..."
	go build

test: ## Execute all unit tests with verbose output."
	@echo "==> Running unit tests..."
	go test -v ./...

testacc: ## Run acceptance tests against local aap-dev instance (https://localhost:8043)."
	@echo "==> Running acceptance tests..."
	TF_ACC=1 AAP_HOST="http://localhost:8043" go test -count=1 -v ./...

testacc-aapdev: ## Run acceptance tests against local aap-dev instance (EXPORT AAP_HOST="http://localhost:9080")
	@echo "==> Running acceptance tests..."
	TF_ACC=1 go test -count=1 -v ./...

generatedocs: ## Format example Terraform configurations and generate plugin documentation."
	@echo "==> Formatting examples and generating docs..."
	terraform fmt -recursive ./examples/
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate

.PHONY: help
help: ## Show this help message
	@grep -hE '^[a-zA-Z0-9._-]+:.*?##' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-24s\033[0m %s\n", $$1, $$2}' | \
	sort
