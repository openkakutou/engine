---
status: todo
depends_on: []
---
# WASM Entrypoint Release Pipeline

## Description
`cmd/wasm` (the `OpenKakutouEngine` WASM entrypoint) exists and is documented in `.vibe/modules/wasm.md`, but this repo has no release workflow that actually builds and publishes `engine.wasm` + `wasm_exec.js` as GitHub Release assets — unlike the sibling `character`/`stage` repos, which both publish these on every tagged release (see their own `.vibe/backlog/done/033-wasm-entrypoint-and-release-pipeline.md` / `006-wasm-entrypoint-and-release-pipeline.md`). Found while implementing `mode-quick-versus` backlog item 005 ("Match Rendering"): `engine`'s latest tag (`v2.1.0`) has zero release assets, so `mode-quick-versus`'s `scripts/download-wasm.mjs` cannot fetch a real `engine.wasm` build the way it already does for `character`/`stage` — a local build from a sibling checkout was used as a stand-in.

## Acceptance Criteria
- [ ] A workflow builds and tests the module normally first (`go build ./...`, `go test ./...`, `go vet ./...`) on every pushed tag matching `v[0-9]*.[0-9]*.[0-9]*` — nothing is published if any of those fail
- [ ] The same workflow then builds `engine.wasm` (`GOOS=js GOARCH=wasm`) and copies the matching `wasm_exec.js` from the same Go toolchain install that produced it
- [ ] Both files are published as assets on that tag's GitHub Release under stable, predictable names (`engine.wasm`, `wasm_exec.js`), mirroring `character`'s/`stage`'s exact asset-naming convention
- [ ] `docs/wasm.md` documents the release pipeline the same way `character`'s/`stage`'s own `docs/wasm.md` do

## Notes
Once published, `mode-quick-versus` can drop its local-build workaround and fetch a real release via its own `wasm:download:engine` script target (already added in that repo's item 005 work) the same way it already does for `character`/`stage`.
