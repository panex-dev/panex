# Audit Tracker

This file tracks follow-up work from [audit-1.md](./audit-1.md) until the original audit can be retired.

## Resolved in PR61

- Agent websocket config now falls back to the default daemon URL unless the stored value stays on the loopback websocket contract.
- Inspector query-param overrides now accept only top-level loopback websocket URLs and ignore all overrides when the inspector is embedded.

## Resolved in PR62

- `make init` now installs the repo's git hooks through the supported bootstrap entrypoint.

## Resolved in PR63

- CI now runs `go mod verify` and `pnpm audit --audit-level high --prod` as an explicit dependency-verification gate.

## Deferred Or Dependent Items

- Broad dev-agent `host_permissions` are still open.
  Reason: narrowing the manifest to `:4317` would break non-default daemon ports, and the correct MV3 permission model needs to be verified before changing product behavior.
- Query-string token transport remains in place.
  Reason: browser WebSocket clients cannot send arbitrary auth headers, so removing `?token=` requires a different handshake/auth contract rather than a local cleanup.
- `internal/protocol/codec.go` still does marshal-then-unmarshal payload decoding.
  Reason: fixing this cleanly needs a protocol decode API change, not just an error-message tweak.
- Browser-side inbound websocket message size limiting is still open.
  Reason: the daemon now enforces a read limit, but browser WebSocket clients do not expose equivalent receive-side caps.
- `cmd/panex` orchestration coverage is still low.
  Reason: needs a dedicated CLI lifecycle test slice instead of being mixed into this client-side hardening PR.
- Go toolchain vulnerability scanning via `govulncheck` is still open.
  Reason: a probe on 2026-03-09 reported multiple standard-library vulnerabilities against the current Go 1.24.0 baseline, so enabling that gate now would redline CI until the project upgrades its Go toolchain policy.
