# Repo Map

This document is the shortest useful map for contributors who need to find the right layer before editing.

## Product Entry Points

- `README.md`: product overview, install/use, config, and release verification
- `CONTRIBUTING.md`: contributor workflow, verification, and release process
- `AGENTS.md`: mandatory coding-agent execution protocol

## Go Runtime

- `cmd/panex/`: CLI entrypoint for `panex version` and `panex dev`
- `cmd/panex-release/`: release packager CLI used by `make release`
- `internal/build/`: extension build pipeline, asset copying, HTML rewriting, and chrome-sim injection
- `internal/config/`: `panex.toml` loading and validation
- `internal/daemon/`: local WebSocket daemon, session lifecycle, command/query routing
- `internal/protocol/`: Go message envelope and protocol helpers
- `internal/store/`: SQLite-backed event store
- `internal/release/`: deterministic archive and checksum generation

## Browser Packages

- `agent/`: Chrome Dev Agent extension
- `inspector/`: inspector UI
- `shared/protocol/`: shared TypeScript protocol contract for browser packages
- `shared/chrome-sim/`: browser shim for simulated `chrome.*` calls

## Scripts And Automation

- `scripts/pr-start.sh`: create a PR branch in a dedicated worktree from latest `origin/main`
- `scripts/pr-ensure-rebased.sh`: enforce latest-`main` branch ancestry before push
- `scripts/pr-finish.sh`: clean up a merged PR branch and worktree
- `.github/workflows/`: CI and release automation

## Project Memory

- `docs/adr/`: architecture decisions and boundary choices
- `docs/build-log/`: one-file-per-PR delivery history and status tracker

## Where To Look For Common Tasks

- CLI/config startup behavior: `cmd/panex/` and `internal/config/`
- Build output problems: `internal/build/`
- WebSocket or daemon behavior: `internal/daemon/`
- Protocol or client/server message drift: `internal/protocol/` and `shared/protocol/`
- Inspector UI behavior: `inspector/`
- Dev Agent runtime behavior: `agent/`
- Release packaging or checksums: `cmd/panex-release/` and `internal/release/`
