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

testacc:
	touch $(PWD)/.testacc_configure.sh
	ansible-playbook ci/awx_configure.yml -v -e 'config_file="$(PWD)/.testacc_configure.sh"'
	source $(PWD)/.testacc_configure.sh && TF_ACC=1 go test -v ./...
	rm -f $(PWD)/.testacc_configure.sh

gofmt:
	@echo "==> Format code using gofmt..."
	gofmt -s -w internal/provider