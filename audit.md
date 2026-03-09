# Audit Tracker

This file tracks follow-up work from [audit-1.md](./audit-1.md) until the original audit can be retired.

## Resolved in PR61

- Agent websocket config now falls back to the default daemon URL unless the stored value stays on the loopback websocket contract.
- Inspector query-param overrides now accept only top-level loopback websocket URLs and ignore all overrides when the inspector is embedded.

## Resolved in PR62

- `make init` now installs the repo's git hooks through the supported bootstrap entrypoint.

## Resolved in PR63

- CI now runs `go mod verify` and `pnpm audit --audit-level high --prod` as an explicit dependency-verification gate.

## Resolved in PR64

- Protocol envelope decoding now preserves raw msgpack payload bytes, and payload decoding unmarshals those bytes directly instead of doing a marshal-then-unmarshal round trip.

## Resolved in PR65

- Daemon client-message handling now inherits the server lifecycle context, and session close/write operations are serialized so failed broadcasts cannot race connection shutdown.

## Resolved in PR66

- `cmd/panex` now has dedicated startup orchestration tests covering injected daemon/build/watcher wiring, which raises package coverage from 46.2% to 76.2%.

## Resolved in PR67

- All browser-side websocket clients now reject inbound frames larger than the 1 MiB daemon contract before decoding them, and close the socket with code `1009` instead of attempting to process oversized payloads.

## Resolved in PR68

- Daemon auth now moves through the initial `hello` payload instead of the websocket URL query string, and browser-side websocket URL builders strip stale `token=` params before connect.

## Deferred Or Dependent Items

- Broad dev-agent `host_permissions` are still open.
  Reason: narrowing the manifest to `:4317` would break non-default daemon ports, and the correct MV3 permission model needs to be verified before changing product behavior.
- Go toolchain vulnerability scanning via `govulncheck` is still open.
  Reason: a probe on 2026-03-09 reported multiple standard-library vulnerabilities against the current Go 1.24.0 baseline, so enabling that gate now would redline CI until the project upgrades its Go toolchain policy.
