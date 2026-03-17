# PR 102 — OS-aware download and install page

**Status:** merged
**Branch:** `feat/pr102-download-page`
**Base:** `main`
**Phase:** 5 (Website and download-page polish)

## What

Add a static download/install page at `site/` that detects the visitor's OS and shows the recommended install method for their platform, with tabs to switch between platforms.

## Why

Phase 5 of the onboarding plan calls for an OS-aware download page so users arriving from search can install Panex and know what to do next without reading long docs. The previous state was no website at all — only GitHub Releases.

## Changes

- `site/index.html` — single-page install guide with per-platform cards (macOS/Linux/Windows), get-started instructions, and Chrome loading steps
- `site/style.css` — responsive styles with dark mode support via `prefers-color-scheme`
- `site/detect.js` — OS detection via `navigator.userAgentData` with `userAgent`/`platform` fallback; shows the matching platform card and tabs for others
- `.github/workflows/pages.yml` — GitHub Pages deployment triggered on pushes to `main` that touch `site/` or the workflow itself

## Design decisions

- **No build step**: plain HTML/CSS/JS ships as-is to GitHub Pages. No framework, no bundler, no node dependency. The page is small enough that a build step adds complexity for no benefit.
- **Progressive disclosure**: detected platform shown first, other platforms accessible via tabs, manual/archive install in a `<details>` element.
- **Dark mode**: `prefers-color-scheme: dark` media query with CSS custom properties. No toggle — respects system preference.
- **OS detection order**: `navigator.userAgentData.platform` (Chromium) first, then `userAgent`/`platform` sniffing as fallback for Firefox/Safari.

## Quality

- No Go or TypeScript changes — `make fmt && make lint && make test && make build` unaffected
- HTML validated manually
- JS uses strict mode, no external dependencies, no ES6+ syntax (broad browser compat)
