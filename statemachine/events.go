package statemachine

import (
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

// eventControllers are reported as events with their parameters, for the
// game to draw (backlog item 052); the full Explod and helper lifecycle is
// item 029.
var eventControllers = [...]string{"PalFX", "RemapPal", "MakeDust", "GameMakeAnim", "ForceFeedback", "EnvShake"}

func init() {
	for _, t := range eventControllers {
		RegisterController(t, func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
			ctx.Events = append(ctx.Events, controllerEvent(t, ctrl, &ctx))
			return ctx, nil
		})
	}
	RegisterController("SprPriority", applySprPriority)
	RegisterController("Width", applyWidth)
	RegisterController("FallEnvShake", applyFallEnvShake)
}

// controllerEvent records ctrl's parameters: each comma-separated part is
// evaluated; one that does not evaluate (an F-prefixed animation, a word)
// leaves only the text.
func controllerEvent(typ string, ctrl cns.Controller, ctx *evaluator.Context) evaluator.ControllerEvent {
	e := evaluator.ControllerEvent{Type: typ}
	for name, raw := range ctrl.Parameters {
		p := evaluator.EventParam{Name: name, Raw: strings.TrimSpace(raw)}
		for _, part := range evaluator.SplitList(raw) {
			v, err := evaluator.EvaluateMut(strings.TrimSpace(part), ctx)
			if err != nil {
				p.Values = nil
				break
			}
			p.Values = append(p.Values, v.Float())
		}
		e.Params = append(e.Params, p)
	}
	return e
}

// applySprPriority is sprPriority.Run: value (0) is the draw priority.
func applySprPriority(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	v, _, err := evalOptional(ctrl, "value", &ctx)
	if err != nil {
		return ctx, err
	}
	ctx.SprPriority = int(v)
	return ctx, nil
}

// applyWidth is width.Run's edge: the front and back widths against the
// screen edges, for this tick.
// shortcut: the player width (value, player) that changes the push box is
// not applied; upgrade with the push.
func applyWidth(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	raw, ok := ctrl.Parameters["edge"]
	if !ok {
		return ctx, nil
	}
	for i, part := range evaluator.SplitList(raw) {
		if i > 1 {
			break
		}
		v, err := evaluator.EvaluateMut(part, &ctx)
		if err != nil {
			return ctx, err
		}
		ctx.WidthEdge[i] = v.Float()
	}
	return ctx, nil
}

// applyFallEnvShake is fallEnvShake.Run (bytecode.go): the HitDef's
// fall.envshake, when it has a time, is reported as an EnvShake event, then
// spent so it does not shake again.
func applyFallEnvShake(_ cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	s := ctx.GetHit.FallEnvShake
	if s.Time <= 0 {
		return ctx, nil
	}
	ctx.Events = append(ctx.Events, evaluator.ControllerEvent{Type: "EnvShake", Params: []evaluator.EventParam{
		{Name: "time", Values: []float64{float64(s.Time)}},
		{Name: "freq", Values: []float64{s.Freq}},
		{Name: "ampl", Values: []float64{s.Ampl}},
		{Name: "phase", Values: []float64{s.Phase}},
	}})
	ctx.GetHit.FallEnvShake.Time = 0
	return ctx, nil
}
