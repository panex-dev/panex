# PR42 - `tabs.query` Simulator Wiring Across Daemon + Chrome-Sim

## Metadata
- Date: 2026-03-05
- PR: 42
- Branch: `feat/pr42-preview-html-injection-path`
- Title: add `tabs.query` simulator support in daemon router and `@panex/chrome-sim`
- Commit(s): pending

## Problem
- Simulator coverage stopped at `storage.*` and `runtime.sendMessage`; `chrome.tabs.query(...)` calls had no supported contract path in either daemon or shim.
- Without a tabs namespace slice, previewed extension code that reads active/current tabs cannot run against the simulator baseline.

## Approach (with file+line citations)
- Added browser shim tabs namespace and installation wiring:
  - introduced `createTabsNamespace()` with payload normalization to `SimulatedTab[]`: `shared/chrome-sim/src/tabs.ts:1-92`
  - installed `chrome.tabs.query` alongside existing storage/runtime shims: `shared/chrome-sim/src/index.ts:6-67`
  - exported new tabs entrypoint: `shared/chrome-sim/package.json:7-14`
  - added tabs adapter tests and install assertions: `shared/chrome-sim/tests/tabs.test.ts:1-61`, `shared/chrome-sim/tests/index.test.ts:12-69`
  - documented tabs coverage in package README scope: `shared/chrome-sim/README.md:5-18`
- Extended daemon `chrome.api.call` routing for tabs namespace:
  - added tabs state on websocket server startup: `internal/daemon/websocket_server.go:56-98`, `internal/daemon/websocket_server.go:122-131`
  - routed `namespace=tabs` in the `chrome.api.call` dispatcher: `internal/daemon/websocket_server.go:539-575`
  - implemented `tabs.query` handler, filter parsing, and match logic for `active`, `currentWindow`, `windowId`, and `url`: `internal/daemon/websocket_server.go:689-712`, `internal/daemon/websocket_server.go:862-1026`
  - added default simulated tabs and cloning helper for safe read paths: `internal/daemon/websocket_server.go:1355-1404`
- Added daemon websocket tests for tabs behavior and adjusted unsupported-namespace coverage:
  - positive `tabs.query` result path with active/currentWindow filtering: `internal/daemon/websocket_server_test.go:1004-1062`
  - invalid tabs filter returns `chrome.api.result` failure without dropping connection: `internal/daemon/websocket_server_test.go:1064-1121`
  - unsupported namespace test now uses a truly unsupported namespace (`bookmarks`): `internal/daemon/websocket_server_test.go:1123-1179`

## Risk and Mitigation
- Risk: tabs simulation data is static and may diverge from richer browser tab behavior.
- Mitigation: scope is explicit (`tabs.query` only), with deterministic fixture tabs and strict validation on unsupported/invalid filters to fail clearly.
- Risk: daemon payload shape drift (`ID` vs `id`) could break shim normalization.
- Mitigation: tabs result struct fields are explicitly tagged for msgpack/json key compatibility (`id`, `windowId`, etc.): `internal/daemon/websocket_server.go:81-88`.

## Verification
- Commands run:
  - `cd shared/chrome-sim && pnpm run check`
  - `cd shared/chrome-sim && pnpm run test`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache golangci-lint run ./internal/daemon`
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -run 'ChromeAPICallTabsQuery|ChromeAPICallUnsupportedNamespaceReturnsFailureResult' -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -count=1`
- Expected:
  - chrome-sim typecheck/tests pass with tabs namespace assertions.
  - daemon lint passes with no `nilerr` regressions.
  - parity check remains green (no protocol schema drift in this PR).
  - daemon websocket tests pass for new tabs paths and existing namespace guards.

## Teach-back (engineering lessons)
- Namespace expansion is safest when shim and daemon are delivered in the same PR slice with shared verification paths.
- For generic `any` payload channels, explicit codec tags are mandatory when consumers depend on exact field names.
- Negative-path websocket tests should verify both failure payloads and connection liveness to prevent transport-level regressions.

## Next Step
- Proceed with preview entrypoint injection wiring so `@panex/chrome-sim` auto-installs in preview surfaces and consumes the now-expanded namespace set (`storage`, `runtime`, `tabs`).
