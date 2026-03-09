# ADR-024: Workbench Chrome API Activity Log Boundary

## Status
Accepted

## Context
Workbench already exposes the first constrained simulator-backed actions:
- namespaced storage presets
- a namespaced runtime probe
- replay of the latest observed runtime probe payload

Those actions all emit or depend on existing `chrome.api.*` traffic in the timeline, but operators still have to infer what happened by reading the raw event stream. The next useful product increment is visibility, not a broader command surface.

The main design question for this slice was whether the Workbench should become a generic protocol inspector for all timeline history or stay focused on the simulator-backed activity families it already exposes.

## Decision
- Add a Workbench activity log that derives from existing timeline entries only.
- Scope the log to `chrome.api.call`, `chrome.api.result`, and `chrome.api.event`.
- Pair `chrome.api.call` and `chrome.api.result` entries by `call_id` so the UI can show status and latency without adding new protocol messages.
- Surface unsupported and failed simulator calls distinctly.
- Keep the activity log inspector-only; do not widen daemon or protocol scope for this milestone.

## Consequences
- Positive:
  - Operators can inspect runtime probes, tabs queries, and unsupported simulator calls in one focused Workbench surface.
  - The product gains observability without creating new backend contracts.
  - Future chrome API work can build on one explicit UI summary instead of teaching operators to parse raw timeline entries manually.
- Tradeoff:
  - The activity log is intentionally not a general-purpose timeline replacement.
  - It only reflects message families that already have clear operator value in Workbench.

## Reversibility
High.
The activity log is a pure inspector-side derivation over existing timeline history, so the boundary can be widened or narrowed later without changing daemon behavior first.
