# Build Status Tracker

As of 2026-03-16.

## Completed
| PR | Scope | Evidence |
|---|---|---|
| 1-5 | Foundation setup sequence | `docs/build-log/2026-02-28-pr-001-005-foundation.md` |
| 6 | File watcher + debounce | `docs/build-log/2026-03-01-pr-006-file-watcher.md` |
| 7 | Esbuild + `build.complete` emission | `docs/build-log/2026-03-01-pr-007-esbuild-build-complete.md` |
| 8 | Dev Agent scaffold + `command.reload` handling | `docs/build-log/2026-03-01-pr-008-dev-agent-scaffold.md` |
| 9 | Daemon reload command emission | `docs/build-log/2026-03-01-pr-009-extension-reload.md` |
| 10 | SQLite event store + `query.events` | `docs/build-log/2026-03-01-pr-010-event-store-query-events.md` |
| 11 | Inspector scaffold | `docs/build-log/2026-03-01-pr-011-inspector-scaffold.md` |
| 12 | Inspector filters/search | `docs/build-log/2026-03-01-pr-012-inspector-filters.md` |
| 13 | Query operators + persistence | `docs/build-log/2026-03-01-pr-013-inspector-query-persistence.md` |
| 14 | Inspector resilience/hardening | `docs/build-log/2026-03-01-pr-014-inspector-resilience.md` |
| 15 | Shared TS protocol module | `docs/build-log/2026-03-01-pr-015-shared-protocol-module.md` |
| 16 | TypeScript CI across TS packages | `docs/build-log/2026-03-04-pr-016-typescript-ci.md` |
| 17 | `hello.ack` + capability negotiation | `docs/build-log/2026-03-02-pr-017-hello-ack-capability-negotiation.md` |
| 18 | Go/TS protocol drift check | `docs/build-log/2026-03-02-pr-018-protocol-drift-check.md` |
| 19 | PR template parity gate + status tracker | `docs/build-log/2026-03-04-pr-019-pr-template-protocol-parity.md` |
| 20 | Numbering reconciliation (plan vs build-log sequence) | `docs/build-log/2026-03-04-pr-020-numbering-reconciliation.md` |
| 21 | Unified inspector shell + hash router decomposition | `docs/build-log/2026-03-04-pr-021-unified-inspector-shell.md` |
| 22 | Sidebar component extraction | `docs/build-log/2026-03-04-pr-022-sidebar-component.md` |
| 23 | Storage protocol extension + daemon stub handler | `docs/build-log/2026-03-04-pr-023-storage-protocol-extension.md` |
| 24 | Storage viewer UI with `query.storage.result` area filtering | `docs/build-log/2026-03-04-pr-024-storage-viewer-ui.md` |
| 25 | Storage diff ingestion + row highlight in inspector storage tab | `docs/build-log/2026-03-04-pr-025-storage-diff-highlights.md` |
| 26 | Daemon storage mutation pipeline + `storage.diff` fanout | `docs/build-log/2026-03-04-pr-026-daemon-storage-diff-pipeline.md` |
| 27 | Branch hygiene workflow + CI guardrails | (process PR, no build-log entry) |
| 28 | Audit cleanup: validation, error handling, debuggability | `docs/build-log/2026-03-04-pr-028-audit-cleanup.md` |
| 29 | Inspector Content-Security-Policy | `docs/build-log/2026-03-04-pr-029-inspector-csp.md` |
| 30 | Replace test time.Sleep with polling+deadline | `docs/build-log/2026-03-04-pr-030-test-sleep-polling.md` |
| 31 | Thread context.Context through store calls | `docs/build-log/2026-03-04-pr-031-store-context-threading.md` |
| 32 | WebSocket origin validation (localhost-only) | `docs/build-log/2026-03-05-pr-032-websocket-origin-validation.md` |
| 33 | Constant-time token comparison | `docs/build-log/2026-03-05-pr-033-token-constant-time.md` |
| 34 | Integration test suite (daemon lifecycle) | `docs/build-log/2026-03-05-pr-034-integration-tests.md` |
| 35 | Storage mutation transport wiring (inspector -> websocket -> daemon) | `docs/build-log/2026-03-05-pr-035-storage-mutation-transport-wiring.md` |
| 36 | Simulator transport protocol messages (`chrome.api.call/result/event`) | `docs/build-log/2026-03-05-pr-036-simulator-transport-protocol.md` |
| 37 | Daemon simulator storage router (`chrome.api.call` -> `chrome.api.result`) | `docs/build-log/2026-03-05-pr-037-daemon-chrome-api-storage-router.md` |
| 38 | `shared/chrome-sim` transport + storage shim scaffold | `docs/build-log/2026-03-05-pr-038-chrome-sim-transport-scaffold.md` |
| 39 | CI trigger coverage for `feat/pr*` + `shared/chrome-sim` TypeScript matrix | `docs/build-log/2026-03-05-pr-039-ci-checks-feature-push-chrome-sim.md` |
| 40 | Runtime namespace extension (`runtime.sendMessage`) + chrome-sim bootstrap query wiring | `docs/build-log/2026-03-05-pr-040-runtime-sendmessage-bootstrap-wiring.md` |
| 41 | Chrome-sim entrypoint injection helper + script bootstrap value resolution | `docs/build-log/2026-03-05-pr-041-chrome-sim-entrypoint-bootstrap-helpers.md` |
| 42 | Tabs namespace extension (`tabs.query`) across daemon router + chrome-sim shim | `docs/build-log/2026-03-05-pr-042-tabs-query-daemon-chrome-sim.md` |
| 45 | Inspector preview build hook calls `injectChromeSimEntrypoint(...)` and emits injected `dist/index.html` | `docs/build-log/2026-03-05-pr-045-inspector-preview-injection-hook.md` |
| 46 | Core Go build pipeline copies HTML surfaces and injects `chrome-sim` bootstrap | `docs/build-log/2026-03-06-pr-046-core-build-html-chrome-sim-injection.md` |
| 47 | Daemon startup build before steady-state watch loop | `docs/build-log/2026-03-06-pr-047-startup-build.md` |
| 48 | Reject overlapping extension source/output directories | `docs/build-log/2026-03-06-pr-048-source-outdir-overlap-guard.md` |
| 49 | Align root commands and CI with the polyglot build surface | `docs/build-log/2026-03-06-pr-049-root-build-ci-coverage.md` |
| 50 | Require frozen lockfiles for every TypeScript package install | `docs/build-log/2026-03-06-pr-050-js-determinism.md` |
| 51 | Consolidate TypeScript dependency installs under one root `pnpm` workspace lockfile | `docs/build-log/2026-03-06-pr-051-pnpm-workspace.md` |
| 52 | Extract shared TypeScript compiler presets while keeping package-local build behavior | `docs/build-log/2026-03-06-pr-052-tsconfig-presets.md` |
| 53 | Move cross-package TypeScript imports onto workspace package entrypoints | `docs/build-log/2026-03-06-pr-053-workspace-entrypoints.md` |
| 54 | Enable the first real Workbench tab as a read-only operator overview | `docs/build-log/2026-03-06-pr-054-workbench-tab.md` |
| 55 | Add reversible namespaced storage presets as the first actionable Workbench tool | `docs/build-log/2026-03-06-pr-055-workbench-storage-presets.md` |
| 56 | Add the first namespaced Workbench runtime probe on top of `runtime.sendMessage` | `docs/build-log/2026-03-06-pr-056-workbench-runtime-probe.md` |
| 57 | Add the first Workbench replay control by replaying the latest observed runtime probe payload | `docs/build-log/2026-03-06-pr-057-workbench-replay-control.md` |
| 58 | Replace inspector CSP-breaking template rendering with CSP-safe Solid hyperscript rendering | `docs/build-log/2026-03-06-pr-058-inspector-csp-safe-rendering.md` |
| 59 | Enable the first focused Replay tab from observed runtime probe history | `docs/build-log/2026-03-07-pr-059-replay-tab.md` |
| 60 | Keep `127.0.0.1` as the local websocket default and document `localhost` override | `docs/build-log/2026-03-07-pr-060-loopback-defaults.md` |
| 61 | Guard loopback daemon URL overrides and start an explicit audit tracker | `docs/build-log/2026-03-09-pr-061-loopback-daemon-url-guards.md` |
| 62 | Add a supported `make init` bootstrap for git hook installation | `docs/build-log/2026-03-09-pr-062-make-init-bootstrap.md` |
| 63 | Add CI dependency verification with `go mod verify` and `pnpm audit` | `docs/build-log/2026-03-09-pr-063-dependency-verification.md` |
| 64 | Decode raw protocol payload bytes without a msgpack round trip | `docs/build-log/2026-03-09-pr-064-raw-payload-decode.md` |
| 65 | Harden daemon session lifecycle cancellation and close serialization | `docs/build-log/2026-03-09-pr-065-daemon-session-lifecycle.md` |
| 66 | Cover panex CLI startup orchestration and raise `cmd/panex` coverage | `docs/build-log/2026-03-09-pr-066-panex-cli-coverage.md` |
| 67 | Guard browser websocket clients against oversized inbound frames | `docs/build-log/2026-03-09-pr-067-browser-websocket-message-guard.md` |
| 68 | Move daemon auth from websocket query params into the hello handshake | `docs/build-log/2026-03-09-pr-068-websocket-hello-auth.md` |
| 69 | Upgrade Go baseline to 1.25.8 and gate CI with `govulncheck` | `docs/build-log/2026-03-09-pr-069-go-toolchain-govulncheck.md` |
| 70 | Centralize the replay contract boundary and keep replay scoped to runtime probes | `docs/build-log/2026-03-09-pr-070-replay-contract-boundary.md` |
| 71 | Fold preserved code-review follow-ups into the roadmap queue | `docs/build-log/2026-03-09-pr-071-review-followup-queue.md` |
| 72 | Add a focused Workbench chrome API activity log over existing timeline history | `docs/build-log/2026-03-09-pr-072-workbench-chrome-api-activity-log.md` |
| 73 | Enforce dev-agent `hello.ack` completion before accepting live commands | `docs/build-log/2026-03-09-pr-073-agent-hello-ack-enforcement.md` |
| 74 | Add daemon websocket read/write deadlines and ping keepalive | `docs/build-log/2026-03-09-pr-074-daemon-websocket-deadlines.md` |
| 75 | Add optional dev-agent websocket lifecycle diagnostics behind stored config | `docs/build-log/2026-03-09-pr-075-agent-diagnostic-logging.md` |
| 76 | Polish inspector keyboard focus treatment and ARIA semantics | `docs/build-log/2026-03-09-pr-076-inspector-accessibility-polish.md` |
| 77 | Publish first-run `panex.toml` config/schema documentation and local-dev guidance | `docs/build-log/2026-03-09-pr-077-panex-config-guide.md` |
| 78 | Support `$PANEX_AUTH_TOKEN` overrides in `panex dev` for automation/package flows | `docs/build-log/2026-03-09-pr-078-panex-auth-token-override.md` |
| 79 | Package reproducible release archives for the `panex` CLI | `docs/build-log/2026-03-10-pr-079-release-archives.md` |
| 80 | Publish tagged CI release artifacts on top of the local release packager | `docs/build-log/2026-03-10-pr-080-release-publish-ci.md` |
| 81 | Publish SHA256 checksums alongside tagged release assets | `docs/build-log/2026-03-10-pr-081-release-checksums.md` |
| 82 | Document download verification against published release checksums | `docs/build-log/2026-03-10-pr-082-release-download-verification.md` |
| 83 | Add paginated `query.events` loading and a Timeline “load older” flow | `docs/build-log/2026-03-10-pr-083-query-events-pagination.md` |
| 84 | Cap default Timeline rendering to a live tail window with explicit older-history reveal controls | `docs/build-log/2026-03-10-pr-084-timeline-render-windowing.md` |
| 85 | Copy manifest and other non-bundled extension assets into the build output | `docs/build-log/2026-03-10-pr-085-extension-static-asset-copy.md` |
| 86 | Split product, contributor, and agent documentation entry points | `docs/build-log/2026-03-10-pr-086-docs-split-readme-audiences.md` |
| 87 | Persist storage mutations transactionally across daemon restarts | `docs/build-log/2026-03-11-pr-087-storage-mutation-persistence.md` |
| 88 | Bound the inspector Timeline to a reloadable working set so older browsing does not grow client memory without limit | `docs/build-log/2026-03-11-pr-088-timeline-history-scalability.md` |
| 89 | Add multi-extension config/build/watch support with targeted reload routing | `docs/build-log/2026-03-11-pr-089-multi-extension-support.md` |
| 90 | Add `panex init` first-run scaffolding and default missing-config recovery guidance | `docs/build-log/2026-03-11-pr-090-first-run-init.md` |
| 91 | Skip infrastructure directories in build discovery and file watching | `docs/build-log/2026-03-16-pr-091-infrastructure-dir-exclusion.md` |
| 92 | Zero-config `panex dev` for manifest.json directories | `docs/build-log/2026-03-16-pr-092-zero-config-manifest-detection.md` |
| 93 | Guided startup messaging with Chrome loading instructions | `docs/build-log/2026-03-16-pr-093-guided-startup-messaging.md` |
| 94 | Add `panex doctor` for project health checks | `docs/build-log/2026-03-16-pr-094-panex-doctor.md` |
| 95 | Add `panex paths` for quick path inspection | `docs/build-log/2026-03-16-pr-095-panex-paths.md` |
| 96 | Add `panex dev --open` to launch `chrome://extensions` | `docs/build-log/2026-03-16-pr-096-dev-open.md` |
| 97 | Add Linux/macOS install script | `docs/build-log/2026-03-16-pr-097-install-script.md` |
| 98 | Enforce all PRs must target main branch | (process PR, no build-log entry) |
| 99 | Add Homebrew tap formula and release automation | `docs/build-log/2026-03-16-pr-099-homebrew-formula.md` |
| 100 | Generate `.deb` packages for Linux releases | `docs/build-log/2026-03-17-pr-100-deb-package.md` |

## In progress
- None.

## Next
- Continue Phase 4 packaging: Windows MSI/winget.

## Queued Follow-Ons
- None.

## Notes
- PR20 is intentionally reserved as documentation reconciliation so sequence alignment is explicit and auditable.
