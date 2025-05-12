.PHONY: build test lint

default: build

build:
	@echo "==> Building package..."
	go build

lint:
	@echo "==> Checking source code against linters..."
	golangci-lint run -v ./...

test:
	@echo "==> Running unit tests..."
	go test -v ./...

testacc:
	@echo "==> Running acceptance tests..."
	TF_ACC=1 AAP_HOST="https://localhost:8043" AAP_INSECURE_SKIP_VERIFY=true go test -count=1 -v ./...

testacc-aapdev:
	@echo "==> Running acceptance tests..."
	TF_ACC=1 AAP_HOST="http://localhost:9080" AAP_INSECURE_SKIP_VERIFY=true go test -count=1 -v ./...

gofmt:
	@echo "==> Format code using gofmt..."
	gofmt -s -w internal/provider

generatedocs:
	@echo "==> Formatting examples and generating docs..."
	terraform fmt -recursive ./examples/
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate
