# @panex/protocol

Canonical TypeScript protocol contract shared by the Dev Agent and Inspector.

## Why this exists

Before this package, both clients defined envelope names/types independently, which allowed silent drift. This module centralizes:

- Envelope and payload type definitions
- Message-name to message-type mapping
- Runtime envelope guards used at websocket boundaries

## Usage

Import directly from source inside this repo:

```ts
import { isEnvelope, type Envelope } from "../../shared/protocol/src/index";
```

## Rule

Any protocol shape change must update both:

- `internal/protocol/types.go`
- `shared/protocol/src/index.ts`

The tests in `shared/protocol/tests` verify the TypeScript side stays self-consistent.
