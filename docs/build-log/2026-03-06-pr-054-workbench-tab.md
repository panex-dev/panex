# PR54 - Read-Only Workbench Tab

## Metadata
- Date: 2026-03-06
- PR: 54
- Branch: `feat/pr54-workbench-tab`
- Title: enable the first real Workbench tab as a read-only operator overview
- Commit(s): pending

## Problem
- The inspector shell and router already reserved a `Workbench` surface, but it remained disabled and provided no product value.
- Leaving it disabled would delay roadmap validation of the shell architecture and postpone feedback on what Workbench should become, while a fully interactive workbench would widen scope too early.

## Approach (with file+line citations)
- Change 1:
  - Why: add pure Workbench summary helpers for storage and timeline state so the new tab can render a stable read-only model from existing connection signals.
  - Where: `inspector/src/workbench.ts:1-101`
  - Where: `inspector/tests/workbench.test.ts:1-92`
- Change 2:
  - Why: enable the Workbench tab in the shell and render a real tab that shows connection, timeline, storage, and planned-tool summaries without adding new daemon/protocol actions.
  - Where: `inspector/src/main.tsx:13-53`
  - Where: `inspector/src/tabs/workbench.ts:1-98`
  - Where: `inspector/src/styles.css:214-276`
  - Where: `inspector/src/styles.css:375-415`
- Change 3:
  - Why: record the decision boundary that Workbench is enabled now as a read-only slice and that actionable tools are deferred to a later PR.
  - Where: `docs/adr/018-read-only-workbench-first-slice.md:1-33`
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/STATUS.md:54-61`
  - Where: `docs/build-log/2026-03-06-pr-054-workbench-tab.md:1-56`

## Risk and Mitigation
- Risk: Workbench could become a second sidebar or duplicate Timeline/Storage information without a clear purpose.
- Mitigation: this slice is explicit about being an operator overview; it aggregates cross-tab state in one place and documents that interactive tools are intentionally deferred.
- Risk: adding Workbench now could invite premature protocol or daemon expansion.
- Mitigation: the ADR and build log set the boundary clearly: no new protocol messages, daemon APIs, or mutating actions in this milestone.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Expected:
  - Workbench becomes a real enabled tab in the inspector.
  - The tab renders only derived state from existing connection/timeline/storage data.
  - No daemon/protocol behavior changes are required for the tab to function.

## Teach-back (engineering lessons)
- Design lesson: enabling a read-only surface first is a good way to validate information architecture before committing to interaction models.
- Testing lesson: even mostly-UI milestones benefit from pure derived-state helpers because they give the product surface durable tests without brittle DOM coupling.
- Team workflow lesson: an ADR is useful when shipping a deliberately limited slice; it documents both the decision and the non-goals so later PRs stay disciplined.

## Next Step
- Add the first actionable Workbench tool on top of existing inspector/daemon capabilities without widening protocol scope prematurely.
