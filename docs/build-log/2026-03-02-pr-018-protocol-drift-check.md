# PR18 - Protocol Drift Check (Go vs TypeScript)

## Metadata
- Date: 2026-03-02
- PR: 18
- Branch: `feat/protocol-drift-check`
- Title: add cross-language protocol parity test
- Commit(s): pending

## Problem
- The protocol contract exists in both Go (`internal/protocol/types.go`) and TypeScript (`shared/protocol/src/index.ts`).
- We had no automated cross-language drift detection, so accidental divergence could pass local checks and fail later in integration.

## Approach (with file+line citations)
- Added a Go parity test that reads `shared/protocol/src/index.ts` and enforces equality for:
  - `PROTOCOL_VERSION`
  - `envelopeTypes`
  - `sourceRoles`
  - `envelopeNames`
  - `messageTypeByName`
  - `internal/protocol/parity_test.go:1-168`
- Documented that cross-language drift checks now exist:
  - `shared/protocol/README.md:23-29`

## Risk and Mitigation
- Risk: test uses source parsing and can become format-sensitive.
- Mitigation: parser intentionally targets stable exported constants and uses tolerant regex patterns for key/value extraction.

## Verification
- Commands run:
  - `go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
  - `go test ./...`

## Teach-back (engineering lessons)
- Shared contracts need shared enforcement; “remember to update both files” does not scale.
- Fast parity checks in existing CI lanes are better than adding heavyweight tooling early.
- Contract tests should fail with clear diff output so fixes are obvious.

## Next Step
- Add a CI-visible summary in PR templates so protocol changes always mention Go+TS parity impact.
