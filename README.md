# engine

A Go library implementing the combat simulation for OpenKakutou fighting games — state execution, physics, hit detection, damage, and round flow — built as part of the [OpenKakutou](https://github.com/openkakutou) project — an open-source alternative to Fighter Factory Studio / Ikemen GO.

<!-- vibe:begin:features -->
This project is early-stage. Available now:

- Match/combat state model — each fighter's position, facing, movement, and current state, plus the round number and round timer, as the live state a match is played out on
- Trigger/expression evaluator for MUGEN CNS syntax — comparisons, boolean/arithmetic operators, and built-in triggers (`Time`, `Ctrl`, `Anim`, `Command`, `var()`, `sysvar()`, `IfElse`, and more to come) evaluated against a fighter's live state, with a clear error instead of a wrong or default result for malformed or unsupported expressions
- State machine execution — drives a fighter through its character's defined states each simulation tick: checks every condition attached to the current state in order, applies the state changes and variable updates for whichever ones are met, and reports a clear error if a state change targets a state that doesn't exist

Planned:

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
- [docs/architecture.md](docs/architecture.md) — how the root package, `engine/match`, `engine/evaluator`, and `engine/statemachine` fit together, and the data flow expected as later packages land
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

Parse and evaluate a MUGEN CNS trigger expression from the `evaluator` package:

```go
package main

import (
	"fmt"

	"github.com/openkakutou/engine/evaluator"
)

func main() {
	ctx := evaluator.Context{
		Ctrl:           true,
		ActiveCommands: map[string]bool{"holdback": true},
	}

	result, err := evaluator.Evaluate(`Command = "holdback" && Ctrl`, ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Bool()) // true
}
```

Drive a fighter's current state forward one simulation tick with the `statemachine` package:

```go
package main

import (
	"fmt"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/statemachine"
)

func main() {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{
					Type:       "ChangeState",
					Triggers:   []string{`Command = "holdfwd"`},
					Parameters: map[string]string{"value": "20"},
				},
			},
		},
		20: {Number: 20},
	}

	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 0},
		ActiveCommands: map[string]bool{"holdfwd": true},
	}

	result, err := statemachine.Step(ctx, states)
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Context.StateNo) // 20
}
```

Usage examples will grow here as `.zss` script execution and the rest of the backlog are implemented.
<!-- vibe:end:usage -->
