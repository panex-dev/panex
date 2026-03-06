# ADR-019: Workbench Storage Presets As The First Action

## Status
Accepted

## Context
PR54 enabled Workbench as a read-only operator overview so the tab, shell, and connection-provider architecture could be validated without widening transport scope.

The next roadmap step is to make Workbench useful, but introducing new daemon APIs, protocol messages, or broad mutating workflows would increase product surface area before the Workbench interaction model is proven.

The inspector already has a working storage mutation path for `storage.set`, `storage.remove`, and `storage.clear`, including snapshot refresh and live diff fanout.

## Decision
- Make the first actionable Workbench tool a set of reversible storage presets.
- Reuse the existing inspector storage mutation commands instead of introducing new transport behavior.
- Restrict presets to namespaced `panex.workbench.*` keys so the new surface is safe to exercise and easy to clean up.
- Derive preset state from live storage snapshots so Workbench can show whether each preset is missing, customized, or already applied.
- Defer broader Workbench tools such as runtime probes or replay controls until this first action shape proves useful.

## Consequences
- Positive:
  - Workbench becomes meaningfully interactive without daemon or protocol expansion.
  - The presets are reversible and low-risk because they affect only namespaced demo keys.
  - The design establishes a pattern for future Workbench tools: prefer existing transport first, widen contracts only when the product requires it.
- Tradeoff:
  - Presets are intentionally narrow and do not yet address arbitrary operator workflows.
  - The first interactive tool is storage-specific, so Workbench still needs follow-up milestones to become a broader cockpit.

## Reversibility
High.
The preset card is a pure inspector-side composition over existing storage commands and can be replaced or removed without affecting daemon, protocol, or simulator contracts.
