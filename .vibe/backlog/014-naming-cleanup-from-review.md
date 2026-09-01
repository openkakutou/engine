---
status: in_progress
---
# Naming Cleanup From Review

## Description
The naming review agent flagged several low-severity naming issues: `fs`/`p1Fs`/`p2Fs` in `tick.go` read as filesystem abbreviations rather than "fighter"; `argString`/`envelope` in `cmd/wasm/main.go` are noun-named despite performing an action; and two inconsistent abbreviations in the `wasm-lang-spike` benchmark (`hbIdx` vs `hurtIdx`, `v0`/`v1`).

## Acceptance Criteria
- [ ] `tick.go`'s `fs`, `p1Fs`, `p2Fs` are renamed to `fighter`, `p1Fighter`, `p2Fighter`
- [ ] `cmd/wasm/main.go`'s `argString` and `envelope` are renamed to verb-based names (e.g. `extractArgString`, `buildEnvelope`)
- [ ] `benchmarks/wasm-lang-spike/go/main.go`'s `hbIdx` is renamed to `hitboxIdx`, and `v0`/`v1` to intent-revealing names
- [ ] All existing tests pass unchanged (pure rename, no behavior change)

## Notes
All Low-severity findings from `/vibe:review`'s naming agent (2026-08-31, commit `019027a`), not auto-fixed per the review skill's own rule (Low findings are reported, not applied).
