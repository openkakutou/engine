# engine

A Go library implementing the combat simulation for OpenKakutou fighting games — state execution, physics, hit detection, damage, and round flow — built as part of the [OpenKakutou](https://github.com/openkakutou) project — an open-source alternative to Fighter Factory Studio / Ikemen GO.

<!-- vibe:begin:features -->
This project is early-stage. Available now:

- Match/combat state model — each fighter's position, facing, movement, and current state, plus the round number and round timer, as the live state a match is played out on

Planned:

- Trigger/expression evaluator for MUGEN CNS syntax — comparisons, arithmetic, built-in functions (`IfElse`, `var()`, etc.) evaluated against live match state
- State machine execution — StateDef/Controller interpretation driving state transitions, built on the evaluator
- `.zss` script execution — Ikemen GO's Lua-like state scripts
- Physics and movement — velocity, gravity, ground/air state, stage-boundary clamping
- Hit detection — Clsn (collision box) hit/hurt box resolution
- Damage/health and combo system — hit results applied as damage, health, and combo-count state
- Input reading and command matching — raw input matched against `.cmd` command definitions
- Round/match flow, WASM entrypoint, integration tests — win conditions (KO, timeout), round reset, match-level flow, WASM build, fixture-driven integration tests

**Scope boundary:** `engine` is combat simulation only — the simulation that runs while two characters fight. It does not cover menus, character selection, or overall game flow; those are the responsibility of `mode-*` game apps (starting with `mode-quick-versus`), which consume `engine` rather than the other way around. See `github.com/openkakutou/roadmap`'s `.vibe/decisions/004` and `.vibe/decisions/008`.
<!-- vibe:end:features -->

<!-- vibe:begin:install -->
Requires [Go](https://go.dev/) 1.26 or later.

```sh
go get github.com/openkakutou/engine
```

Verify the install by importing the module in a Go file and running `go build`:

```go
import "github.com/openkakutou/engine"
```

To update to the latest version:

```sh
go get -u github.com/openkakutou/engine
```
<!-- vibe:end:install -->

<!-- vibe:begin:docs-index -->
- [docs/architecture.md](docs/architecture.md) — how the root package and `engine/match` fit together, and the data flow expected as later packages land
<!-- vibe:end:docs-index -->

<!-- vibe:begin:usage -->
Build a match's live state from the `match` package:

```go
package main

import (
	"fmt"

	"github.com/openkakutou/engine/match"
)

func main() {
	p1 := match.FighterState{Side: match.SideP1, Position: match.Position{X: -50}, Facing: match.FacingRight, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 50}, Facing: match.FacingLeft, Health: 1000}

	ms, err := match.NewMatchState(1, 5940, p1, p2)
	if err != nil {
		panic(err)
	}

	fmt.Println(ms.Fighter(match.SideP1).Health) // 1000
}
```

Usage examples will grow here as the trigger evaluator, state machine execution, and the rest of the backlog are implemented.
<!-- vibe:end:usage -->
