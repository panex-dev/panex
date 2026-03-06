# ADR-021: Workbench Runtime Replay Control

## Status
Accepted

## Context
PR56 added the first Workbench runtime probe using the existing `runtime.sendMessage` transport and timeline-derived result summaries.

The next roadmap step is to validate a replay control, but a full replay engine or dedicated daemon-side replay API would widen scope before the team knows which actions are actually worth replaying.

The Workbench runtime probe already produces namespaced, timeline-observable payloads that can be safely recognized and resent through the existing inspector transport.

## Decision
- Add the first replay control as a Workbench action that resends the latest observed namespaced runtime probe payload from timeline state.
- Keep replay constrained to runtime probe payloads that match the existing `panex.workbench.runtime-probe` namespace contract.
- Derive replay availability and replay source entirely from timeline history rather than maintaining a separate local replay cache.
- Reuse the existing `sendRuntimeMessage(...)` inspector helper instead of introducing a new replay-specific transport path.
- Defer broader replay features, payload editing, and a dedicated replay engine until real operator workflows justify them.

## Consequences
- Positive:
  - Workbench gains a real replay control without any daemon or protocol expansion.
  - Replay remains truthful because it is driven by payloads the system actually observed.
  - The feature establishes a low-risk replay pattern that can be generalized later if product needs emerge.
- Tradeoff:
  - Replay is intentionally limited to the namespaced runtime probe path.
  - A broader replay experience still requires follow-up milestones once the team has stronger evidence about operator needs.

## Reversibility
High.
The replay control is a pure inspector-side composition over existing timeline state and runtime transport, so it can be revised or removed without changing daemon, protocol, or simulator contracts.
