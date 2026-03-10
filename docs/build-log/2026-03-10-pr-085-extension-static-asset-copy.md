# PR85 - Extension Static Asset Copy

## Metadata
- Date: 2026-03-10
- PR: 85
- Branch: `feat/pr85-extension-static-asset-copy`
- Title: copy manifest and other non-bundled extension assets into the build output
- Commit(s):
  - `fix(build): copy static extension assets into outDir`

## Problem
- The first `v0.1.0-rc.1` Windows release validation showed that `panex dev` emitted bundled scripts into `.panex/dist` but did not copy `manifest.json`.
- That left the generated output impossible to load directly in `chrome://extensions` without a manual file copy, which broke the expected first-run extension workflow even though the daemon and rebuild loop were otherwise working.

## Approach (with file+line citations)
- Change 1:
  - Why: extend the shared Go builder to discover non-bundled, non-HTML files in the extension source tree and copy them into `outDir` after JS bundling and HTML processing.
  - Where: `internal/build/esbuild_builder.go:132-182`
  - Where: `internal/build/static_assets.go:11-65`
- Change 2:
  - Why: keep HTML classification shared between HTML rewriting and static-asset discovery so `.html` files continue through the HTML pipeline instead of being copied raw.
  - Where: `internal/build/html_assets.go:24-41`
  - Where: `internal/build/static_assets.go:63-65`
- Change 3:
  - Why: lock in the released Windows failure mode with a regression test that verifies `manifest.json`, icon assets, and other static files are copied while source `.ts` files are not.
  - Where: `internal/build/esbuild_builder_test.go:183-244`
- Change 4:
  - Why: record the hotfix in the project tracker while leaving the post-release roadmap queue unchanged.
  - Where: `docs/build-log/STATUS.md:82-95`
  - Where: `docs/build-log/README.md:44-48`

## Risk and Mitigation
- Risk: generic static-asset copying could accidentally duplicate source files that should be bundled or rewritten.
- Mitigation: discovery excludes bundle entry extensions and `.html`, and the regression test asserts that source `background.ts` is not copied into the build output.
- Risk: copying the full source tree could drag dependency payloads from `node_modules` into the extension output.
- Mitigation: static-asset discovery skips `node_modules` directories entirely and only preserves files that are part of the extension source surface.

## Verification
- Commands run:
  - `GOCACHE=/tmp/go-build go test ./internal/build -count=1`
  - `GOCACHE=/tmp/go-build go test ./cmd/panex ./internal/build -count=1`
  - `make fmt`
  - `make check`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - Reproduced the release-validation symptom conceptually from user feedback: `.panex/dist` contained `background.js` but not `manifest.json` before this builder change.
  - Confirmed the new builder test copies `manifest.json`, CSS, and icon assets while still emitting bundled `background.js`/`popup.js`.

## Teach-back
- Design lesson: when a build pipeline owns the extension output directory, it has to own the full extension surface, not only the bundled scripts.
- Testing lesson: release-validation bugs are best fixed by turning the observed operator symptom into a narrow package-level regression test rather than trying to exercise the whole browser workflow in CI.
- Workflow lesson: post-release bug slices should still be recorded in the build log so the roadmap and shipped behavior do not drift apart.

## Next Step
- Select the next post-release milestone from the remaining queued follow-ons.
