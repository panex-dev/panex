# Configloader Windows Timeout Follow-Up

## Problem

PR #185's rebased CI run failed only on `go-test (windows-latest)`. The failing package was `internal/configloader`, where the first two TypeScript config evaluation tests hit the default 15 second Node subprocess evaluation deadline under `go test -race -count=1 ./...`.

The README PR did not touch configloader code, and the same CI run passed dependency verification, lint, TypeScript checks, and the Linux/macOS Go jobs. Treating that as a retry-only failure would leave a host-sensitive timeout in the TypeScript config loader.

## Change

- Increased the default TypeScript config evaluation timeout from 15 seconds to 45 seconds.
- Preserved the timeout behavior and its test hook; `TestEvaluateBundledConfigTimeout` still injects a 10 ms deadline and verifies the timeout error path.

## Verification

- `git diff --check`
- `GOTOOLCHAIN=go1.25.12+auto make fmt`
- `GOTOOLCHAIN=go1.25.12+auto GOCACHE=/tmp/go-build go test ./internal/configloader -count=1`
- `GOTOOLCHAIN=go1.25.12+auto GOCACHE=/tmp/go-build go test ./internal/configloader -race -count=1`
- `pnpm install --frozen-lockfile`
- `make check`
- `GOTOOLCHAIN=go1.25.12+auto GOCACHE=/tmp/go-build make test`
- `GOFLAGS=-buildvcs=false GOTOOLCHAIN=go1.25.12+auto GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOFLAGS=-buildvcs=false GOTOOLCHAIN=go1.25.12+auto GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Teach-Back

Timeouts that cover real subprocess startup need to account for slow CI hosts and race instrumentation, especially on Windows. Keep the negative-path timeout test small and injected, but give the production path enough budget to avoid failing valid config loads.
