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

## Local checks

```bash
cd shared/chrome-sim
pnpm run check
pnpm run test
```
