---
date: 2026-09-22
status: accepted
---
# Release workflow mirrors `stage`'s fail-loud `wasm_exec.js` copy, not `character`'s bare `cp`

**Context:** Adding `.github/workflows/release.yml` to build and publish
`engine.wasm` + `wasm_exec.js` as GitHub Release assets on every tagged
release, following the precedent already set by the sibling `character`
and `stage` repos' own release workflows.

**Decision:** The `wasm_exec.js` copy step checks the source path exists
(`test -f "$src" || { echo ... >&2; exit 1; }`) before copying, failing the
whole workflow loudly if it doesn't — `stage`'s pattern — rather than a
bare `cp` that would fail with a less diagnostic shell error, or worse,
silently produce a missing/stale asset if `cp`'s own error were ever
swallowed — `character`'s pattern.

**Reason:** `engine` is the third repo in the org to add this workflow.
`stage`'s hardened version already exists as the improved precedent;
carrying it forward avoids reintroducing a known-weaker pattern `character`
predates the fix for.

**Rejected alternatives:** Mirroring `character`'s bare `cp` for the
closest byte-for-byte parity across all three repos — rejected because
parity with the older, weaker precedent has no value once a better one
exists in the same org.
