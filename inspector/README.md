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
Protocol definitions are imported from `../shared/protocol/src/index.ts`.
The timeline supports search + filter controls for message type and source role.
Search operators:
- `name:<message-name>`
- `src:<role-or-id>`
- `type:<lifecycle|event|command>`
Filter values are persisted in browser localStorage per host.
Inspector reconnects automatically with exponential backoff if the daemon drops.

Optional URL params:
- `ws`: daemon websocket endpoint (default `ws://127.0.0.1:4317/ws`)
- `token`: daemon auth token (default `dev-token`)

Example:

```text
file:///.../inspector/index.html?ws=ws://127.0.0.1:4317/ws&token=dev-token
```
