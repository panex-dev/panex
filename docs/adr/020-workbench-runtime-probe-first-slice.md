# ADR-020: Workbench Runtime Probe First Slice

## Status
Accepted

## Context
PR55 made Workbench interactive by adding reversible storage presets on top of the existing storage mutation transport.

The next roadmap step is to validate a non-storage Workbench action, but a generic runtime message console would create a second interaction model before the product learns what operators actually need.

The daemon already supports `chrome.api.call` for `runtime.sendMessage`, and the behavior is covered by existing transport and daemon tests.

## Decision
- Add exactly one Workbench runtime probe in this slice.
- Reuse the existing `chrome.api.call` -> `runtime.sendMessage` transport path.
- Keep the probe payload namespaced under `panex.workbench.runtime-probe` so timeline-derived results and events can be recognized safely.
- Derive the last observed result and `runtime.onMessage` event from the timeline instead of inventing a separate client-side status store.
- Defer generic runtime payload editing and additional probe types until operator usage justifies a broader interaction model.

## Consequences
- Positive:
  - Workbench proves that its tool model can extend beyond storage without widening daemon or protocol scope.
  - The probe stays easy to reason about because it is canned, namespaced, and timeline-observable.
  - Future runtime tools can reuse the same truthful result/event pattern instead of inventing optimistic local state.
- Tradeoff:
  - The first runtime tool is intentionally narrow and does not yet cover arbitrary runtime payloads.
  - Follow-up work is still needed before Workbench becomes a full operator console.

## Reversibility
High.
The probe is a pure inspector-side composition over existing transport, and can be changed or removed without modifying daemon, protocol, or simulator contracts.
