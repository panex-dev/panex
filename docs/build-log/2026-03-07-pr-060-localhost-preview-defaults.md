# PR60 - Prefer localhost for browser-facing local websocket defaults

## Summary
- Switched browser-facing Panex local defaults from `ws://127.0.0.1:4317/ws` to `ws://localhost:4317/ws`.
- Aligned inspector preview, chrome-sim fallback transport, agent config defaults, and `panex dev` startup output on the same local endpoint.
- Updated inspector preview docs so the default local URL matches the browser environment that actually opens successfully.

## Why
- Some local browser environments reached the daemon successfully via `localhost` but failed and stayed in `reconnecting` when the inspector defaulted to `127.0.0.1`.
- The daemon already allows both `localhost` and `127.0.0.1` origins, so the failure mode was not a server policy problem; it was a client-facing default mismatch.
- Keeping browser entrypoints on one default avoids a split-brain preview loop where the inspector, chrome-sim injection, and CLI banner suggest different websocket addresses.

## Changes
- Updated the inspector query-param fallback websocket URL in `inspector/src/connection.ts`.
- Updated the preview build hook default injected chrome-sim daemon URL in `inspector/scripts/build.ts`.
- Updated the chrome-sim runtime fallback daemon URL in `shared/chrome-sim/src/transport.ts`.
- Updated the dev-agent stored-config default websocket URL in `agent/src/config.ts`.
- Updated `panex dev` startup output and auto-injected daemon URL in `cmd/panex/main.go`.
- Updated preview usage guidance in `inspector/README.md`.

## Verification
- `pnpm install`
- `pnpm --dir agent run test`
- `pnpm --dir inspector run test`
- `pnpm --dir shared/chrome-sim run test`
- `pnpm --dir inspector run build`
- `GOCACHE=/tmp/go-build make test`
- `GOCACHE=/tmp/go-build make build`

## Outcome
- Local browser-facing defaults now prefer `localhost`, which matches the verified working preview path without weakening daemon origin validation or widening the CSP.
