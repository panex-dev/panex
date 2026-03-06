# ADR-022: Replay Tab from Observed Runtime Probe History

## Status
Accepted

## Context
PR57 proved the first replay interaction inside Workbench by resending the latest observed namespaced runtime probe payload from timeline state.

That validated the replay pattern, but the dedicated `#replay` tab in the inspector shell remained disabled. Keeping it disabled would leave replay buried inside Workbench and would not test whether operators benefit from a history-driven replay surface before the team considers broader replay scope.

At the same time, introducing a generic replay engine, payload editing, or daemon-side replay APIs would still be premature. The product only has evidence for replaying the existing namespaced runtime probe contract.

## Decision
- Enable the `#replay` tab as a focused inspector surface over observed runtime probe history.
- Populate the tab entirely from timeline entries that match the existing `panex.workbench.runtime-probe` payload contract.
- Show replayable items newest-first, including whether each payload was observed from a runtime result or a `runtime.onMessage` event.
- Reuse the existing `sendRuntimeMessage(...)` inspector helper to replay an observed payload.
- Do not add payload editing, arbitrary message replay, daemon-side replay state, or broader protocol changes in this milestone.

## Consequences
- Positive:
  - Replay becomes a visible first-class workflow instead of a hidden quick action.
  - The product validates whether operators actually use replay as history navigation rather than only as a single-button repeat action.
  - The feature keeps replay truthful by limiting it to payloads the system actually observed.
- Tradeoff:
  - Replay remains intentionally narrow and does not yet cover arbitrary timeline entries or edited payloads.
  - Some replay workflows will still require later decisions once real usage is observed.

## Reversibility
High.
The Replay tab is a pure inspector-side composition over timeline state and the existing runtime transport, so it can be revised or removed without changing daemon or protocol contracts.
