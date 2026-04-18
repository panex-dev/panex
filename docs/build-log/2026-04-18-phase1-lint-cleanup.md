# Phase 1 Lint Cleanup

**Status:** PR pending
**Date:** 2026-04-18

## Problem

`make lint` failed with 14 `errcheck` violations across the Phase 1 packages:

- 13 unchecked errors in test files (`os.MkdirAll`, `os.WriteFile`, `os.Chtimes`, `f.Close`, `os.Remove`).
- 4 unchecked `Close()` calls in production code (`internal/target/chrome.go` `createZip` and `fileDigest`). The `f.Close()` on the zip output file and `w.Close()` on `*zip.Writer` were the most consequential — both are write-side closes, where a dropped error silently masks flush/sync failures and could let a corrupt artifact appear successful.

The Phase 1 build logs claimed lint was "clean on new packages." That was inaccurate — the failures were always present in the new test files and the new chrome adapter.

## Approach

- **`internal/target/chrome.go:319-374`** — convert `createZip` to a named-return function so deferred close errors propagate. Surface errors from both `f.Close()` (zip output file) and `w.Close()` (zip writer). Read-side closes (`src.Close`, `fileDigest`'s `f.Close`) made explicit `_ = ...` since a failed read close is benign.
- **`internal/doctor/doctor_test.go`** — added `mustMkdirAll`, `mustWriteFile`, `mustChtimes` helpers; routed every previously unchecked call through them.
- **`internal/target/chrome_test.go`** — added `mustWrite`, `mustMkdir` helpers; replaced unchecked calls in `TestChrome_PackageArtifact`.
- **`internal/inspector/inspector_test.go`** — added `mkdir` helper; replaced unchecked `os.MkdirAll` calls in `TestInspect_DetectsEntrypoints_FromConvention`.
- **`internal/fsmodel/fsmodel_test.go`**, **`internal/policy/policy_test.go`**, **`internal/graph/builder_test.go`** — single-call sites converted to inline `if err := …; err != nil { t.Fatal(err) }`.

## Risk and Mitigation

- **Risk:** `createZip`'s named return changes its error-flow semantics. If the body returns a non-nil error AND a deferred close also fails, only the body error is reported (deferred close errors are skipped when an error is already pending).
- **Mitigation:** This is the documented Go pattern and matches user expectations (root cause first). Verified by re-running `go test -race ./internal/target/...`, including `TestChrome_PackageArtifact` which exercises the full zip path.

## Verification

- `make fmt` — clean
- `make lint` — clean (was 14 errcheck violations, now 0)
- `go test -race -count=1 ./internal/... ./cmd/panex/...` — 24/24 pass

## Next Step

Phase 2 planning. The Phase 1 audit (PR #138) was the last correctness sweep; Phase 1 is now lint-clean as well as test-clean.
