# PR64 - Raw Payload Decode

## Metadata
- Date: 2026-03-09
- PR: 64
- Branch: `fix/pr64-protocol-raw-payload-decode`
- Title: decode raw protocol payload bytes without a msgpack round trip
- Commit(s): `fix(protocol): decode raw payload bytes directly`

## Problem
- The audit still had an open protocol codec issue: `DecodePayload` was re-marshaling already-encoded msgpack payloads before unmarshaling them into the target type.
- That extra encode/decode hop added avoidable work to every message path that flows through `DecodeEnvelope`, including daemon websocket traffic and stored event replay.

## Approach (with file+line citations)
- Change 1:
  - Why: preserve the envelope `data` field as `msgpack.RawMessage` when decoding so downstream callers can unmarshal the original payload bytes directly.
  - Where: `internal/protocol/codec.go:13-31`
- Change 2:
  - Why: teach `DecodePayload` to fast-path raw msgpack bytes while keeping a typed-value compatibility path for direct callers and tests that still pass concrete payload structs.
  - Where: `internal/protocol/codec.go:34-57`
- Change 3:
  - Why: lock the new contract with tests that prove round-trip decode now preserves raw bytes and that the compatibility path still works for typed inputs.
  - Where: `internal/protocol/codec_test.go:9-97`
- Change 4:
  - Why: mark the audit item resolved and record the slice in the repo status log.
  - Where: `audit.md:5-22`
  - Where: `docs/build-log/STATUS.md:61-65`
  - Where: `docs/build-log/2026-03-09-pr-064-raw-payload-decode.md:1-35`

## Protocol parity impact (required when protocol changes)
- [x] This PR updates Go protocol definitions (`internal/protocol/types.go`) when required.
- [x] This PR updates TypeScript protocol definitions (`shared/protocol/src/index.ts`) when required.
- [x] `go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1` passes.
- [x] I described protocol compatibility impact (additive vs breaking) in this PR.

Compatibility impact: non-breaking. The wire format is unchanged; only Go-side decode behavior now preserves already-encoded payload bytes instead of re-encoding them.

## Branch base guard
- [x] Branch was created from latest `origin/main` in its own worktree (`./scripts/pr-start.sh`).
- [x] Branch is rebased onto latest `origin/main` (`./scripts/pr-ensure-rebased.sh` passes).

## Risk and mitigation
- Risk: changing `DecodeEnvelope` to return raw msgpack bytes could break callers that were implicitly relying on `Data` being a decoded map or struct.
- Mitigation: the repo's message consumers already call `DecodePayload`, and the new tests pin the raw-byte contract plus typed compatibility fallback.
- Risk: a protocol-adjacent change could drift from the TypeScript definitions even without a wire-format change.
- Mitigation: this PR includes the protocol parity test in verification and explicitly documents that the change is wire-compatible.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/store -count=1`
  - `GOCACHE=/tmp/go-build go test ./internal/daemon -count=1`
- Additional checks:
  - `GOCACHE=/tmp/go-build go test ./internal/protocol -run TestTypeScriptProtocolParity -count=1`
  - Expected result: passes because the wire format is unchanged.

## Teach-back
- Design lesson: if a protocol layer already has encoded payload bytes in hand, preserve that stable representation until a concrete consumer asks for a typed decode.
- Testing lesson: protocol refactors need one test for the live path and one for compatibility fallbacks so optimizations do not silently narrow supported call patterns.
- Workflow lesson: audit cleanup is easier to review when each resolved item also updates the tracker and build-log in the same PR.
