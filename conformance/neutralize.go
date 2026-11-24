package conformance

import (
	"sort"
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/statemachine"
)

// neverTrue is the trigger a neutralized controller gets: it never passes.
const neverTrue = "0"

// evaluatedParameters are the controller parameters the engine evaluates as
// expressions for the supported controller types: statemachine's handlers
// read "value" and "v" (ChangeState, VarSet, PowerAdd, PlaySnd, ChangeAnim,
// CtrlSet, LifeAdd, LifeSet), "x" and "y" (VelSet, VelAdd, VelMul, PosSet,
// PosAdd), and "elem", "elemtime" and "kill" (ChangeAnim, LifeAdd). Other
// parameters of a supported type (e.g. StateTypeSet's statetype, a letter)
// are not expressions.
var evaluatedParameters = []string{"value", "v", "x", "y", "elem", "elemtime", "kill"}

// attributeValued reports a controller whose value is an attribute list,
// not an expression: HitBy and NotHitBy (statemachine applyHitBy, Ikemen GO
// hitBySub), as in common1.cns's NotHitBy value = SCA.
func attributeValued(ctrlType string) bool {
	return strings.EqualFold(ctrlType, "HitBy") || strings.EqualFold(ctrlType, "NotHitBy")
}

// inspectController lists the unknown trigger names the controller's
// triggers (and, for a supported type, its evaluated parameters) mention,
// and whether one of its triggers fails to parse. Names of a call are
// suffixed "()".
func inspectController(ctrl cns.Controller) (unknown []string, unparsable bool) {
	found := map[string]bool{}
	collect := func(expr string) error {
		names, err := evaluator.TriggerNames(expr)
		if err != nil {
			return err
		}
		for _, n := range names {
			if evaluator.IsKnownTrigger(n.Name, n.Call) {
				continue
			}
			if n.Call {
				found[n.Name+"()"] = true
			} else {
				found[n.Name] = true
			}
		}
		return nil
	}
	for _, trigger := range ctrl.Triggers {
		if err := collect(trigger); err != nil {
			unparsable = true
		}
	}
	if statemachine.IsSupportedController(ctrl.Type) {
		for _, key := range evaluatedParameters {
			value, ok := ctrl.Parameters[key]
			if !ok {
				continue
			}
			if key == "value" && attributeValued(ctrl.Type) {
				continue
			}
			// PlaySnd's "group,sample" pair is split as statemachine does and
			// each half inspected. shortcut: a parameter that does not parse
			// is ignored, not counted unparsable; upgrade if it matters.
			if strings.EqualFold(ctrl.Type, "PlaySnd") && key == "value" {
				halves, err := statemachine.SplitPlaySndValue(value)
				if err != nil {
					continue
				}
				_ = collect(halves[0])
				_ = collect(halves[1])
				continue
			}
			_ = collect(value)
		}
	}
	unknown = make([]string, 0, len(found))
	for name := range found {
		unknown = append(unknown, name)
	}
	sort.Strings(unknown)
	return unknown, unparsable
}

// neutralize returns a copy of states in which every controller that
// mentions an unknown trigger, or whose trigger fails to parse, can never
// fire (its triggers are replaced by a never-true expression), so the run
// goes on instead of failing on the first unsupported trigger. It also
// returns the number of controllers neutralized for an unparsable trigger.
// The given table is not modified.
func neutralize(states map[int]cns.StateDef) (map[int]cns.StateDef, int) {
	out := make(map[int]cns.StateDef, len(states))
	unparsableCount := 0
	for number, def := range states {
		controllers := make([]cns.Controller, len(def.Controllers))
		copy(controllers, def.Controllers)
		for i, ctrl := range controllers {
			unknown, unparsable := inspectController(ctrl)
			if unparsable {
				unparsableCount++
			}
			if unparsable || len(unknown) > 0 {
				controllers[i].Triggers = []string{neverTrue}
				controllers[i].TriggerAll = nil
				controllers[i].TriggerGroups = [][]string{{neverTrue}}
			}
		}
		def.Controllers = controllers
		out[number] = def
	}
	return out, unparsableCount
}

// stateExposure is what a fighter standing in a state is exposed to each
// tick: the unsupported controller types and unknown trigger names its
// controllers use (each once per state), and its controller counts.
type stateExposure struct {
	unsupportedTypes []string
	unknownTriggers  []string
	controllers      int
	supported        int
}

// engineHandles reports whether the engine acts on a controller type: the
// statemachine's own types plus HitDef, which Tick handles itself.
func engineHandles(controllerType string) bool {
	if strings.EqualFold(controllerType, statemachine.ControllerTypeHitDef) {
		return true
	}
	return statemachine.IsSupportedController(controllerType)
}

// exposureOf computes the exposure of one state.
func exposureOf(def cns.StateDef) stateExposure {
	// Controller types are matched case-insensitively (as statemachine does),
	// so they are reported lower-cased: one entry per type across characters.
	types := map[string]bool{}
	triggers := map[string]bool{}
	e := stateExposure{controllers: len(def.Controllers)}
	for _, ctrl := range def.Controllers {
		if engineHandles(ctrl.Type) {
			e.supported++
		} else {
			types[strings.ToLower(ctrl.Type)] = true
		}
		unknown, _ := inspectController(ctrl)
		for _, name := range unknown {
			triggers[name] = true
		}
	}
	for name := range types {
		e.unsupportedTypes = append(e.unsupportedTypes, name)
	}
	for name := range triggers {
		e.unknownTriggers = append(e.unknownTriggers, name)
	}
	sort.Strings(e.unsupportedTypes)
	sort.Strings(e.unknownTriggers)
	return e
}
