# engine

A Go library implementing the combat simulation for OpenKakutou fighting games — state execution, physics, hit detection, damage, and round flow — built as part of the [OpenKakutou](https://github.com/openkakutou) project — an open-source alternative to Fighter Factory Studio / Ikemen GO.

<!-- vibe:begin:features -->
This project is early-stage. Available now:

- Match/combat state model — each fighter's position, facing, movement, and current state, plus the round number and round timer, as the live state a match is played out on
- Trigger/expression evaluator for MUGEN CNS syntax — comparisons, boolean/arithmetic operators, and built-in triggers (`Time`, `Ctrl`, `Anim`, `Command`, `var()`, `sysvar()`, `IfElse`, and more to come) evaluated against a fighter's live state, with a clear error instead of a wrong or default result for malformed or unsupported expressions
- State machine execution — drives a fighter through its character's defined states each simulation tick: checks every condition attached to the current state in order, applies the state changes and variable updates for whichever ones are met, and reports a clear error if a state change targets a state that doesn't exist
- Input reading and command matching — recognizes a character's special-move motions (e.g. quarter-circle-forward + punch) from raw per-tick input, within the timing window the character's own command file declares, correctly rejecting a close-but-incomplete motion and forgetting one started too long ago; a recognized motion becomes available to the combat-logic conditions that check for it
- Physics and movement — advances a fighter one simulation tick at a time: gravity pulls it down while airborne, it lands cleanly back on the ground with its fall stopped, and it can never be pushed outside the current stage's boundaries even by a single fast-moving tick
- `.zss` script execution — runs Ikemen GO's Lua-like state scripts the same way classic state definitions are driven: conditions are checked, variables and state changes are applied, and a script can call another one as a helper, all producing the same real results a classic combat-logic file would for the same behavior; an unsupported script construct reports a clear error instead of silently doing nothing
- Hit detection — resolves each fighter's currently active hit boxes against the other's vulnerable boxes every simulation tick, correctly positioned and mirrored for whichever way each fighter is facing; a frame with no collision boxes at all (like most idle frames) simply produces no hits
- Damage, health, and combos — a landed hit subtracts its declared damage from the defender's health, never below zero even on an overkill hit, and builds a combo counter that keeps climbing while hits keep landing close together, resetting once too much time passes between them; a hit with a missing or unreadable damage amount still lands safely instead of crashing the match
- Round/match flow — decides how a round ends (a knockout, both fighters knocked out at once, or the round timer running out with the healthier fighter winning), resets both fighters to a fresh start for the next round, and tracks who has won how many rounds across a full best-of-N match
- WebAssembly build — a game running in a browser can load the engine as a compiled module, start a match from two loaded characters, advance it tick by tick, reset between rounds, and release a finished match's memory when it's done, all without needing a Go toolchain of its own; every match update also reports each fighter's current animation number and how long it's been playing, so the game knows exactly which sprite frame to show

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
- [docs/architecture.md](docs/architecture.md) — how the root package's `Tick`, `engine/match`, `engine/evaluator`, `engine/statemachine`, `engine/zssexec`, `engine/input`, `engine/physics`, `engine/hitdetect`, `engine/combat`, `engine/round`, and the `cmd/wasm` entrypoint fit together, and the resulting data flow
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

Recognize a character's command-file motions from raw per-tick input with the `input` package:

```go
package main

import (
	"fmt"

	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
)

func main() {
	commands := cmd.CommandFile{
		Defaults: cmd.CommandDefaults{Time: 15, BufferTime: 1},
		Commands: []cmd.Command{{Name: "QCF_a", Input: "~D, DF, F, a"}},
	}

	var state input.State
	var active map[string]bool
	ticks := []input.TickInput{
		{Down: true},
		{Down: true, Right: true},
		{Right: true},
		{Buttons: map[string]bool{"a": true}},
	}
	for i, tick := range ticks {
		state, active = input.Step(state, i, match.FacingRight, tick, commands)
	}

	fmt.Println(active["QCF_a"]) // true
}
```

`active` is directly assignable to `evaluator.Context.ActiveCommands`, so a state's `Command = "QCF_a"` trigger resolves correctly on the same tick.

Advance a fighter one simulation tick of physics with the `physics` package:

```go
package main

import (
	"fmt"

	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/physics"
	"github.com/openkakutou/stage"
)

func main() {
	fighter := match.FighterState{
		Position: match.Position{X: 0, Y: 0},
		Velocity: match.Velocity{X: 0, Y: 2},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	next, err := physics.Step(fighter, bounds, 0.5)
	if err != nil {
		panic(err)
	}

	fmt.Println(next.Position.Y) // 1.5 — one tick into the jump
}
```

Run a character's Ikemen GO `.zss` state script with the `zssexec` package:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/zssexec"
)

