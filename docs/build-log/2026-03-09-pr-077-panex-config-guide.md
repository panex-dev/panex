# PR77 - First-Run panex.toml Config Guide

## Metadata
- Date: 2026-03-09
- PR: 77
- Branch: `docs/pr77-panex-config-guide`
- Title: publish first-run `panex.toml` config/schema documentation and local-dev guidance
- Commit(s):
  - `docs: publish first-run panex.toml config guide`

## Problem
- The repo documented quality gates and branch workflow, but it still lacked a first-run guide for the required `panex.toml` contract.
- New contributors had to infer required keys, validation rules, the default config path, and the emitted loopback websocket URL by reading Go source and tests instead of following one operator-facing document.

## Approach (with file+line citations)
- Change 1:
  - Why: add a single README section that shows a working starter config, distinguishes real defaults from recommendations, and documents the daemon auth and loopback websocket contract in the place operators look first.
  - Where: `README.md:22-58`
  - Where: `internal/config/config.go:14-19`
  - Where: `internal/config/config.go:37-66`
  - Where: `internal/config/config.go:84-105`
  - Where: `cmd/panex/main.go:21-26`
  - Where: `cmd/panex/main.go:112-140`
  - Where: `cmd/panex/main.go:153-177`
- Change 2:
  - Why: close the current roadmap item and move the tracker to the next release-oriented slice once the docs exist.
  - Where: `docs/build-log/STATUS.md:70-87`
  - Where: `docs/build-log/README.md:44-48`

## Risk and Mitigation
- Risk: docs could claim defaults or transport behavior that do not match the actual CLI/config contract.
- Mitigation: the guide is derived directly from the config loader and CLI output paths, and it avoids inventing defaults for values like `server.port` that are still explicitly required.
- Risk: a docs-only PR could drift from the current roadmap if the tracker is not updated in the same change.
- Mitigation: the status tracker and build-log index were updated together with the README so completion is explicit and auditable.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Read `internal/config/config.go` and `cmd/panex/main.go` to ensure the new README text matches the enforced config keys, validation rules, default event-store path, and emitted `ws_url`.

## Teach-back
- Design lesson: if a config contract is already stable enough to validate strictly in code, it is stable enough to document as an explicit first-run surface instead of leaving contributors to reverse-engineer it.
- Documentation lesson: example config should separate true product defaults from recommended local values so operators know which behavior is guaranteed and which is only a starting point.
- Workflow lesson: docs-only milestones still need the same branch hygiene and full root verification, otherwise the roadmap becomes less trustworthy than code changes.

## Next Step
- Support a `$PANEX_AUTH_TOKEN` override so local automation and packaging flows can change daemon auth without rewriting `panex.toml`.
