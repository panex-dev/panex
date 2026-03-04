# PR29 - Inspector Content-Security-Policy

## Metadata
- Date: 2026-03-04
- PR: 29
- Branch: `fix/pr29-inspector-csp`
- Title: add Content-Security-Policy meta tag to inspector HTML
- Commit(s): pending

## Problem
- Inspector HTML had no CSP. Inline injection possible if served over HTTP or if a malicious payload reached the DOM.
- Audit §2 flagged this as a high-severity security gap.

## Approach (with file+line citations)
- Added CSP meta tag to inspector/index.html:
  - Why: restricts script/style sources to 'self' and WebSocket connections to localhost only
  - Where: `inspector/index.html:6`
  - Policy: `default-src 'none'; script-src 'self'; style-src 'self'; connect-src ws://127.0.0.1:* ws://localhost:*;`

## Risk and Mitigation
- Risk: overly restrictive CSP could break inspector functionality.
- Mitigation: script-src/style-src allow 'self' (bundled assets), connect-src allows WebSocket to any localhost port (supports configurable daemon port). No inline scripts or styles in the inspector.

## Verification
- Commands run:
  - `cd inspector && pnpm run check && pnpm run test && pnpm run build`
  - `make fmt && make lint && make test && make build`

## Teach-back (engineering lessons)
- Design lesson: CSP is cheapest when added early — before any inline scripts or eval() patterns creep in. Adding it later means refactoring to remove violations.
- Testing lesson: CSP can only be fully verified in a browser. The build/test gates confirm no regressions, but manual verification in Chrome DevTools confirms policy enforcement.

## Next Step
- Replace time.Sleep synchronization in tests with polling+deadline patterns to prevent CI flakiness.
