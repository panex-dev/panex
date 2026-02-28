# ADR-001: Go as Primary Language for Panex Daemon

## Status
Accepted

## Context
Panex is a development runtime for Chrome extensions. The core daemon handles:
- File system watching (build trigger)
- Embedded bundling (esbuild, which is written in Go)
- WebSocket server (bidirectional streaming to Dev Agent + Inspector)
- Chrome DevTools Protocol (CDP) communication
- SQLite-based event trace store

We need a langunage that produces a single binary (no runtime dependencies for end users), handles concurrency well (many simultaneous WebSocket connections + file watchers), and integrates cleanly with esbuild and CDP libraries.

## Options Considered

### Go
- Esbuild is Go-native; embed as library, we have  zero subprocess overhead
- `chromedp` for CDP: mature, well-maintained
- Single static binary via `CGO_ENABLED=0` (or with CGO for SQLite)
- Goroutines + channels map directly to our concurrency model
- Fast compilation, simple toolchain

### Rust
- Better runtime performance (marginal for our workloads though)
- `chromiumoxide` for CDP (its less mature than `chromedp`
- esbuild integration requires subprocess or WASM bridge when it comes to rust

### Node.js/Typescript
- Farmiliar to extension devs
- esbuild has JS API
- Single binary requires bundling (pkg/nexe) which is fragile
- Concurrency model (event loop) is less natural for this workload

## Decision
**Go.** The esbuild-in-process and chromedp advantages are decisive.
Rust is defensible long-term but adds integration friction during the discovery phase, we might actually consider it later.

## Consequences
- Team must be proficient or willing ro learn Go
- TS clients (Dev Agent, Inspector) communicate via network protocol, not FFI
- SQLite integration requires CGO (via mattn/go-sqlite3) or pure-Go (modernc.org/sqlite)
- If we later need Rust-level performance, the protocol boundary makes migration possible per-component
