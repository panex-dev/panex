# Panex Inspector

SolidJS timeline UI for Panex protocol activity.

## Local development

```bash
cd inspector
pnpm install
pnpm run check
pnpm run test
pnpm run build
```

Open `inspector/index.html` after building.
`pnpm run build` also emits `inspector/dist/index.html` with an injected chrome-sim entrypoint (`./chrome-sim.js`) using `data-panex-*` bootstrap attributes.
Injection defaults can be overridden via:
- `PANEX_DAEMON_URL`
- `PANEX_DAEMON_TOKEN`
- `PANEX_EXTENSION_ID`
Protocol definitions are imported from the workspace package entrypoint `@panex/protocol`.
The timeline supports search + filter controls for message type and source role.
Search operators:
- `name:<message-name>`
- `src:<role-or-id>`
- `type:<lifecycle|event|command>`
Filter values are persisted in browser localStorage per host.
Inspector reconnects automatically with exponential backoff if the daemon drops.

Optional URL params:
- `ws`: daemon websocket endpoint (default `ws://127.0.0.1:4317/ws`; only top-level loopback `ws://127.0.0.1:<port>/ws` or `ws://localhost:<port>/ws` values are honored, and any embedded `token=` query param is stripped before connect)
- `token`: daemon auth token (default `dev-token`)

Embedded inspector loads ignore URL param overrides and fall back to the built-in defaults.

Example:

```text
file:///.../inspector/index.html?ws=ws://127.0.0.1:4317/ws&token=dev-token
```

If your browser environment cannot connect cleanly to `127.0.0.1`, pass `ws=ws://localhost:4317/ws`
explicitly or set `PANEX_DAEMON_URL=ws://localhost:4317/ws` before building the preview bundle.
