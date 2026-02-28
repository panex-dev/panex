# ADR-003: MVP WebSocket Protocol Envelope and Message Taxonomy

## Status
Accepted

## Context
Panex components (daemon, Dev Agent, Inspector) need a shared wire-level contract before networking code is introduced.
Without an explicit envelope and message taxonomy, components tend to drift and integration failures move from compile time to runtime.

The MVP needs to support:
- connection handshake (`hello`, `welcome`)
- daemon event stream (`build.complete`, `context.log`)
- daemon-to-agent control command (`command.reload`)

## Decision
Use a single MessagePack envelope for all messages:
- `v`: protocol version
- `t`: message type category (`lifecycle`, `event`, `command`)
- `name`: concrete message name (`hello`, `build.complete`, etc.)
- `src`: source identity `{ role, id }`
- `data`: message payload object

Define payload structs for the MVP message set in Go (`internal/protocol`) with MessagePack tags on all fields.
Reject unknown protocol versions at envelope validation time.

## Consequences
- All components can route messages by `name` while preserving coarse controls by `t`.
- Protocol upgrades are explicit via `v`.
- Source attribution is always available for logs and access control decisions.
- Runtime decoding stays flexible (`data` is envelope-generic), but each known message still has a typed payload.

## Reversibility
High.
If the taxonomy proves insufficient, we can add a new message type category or migrate envelope fields under a version bump (`v=2`) without breaking existing `v=1` consumers.
