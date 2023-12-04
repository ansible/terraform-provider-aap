.PHONY: build test lint

default: build

build: 
	@echo "==> Building package..."
	go build ./...

lint:
	@echo "==> Checking source code against linters..."
	golangci-lint run ./...

test:
	@echo "==> Running unit tests..."
	go test -v ./...

testacc:
	TF_ACC=1 go test -v ./...