# Panex

A development runtime for Chrome extensions. Save and instantly see behavior across contexts with state inspection and replay.

> **Status:** Early development. Not usable yet.

## Prerequisites

- Go 1.24+
- [golangci-lint](https://golangci-lint.run/welcome/install/) v1.64.5
- [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)

## Setup

```bash
go mod verify
```

## Development
```bash
make check  # type-check TypeScript packages
make fmt    # format code
make lint   # run linters
make test   # run Go tests with race detector + TypeScript package tests
make build  # compile ./bin/panex + frontend build outputs
```

## Branch Workflow

Start every new PR from latest `origin/main` in a dedicated worktree:

```bash
./scripts/pr-start.sh feat/my-change
```

Install pre-push hooks once per clone to block stale branch pushes:

```bash
./scripts/install-git-hooks.sh
```

Before push (and in CI), verify branch base:

```bash
./scripts/pr-ensure-rebased.sh
```

After a PR is merged, delete branch/worktree and return to `main`:

```bash
./scripts/pr-finish.sh feat/my-change
```

## Frontend Packages

- `agent/`: Chrome Dev Agent extension (`pnpm run check|test|build`)
- `inspector/`: SolidJS timeline inspector (`pnpm run check|test|build`)
- `shared/protocol/`: shared TypeScript protocol contract consumed by both clients
- `shared/chrome-sim/`: browser shim that routes `chrome.*` simulator calls over daemon WebSocket

Install TypeScript dependencies once from the repo root before using the root TypeScript targets:

```bash
pnpm install --frozen-lockfile
```

## Architecture Decisions

See [docs/adr/](docs/adr/) for all architecture decision records.
