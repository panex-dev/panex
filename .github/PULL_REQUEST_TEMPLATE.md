## Problem
- What is missing, broken, or risky?
- Who is impacted?

## Approach (with file+line citations)
- Change 1:
  - Why:
  - Where: `path/to/file.ext:line-line`
- Change 2:
  - Why:
  - Where: `path/to/file.ext:line-line`

## Protocol parity impact (required when protocol changes)
- [ ] This PR updates Go protocol definitions (`internal/protocol/types.go`) when required.
- [ ] This PR updates TypeScript protocol definitions (`shared/protocol/src/index.ts`) when required.
- [ ] `go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1` passes.
- [ ] I described protocol compatibility impact (additive vs breaking) in this PR.

## Branch base guard
- [ ] Branch was created from latest `origin/main` in its own worktree (`./scripts/pr-start.sh`).
- [ ] Branch is rebased onto latest `origin/main` (`./scripts/pr-ensure-rebased.sh` passes).

## Risk and mitigation
- Risk:
- Mitigation:

## Verification
- Commands run:
  - `make fmt`
  - `make lint`
  - `make test`
  - `make build`
- Additional checks:
  - command + expected result

## Teach-back
- Design lesson:
- Testing lesson:
- Workflow lesson:
