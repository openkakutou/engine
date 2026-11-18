---
status: todo
---
# Module path `/v2` and release propagation to consumers

## Description
The engine is tagged `v2.x.y`, but `go.mod` declares `module github.com/openkakutou/engine` without the `/v2` suffix Go's semantic import versioning requires for major version 2 and above. `go get github.com/openkakutou/engine@v2.10.0` therefore fails ("module path must match major version"), and the only Go consumer, `game`'s native module (`game/native/go.mod`), has to pin a pseudo-version of a commit instead of a tag. Engine releases 2.7.0 to 2.9.0 never reached `game`, partly because of this: the bump to v2.10.0 (2026-11-18) had to resolve the tag's commit by hand.

Fix the module path and set up release propagation as `character`, `sff` and `fnt` do (roadmap decisions `016` and `025`):
- Change the module path to `github.com/openkakutou/engine/v2` and every internal import (`.../engine/v2/match`, `.../engine/v2/statemachine`, ...) in one release, the next one (a v2.x tag whose `go.mod` carries `/v2`). Check that the WASM build, `cmd/conformance` and the release workflow still work.
- Move `game/native` to `github.com/openkakutou/engine/v2@<that tag>`, updating its imports, and check that its tests and smoke run pass.
- Add a "Release propagation" section to this repo's `CLAUDE.md`, marked `<!-- keep -->`, listing the consumers and how to bump each one: the web WASM pin in `game/.github/workflows/deploy-pages.yml` plus its README example, and the Go module in `game/native/go.mod` through its Docker scripts. Then run the consumer's tests, commit and push, or stop and flag on red.

## Acceptance Criteria
- [ ] `go get github.com/openkakutou/engine/v2@vX.Y.Z` works for a tagged release
- [ ] `game/native` depends on the engine through a tag, not a commit pseudo-version, with green tests and smoke run
- [ ] `CLAUDE.md` documents the propagation steps for every consumer, so the next release reaches `game` in the same session

## Notes
Raised while bumping `game` to engine v2.10.0 (2026-11-18). This is a breaking change for import paths only, so the release that carries it says so in its changelog.
