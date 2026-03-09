# PR69 - Go Toolchain Govulncheck

## Metadata
- Date: 2026-03-09
- PR: 69
- Branch: `fix/pr69-go-toolchain-govulncheck`
- Title: upgrade Go baseline to 1.25.8 and add a pinned `govulncheck` CI gate
- Commit(s):
  - `build(go): upgrade to Go 1.25.8 and gate CI with govulncheck`

## Problem
- The audit still had one open security-process item: `govulncheck` was not in CI because the project baseline was pinned to Go 1.24.0, and a real probe on that baseline reported reachable standard-library vulnerabilities.
- Leaving the repo on an unsupported toolchain also meant the earlier dependency-verification job could not honestly claim full Go-side vulnerability coverage.

## Approach (with file+line citations)
- Change 1:
  - Why: move the repository onto the smallest supported patched Go baseline that clears the standard-library findings.
  - Where: `go.mod:1-3`
- Change 2:
  - Why: align CI with that baseline and add a pinned `govulncheck` step inside the existing dependency-verification job.
  - Where: `.github/workflows/ci.yml:23-52`
  - Where: `.github/workflows/ci.yml:58-61`
- Change 3:
  - Why: update the local setup/development contract so contributor expectations match the new security gate.
  - Where: `README.md:7-30`
- Change 4:
  - Why: mark the audit item resolved and record the product increment in the tracker/build log.
  - Where: `audit.md:34-42`
  - Where: `docs/build-log/STATUS.md:67-70`
  - Where: `docs/build-log/2026-03-09-pr-069-go-toolchain-govulncheck.md:1-38`

## Risk and mitigation
- Risk: a Go minor-version bump could expose hidden compatibility issues in builds, tests, or linting.
- Mitigation: the branch was verified on the upgraded toolchain with the full root gate set plus explicit `govulncheck` runs on both the old and new baselines.
- Risk: floating the scanner version in CI would make future failures harder to attribute.
- Mitigation: CI installs pinned `govulncheck@v1.1.4` instead of using `@latest`.

## Verification
- Commands run:
  - `go version`
  - `go install golang.org/x/vuln/cmd/govulncheck@latest`
  - `GOCACHE=/tmp/go-build $(go env GOPATH)/bin/govulncheck ./...`
  - `GOTOOLCHAIN=go1.25.8+auto go install golang.org/x/vuln/cmd/govulncheck@v1.1.4`
  - `GOCACHE=/tmp/go-build $(go env GOPATH)/bin/govulncheck ./...`
  - `CI=1 pnpm install --frozen-lockfile`
  - `GOTOOLCHAIN=go1.25.8+auto go mod verify`
  - `pnpm audit --audit-level high --prod`
  - `make fmt`
  - `make check`
  - `GOTOOLCHAIN=go1.25.8+auto go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Baseline probe on Go 1.24.0 reported 17 reachable standard-library vulnerabilities.
  - Probe on Go 1.25.8 reported `No vulnerabilities found.`
  - `pnpm audit --audit-level high --prod` reported `No known vulnerabilities found`.

## Teach-back
- Design lesson: security gates should verify the supported platform baseline, not merely the application code layered on top of an outdated runtime.
- Testing lesson: when a tool reports standard-library vulnerabilities, probe the same tree under a patched toolchain before concluding the fix requires application changes.
- Workflow lesson: pin scanner versions in CI when turning a security check from an investigative tool into a required gate.
