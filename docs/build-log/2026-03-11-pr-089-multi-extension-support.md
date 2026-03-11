# PR89 Build Log: Multi-extension support

## Metadata
- Date: 2026-03-11
- PR: 89
- Branch: `feat/pr89-multi-extension-support`
- Title: `feat(config): add multi-extension support`
- Commit(s): `feat(config): add multi-extension support (#89)`

## Problem
- Panex only accepted one `[extension]` target, so operators could not run more than one extension build/watch loop from a single daemon config.
- Live reload broadcasts were global, so even if multiple targets were added later, agents would have no stable way to distinguish which extension a `command.reload` belonged to.

## Approach (with file+line citations)
- Change 1:
  - Why: accept either the legacy single-extension config or a new `[[extensions]]` list, while validating unique IDs and rejecting overlapping target paths.
  - Where: `internal/config/config.go:14-227`
  - Where: `internal/config/config_test.go:10-323`
- Change 2:
  - Why: start one builder/watch loop per configured extension target and tag build/reload envelopes with a stable `extension_id`.
  - Where: `cmd/panex/main.go:183-366`
  - Where: `cmd/panex/main_test.go:425-756`
  - Where: `internal/protocol/types.go:92-130`
- Change 3:
  - Why: route targeted reload traffic only to the matching dev-agent or `chrome-sim` session while keeping inspectors subscribed to the shared event stream.
  - Where: `internal/daemon/websocket_server.go:102-115`
  - Where: `internal/daemon/websocket_server.go:250-363`
  - Where: `internal/daemon/websocket_server.go:448-485`
  - Where: `internal/daemon/websocket_server.go:1365-1440`
  - Where: `agent/src/handshake.ts:16-97`
  - Where: `agent/src/reload.ts:9-30`
  - Where: `shared/chrome-sim/src/transport.ts:34-69`
  - Where: `shared/chrome-sim/src/transport.ts:204-257`
- Change 4:
  - Why: lock the behavior with config, daemon integration, CLI orchestration, and TypeScript package tests so the first multi-extension slice stays honest.
  - Where: `internal/daemon/integration_test.go:353-454`
  - Where: `agent/tests/handshake.test.ts:14-223`
  - Where: `shared/chrome-sim/tests/transport.test.ts:30-74`
  - Where: `shared/chrome-sim/tests/transport.test.ts:207-260`
  - Where: `README.md:27-145`
  - Where: `inspector/src/timeline.ts:160-176`
  - Where: `docs/build-log/STATUS.md:89-99`
  - Where: `docs/build-log/README.md:44-46`

## Risk and Mitigation
- Risk: operators may read “multi-extension support” as full per-extension product isolation even though the inspector event stream is still shared.
- Mitigation: the README and PR description explicitly scope this PR to config/build/watch/reload targeting, and the timeline now surfaces `ext=<id>` on relevant events.

## Verification
- Commands run:
  - `GOCACHE=/tmp/go-build go test ./internal/config ./internal/protocol ./cmd/panex -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -count=1 -run 'TestIntegrationTargetedReloadRoutesByExtensionID|TestIntegrationDaemonLifecycle|TestIntegrationStoragePersistsAcrossDaemonRestart'`
  - `CI=1 pnpm install --frozen-lockfile`
  - `pnpm --dir agent test`
  - `pnpm --dir agent check`
  - `pnpm --dir shared/protocol test`
  - `pnpm --dir shared/protocol check`
  - `pnpm --dir shared/chrome-sim test`
  - `pnpm --dir shared/chrome-sim check`
  - `pnpm --dir inspector test`
  - `pnpm --dir inspector check`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - `go test ./internal/daemon ...` was rerun outside the sandbox because the integration suite needs loopback `httptest` binding.

## Teach-back (engineering lessons)
- Design lesson: a stable target identifier is the minimum viable boundary for multi-extension support; broader isolation can be layered later without guessing hidden coupling first.
- Testing lesson: multi-target fanout needs both positive routing assertions and negative “other client receives nothing” coverage, otherwise broadcast regressions hide behind happy-path passes.
- Team workflow lesson: when a milestone is larger than one safe PR, document the exact slice boundary in the README and build log so reviewers can judge what shipped versus what remains.

## Next Step
- Decide whether to deepen multi-extension isolation next or return to a smaller follow-on such as inspector/workbench target selection; this PR intentionally stops at config, build/watch, and reload routing.
