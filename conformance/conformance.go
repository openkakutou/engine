// Package conformance runs real characters through the engine and reports
// which controller types and triggers they use that the engine does not
// support yet (backlog item 030). See docs/conformance.md for the method.
package conformance

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	engine "github.com/openkakutou/engine"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/round"
	"github.com/openkakutou/stage"
)

// DefaultTicks is the tick count the command uses when none is given.
const DefaultTicks = 3600

// Mirror-match constants. Both fighters start in state 0 facing each other,
// 140 units apart, inside a stage whose boundaries are +-500; they start
// with 1000 health, the round timer is 99 seconds of 60 ticks, gravity is
// MUGEN's default yaccel (0.44). When a round ends (KO or timeout) the round
// is reset the way integration_test.go does, so the run always reaches the
// requested tick count unless Tick fails.
const (
	startX       = 70
	stageBound   = 500
	startHealth  = 1000
	roundTimer   = 99 * 60
	gravity      = 0.44
	comboWindow  = 60
	notCharacter = "doesn't look like a character definition"
)

// Options configures Run.
type Options struct {
	// Ticks is the number of ticks to simulate per character.
	Ticks int
	// SearchDirs are the extra folders a character's state files are looked
	// for in, after its .def's folder (engine.LoadFighterProgram), such as
	// the game's data/ folder holding the common state file.
	SearchDirs []string
}

// Run runs every character found under paths, each a character .def file or
// a folder searched recursively for .def files, and returns the report. It
// returns an error only when a path cannot be read; a character that fails
// is recorded in the report and the run continues with the next one.
func Run(paths []string, opts Options) (Report, error) {
	report := Report{Ticks: opts.Ticks, Characters: []CharacterReport{}}
	var unsupported, unknown [][]Count
	for _, arg := range paths {
		found, err := discover(arg)
		if err != nil {
			return Report{}, err
		}
		for _, f := range found {
			c := runCharacter(f.full, f.display, opts)
			report.Characters = append(report.Characters, c)
			unsupported = append(unsupported, c.UnsupportedControllers)
			unknown = append(unknown, c.UnknownTriggers)
		}
	}
	report.UnsupportedControllers = aggregate(unsupported)
	report.UnknownTriggers = aggregate(unknown)
	return report, nil
}

type defFile struct {
	full    string // path to open
	display string // path shown in the report, relative to the argument
}

// discover lists the .def files an argument designates: the file itself
// (shown by its base name) or every .def under a folder in lexical order
// (shown relative to the folder).
func discover(arg string) ([]defFile, error) {
	info, err := os.Stat(arg)
	if err != nil {
		return nil, fmt.Errorf("conformance: %w", err)
	}
	if !info.IsDir() {
		return []defFile{{full: arg, display: filepath.Base(arg)}}, nil
	}
	var found []defFile
	err = filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".def") {
			return nil
		}
		rel, err := filepath.Rel(arg, path)
		if err != nil {
			return err
		}
		found = append(found, defFile{full: path, display: rel})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("conformance: %w", err)
	}
	return found, nil
}

func runCharacter(defPath, display string, opts Options) CharacterReport {
	report := CharacterReport{
		Path:                   display,
		UnsupportedControllers: []Count{},
		UnknownTriggers:        []Count{},
	}
	ch, err := character.Load(defPath)
	if err != nil {
		// shortcut: character.Load has no typed error for "not a
		// character", so match its message; upgrade when it exports one.
		if strings.Contains(err.Error(), notCharacter) {
			report.Status = StatusSkipped
			report.Error = "not a character definition (no [Files] section)"
			return report
		}
		report.Status = StatusLoadError
		report.Error = err.Error()
		return report
	}
	for _, script := range ch.Scripts {
		for _, block := range script.Script.Blocks {
			if block.Kind == zss.BlockKindStatedef {
				report.ZssStatedefBlocks++
			}
		}
	}
	prog, _, err := engine.LoadFighterProgram(ch, defPath, opts.SearchDirs...)
	if err != nil {
		report.Status = StatusLoadError
		report.Error = err.Error()
		return report
	}
	if _, err := prog.NewRuntime(match.FighterState{}); err != nil {
		report.Status = StatusLoadError
		report.Error = "no state 0 in the character's state files"
		return report
	}
	simulate(&report, ch, prog, opts.Ticks)
	return report
}

// safeTick calls engine.Tick and turns an engine panic into an error, so one
// character crashing the engine is recorded and the run moves on to the next.
func safeTick(state match.MatchState, programs [2]engine.FighterProgram, runtimes [2]engine.FighterRuntime, inputs [2]input.TickInput, cfg engine.TickConfig) (result engine.TickResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("engine panic: %v", r)
		}
	}()
	return engine.Tick(state, programs, runtimes, inputs, cfg)
}

