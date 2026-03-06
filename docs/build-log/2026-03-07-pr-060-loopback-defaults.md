# PR60 - Keep 127.0.0.1 as the product default and document localhost override

## Summary
- Kept browser-facing Panex local defaults on `ws://127.0.0.1:4317/ws`.
- Preserved explicit override paths so operators can opt into `localhost` when their browser environment requires it.
- Updated preview guidance to document that override instead of shifting the product default.

## Why
- Product defaults should stay deterministic and explicit. `127.0.0.1` avoids resolver and host-mapping ambiguity that can come with `localhost`.
- The daemon already allows both `127.0.0.1` and `localhost` origins, so the product can remain opinionated without blocking local operator choice.
- A local environment that needs `localhost` is better handled as an explicit override than as a repo-wide default change.

## Changes
- Kept inspector query-param fallback websocket URL at `ws://127.0.0.1:4317/ws` in `inspector/src/connection.ts`.
- Kept preview build injection default at `ws://127.0.0.1:4317/ws` in `inspector/scripts/build.ts`.
- Kept chrome-sim fallback transport default at `ws://127.0.0.1:4317/ws` in `shared/chrome-sim/src/transport.ts`.
- Kept dev-agent stored-config default at `ws://127.0.0.1:4317/ws` in `agent/src/config.ts`.
- Kept `panex dev` startup output and auto-injected daemon URL on `127.0.0.1` in `cmd/panex/main.go`.
- Documented the explicit `localhost` override path in `inspector/README.md`.

## Verification
- `pnpm install`
- `GOCACHE=/tmp/go-build make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `git diff --check`

## Outcome
- Product defaults stay on `127.0.0.1`.
- Operators who need `localhost` can still opt in via URL or preview-build environment overrides.
