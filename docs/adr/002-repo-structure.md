#ADR-002: Monorepo with Go Standard Layout

## Status
Accepted

## Context
Panex consists of multiple components:
- Go daemon (CLI + orchestrator)
- Typescript Dev Agent (Chrome extension)
- Typescript INspector UI (web app)
- Typescript Chrome API similator (npm package)
- Shared protocol definitions

These could live in separate repos or one monorepo.

## Options Considered

### Monorepo
- Single PR can touch daemon + protocol + agent
- Shared CI, shared versioning during early development
- Easier to keep protocol definitions in sync
- Tooling cost: need polygot build (Go + Node)

### Multi-repo
- Cleaner dependency boundaries
- Independent release cycles
- Higher coordination overhead during rapid early iteration

## Decision
**Monorepo.** During the discovery phase, protocol changes touch 2-3 components simultaenously. A monorepo makes atomic cross-component changes possible in a single PR.

We'll actually revisit when any component needs an independent releasecycle.

## Consequences
- CI must handle both Go and Node.js
- Directory structure must clearly separate components
- We'll use a top-level Makefile as the entry point for all ops

## Structure
```

panex/
├── cmd/panex/          # Go binary entry point
├── internal/           # Go packages (not importable externally)
│   ├── daemon/         # WebSocket server, orchestration
│   ├── build/          # esbuild integration
│   ├── protocol/       # Message types, envelope, codec
│   ├── store/          # SQLite event store
│   └── cdp/            # Chrome DevTools Protocol
├── agent/              # TypeScript Dev Agent (Chrome extension)
├── inspector/          # TypeScript Inspector UI
├── sim/                # TypeScript chrome.* simulator
├── proto/              # Shared protocol schema (MessagePack)
├── docs/
│   └── adr/
├── .github/workflows/
├── Makefile
└── go.mod
```

## Reversibility
High. Extracting a subsdirectory into its own repo is straightforward with `git filter-branch` or `git subtree split`. The protocol boundarY (WebSocket + MessagePack) means components are already loosely coupled at runtime. 
