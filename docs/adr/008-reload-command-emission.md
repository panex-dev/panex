# ADR-008: Reload Command Emission Policy

## Status
Accepted

## Context
Panex already emits `build.complete`, and the Dev Agent can execute `command.reload`.
The daemon still needs a deterministic rule for when reload commands should be sent to avoid reloading stale or failed outputs.

## Decision
- Emit `command.reload` only when a build result is successful.
- Emit reload only after publishing `build.complete` for the same build id.
- Include `build_id` and reason (`build.complete`) in the reload payload so consumers can correlate command and event streams.
- Treat reload broadcast failures as non-fatal runtime diagnostics, matching current `build.complete` handling.

## Consequences
- Clients can interpret reload as "safe to apply new artifacts now".
- Failed builds do not trigger noisy reload loops in development.
- The protocol timeline becomes causally ordered for future inspector/event-store correlation.

## Reversibility
High.
If we later support partial reloads or capability-gated commands, this policy can be expanded without changing existing message names.