// simulate runs the mirror match and fills the report's run figures.
func simulate(report *CharacterReport, ch *character.Character, prog engine.FighterProgram, ticks int) {
	table := prog.States
	neutral, unparsable := neutralize(table)
	report.ControllersUnparsable = unparsable
	exposures := make(map[int]stateExposure, len(table))
	for number, def := range table {
		exposures[number] = exposureOf(def)
	}
	// The .zss states count as states and as reached, but are neither
	// neutralized nor measured for exposure: an element the engine rejects
	// stops the run as a tick error naming the block (decision 024).
	zssStates := zssStateNumbers(ch, table)
	report.StatesTotal = len(table) + len(zssStates)

	prog.States = neutral
	programs := [2]engine.FighterProgram{match.SideP1: prog, match.SideP2: prog}
	bounds := &stage.StageBoundaries{Left: -stageBound, Right: stageBound}
	p1Start := match.FighterState{Side: match.SideP1, Position: match.Position{X: -startX}, Facing: match.FacingRight, Health: startHealth}
	p2Start := match.FighterState{Side: match.SideP2, Position: match.Position{X: startX}, Facing: match.FacingLeft, Health: startHealth}

	state, runtimes, err := startRound(1, p1Start, p2Start, prog)
	if err != nil {
		report.Status = StatusLoadError
		report.Error = err.Error()
		return
	}

	unsupportedCounts := map[string]int{}
	unknownCounts := map[string]int{}
	reached := map[int]bool{}
	report.Status = StatusRan
	for tick := 1; tick <= ticks; tick++ {
		for _, side := range []match.Side{match.SideP1, match.SideP2} {
			number := runtimes[side].Context.StateNo
			e, ok := exposures[number]
			if !ok {
				if zssStates[number] {
					reached[number] = true
				}
				continue
			}
			reached[number] = true
			for _, name := range e.unsupportedTypes {
				unsupportedCounts[name]++
			}
			for _, name := range e.unknownTriggers {
				unknownCounts[name]++
			}
		}
		var inputs [2]input.TickInput
		for _, side := range []match.Side{match.SideP1, match.SideP2} {
			inputs[side] = inputFor(side, tick-1, state.Fighter(side).Facing)
		}
		result, err := safeTick(*state, programs, runtimes, inputs, engine.TickConfig{Bounds: bounds, Gravity: gravity, Tick: tick, ComboWindow: comboWindow})
		if err != nil {
			report.Status = StatusTickError
			report.Error = err.Error()
			break
		}
		report.TicksRun = tick
		*state = result.State
		runtimes = result.Fighters
		if result.Round.Outcome != round.OutcomeNone {
			state, runtimes, err = startRound(state.Round+1, p1Start, p2Start, prog)
			if err != nil {
				report.Status = StatusLoadError
				report.Error = err.Error()
				break
			}
		}
	}

	report.StatesReached = len(reached)
	for number := range reached {
		report.ControllersTotal += exposures[number].controllers
		report.ControllersSupported += exposures[number].supported
	}
	report.UnsupportedControllers = sortedCounts(unsupportedCounts)
	report.UnknownTriggers = sortedCounts(unknownCounts)
}

// startRound builds the match state and both fighters' runtimes for a round.
func startRound(number int, p1Start, p2Start match.FighterState, prog engine.FighterProgram) (*match.MatchState, [2]engine.FighterRuntime, error) {
	state, err := round.ResetRound(number, roundTimer, p1Start, p2Start)
	if err != nil {
		return nil, [2]engine.FighterRuntime{}, err
	}
	var runtimes [2]engine.FighterRuntime
	for side, start := range map[match.Side]match.FighterState{match.SideP1: p1Start, match.SideP2: p2Start} {
		runtimes[side], err = prog.NewRuntime(start)
		if err != nil {
			return nil, [2]engine.FighterRuntime{}, err
		}
	}
	return state, runtimes, nil
}

// zssStateNumbers is the set of state numbers the character's .zss scripts
// define that the .cns table does not already hold (a .cns state that came
// first wins, and NewFighterProgramFromFiles dropped the one it beat).
func zssStateNumbers(ch *character.Character, table map[int]cns.StateDef) map[int]bool {
	numbers := map[int]bool{}
	for _, script := range ch.Scripts {
		for _, block := range script.Script.Blocks {
			if _, inCns := table[block.Number]; block.Kind == zss.BlockKindStatedef && block.NumberRaw == "" && !inCns {
				numbers[block.Number] = true
			}
		}
	}
	return numbers
}
