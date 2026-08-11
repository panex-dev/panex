# Panex

Panex is an agent-oriented engineering runtime for browser extensions. It gives humans and coding agents a shared project model, a plan/apply workflow for extension changes, target-aware manifest generation, local health checks, release packaging, and a Chrome development bridge.

> **Status:** Early development. The Chrome extension dev loop is usable for local projects, and the newer project automation/MCP surface is active but still moving.

## What Panex Does

- Inspects JavaScript and TypeScript extension projects and records a normalized `.panex/project.graph.json`.
- Plans and applies generated extension changes with drift checks, run logs, rollback-on-failure inside apply, and project locks.
- Compiles target-specific manifests from declared capabilities. Chrome is the implemented target adapter today.
- Packages extension artifacts and records run reports under `.panex/runs/`.
- Runs a local Chrome extension build/watch loop with a loopback WebSocket daemon, Dev Agent integration, inspector UI, storage tooling, runtime probes, and replay-oriented event history.
- Exposes the same project operations over a stdio MCP server so agents can inspect, plan, apply, verify, repair, package, report, and start dev sessions through tools.

## Install

Download a prerelease from GitHub Releases, install from the supported package channel for your OS, or build from source with the contributor workflow in [CONTRIBUTING.md](./CONTRIBUTING.md).

On Windows, run `panex.exe` from PowerShell or Command Prompt. Panex is a CLI runtime, not a desktop GUI.

## CLI Surface

```text
panex [--cwd path] [--json] version
panex [--cwd path] [--json] init [--force]
panex [--cwd path] [--json] add-target <target>
panex [--cwd path] [--json] inspect
panex [--cwd path] [--json] plan
panex [--cwd path] [--json] apply [--force]
panex [--cwd path] [--json] dev [--config path/to/panex.toml] [--open]
panex [--cwd path] [--json] test
panex [--cwd path] [--json] verify
panex [--cwd path] [--json] package [--version v0.1.0]
panex [--cwd path] [--json] report [--run-id id]
panex [--cwd path] [--json] resume [--run-id id]
panex [--cwd path] [--json] doctor [--fix]
panex [--cwd path] [--json] paths
panex [--cwd path] [--json] mcp
```

- `--cwd` points Panex at a project directory without changing your shell location.
- `--json` returns command envelopes for automation and agent callers.
- `panex mcp` starts the stdio MCP server.

## Quick Start: Chrome Dev Loop

Use this when you want Panex to scaffold, build, watch, and reload a local Chrome extension.

```bash
panex init
panex dev --open
```

Then open `chrome://extensions`, enable Developer Mode, choose `Load unpacked`, and select:

```text
.panex/dist
```

`panex init` writes a starter Chrome extension and a `panex.toml` file:

```text
panex.toml
panex-extension/manifest.json
panex-extension/background.js
panex-extension/popup.html
panex-extension/popup.js
```

For an existing unpacked Chrome extension, run Panex from the directory containing `manifest.json`; if there is no `panex.toml`, `panex dev` infers `source_dir = "."` and `out_dir = ".panex/dist"`.

Useful checks:

```bash
panex paths
panex doctor
```

## Quick Start: Project Automation

Use this when you want Panex to inspect a project, build a graph, plan generated changes, apply them, verify the result, and package artifacts.

```bash
panex inspect
panex add-target chrome
panex plan
panex apply
panex verify
panex package --version v0.1.0
panex report
```

Panex stores automation state in `.panex/`, including the project graph, policy, current plan, run ledger, generated manifests, session metadata, and package artifacts.

Project automation reads `panex.config.ts` first, then `panex.config.json`. If neither exists, Panex can infer a graph from the project structure and `add-target` can bootstrap a JSON config. `add-target` cannot rewrite a TypeScript-authored config; update `panex.config.ts` manually in that case.

Minimal `panex.config.json`:

```json
{
  "project": {
    "name": "my-extension",
    "id": "my-extension"
  },
  "entries": {
    "background": { "path": "src/background.ts" },
    "popup": { "path": "src/popup.ts" }
  },
  "targets": {
    "chrome": { "enabled": true }
  },
  "capabilities": {
    "storage": true,
    "tabs": true
  }
}
```

## MCP Server

`panex mcp` exposes Panex over JSON-RPC stdio. Current tools include:

```text
inspect_project
initialize_project
add_target
plan_changes
apply_changes
verify_project
test_project
doctor_project
repair_failure
package_release
read_report
resume_run
start_dev_session
```

Current resources include:

```text
panex://project/graph
panex://project/config-lock
panex://environment
panex://runs/latest
```

## Chrome Dev Configuration

The Chrome dev loop uses `panex.toml`.

Single extension:

```toml
[extension]
source_dir = "panex-extension"
out_dir = ".panex/dist"

[server]
port = 4317
auth_token = "replace-this-dev-token"
event_store_path = ".panex/events.db"
```

Multiple extension build/watch targets:

```toml
[[extensions]]
id = "popup"
source_dir = "extensions/popup"
out_dir = ".panex/dist/popup"

[[extensions]]
id = "admin"
source_dir = "extensions/admin"
out_dir = ".panex/dist/admin"

[server]
port = 4317
auth_token = "replace-this-dev-token"
event_store_path = ".panex/events.db"
```

Multi-extension build, watch, and reload routing are supported. Runtime and storage isolation are still shared in some inspector surfaces.

Config rules:

- Use either `[extension]` or `[[extensions]]`, not both.
- `source_dir` and `out_dir` are required and must not overlap.
- Multi-extension IDs must be unique.
- `server.port` must be between `1024` and `65535`.
- `server.auth_token` is required; `PANEX_AUTH_TOKEN` can override it at runtime for `panex dev`.

## Supported Chrome Runtime Surface

The browser-side simulator currently supports:

- `chrome.runtime.sendMessage` / `chrome.runtime.onMessage`
- `chrome.tabs.query`
- `chrome.storage.local` / `chrome.storage.sync` / `chrome.storage.session`
- `chrome.storage.onChanged`

Other Chrome extension APIs may be recognized as capabilities for manifest generation, but they are not all implemented in the runtime simulator.

## Release Verification

After downloading a release archive, download the matching `panex_<version>_SHA256SUMS` file from the same GitHub release and verify the asset you fetched.

Linux:

```bash
grep ' panex_v0.1.0_linux_amd64.tar.gz$' panex_v0.1.0_SHA256SUMS | sha256sum -c -
```

macOS:

```bash
grep ' panex_v0.1.0_darwin_arm64.tar.gz$' panex_v0.1.0_SHA256SUMS | shasum -a 256 -c -
```

PowerShell:

```powershell
$expected = ((Select-String ' panex_v0.1.0_windows_amd64.zip$' .\panex_v0.1.0_SHA256SUMS).Line -split '\s+')[0].ToLower()
$actual = (Get-FileHash .\panex_v0.1.0_windows_amd64.zip -Algorithm SHA256).Hash.ToLower()
if ($actual -ne $expected) { throw "checksum mismatch" }
Write-Host "checksum ok"
```

Replace the version and filename with the asset you actually downloaded.

## More Docs

- Contributors: [CONTRIBUTING.md](./CONTRIBUTING.md)
- Repo map: [docs/repo-map.md](./docs/repo-map.md)
- Coding agents: [AGENTS.md](./AGENTS.md)
- ADRs: [docs/adr/](./docs/adr/)
- Build history: [docs/build-log/](./docs/build-log/)
