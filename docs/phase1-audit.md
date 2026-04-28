# Phase 1 Audit — Composability & Correctness

**Date:** 2026-04-18
**Scope:** 17 Go packages introduced in Phase 1 (`configloader`, `fsmodel`, `panexerr`, `inspector`, `graph`, `target`, `capability`, `policy`, `ledger`, `verify`, `doctor`, `manifest`, `plan`, `lock`, `session`, `mcp`, `cli`) plus Phase 1 extensions to `daemon`.
**Starting state:** strict-green CI across linux/macos/windows for `go test -race`, lint, deps, typescript.
**What this audit is:** a design-quality read, not a bug triage. CI green ≠ production-ready.

> **Living doc.** When a finding is addressed, flip its **Status** below and link the commit/PR. The summary table is the index.

---

## Status legend

- `open` — nothing started.
- `wip` — branch open or PR in flight.
- `fixed` — landed on `main`. Append `<commit>` or `(#PR)`.
- `wontfix` — explicit decision to leave; record reason.

---

## Summary table

| ID | Severity | Status | Title | Primary location |
|----|----------|--------|-------|------------------|
| [C1](#c1) | Critical | open | `applyGenerateManifest` overwrites for multi-target | `internal/plan/plan.go:249` |
| [C2](#c2) | Critical | open | MCP's apply bypasses the project lock | `internal/mcp/mcp.go:480` |
| [C3](#c3) | Critical | open | `ValidateHandshake` has no capability enforcement | `internal/session/session.go:275` |
| [C4](#c4) | Critical | open | `lock.Acquire` has a TOCTOU race | `internal/lock/lock.go:50` |
| [C5](#c5) | Critical | open | `Graph.ComputeHash` mutates the receiver non-atomically | `internal/graph/types.go:53` |
| [H1](#h1) | High | open | Graph hash includes absolute `SourceRoot` | `internal/graph/types.go:16` |
| [H2](#h2) | High | open | Project version is not plumbed | `internal/graph/types.go:32` |
| [H3](#h3) | High | open | `session.findBrowser()` is Linux-only | `internal/session/session.go:288` |
| [H4](#h4) | High | open | `TargetsResolved` is a verbatim copy of `TargetsRequested` | `internal/graph/builder.go:71` |
| [H5](#h5) | High | open | `Transition` errors silently ignored in apply | `internal/plan/plan.go:176` |
| [H6](#h6) | High | open | Apply has no rollback | `internal/plan/plan.go:181` |
| [H7](#h7) | High | open | Zip artifacts are non-reproducible | `internal/target/chrome.go:323` |
| [M1](#m1) | Medium | open | `readLock` accepts legacy `pid:1234` plaintext | `internal/lock/lock.go:180` |
| [M2](#m2) | Medium | open | `rand.Read` errors ignored in ID generators | `session.go:319,325`, `ledger.go:88` |
| [M3](#m3) | Medium | open | RunIDs not time-sortable; reader assumes they are | `ledger.go:86`, `mcp.go:564` |
| [M4](#m4) | Medium | open | Chrome adapter supports only one `content_script` | `internal/target/chrome.go:179` |
| [M5](#m5) | Medium | open | RFC3339 (second resolution) everywhere | repo-wide |
| [M6](#m6) | Medium | open | MCP silently ignores `json.Unmarshal` errors on params | `internal/mcp/mcp.go:192,236` |
| [M7](#m7) | Medium | open | `HostPermissions` is matrix-level, not per-target | `internal/manifest/manifest.go:154` |
| [M8](#m8) | Medium | open | `Capabilities map[string]any` is untyped at every boundary | `graph/types.go:25`, `capability/capability.go:30` |

Phase 2 leverage points are tracked separately as [L1–L7](#leverage).

---

## Severity rubric

- **Critical** — silent correctness loss, data-corruption potential, or an enforced invariant that isn't actually enforced.
- **High** — broken invariant that users will hit on a supported workflow (multi-target, cross-platform, cross-machine).
- **Medium** — data-model weakness or latent bug that hasn't bitten yet but will.

---

## Critical

<a id="c1"></a>
### C1 — `applyGenerateManifest` overwrites for multi-target projects

- **Status:** open
- **Location:** `internal/plan/plan.go:249-264`
- **Symptom:** A two-target project ends with the last target's manifest at every target's path.
- **Evidence:** Loop iterates every entry in `input.ManifestResult.Outputs` and writes each to the single `action.Path`. `Action` has no `Target` field; apply cannot disambiguate which output belongs at this path.
- **Root cause:** Mutation model is `{Type string, Path string, Description string}` — a bag of strings. The action doesn't carry the data it needs to execute.
- **Fix:** Make `Action` a typed sum (interface with `Execute(ctx) error` per variant). Each variant owns its own data. See [L1](#l1).

<a id="c2"></a>
### C2 — MCP's apply bypasses the project lock

- **Status:** open
- **Location:** `internal/mcp/mcp.go:480` (vs `internal/cli/cli.go:470`)
- **Symptom:** CLI and MCP `apply` can run concurrently and corrupt state.
- **Evidence:** CLI passes `LockManager: mgr`; MCP omits the field. `plan.Apply` treats nil-manager as "no locking".
- **Root cause:** Both CLI and MCP construct `plan.ApplyInput` directly. Two-place wiring drifts.
- **Fix:** Extract `core.Apply(input)` that both surfaces delegate to. See [L6](#l6).

<a id="c3"></a>
### C3 — `Session.ValidateHandshake` has no capability enforcement

- **Status:** open
- **Location:** `internal/session/session.go:275-283`
- **Symptom:** Whatever the bridge declares is granted.
- **Evidence:**
  ```go
  return HandshakeReply{
      Status:                  "accepted",
      AllowedCapabilities:     payload.DeclaredCapabilities, // echoes input
      FingerprintMatch:        true,                          // hardcoded
      ...
  }
  ```
- **Root cause:** Capability boundary is a stub. Previously flagged in `execution-drift-audit-iii-protocol.md` (PX-DRIFT-006, S1); Phase 1 reimplements the surface but preserves the stub.
- **Fix:** Validate `DeclaredCapabilities` against a project-side allowlist; compute `FingerprintMatch` from the build artifact.

<a id="c4"></a>
### C4 — `lock.Acquire` has a TOCTOU race

- **Status:** open
- **Location:** `internal/lock/lock.go:50-87`
- **Symptom:** Two concurrent `Acquire` calls can both believe they hold the lock.
- **Evidence:** Read existing lock → if stale, `os.Remove` → write new. Between remove and write, another `Acquire` can race.
- **Root cause:** Check-then-write on a non-atomic primitive.
- **Fix (escalating):**
  1. `os.OpenFile(path, O_CREATE|O_EXCL|O_WRONLY, 0o644)`. EEXIST → check staleness.
  2. OS advisory locks (`flock` unix, `LockFileEx` windows). Kernel becomes source of truth; lock dies with the process. Removes stale-lock handling, `isProcessAlive`, and PID-recycling risk together. See [L5](#l5).

<a id="c5"></a>
### C5 — `Graph.ComputeHash` mutates the receiver non-atomically

- **Status:** open
- **Location:** `internal/graph/types.go:53-65`
- **Symptom:** Concurrent `ComputeHash` calls can return wrong hashes.
- **Evidence:** Zeroes `g.GraphHash`, marshals, restores via defer. No mutex.
- **Fix:** Marshal a view struct (`struct{ ... without GraphHash }`) without touching the receiver.

---

## High

<a id="h1"></a>
### H1 — Graph hash includes absolute `SourceRoot`

- **Status:** open
- **Location:** `internal/graph/types.go:16` flows into `ComputeHash` at line 53
- **Symptom:** Two devs on the same commit get different `ProjectHash`. Cross-machine `plan`→`apply` always reports drift.
- **Fix:** Either omit `SourceRoot` from hash input, or expose two hashes — `ContentHash` (cross-machine) and `LocalHash` (with sourceRoot, for local drift). See [L4](#l4).

<a id="h2"></a>
### H2 — Project version is not plumbed

- **Status:** open
- **Location:** `internal/graph/types.go:32` (`ProjectIdentity` has no `Version`) → `internal/manifest/manifest.go:127` (falls back to `"0.0.1"`) → `internal/mcp/mcp.go:509` (hardcodes `"0.1.0"`)
- **Symptom:** Every built extension ships as 0.0.1 or 0.1.0.
- **Fix:** Add `Version` to `ProjectIdentity`; populate from `configloader`; remove the literal fallbacks.

<a id="h3"></a>
### H3 — `session.findBrowser()` is Linux-only

- **Status:** open
- **Location:** `internal/session/session.go:288-303`
- **Symptom:** `panex dev` cannot launch Chrome on Windows or macOS.
- **Evidence:** `findBrowser()` only checks `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser` on PATH.
- **Root cause:** Duplicates `internal/target/chrome.go:findChromeBinary` which is already cross-platform.
- **Fix:** Delete `session.findBrowser`; delegate to the target adapter's `InspectEnvironment`.

<a id="h4"></a>
### H4 — `TargetsResolved` is a verbatim copy of `TargetsRequested`

- **Status:** open
- **Location:** `internal/graph/builder.go:71-74` and `:99-102`
- **Symptom:** Targets with no adapter still flow downstream and fail late at manifest compile time.
- **Fix:** Filter `TargetsResolved` against `target.DefaultRegistry().All()` at graph-build time; record dropped targets as warnings.

<a id="h5"></a>
### H5 — `Transition` errors silently ignored in apply

- **Status:** open
- **Location:** `internal/plan/plan.go:176, 203, 206`
- **Symptom:** State machine transitions silently fail; run reports success while staying in a stale state.
- **Evidence:** `_ = run.Transition(...)`. The state machine validates transitions and returns errors that are dropped.
- **Fix:** Surface the error. Failure to transition is a bug, not a recoverable condition.

<a id="h6"></a>
### H6 — Apply has no rollback

- **Status:** open
- **Location:** `internal/plan/plan.go:181-214`
- **Symptom:** Step 3/5 fails → steps 1-2 stay on disk, 4-5 still run. No transactional semantics.
- **Evidence:** Actions declare `Reversible: true` with no rollback implementation anywhere.
- **Fix:** Downstream of [C1](#c1)/[L1](#l1). With polymorphic `Action`, each variant owns `Rollback(ctx) error`; apply runs rollback in reverse on first failure.

<a id="h7"></a>
### H7 — Zip artifacts are non-reproducible

- **Status:** open
- **Location:** `internal/target/chrome.go:323-370`
- **Symptom:** Two builds of the same source at different times produce different SHA-256.
- **Evidence:** File mtimes go into zip headers; DEFLATE output varies with zlib version/level.
- **Fix:** Zero mtimes, sort entries lexicographically, pin compression method (`zip.Store` or a deterministic deflater).

---

## Medium

<a id="m1"></a>
**M1 — Lock legacy plaintext format.** `internal/lock/lock.go:180-185`. Accepts `pid:1234`. No reason in a new package. Delete the branch.

<a id="m2"></a>
**M2 — `rand.Read` errors ignored.** `internal/session/session.go:319,325`, `internal/ledger/ledger.go:88`. Token/run-id collision risk if `crypto/rand` returns zero buffer. Check the error.

<a id="m3"></a>
**M3 — RunIDs not time-sortable.** `internal/ledger/ledger.go:86-90`. Random hex. `internal/mcp/mcp.go:564` does `entries[len(entries)-1]` assuming chronological order. Use ULID or `<rfc3339nano>-<random>`.

<a id="m4"></a>
**M4 — One `content_script` only.** `internal/target/chrome.go:179-186`. Real extensions have many with different match patterns. Data model `Entries map[string]Entry` can't express it.

<a id="m5"></a>
**M5 — RFC3339 second resolution everywhere.** Steps within a run can collide on `StartedAt`. Use `time.RFC3339Nano`.

<a id="m6"></a>
**M6 — MCP silently ignores `json.Unmarshal` of params.** `internal/mcp/mcp.go:192,236`. Should return JSON-RPC `invalid_params` (-32602).

<a id="m7"></a>
**M7 — `HostPermissions` matrix-level not per-target.** `internal/manifest/manifest.go:154-162`. Cannot express different host perms per target.

<a id="m8"></a>
**M8 — `Capabilities map[string]any` untyped at every boundary.** `internal/graph/types.go:25`, `internal/capability/capability.go:30`. Adapters guess the shape. See [L3](#l3).

---

<a id="leverage"></a>
## Phase 2/3 leverage points

These are the structural wins. Each eliminates a class of bugs rather than one bug.

<a id="l1"></a>
### L1 — `Action` as a typed sum, not a record

```go
type Action interface {
    Kind() string
    Execute(ctx context.Context, env ApplyEnv) error
    Rollback(ctx context.Context, env ApplyEnv) error
}

type GenerateManifestAction struct {
    Target   string
    Path     string
    Manifest map[string]any
}
```

Collapses [C1](#c1), [H5](#h5), [H6](#h6). New action types become new types instead of new switch arms in a central file.

<a id="l2"></a>
### L2 — Apply as a streaming pipeline

Channel-based progress lets inspector/agent render live state. Dependency-aware ordering lets Phase 2 insert build-before-package without caller-side gymnastics.

<a id="l3"></a>
### L3 — Typed capability registry with schemas

`map[string]any` flows from config to adapter untouched. Adapter decodes, guesses, errors at apply time. A typed `Capability[T]` registry with schema validation moves errors to config-load time. Resolves [M8](#m8) and reduces [C3](#c3) blast radius.

<a id="l4"></a>
### L4 — Two hashes, not one

`ContentHash` (cross-machine replay) + `LocalHash` (with sourceRoot, for local drift). Resolves [H1](#h1).

<a id="l5"></a>
### L5 — OS-advisory locks

Removes ~60 lines of `lock.go`: stale-lock handling, `isProcessAlive`, PID recycling. Kernel is source of truth. Resolves [C4](#c4).

<a id="l6"></a>
### L6 — Surface-agnostic core API

Extract `core.Apply(input)`, `core.Plan(input)`, etc. CLI and MCP both delegate. Resolves [C2](#c2) and prevents the same drift on every future internal call.

<a id="l7"></a>
### L7 — Required dependencies positional, not in options struct

`plan.Apply(ApplyInput)` with nil-ok `LockManager` is exactly how [C2](#c2) happened. Move required dependencies to positional args so nil becomes a compile error.

---

## What's actually good (don't rewrite)

- **Atomic writes** (tmp + rename) used consistently across plan/manifest/graph/state/session/lock persistence.
- **Permission-authority invariant** in `manifest.validatePermissionAuthority` — the right shape of guard. Keep this pattern when adding new domains.
- **Exit-code taxonomy** in `internal/cli/cli.go:29-40` and `panexerr` categories give agents structured, actionable failure signals.
- **CLI ↔ MCP duality** — every CLI command has an MCP tool. The execution drifts ([C2](#c2)) but the shape is right; fix via [L6](#l6).
- **Cross-platform fixes landed properly** — lock uses OS-specific `isProcessAlive`, file watcher survives Windows ReadDirectoryChangesW quirks, protocol parity strips CRLF. No build-tag tricks hiding real divergence.

---

## Recommended sequencing

1. **[L1](#l1)** (polymorphic `Action`) — closes [C1](#c1) + [H5](#h5) + [H6](#h6) in one structural PR.
2. **[C5](#c5) + [H1](#h1)** via the hash-view refactor — small, isolated, ships independently.
3. **[L6](#l6)** (`core.Apply` extraction) — closes [C2](#c2) and prevents the drift pattern from repeating.
4. **[L5](#l5)** (OS-advisory locks) — closes [C4](#c4) and [M1](#m1) together.
5. **[C3](#c3)** as a focused security-boundary PR.
6. **[H2](#h2), [H3](#h3), [H4](#h4), [H7](#h7)** as smaller follow-ups.
7. Mediums roll in opportunistically when the surrounding code is already being changed.

---

## Bottom line

Phase 1 compiles, tests green, and ships. That's not the same as ready to carry Phase 2 weight. **C1–C5** are silent-failure bugs masked by single-target / single-caller / single-goroutine test coverage. **H1–H7** break real workflows. The Phase 2 leverage comes from three primitives: polymorphic `Action`, typed `Capabilities`, OS-level locks.
