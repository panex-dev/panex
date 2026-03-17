# PR 103 — Inspector dark mode, font fix, and client ID hardening

**Status:** merged
**Branch:** `feat/pr103-inspector-dark-mode`
**Base:** `main`

## What

Add `prefers-color-scheme: dark` support to the inspector, fix the unloaded IBM Plex font declarations, harden the `safeClientID` fallback against same-millisecond collisions, and fix the inverted responsive timeline max-height.

## Why

The inspector hardcoded a light-only color scheme, declared IBM Plex Sans/Mono fonts that were never loaded (always falling back to system fonts), and used `Date.now()` as the UUID fallback — which collides if two tabs open in the same millisecond. The responsive `max-height` went from 66vh to 70vh on mobile (taller on smaller screens).

## Changes

- `inspector/src/styles.css` — extract all hardcoded colors into CSS custom properties on `:root`, add `@media (prefers-color-scheme: dark)` block with dark palette, replace IBM Plex with system font stack, fix mobile timeline `max-height` from `70vh` to `55vh`
- `inspector/src/connection.ts` — add `crypto.getRandomValues` intermediate fallback in `safeClientID()`, append random suffix to `Date.now()` final fallback

## Quality

- `pnpm run check` — clean
- `pnpm run test` — 62/62 pass
- `pnpm run build` — clean (12.2kb CSS)
