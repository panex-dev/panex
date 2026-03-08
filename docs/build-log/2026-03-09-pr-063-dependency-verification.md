# PR63 - Dependency Verification

## Metadata
- Date: 2026-03-09
- PR: 63
- Branch: `fix/pr63-dependency-verification`
- Title: add CI dependency verification with `go mod verify` and `pnpm audit`
- Commit(s): pending

## Problem
- The audit still had an open dependency-verification gap: CI checked build/test/lint surfaces, but it never verified Go module integrity or audited JavaScript dependencies against the registry advisory feed.
- That left the repo without any automated gate for dependency tampering or known high-severity npm vulnerabilities.

## Approach (with file+line citations)
- Change 1:
  - Why: add a dedicated CI job that verifies Go modules and audits production JavaScript dependencies once per workflow run, instead of duplicating audit work across the TypeScript matrix.
  - Where: `.github/workflows/ci.yml:11-47`
- Change 2:
  - Why: document the dependency-verification commands alongside the existing repo development commands so local verification matches the new CI contract.
  - Where: `README.md:20-28`
- Change 3:
  - Why: mark the original audit item resolved while keeping the separate `govulncheck` toolchain-lifecycle blocker explicit.
  - Where: `audit.md:5-28`
  - Where: `docs/build-log/STATUS.md:61-64`
  - Where: `docs/build-log/2026-03-09-pr-063-dependency-verification.md:1-47`

## Risk and Mitigation
- Risk: `pnpm audit` can be noisy or flaky if run redundantly in each package job.
- Mitigation: the audit runs once in a dedicated job after a single root install, which keeps the signal centralized and reduces registry churn.
- Risk: `govulncheck` would currently fail the repo because the Go 1.24.0 baseline itself is behind standard-library security fixes.
- Mitigation: this PR does not pretend that blocker is solved; it records the `govulncheck` result in `audit.md` as a separate follow-up.

## Verification
- Commands run:
  - `go mod verify`
  - `CI=1 pnpm install --frozen-lockfile`
  - `pnpm audit --audit-level high --prod`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
  - Result: this currently reports standard-library vulnerabilities against the Go 1.24.0 baseline, so it remains tracked but is not added as a failing CI gate in this PR.

## Teach-back
- Design lesson: dependency verification belongs in its own CI job when the signal is orthogonal to build/test execution.
- Testing lesson: before adding a new security gate, probe the live dependency surface first; otherwise the first CI run becomes a discovery mechanism instead of a verification mechanism.
- Workflow lesson: when a new check exposes a separate systemic blocker, record that blocker explicitly instead of burying it under a partially “resolved” audit item.
