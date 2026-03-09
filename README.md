# Panex

A development runtime for Chrome extensions. Save and instantly see behavior across contexts with state inspection and replay.

> **Status:** Early development. Not usable yet.

## Prerequisites

- Go 1.25.8+
- [golangci-lint](https://golangci-lint.run/welcome/install/) v1.64.5
- [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) v1.1.4

## Setup

```bash
make init
go mod verify
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
```

## First-Run Config

`panex dev` reads `./panex.toml` by default. Override the location with `panex dev --config path/to/panex.toml`.

Start with this local-development config:

```toml
[extension]
source_dir = "examples/hello-extension"
out_dir = ".panex/dist"

[server]
port = 4317
auth_token = "replace-this-dev-token"
event_store_path = ".panex/events.db"
```

Config contract:

- `[extension].source_dir`: required path to the unpacked extension source tree that Panex watches and rebuilds.
- `[extension].out_dir`: required build output directory. It must not overlap `source_dir`.
- `[server].port`: required TCP port for the local daemon. Use any value from `1` to `65535`.
- `[server].auth_token`: required shared secret for local websocket clients. The daemon stays on `ws://127.0.0.1:<port>/ws`, and clients authenticate with this token during the `hello` handshake rather than in the URL.
- `[server].event_store_path`: optional SQLite path for the event log. If omitted, Panex defaults it to `.panex/events.db`.

Runtime override:

- Set `PANEX_AUTH_TOKEN` before `panex dev` to override `server.auth_token` for local automation or packaging flows without editing `panex.toml`.
- If `PANEX_AUTH_TOKEN` is set, it must be non-empty after trimming whitespace.

Validation rules:

- Unknown config keys are rejected.
- Empty required values are rejected.
- `source_dir` and `out_dir` cannot be the same directory or nested inside each other.

On startup, `panex dev` prints the loopback websocket URL for browser tooling:

```text
panex dev
ws_url=ws://127.0.0.1:4317/ws
```

## Development
```bash
make check  # type-check TypeScript packages
make fmt    # format code
make lint   # run linters
make test   # run Go tests with race detector + TypeScript package tests
make build  # compile ./bin/panex + frontend build outputs
go mod verify
govulncheck ./...
pnpm audit --audit-level high --prod
```

## Branch Workflow

Repository-wide agent operating rules live in [`AGENTS.md`](./AGENTS.md). Coding agents are expected to follow that protocol in addition to the branch workflow below.

Start every new PR from latest `origin/main` in a dedicated worktree:

```bash
./scripts/pr-start.sh feat/my-change
```

Install pre-push hooks once per clone to block stale branch pushes:

```bash
make init
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

## Agent Diagnostics

To enable temporary Chrome Dev Agent websocket lifecycle and command-handling diagnostics, set `chrome.storage.local.panex.diagnosticLogging = true`. The flag is off by default and emits structured service-worker console logs only when explicitly enabled.

Install TypeScript dependencies once from the repo root before using the root TypeScript targets:

```bash
pnpm install --frozen-lockfile
```

## Architecture Decisions

See [docs/adr/](docs/adr/) for all architecture decision records.
