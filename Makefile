.PHONY: init build build-go build-ts check check-ts test test-go test-ts lint fmt clean

# Pin tool versions for reproductivity
GOLANGCI_LINT_VERSION := v1.64.5

# Default binary output
BIN_DIR := ./bin
BIN_NAME := panex
TS_CHECK_DIRS := shared/protocol agent inspector shared/chrome-sim
TS_BUILD_DIRS := agent inspector

init:
	./scripts/install-git-hooks.sh

build: build-go build-ts

build-go:
	go build -o $(BIN_DIR)/$(BIN_NAME) ./cmd/panex/...

build-ts:
	@set -e; \
	for dir in $(TS_BUILD_DIRS); do \
		echo "pnpm --dir $$dir run build"; \
		pnpm --dir "$$dir" run build; \
	done

check: check-ts

check-ts:
	@set -e; \
	for dir in $(TS_CHECK_DIRS); do \
		echo "pnpm --dir $$dir run check"; \
		pnpm --dir "$$dir" run check; \
	done

test: test-go test-ts

test-go:
	@if find . -name '*_test.go' | grep -q .; then \
		go test -race -count=1 ./...; \
	else \
		echo "No test files found (ok)"; \
	fi

test-ts:
	@set -e; \
	for dir in $(TS_CHECK_DIRS); do \
		echo "pnpm --dir $$dir run test"; \
		pnpm --dir "$$dir" run test; \
	done

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