func main() {
	script, err := zss.Parse(strings.NewReader(`[Statedef 0]
if Command = "holdfwd" {
	changeState{value: 20;}
}

[Statedef 20]`))
	if err != nil {
		panic(err)
	}

	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 0},
		ActiveCommands: map[string]bool{"holdfwd": true},
	}

	result, err := zssexec.Step(ctx, script)
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Context.StateNo) // 20
}
```

Advance a full simulation tick for both fighters and check the round outcome with the root package and the `round` package:

```go
package main

import (
	"fmt"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/round"
	"github.com/openkakutou/stage"
)

func main() {
	states := map[int]cns.StateDef{0: {Number: 0}}
	prog := engine.FighterProgram{States: states}

	p1 := match.FighterState{Side: match.SideP1, Health: 1}
	p2 := match.FighterState{Side: match.SideP2, Health: 1000}
	p1Runtime, _ := engine.NewFighterRuntime(p1, states)
	p2Runtime, _ := engine.NewFighterRuntime(p2, states)

	state, _ := match.NewMatchState(1, 5940, p1, p2)
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	result, err := engine.Tick(
		*state,
		[2]engine.FighterProgram{prog, prog},
		[2]engine.FighterRuntime{p1Runtime, p2Runtime},
		[2]input.TickInput{},
		engine.TickConfig{Bounds: bounds, Gravity: 1.0, Tick: 1, ComboWindow: 60},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Round.Outcome == round.OutcomeNone) // true — 1 health, but nothing hit it yet
}
```

Track match-level progress across a best-of-N with the `round` package:

```go
package main

import (
	"fmt"

	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/round"
)

func main() {
	progress, err := round.NewProgress(3)
	if err != nil {
		panic(err)
	}

	progress = progress.RecordRoundResult(round.RoundResult{Outcome: round.OutcomeKO, Winner: match.SideP1})
	progress = progress.RecordRoundResult(round.RoundResult{Outcome: round.OutcomeKO, Winner: match.SideP1})

	decided, winner := progress.MatchOutcome()
	fmt.Println(decided, winner == match.SideP1) // true true — 2 round wins decide a best-of-3
}
```

A game running in a browser drives a match through the WASM build instead — see `cmd/wasm/main.go` for the full contract; the shape is:

```js
const created = OpenKakutouEngine.newMatch(JSON.stringify({ programs, starting, roundTimer, bestOf, bounds, gravity, comboWindow }));
const { matchId } = JSON.parse(created.data);

const advanced = OpenKakutouEngine.tick(JSON.stringify({ matchId, inputs: [p1Input, p2Input] }));
const { state, round, matchOver, matchWinner, animations } = JSON.parse(advanced.data);
// animations[0]/animations[1] — each fighter's current animation number (animNo) and
// how many ticks it's been playing (animTime), enough to resolve the right sprite frame

// Once the match is over (or the player leaves), release its memory:
OpenKakutouEngine.closeMatch(JSON.stringify({ matchId }));
```
<!-- vibe:end:usage -->
