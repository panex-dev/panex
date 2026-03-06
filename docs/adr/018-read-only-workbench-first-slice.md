# ADR-018: Read-Only Workbench First Slice

## Status
Accepted

## Context
The inspector shell already reserves `#workbench`, but the tab is still disabled even though the router, connection provider, and shell composition were introduced specifically to make new tabs safe to add.

The product roadmap needs Workbench to become a real surface, but there is still no settled interaction model for mutating tools, replay controls, or daemon-side workbench APIs.

## Decision
- Enable the Workbench tab now as a read-only operator overview.
- Build it entirely from existing inspector state:
  - connection status
  - daemon websocket URL
  - current error state
  - storage snapshot counts
  - timeline event counts and latest event identity
- Do not add new daemon APIs, protocol messages, or mutating UI actions in this slice.
- Defer actionable workbench tools to later PRs after the read-only surface proves the information architecture.

## Consequences
- Positive:
  - Workbench becomes a real roadmap surface without expanding protocol or daemon scope.
  - The tab validates that the shell and connection-provider architecture scales beyond Timeline and Storage.
  - Future workbench actions can be layered onto an existing user-facing surface instead of appearing all at once.
- Tradeoff:
  - The first Workbench slice is intentionally informational rather than interactive.
  - Some information duplicates sidebar/storage/timeline state while the interaction model is still being shaped.

## Reversibility
High.
The read-only card layout can be replaced later without affecting router, connection, or protocol contracts because this slice adds no new transport behavior.
