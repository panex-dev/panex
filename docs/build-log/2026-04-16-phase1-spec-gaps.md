# Phase 1 Audit — Spec Gap Inventory

**Status:** complete
**Date:** 2026-04-16

## What

Document the known gaps between the Phase 1 implementation and the full specification, so Phase 2 planning can account for them without rediscovering each one.

## Why

Phase 1 intentionally deferred several spec features to keep scope manageable. Leaving them undocumented creates a risk of either forgetting them or re-auditing the same ground in Phase 2. This document pins each gap to its spec section and assigns a target phase.

## Gap Inventory

### CLI Surface (spec §34)

| Gap | Spec Reference | Description | Target Phase |
|---|---|---|---|
| `--interactive` global flag | §34.1 | Enable interactive prompting for confirmation (currently always non-interactive) | Phase 3 |

### Plan/Apply Model (spec §21)

| Gap | Spec Reference | Description | Target Phase |
|---|---|---|---|
| Resume step replay | §22.4 | Replay failed/incomplete steps on `panex resume` instead of marking the run as succeeded | Phase 2 |

### Runtime (spec §23–24)

| Gap | Spec Reference | Description | Target Phase |
|---|---|---|---|
| — | — | All previously deferred runtime gaps in this section have been completed. | — |

### MCP Tools (spec §35)

| Gap | Spec Reference | Description | Target Phase |
|---|---|---|---|
| `publish_extension` tool | §35 | Upload artifacts to the Chrome Web Store / addons.mozilla.org | Phase 4 |
| `rollback_changes` tool | §35 | MCP-exposed rollback of a failed apply | Phase 2 |
| `query_run_history` tool | §35 | Paginated query over the run ledger | Phase 3 |
| `configure_project` tool | §35 | Modify panex.config.json fields programmatically | Phase 3 |

### Config (spec §11)

| Gap | Spec Reference | Description | Target Phase |
|---|---|---|---|
| — | — | All previously deferred config gaps in this section have been completed. | — |

## Impact

- 0 code changes — documentation only
- No test or build impact

## Quality

- Verified each gap against the current codebase via grep/search
- Assigned target phases based on dependency ordering and priority
