# ADR-016: Unified Inspector Shell and Hash Router

## Status
Accepted

## Context
The inspector runtime had all transport, filtering, persistence, and rendering responsibilities in `inspector/src/main.tsx`.
That made upcoming milestones (Storage, Workbench, Replay) risky because each new tab would increase coupling in one file.

The roadmap requires one unified cockpit with shared connection state and tab-local rendering behavior.

## Decision
- Extract WebSocket lifecycle, handshake/query bootstrap, reconnect policy, and timeline state into `ConnectionProvider`.
- Add a hash-based inspector router with tab IDs:
  - `#timeline`
  - `#storage`
  - `#workbench`
  - `#replay`
- Introduce a shared shell component that renders:
  - tab bar
  - sidebar slot
  - tab content slot
- Move timeline-specific filter/search UI into `tabs/timeline.ts`.
- Add `tabs/storage.ts` placeholder and keep Workbench/Replay disabled for this milestone.
- Keep the protocol and timeline merge semantics unchanged.

## Consequences
- Positive:
  - We can add new tabs without touching transport code paths.
  - Connection state becomes single-source and reusable by all tabs.
  - Hash routing supports direct links and browser navigation semantics.
- Tradeoff:
  - More files and indirection in inspector package.
  - Shell/sidebar structure exists before PR22 sidebar feature completion.

## Reversibility
High.
If router/shell choices are wrong, tab components and connection provider can be retained while changing container composition.
