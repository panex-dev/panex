# PR40 - Runtime Namespace Extension + Chrome-Sim Bootstrap Wiring

## Metadata
- Date: 2026-03-05
- PR: 40
- Branch: `feat/pr40-chrome-sim-preview-bootstrap`
- Title: extend chrome-sim beyond storage with runtime.sendMessage and bootstrap parameter resolution
- Commit(s): pending

## Problem
- `@panex/chrome-sim` only exposed `chrome.storage.*`, so simulator namespace coverage had no runtime path and no way to observe runtime message events from transport.
- `installChromeSim()` had no bootstrap resolution for URL query parameters used by preview surfaces (`ws`, `token`, `extension_id`), which blocked predictable daemon wiring during preview entrypoint injection.

## Approach (with file+line citations)
- Extended daemon-side `chrome.api.call` router to support namespace dispatch for `runtime.sendMessage`:
  - `chrome.api.call` now routes `runtime` separately from storage namespaces:
  - `internal/daemon/websocket_server.go:516-549`
  - moved storage operations into a dedicated helper:
  - `internal/daemon/websocket_server.go:551-618`
  - added runtime handler:
  - `runtime.sendMessage` validates args, broadcasts `chrome.api.event` (`runtime.onMessage`), and returns correlated success payload:
  - `internal/daemon/websocket_server.go:620-661`
- Added daemon websocket tests for runtime simulator behavior:
  - `runtime.sendMessage` emits `chrome.api.event` and `chrome.api.result`:
  - `internal/daemon/websocket_server_test.go:901-961`
  - missing message argument returns failure result (non-transport-fatal):
  - `internal/daemon/websocket_server_test.go:963-1002`
- Extended `@panex/chrome-sim` runtime surface and bootstrap resolution:
  - added runtime namespace adapter with:
  - `id`,
  - `sendMessage(...args)` routing to `chrome.api.call`,
  - `onMessage` listener registry wired from `chrome.api.event`:
  - `shared/chrome-sim/src/runtime.ts:1-99`
  - `installChromeSim()` now installs both `chrome.storage.*` and `chrome.runtime`, and resolves `ws/token/extension_id` from URL + globals:
  - `shared/chrome-sim/src/index.ts:1-118`
  - package export updated for runtime entrypoint:
  - `shared/chrome-sim/package.json:7-13`
- Added/updated tests for runtime namespace and bootstrap install behavior:
  - runtime adapter unit tests:
  - `shared/chrome-sim/tests/runtime.test.ts:1-78`
  - install shim now validates runtime installation + extension ID precedence:
  - `shared/chrome-sim/tests/index.test.ts:12-70`
  - README scope updated:
  - `shared/chrome-sim/README.md:5-17`

## Risk and Mitigation
- Risk: runtime message fanout could alter message ordering around correlated `chrome.api.result`.
- Mitigation: tests assert both event and result are delivered with expected payload contracts (`internal/daemon/websocket_server_test.go:901-961`).
- Risk: bootstrap source precedence can produce wrong daemon routing in preview if query/global parsing is inconsistent.
- Mitigation: install tests verify query-derived extension ID and explicit-option override behavior (`shared/chrome-sim/tests/index.test.ts:12-70`).

## Verification
- Commands run:
  - `cd shared/chrome-sim && pnpm run check`
  - `cd shared/chrome-sim && pnpm run test`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint-cache golangci-lint run ./internal/daemon`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -run '^$' -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -count=1 -run 'TestWebSocketChromeAPICallRuntimeSendMessage'`
- Expected:
  - chrome-sim TS check + tests pass, including new runtime tests.
  - daemon lint passes with no `nilerr` regressions.
  - daemon package compiles.
  - targeted runtime websocket tests pass.

## Teach-back (engineering lessons)
- Namespace expansion is safer when protocol transport remains generic (`chrome.api.call/result/event`) and runtime semantics are added in router-level handlers.
- Shim bootstrap resolution should be centralized so preview entrypoints can stay thin and deterministic.
- Simulated runtime behavior should emit both correlated result and event stream artifacts so inspector timelines stay truthful.

## Next Step
- Wire `@panex/chrome-sim` install bootstrap into preview entrypoint injection flow (once preview server/plugin wiring lands) and extend simulator coverage with another namespace (`tabs` or deeper runtime APIs).
