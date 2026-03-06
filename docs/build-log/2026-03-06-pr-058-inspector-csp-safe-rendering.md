# PR58 - Inspector CSP-Safe Rendering

## Metadata
- Date: 2026-03-06
- PR: 58
- Branch: `feat/pr58-inspector-csp-jsx`
- Title: replace inspector CSP-breaking template rendering with CSP-safe Solid hyperscript rendering
- Commit(s): pending

## Problem
- The inspector rendered a white screen in the browser even when the daemon and static assets were loading correctly.
- Browser console output showed `Uncaught EvalError` from `solid-js/html` template compilation under the inspector's CSP, which disallows `unsafe-eval`.
- The preview HTML also failed to load the generated `main.css`, so even a successful boot would have rendered without the intended styling.

## Approach (with file+line citations)
- Change 1:
  - Why: replace every inspector UI surface that depended on `solid-js/html` with CSP-safe Solid hyperscript rendering so the UI can boot under `script-src 'self'` without weakening policy.
  - Where: `inspector/src/main.tsx:1-120`
  - Where: `inspector/src/shell.tsx:1-54`
  - Where: `inspector/src/sidebar.tsx:1-32`
  - Where: `inspector/src/tabs/timeline.tsx:1-192`
  - Where: `inspector/src/tabs/storage.tsx:1-265`
  - Where: `inspector/src/tabs/workbench.tsx:1-238`
- Change 2:
  - Why: update the inspector browser build to compile JSX through Solid's hyperscript runtime instead of the automatic JSX runtime path that was not previously configured in this repo.
  - Where: `inspector/scripts/build.ts:14-56`
- Change 3:
  - Why: ensure the source preview HTML references the built stylesheet and that the generated `dist/index.html` rewrites both JS and CSS paths for local static serving.
  - Where: `inspector/index.html:1-14`
  - Where: `inspector/scripts/build.ts:33-56`
  - Where: `docs/build-log/README.md:44-46`
  - Where: `docs/build-log/STATUS.md:59-66`
  - Where: `docs/build-log/2026-03-06-pr-058-inspector-csp-safe-rendering.md:1-66`

## Risk and Mitigation
- Risk: switching the renderer could change reactive behavior or event wiring across Timeline, Storage, and Workbench.
- Mitigation: the change keeps existing state/connection logic intact and only swaps the rendering layer, then verifies the full inspector test/build surface plus repo-wide `make check`, `make test`, and `make build`.
- Risk: solving the white screen by weakening CSP would hide the regression instead of fixing it.
- Mitigation: CSP remains unchanged; the fix removes the `unsafe-eval` dependency from the inspector bundle.
- Risk: preview HTML could still boot unstyled even after the CSP fix.
- Mitigation: `inspector/index.html` now includes `./dist/main.css`, and the built preview normalizes that path to `./main.css` in `dist/index.html`.

## Verification
- Commands run:
  - `pnpm install`
  - `pnpm --dir inspector run check`
  - `pnpm --dir inspector run build`
  - `CI=1 pnpm install --frozen-lockfile`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `git diff --check`
  - `rg -n 'new Function|unsafe-eval|solid-js/html|functionBuilder|parseTemplate' inspector/dist/main.js`
- Expected:
  - Inspector loads under the existing CSP without `EvalError`.
  - `inspector/dist/index.html` includes `main.css`, `main.js`, and `chrome-sim.js`.
  - Built inspector bundle contains no `solid-js/html` or `new Function` path.

## Teach-back (engineering lessons)
- Security lesson: CSP regressions should be fixed by removing unsafe runtime behavior, not by punching holes in policy.
- Build lesson: a repo that has never compiled real JSX before can hide renderer/runtime assumptions until the first migration; treat the build pipeline as part of the feature, not background infrastructure.
- Workflow lesson: when a user reports "white screen", capture the first browser console error before patching anything; it collapses a wide search space into a concrete fix boundary.

## Next Step
- Decide whether to graduate the disabled Replay tab into a focused history-driven surface using the Workbench-proven replay pattern without widening daemon scope.
