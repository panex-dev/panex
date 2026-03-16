# PR91 Build Log: Skip infrastructure directories in build discovery and file watching

## Metadata
- Date: 2026-03-16
- PR: 91
- Branch: `fix/pr91-infrastructure-dir-exclusion`
- Title: `fix(build): skip infrastructure directories in source discovery and file watching`
- Commit(s): `fix(build): skip infrastructure directories in source discovery and file watching (#91)`

## Problem
- `discoverEntryPoints` walked the entire source directory tree without exclusions. If `node_modules` or version control directories existed inside the source tree, their `.js`/`.ts` files were treated as extension entry points and sent to esbuild as bundle inputs.
- `discoverHTMLAssets` similarly walked into `node_modules` and dot-directories, discovering HTML files that are not part of the extension.
- `discoverStaticAssets` already skipped `node_modules` but not dot-prefixed directories like `.git` or `.panex`, so their contents could be copied into the build output.
- The file watcher added every subdirectory to fsnotify without exclusions. In a source tree containing `node_modules` or `.git`, the watcher consumed inotify watches on thousands of irrelevant paths and triggered unnecessary rebuild cycles on unrelated file changes.

## Approach (with file+line citations)
- Change 1:
  - Why: unify directory exclusion logic for all source discovery functions in the build package behind a shared `isInfrastructureDir` predicate that skips `node_modules` and dot-prefixed directory names.
  - Where: `internal/build/esbuild_builder.go:200-206` (entry point discovery)
  - Where: `internal/build/esbuild_builder.go:237-242` (`isInfrastructureDir` definition)
  - Where: `internal/build/html_assets.go:31-37` (HTML asset discovery)
  - Where: `internal/build/static_assets.go:18-22` (static asset discovery, replaces bare `node_modules` check)
- Change 2:
  - Why: apply the same exclusion to the file watcher so it never adds infrastructure directories to fsnotify, preventing wasted inotify watches and spurious rebuild triggers.
  - Where: `internal/daemon/file_watcher.go:176-178` (directory tree walker)
  - Where: `internal/daemon/file_watcher.go:190-195` (`isInfrastructureDir` definition)
- Change 3:
  - Why: lock the new exclusion behavior with tests that plant files inside skipped directories and verify they are invisible to discovery and watching.
  - Where: `internal/build/esbuild_builder_test.go:384-446` (entry point, HTML, static, and predicate tests)
  - Where: `internal/daemon/file_watcher_test.go:167-259` (initial-tree and new-directory exclusion tests plus predicate tests)

## Risk and Mitigation
- Risk: a user places extension source files inside a dot-prefixed directory (e.g., `.well-known/`), and those files silently disappear from the build.
- Mitigation: dot-prefixed directories are a well-established infrastructure convention. No standard Chrome extension layout uses dot-prefixed source directories. Users who need this can switch to explicit `panex.toml` config with a non-dot source directory.
- Risk: the file watcher no longer watches newly created dot-directories at runtime, so hot-reload stops working for output written to `.panex/dist`.
- Mitigation: `.panex/dist` is the build output directory, not the source tree. The watcher is supposed to watch source, not output. Watching output would cause infinite rebuild loops.

## Verification
- Commands run:
  - `make fmt`
  - `GOCACHE=/tmp/go-build GOLANGCI_LINT_CACHE=/tmp/golangci-lint make lint`
  - `GOCACHE=/tmp/go-build make test`
  - `GOCACHE=/tmp/go-build make build`
  - `./scripts/pr-ensure-rebased.sh`
- Additional checks:
  - New tests verify entry points, HTML assets, and static assets all exclude `node_modules` and dot-directories.
  - New tests verify the file watcher skips infrastructure directories during initial tree scan and when new directories are created at runtime.

## Teach-back (engineering lessons)
- Design lesson: consistent exclusion across all directory walkers is more important than getting one right. Four separate WalkDir calls with inconsistent skip rules create subtle bugs that only surface in specific project layouts.
- Testing lesson: infrastructure directory tests need files planted inside the excluded directories, not just the directories themselves. The exclusion must be verified by absence of those files from results, not just by checking that the directory was skipped.
- Team workflow lesson: known-issue entries (like `discoverEntryPoints treats all files as entries`) should be resolved by the infrastructure change that makes them impossible, not by individual workarounds in each caller.

## Next Step
- Relax the source/output directory overlap validation to allow `.panex/`-prefixed output directories nested inside the source tree, then wire zero-config `panex dev` for directories containing `manifest.json`.
