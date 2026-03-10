# Contributing To Panex

This document is for human contributors and maintainers. It covers local setup, repository workflow, release workflow, and where to look in the codebase.

Coding agents must also follow the stricter contract in [AGENTS.md](./AGENTS.md).

## Start Here

- Product and operator usage: [README.md](./README.md)
- Repository layout: [docs/repo-map.md](./docs/repo-map.md)
- Architecture decisions: [docs/adr/](./docs/adr/)
- Increment history and roadmap tracker: [docs/build-log/](./docs/build-log/)

## Tooling Prerequisites

- Go `1.25.8+`
- `pnpm`
- [golangci-lint](https://golangci-lint.run/welcome/install/) `v1.64.5`
- [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) `v1.1.4`

## Local Setup

```bash
make init
pnpm install --frozen-lockfile
go mod verify
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
```

`make init` installs the repo git hooks, including the branch-base pre-push guard.

## Daily Workflow

Start every PR from latest `origin/main` in a dedicated worktree:

```bash
git fetch origin
./scripts/pr-start.sh feat/my-change
```

Work inside the created worktree. Before push, verify the branch is still based on latest `origin/main`:

```bash
./scripts/pr-ensure-rebased.sh
```

After a PR is merged, clean up with:

```bash
./scripts/pr-finish.sh feat/my-change
```

## Verification Expectations

Minimum default verification for a normal PR:

```bash
make fmt
make check
GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint
GOCACHE=/tmp/go-build make test
GOCACHE=/tmp/go-build make build
```

Additional release-oriented checks when relevant:

```bash
go mod verify
govulncheck ./...
pnpm audit --audit-level high --prod
```

## Release Workflow

Package deterministic release archives locally with:

```bash
make release VERSION=v0.1.0
```

Limit the matrix with `TARGETS=` when needed:

```bash
make release VERSION=v0.1.0 TARGETS=linux/amd64,darwin/arm64
```

Tagged releases are published from CI. Push a version tag that already points at a commit on `main`:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The `Release` workflow reruns dependency verification, lint, tests, and builds before publishing the generated `dist/release/*` archives plus `panex_<version>_SHA256SUMS` to the GitHub release. Tags with a hyphen, such as `v0.1.0-rc.2`, publish as prereleases.

## Frontend Packages

- `agent/`: Chrome Dev Agent extension
- `inspector/`: SolidJS inspector UI
- `shared/protocol/`: shared TypeScript protocol package
- `shared/chrome-sim/`: browser shim that routes `chrome.*` simulation calls over the daemon WebSocket

Each package has its own package-level README where the package needs one.

## Notes For Agents

Agents should not treat this file as a replacement for [AGENTS.md](./AGENTS.md). `AGENTS.md` remains the mandatory execution contract for coding agents.
