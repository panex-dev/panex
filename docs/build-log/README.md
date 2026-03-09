# Build Log

This folder is the project memory for how Panex was built, one PR-sized increment at a time.

## Why this exists
- Preserve engineering intent, not only final code.
- Help new contributors understand sequencing and tradeoffs.
- Make PRs auditable with exact file and line citations.
- Train repeatable staff-level execution: small diffs, explicit contracts, and verification-first delivery.

## File conventions
- One file per meaningful increment, named: `YYYY-MM-DD-pr-<n>-<slug>.md`.
- Keep entries append-only; do not rewrite history unless correcting factual errors.
- Use [TEMPLATE.md](./TEMPLATE.md) for every new entry.
- Keep [STATUS.md](./STATUS.md) current with completed/in-flight/upcoming increments.

## Citation standard (required)
When documenting approach or PR descriptions, include exact file and line references.

Format:
- `path/to/file.go:12`
- `path/to/file.go:12-30`

Example:
- Added WebSocket handshake gate in `internal/daemon/websocket_server.go:146-199`.
- Enforced config validation in `internal/config/config.go:78-93`.

## PR writing standard (required)
In every PR description:
1. `Problem`: what breaks or is missing.
2. `Approach`: each bullet must include file+line references.
3. `Risk`: technical and operational risks.
4. `Verification`: exact commands and expected outputs.
5. `Teach-back`: one short section on reusable engineering lessons.

## Build-from-scratch operating model
1. Lock quality gates first (format, lint, CI, deterministic toolchain).
2. Define interfaces/contracts before implementation (CLI, config schema, protocol envelope).
3. Implement the thinnest vertical slice that can be tested end-to-end.
4. Add negative-path tests in the same PR as feature code.
5. Record architectural decisions in ADRs at decision time, not later.
6. Keep branches short-lived and non-stacked; every PR branch starts from latest `origin/main` in its own worktree.

## Current build check (2026-03-09)
- Completed log entries: PR1-PR42 and PR45-PR76 (PR20 reserved for numbering reconciliation, PR27 process-only).
- Next target increment: publish first-run config/schema documentation for `panex.toml`, including the auth token contract and supported local-development defaults.
- Queued follow-ons from the preserved 2026-03-07 review:
  - longer-horizon release work (multi-extension support, auth token override, packaging, timeline scalability, transactional storage persistence)
