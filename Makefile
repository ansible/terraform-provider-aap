.PHONY: build test lint

default: build

build: 
	@echo "==> Building package..."
	go build ./...

lint:
	@echo "==> Checking source code against linters..."
	golangci-lint run -v ./...

test:
	@echo "==> Running unit tests..."
	go test -v ./...

gofmt:
	@echo "==> Format code using gofmt..."
	gofmt -s -w internal/provider