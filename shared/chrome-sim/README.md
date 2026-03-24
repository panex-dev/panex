# @panex/chrome-sim

Browser-side shim for routing `chrome.*` calls to the Panex daemon over WebSocket.

## Current scope

- Transport handshake + reconnect scaffold
- `call_id` correlation for `chrome.api.call` -> `chrome.api.result`
- `chrome.storage.{local,sync,session}` method wiring:
  - `get`
  - `set`
  - `remove`
  - `clear`
  - `getBytesInUse`
- `chrome.runtime.sendMessage(...)` wiring via `chrome.api.call`
- `chrome.runtime.onMessage` listener fanout from `chrome.api.event`
- `chrome.tabs.query(...)` wiring via `chrome.api.call`
- Bootstrap parameter resolution from URL search (`ws`, `token`, `extension_id`)
- Script-tag bootstrap dataset support (`data-panex-ws`, `data-panex-token`, `data-panex-extension-id`)
- Auth token delivery via the initial `hello` payload instead of the websocket URL query string
- Entrypoint injection helper via `@panex/chrome-sim/bootstrap`
- Extension-aware hello handshake and reload targeting via `extension_id`

## Status: transport only

chrome-sim provides API shims and WebSocket transport. It does **not** provide:

- A rendering host (no Vite subprocess, no dev server proxy)
- Iframe embedding of extension UI surfaces
- Surface discovery (popup, side panel, options page enumeration)
- Any visual preview of extension pages in the inspector

Preview Mode -- rendering extension HTML surfaces inside the inspector --
is not implemented. The workbench tab in the inspector is a diagnostics
panel for transport, storage, and runtime probing; it is not a preview
surface.

Contributors should not attempt to wire up iframe rendering or Vite
integration against chrome-sim. When preview mode is eventually built it
will require its own explicit design and build-pipeline support.

## Local checks

```bash
cd shared/chrome-sim
pnpm run check
pnpm run test
```
