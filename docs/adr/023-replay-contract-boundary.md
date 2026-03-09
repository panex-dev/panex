# ADR-023: Replay Contract Boundary

## Status
Accepted

## Context
PR57 and PR59 proved that replay is useful when it resends payloads the system actually observed, not speculative drafts.

After those milestones, the next open question was whether replay should broaden into a generic inspector-side history filter for additional message families or stay constrained to the current runtime-probe contract.

The current product only has one replay-safe family with an explicit operator meaning:
- namespaced `panex.workbench.runtime-probe` payloads observed through `runtime.sendMessage` results or `runtime.onMessage`

Other current history families do not yet meet the same bar:
- storage mutations are intentionally stateful and should not become casually replayable without clearer operator safeguards
- arbitrary `chrome.api.*` traffic can be unsupported, partial, or nondeterministic
- there is still no daemon-side record/replay contract for broader session semantics

## Decision
- Keep replay constrained to the single runtime-probe family for now.
- Centralize replay eligibility in one inspector-side contract module instead of duplicating the boundary across Workbench and Replay helpers.
- Require any future replay family to add its own explicit contract, tests, and product rationale before it is included in replay history.
- Do not infer replayability from broad message-name matching alone.

## Consequences
- Positive:
  - Replay scope is now explicit and reviewable instead of being an implicit coincidence across multiple files.
  - Workbench and Replay stay aligned because they consume the same replay-family boundary.
  - Future replay expansion has a clear engineering bar: explicit contract first, UI widening second.
- Tradeoff:
  - Replay remains intentionally narrow until another family has deterministic semantics and operator value.

## Reversibility
High.
The boundary lives entirely in inspector-side derivation code and documentation, so future replay families can be added without changing daemon behavior first.
