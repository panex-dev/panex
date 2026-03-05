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
- Bootstrap parameter resolution from URL search (`ws`, `token`, `extension_id`)
- Script-tag bootstrap dataset support (`data-panex-ws`, `data-panex-token`, `data-panex-extension-id`)
- Entrypoint injection helper via `@panex/chrome-sim/bootstrap`

## Local checks

```bash
cd shared/chrome-sim
pnpm run check
pnpm run test
```
