# PR15 - Shared TypeScript Protocol Module

## Metadata
- Date: 2026-03-01
- PR: 15
- Branch: `feat/shared-protocol-package`
- Title: extract shared protocol contract for agent and inspector
- Commit(s): pending

## Problem
- Protocol definitions were duplicated between agent and inspector, which made drift likely whenever message names or guards changed.
- The divergence had already started (`query.events*` existed in inspector-only definitions), creating avoidable integration risk for every future protocol change.

## Approach (with file+line citations)
- Added canonical shared protocol module with all message names, roles, payload types, and runtime guards:
  - `shared/protocol/src/index.ts:1-148`
- Added protocol consistency tests, including name-to-type map exhaustiveness and guard behavior:
  - `shared/protocol/tests/index.test.ts:1-81`
- Migrated agent runtime + tests to consume the shared protocol source:
  - `agent/src/background.ts:1-55`
  - `agent/src/reload.ts:1-16`
  - `agent/tests/protocol.test.ts:1-32`
  - `agent/tests/reload.test.ts:1-53`
- Migrated inspector runtime + timeline utilities/tests to consume the shared protocol source:
  - `inspector/src/main.tsx:1-151`
  - `inspector/src/timeline.ts:1-193`
  - `inspector/tests/timeline.test.ts:1-185`
- Removed duplicated protocol files from both clients:
  - `agent/src/protocol.ts` (deleted)
  - `inspector/src/protocol.ts` (deleted)
- Documented the contract location and rationale:
  - `shared/protocol/README.md:1-25`
  - `docs/adr/014-shared-typescript-protocol-module.md:1-40`
  - `README.md:24-28`
  - `agent/README.md:14-15`
  - `inspector/README.md:15-16`

## Risk and Mitigation
- Risk: stricter envelope validation could reject previously tolerated malformed payloads.
- Mitigation: guard strictness is constrained to envelope structure + canonical name/type pairing; payload-specific validation remains handler-local (`shared/protocol/src/index.ts:103-132`).
- Risk: shared module path changes could break bundling in both clients.
- Mitigation: both client builds were executed after migration and compiled successfully.

## Verification
- Commands run:
  - `cd agent && pnpm run check`
  - `cd agent && pnpm run test`
  - `cd agent && pnpm run build`
  - `cd agent && node --import tsx --test ../shared/protocol/tests/*.test.ts`
  - `cd inspector && pnpm run test`
  - `cd inspector && pnpm run check`
  - `cd inspector && pnpm run build`
  - `make test`
  - `make build`
- Notes:
  - `make lint` still fails in this environment with golangci-lint loader error (`no go files to analyze`), unrelated to this protocol change.

## Teach-back (engineering lessons)
- Shared contracts are a product feature: centralizing protocol definitions reduces integration bugs more than adding another runtime check in each client.
- Runtime guards should enforce transport invariants (shape + name/type pairing) and let feature handlers own payload semantics.
- Extracting duplication early is cheaper than coordinating backwards compatibility across diverged clients later.

## Next Step
- Add a cross-language protocol parity check (Go vs TypeScript constants) so drift is detected automatically in CI before review.
