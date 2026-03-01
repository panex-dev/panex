# ADR-005: File Watching Strategy for Build Triggers

## Status
Accepted

## Context
Panex needs a low-latency trigger from extension source edits to downstream build/reload steps.
The watcher must handle bursty editor write patterns and avoid emitting noisy per-write events.

## Decision
- Use `fsnotify` for filesystem event delivery.
- Watch all directories under the configured extension source root.
- When a new directory is created, add a watcher for that subtree immediately.
- Batch changed paths with a 50ms debounce window (`DefaultWatchDebounce`).
- Emit relative, slash-normalized paths to make downstream protocol payloads stable across platforms.

## Consequences
- Low overhead compared to polling.
- Burst writes collapse into one internal change event, which reduces redundant builds.
- Some platforms can emit duplicate low-level events; deduplication by path in the debounce window handles this for MVP.

## Reversibility
High.
If we outgrow `fsnotify` behavior in specific environments, the watcher API can remain stable while implementation switches to a polling or hybrid backend.
