.PHONY: build test lint fmt clean

# Pin tool versions for reproductivity
GOLANGCI_LINT_VERSION := v1.64.5

# Default binary output
BIN_DIR := ./bin
BIN_NAME := panex

build:
	go build -o $(BIN_DIR)/$(BIN_NAME) ./cmd/panex/...

test:
	@if find . -name '*_test.go' | grep -q .; then \
		go test -race -count=1 ./...; \
	else \
		echo "No test files found (ok)"; \
	fi

lint:
	@if find . -name '*.go' | grep -q .; then \
		golangci-lint run ./...; \
	else \
		echo "No Go files to lint (ok)"; \
	fi

fmt:
	goimports -w -local github.com/panex-dev/panex .
	gofmt -s -w .

clean:
	rm -rf $(BIN_DIR)
