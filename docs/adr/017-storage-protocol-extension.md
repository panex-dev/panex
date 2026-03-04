# ADR-017: Storage Query Protocol Extension

## Status
Accepted

## Context
The inspector shell now includes a Storage tab placeholder, but the protocol only supports timeline history queries (`query.events`).
To unblock storage UI development, the daemon and clients need a dedicated storage query contract that is explicit in both Go and TypeScript definitions.

## Decision
- Add storage protocol messages:
  - `query.storage` (command)
  - `query.storage.result` (event)
  - `storage.diff` (event)
- Add payload types:
  - `QueryStorage { area?: "local" | "sync" | "session" }`
  - `QueryStorageResult { snapshots: StorageSnapshot[] }`
  - `StorageSnapshot { area, items }`
  - `StorageDiff { area, changes }`
  - `StorageChange { key, old_value?, new_value? }`
- Implement daemon-side `query.storage` stub handler that returns empty snapshots:
  - all areas when no area is requested
  - one area when `area` is specified
- Validate area values (`local`, `sync`, `session`) and reject unsupported areas.

## Consequences
- Positive:
  - Storage tab implementation can proceed without waiting for full simulator state.
  - Protocol now expresses storage intent explicitly and supports future diff streaming.
  - Capability negotiation can advertise `query.storage`.
- Tradeoff:
  - `storage.diff` is defined before full mutation pipeline exists, so it is schema-first in this increment.

## Reversibility
High.
If storage behavior changes, schema adjustments are additive in protocol v1 as long as existing fields remain valid.
