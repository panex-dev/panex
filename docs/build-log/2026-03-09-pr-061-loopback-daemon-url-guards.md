# PR61 - Loopback Daemon URL Guards

## Metadata
- Date: 2026-03-09
- PR: 61
- Branch: `fix/pr61-loopback-daemon-url-guards`
- Title: guard loopback daemon URL overrides and track deferred audit items
- Commit(s): pending

## Problem
- `audit-1.md` still had an open client-side hardening gap: the dev agent trusted any stored websocket URL string, and the inspector trusted `?ws=` / `?token=` overrides even when embedded.
- The current repo also had no living tracker for which audit findings are resolved versus intentionally deferred, making it too easy to delete the original audit before the remaining work is actually closed.

## Approach (with file+line citations)
- Change 1:
  - Why: keep the dev agent on the daemon contract unless the stored websocket URL remains a loopback `ws://.../ws` endpoint.
  - Where: `agent/src/config.ts:7-67`
- Change 2:
  - Why: add negative-path coverage for agent config fallback when the stored websocket URL leaves the loopback daemon contract.
  - Where: `agent/tests/config.test.ts:27-101`
- Change 3:
  - Why: only honor inspector websocket overrides for top-level loopback websocket URLs and ignore all URL overrides when the inspector is embedded, which closes the iframe-override path called out by the audit.
  - Where: `inspector/src/connection.ts:46-48`
  - Where: `inspector/src/connection.ts:468-542`
- Change 4:
  - Why: lock the new inspector connection rules with direct unit coverage for defaults, accepted loopback overrides, rejected remote overrides, and embedded-mode fallback.
  - Where: `inspector/tests/connection.test.ts:1-46`
- Change 5:
  - Why: document the stricter top-level-only override contract for local inspector usage.
  - Where: `inspector/README.md:30-43`
- Change 6:
  - Why: add a durable audit tracker that keeps unresolved or dependent findings explicit until `audit-1.md` can be retired honestly.
  - Where: `audit.md:1-25`
  - Where: `docs/build-log/STATUS.md:1-65`

## Risk and Mitigation
- Risk: stricter URL validation could reject legitimate local overrides.
- Mitigation: the accepted contract still allows both `127.0.0.1` and `localhost`, any local port, and existing query parameters, while rejecting only non-loopback hosts, non-`ws:` schemes, auth-bearing URLs, and wrong paths.
- Risk: blindly narrowing extension `host_permissions` would look like security progress but would silently break non-default daemon ports.
- Mitigation: that audit item stays explicitly deferred in `audit.md` until the MV3 permission contract is verified and product behavior is decided.

## Verification
- Commands run:
  - `CI=1 pnpm install --frozen-lockfile`
  - `pnpm --dir agent run test`
  - `pnpm --dir inspector run test`
  - `pnpm --dir agent run check`
  - `pnpm --dir inspector run check`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
- Additional checks:
  - `./scripts/pr-ensure-rebased.sh`

## Teach-back
- Design lesson: local override hooks are still API surface. If a daemon URL can be injected through config or query params, it needs the same contract validation as any other external input.
- Testing lesson: a hardening change is not done until the rejected paths have first-class tests, not just the happy path.
- Workflow lesson: an audit should stay live until the unresolved items are written down somewhere smaller and current; deleting the source audit early just turns known debt back into tribal knowledge.
