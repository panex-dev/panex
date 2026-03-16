# Panex — Agent Instructions

## PR Workflow

**All PRs must target `main`.** Never create a PR with a feature branch as the base.

- Use `scripts/pr-start.sh <branch-name>` to create branches (always from `origin/main`)
- Use `scripts/pr-create.sh` (or `make pr`) to create PRs (enforces `--base main`)
- Use `scripts/pr-finish.sh <branch-name>` to clean up after merge
- CI `branch-base-guard` will reject any PR not targeting `main`

Chained PRs (where PR-B targets PR-A's branch instead of main) are prohibited. Each PR must stand alone against main.

## Quality Gates

Before committing, run:
```
make fmt && make lint && make test && make build
```

For TypeScript packages:
```
cd <package> && pnpm run check && pnpm run test && pnpm run build
```

## Commit Messages

- Do not add `Co-Authored-By` lines to commit messages
- Use conventional commit format: `type(scope): description (#PR)`

## Build Logs

Every PR gets a build log in `docs/build-log/` and an entry in `docs/build-log/STATUS.md`.
