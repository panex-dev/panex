.PHONY: init build build-go build-ts check check-ts test test-go test-ts lint fmt release clean pr

# Pin tool versions for reproductivity
GOLANGCI_LINT_VERSION := v1.64.5

# Default binary output
BIN_DIR := ./bin
BIN_NAME := panex
RELEASE_DIR := ./dist/release
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

release:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required (example: make release VERSION=v0.1.0)"; \
		exit 1; \
	fi
	@set -e; \
	args="--version $(VERSION) --out-dir $(RELEASE_DIR)"; \
	if [ -n "$(TARGETS)" ]; then \
		args="$$args --targets $(TARGETS)"; \
	fi; \
	go run ./cmd/panex-release $$args

pr:
	./scripts/pr-create.sh

clean:
	rm -rf $(BIN_DIR) $(RELEASE_DIR)
