# Phase 2 config loader — evaluate `panex.config.ts`

**Status:** PR pending
**Date:** 2026-05-10

## Problem

The repo had converged on `panex.config.ts` as the canonical authored config path in several places:

- `internal/fsmodel` exposes `ConfigFilePath()` as `panex.config.ts`
- `internal/inspector` recommends `generate_panex_config` when `panex.config.ts` is missing
- `internal/graph` comments already described authored config as `panex.config.ts`

But `internal/configloader` still only understood `panex.config.json`. That left a real product mismatch: the loader could not read the config shape the rest of the repo was already naming as the intended authoring surface.

That mismatch also interacted with the new `add-target` mutation path. Once the loader can read TypeScript-authored config, `add-target` must stop pretending it can rewrite that authored file by silently creating or updating `panex.config.json`.

## Approach

- Extended `internal/configloader/configloader.go` to search `panex.config.ts` before `panex.config.json`.
- Added TypeScript config evaluation in `internal/configloader/configloader.go` by:
  - bundling the TS config entrypoint with Go-side esbuild,
  - emitting a temporary CommonJS bundle,
  - evaluating that bundle through a plain `node` subprocess,
  - decoding the JSON-serializable exported value back into `configloader.Config`.
- Kept `ConfigHash` anchored to the raw authored file bytes, so drift detection still reflects the source config text rather than the transpiled bundle output.
- Added a JSON-only mutation guard in `internal/cli/cli.go` so `add-target` now fails fast when the active authored config is `panex.config.ts`, instead of writing a sidecar JSON file that would drift from the real source of truth.
- Updated graph/config comments and the deferred-feature inventory to reflect that TS config evaluation is no longer pending, and removed the stale “plan rollback” gap because rollback already landed earlier in `internal/plan`.
- Added focused coverage in:
  - `internal/configloader/configloader_test.go`
  - `internal/cli/cli_test.go`

## Risk and mitigation

- Risk: TS config evaluation could introduce a new runtime dependency surface into Go-only paths.
- Mitigation: the loader uses only plain `node` at evaluation time, not a workspace-installed TypeScript runner, and returns a precise error if `node` is unavailable.

- Risk: bundling TS config through esbuild could change the meaning of imported helper modules.
- Mitigation: the loader bundles the config entrypoint as a single Node-targeted CommonJS artifact, and tests cover both direct default exports and local imported helper modules.

- Risk: enabling TS config reads without guarding config mutations would let mutation commands create misleading JSON sidecars.
- Mitigation: `add-target` now explicitly refuses to rewrite `panex.config.ts` and tells the caller to update the authored TypeScript config manually.

## Verification

- `pnpm install --frozen-lockfile`
- `make fmt`
- `go test ./internal/configloader ./internal/cli`
- `go test ./internal/configloader ./internal/cli ./internal/graph`
- `make check`
- `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`
- `./scripts/pr-ensure-rebased.sh`

## Next Step

Continue the remaining deferred-feature inventory with the next real behavioral gap, most likely `resume` step replay or MCP rollback exposure, now that the config layer matches the authored file path the rest of the repo already advertises.
