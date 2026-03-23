# Panex

Panex is a development runtime for Chrome extensions. It watches an unpacked extension source tree, rebuilds it into a separate output directory, serves a local daemon for browser tooling, and captures runtime activity for inspection and replay.

> **Status:** Early development. Not usable yet.

## What It Is

- A local CLI runtime, not a desktop GUI app.
- A build-and-watch loop for Chrome extension source code.
- A loopback daemon that browser tooling connects to over WebSocket.
- A foundation for inspecting extension events, storage activity, and replayable runtime probes.

## Install Or Download

Download the latest prerelease from the GitHub releases page or build from source with the contributor workflow in [CONTRIBUTING.md](./CONTRIBUTING.md).

The CLI surface today is:

```text
panex version
panex init [--force]
panex dev [--config path/to/panex.toml]
```

On Windows, run `panex.exe` from PowerShell or Command Prompt. Double-clicking it will not open a GUI.

## Quick Start

1. Put `panex` on your machine and open a terminal in an empty working folder.
2. Run:

```bash
panex init
```

3. Start the runtime:

```bash
panex dev
```

4. Open `chrome://extensions`, enable Developer Mode, choose `Load unpacked`, and select:

```text
.panex/dist
```

5. Click the loaded starter extension to confirm the generated popup works.

`panex init` writes:

- `panex.toml`
- `panex-extension/manifest.json`
- `panex-extension/background.js`
- `panex-extension/popup.html`
- `panex-extension/popup.js`

Starter config created by `panex init`:

```toml
[extension]
source_dir = "panex-extension"
out_dir = ".panex/dist"

[server]
port = 4317
auth_token = "dev-token"
event_store_path = ".panex/events.db"
```

Multi-extension config:

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

When Panex starts successfully, it stays running and prints the local daemon URL:

```text
panex dev
ws_url=ws://127.0.0.1:4317/ws
```

## How To Use It

### 1. Point Panex at an extension source tree

`panex init` is the fastest path for a first run. It scaffolds a visible starter extension and the default `panex.toml` in the current directory.

If you already have an extension project, `[extension].source_dir` must point at that unpacked Chrome extension directory. Panex watches that tree, bundles extension entrypoints, rewrites HTML surfaces, and copies non-bundled assets such as `manifest.json` into `[extension].out_dir`.

For more than one extension target, switch to `[[extensions]]` and give each entry a unique `id`. Panex then runs one build/watch loop per configured target and tags `build.complete` and `command.reload` events with that `id`.

### 2. Start the local runtime

Run:

```bash
panex dev
```

Or point at a different config file:

```bash
panex dev --config path/to/panex.toml
```

If you want Panex to regenerate the default starter files, rerun:

```bash
panex init --force
```

### 3. Load the built extension in Chrome

Open `chrome://extensions`, enable Developer Mode, choose `Load unpacked`, and select the `out_dir` from your config.

For a multi-extension config, load each generated output directory separately.

### 4. Verify basic behavior

- `panex version` prints the installed version.
- `panex dev` keeps running instead of exiting.
- The configured `out_dir` contains your built extension output, including `manifest.json`.
- The daemon prints `ws_url=...` so browser tooling can connect locally.

For multi-extension configs:

- each configured `out_dir` is built independently
- reload messages are targeted by extension `id`
- dev-agent and `chrome-sim` clients must present the matching extension ID for non-default targets

## Supported Chrome APIs

Panex includes a Chrome API simulator (`chrome-sim`) for use in preview and testing contexts. The following APIs are currently supported:

- `chrome.runtime.sendMessage` / `chrome.runtime.onMessage`
- `chrome.tabs.query`
- `chrome.storage.local` / `chrome.storage.sync` / `chrome.storage.session` (get, set, remove, clear, getBytesInUse)
- `chrome.storage.onChanged`

Other Chrome extension APIs (`chrome.action`, `chrome.scripting`, `chrome.alarms`, `chrome.notifications`, `chrome.contextMenus`, `chrome.identity`, `chrome.webRequest`, etc.) are not yet implemented and will return an "unsupported" error from the daemon. This surface is expected to grow over time.

## Config Reference

- `[extension].source_dir`: required path to the unpacked extension source tree that Panex watches and rebuilds. `panex init` creates `panex-extension` for the default single-extension path.
- `[extension].out_dir`: required build output directory. It must not overlap `source_dir`.
- `[[extensions]].id`: required unique identifier for a multi-extension target.
- `[[extensions]].source_dir`: required source directory for that target.
- `[[extensions]].out_dir`: required build output directory for that target.
- `[server].port`: required TCP port for the local daemon. Use any value from `1` to `65535`.
- `[server].auth_token`: required shared secret for local WebSocket clients. Clients send this token during the `hello` handshake.
- `[server].event_store_path`: optional SQLite path for the event log. If omitted, Panex defaults it to `.panex/events.db`.

Runtime override:

- Set `PANEX_AUTH_TOKEN` before `panex dev` to override `server.auth_token` without editing `panex.toml`.
- If `PANEX_AUTH_TOKEN` is set, it must be non-empty after trimming whitespace.

Validation rules:

- Unknown config keys are rejected.
- Empty required values are rejected.
- Use either `[extension]` or `[[extensions]]`, not both.
- `source_dir` and `out_dir` cannot be the same directory or nested inside each other.
- Multi-extension targets must use unique IDs.
- Multi-extension source and output paths must not overlap each other.

Current multi-extension scope:

- build, watch, and reload targeting are extension-aware
- the inspector still shows one shared event stream
- broader per-extension runtime and storage isolation is not complete yet

## Release Verification

After downloading a release archive, download the matching `panex_<version>_SHA256SUMS` file from the same GitHub release and verify the specific asset you fetched.

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
