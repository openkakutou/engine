package statemachine

import (
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("Explod", applyExplod)
	RegisterController("RemoveExplod", applyRemoveExplod)
	RegisterController("ModifyExplod", applyModifyExplod)
	RegisterController("Trans", applyTrans)
}

// applyExplod is the Explod controller (bytecode.go:6773-7074, defaults
// char.go:1885-1930): it records an explod for the engine to make. anim
// defaults to 0, F names a common fightfx animation; postype p1, bindtime 1,
// removetime -2 (until the animation ends), facing and vfacing 1. A MUGEN
// 1.1 character's postype defaults to none.
// shortcut: space, random, the interpolation, palfx,
// afterimage, angle and the Ikemen GO extensions are not read.
func applyExplod(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	e := evaluator.ExplodSpawn{PosType: "p1", Facing: 1, VFacing: 1, BindTime: 1, RemoveTime: -2, Scale: [2]float64{1, 1}}
	if ctx.MugenVersion == [2]int{1, 1} && !ctx.IkemenChar {
		e.PosType = ""
	}
	if err := readExplod(ctrl, &ctx, &e); err != nil {
		return ctx, err
	}
	e.ID = max(e.ID, 0)
	ctx.NewExplods = append(ctx.NewExplods, e)
	return ctx, nil
}

// transAlpha is each trans type's default source and destination alpha
// (paramTrans, compiler.go:6874-6904).
var transAlpha = map[string][2]int{
	"default": {255, 0}, "none": {255, 0}, "add": {255, 255}, "add1": {255, 128},
	"addalpha": {255, 0}, "sub": {255, 255}, "subadd": {255, 255},
}

// readTrans reads trans and alpha (paramTrans): the type, its default alpha
// unless alpha gives one; a blank or unknown type, or alpha without trans,
// is ignored as Ikemen GO does in a CNS file.
func readTrans(ctrl cns.Controller, ctx *evaluator.Context, e *evaluator.ExplodSpawn) error {
	trans, alpha, ok, err := transOf(ctrl, ctx)
	if err != nil || !ok {
		return err
	}
	e.Trans, e.Alpha = trans, alpha
	return nil
}

// transOf is ctrl's trans type and alpha; ok is false without a known type.
func transOf(ctrl cns.Controller, ctx *evaluator.Context) (string, [2]int, bool, error) {
	raw, ok := ctrl.Parameters["trans"]
	trans := strings.ToLower(strings.TrimSpace(raw))
	alpha, known := transAlpha[trans]
	if !ok || !known {
		return "", [2]int{}, false, nil
	}
	if a, ok := ctrl.Parameters["alpha"]; ok {
		for i, part := range evaluator.SplitList(a) {
			if i > 1 || part == "" {
				break
			}
			v, err := evalPart(ctrl, "alpha", part, ctx)
			if err != nil {
				return "", [2]int{}, false, err
			}
			alpha[i] = min(max(v.Int(), 0), 255)
		}
	}
	return trans, alpha, true, nil
}

// readExplod reads into e the Explod parameters ctrl writes, leaving the
// others as they are.
func readExplod(ctrl cns.Controller, ctx *evaluator.Context, e *evaluator.ExplodSpawn) error {
	if err := readTrans(ctrl, ctx, e); err != nil {
		return err
	}
	if raw, ok := ctrl.Parameters["anim"]; ok {
		raw = strings.TrimSpace(raw)
		if len(raw) > 1 && (raw[0] == 'F' || raw[0] == 'f') {
			e.Common, raw = true, raw[1:]
		}
		if len(raw) > 1 && (raw[0] == 'S' || raw[0] == 's') {
			raw = raw[1:]
		}
		v, err := evaluator.EvaluateMut(raw, ctx)
		if err != nil {
			return err
		}
		e.Anim = v.Int()
	}
	ints := []struct {
		key string
		dst *int
	}{{"id", &e.ID}, {"bindtime", &e.BindTime}, {"removetime", &e.RemoveTime}, {"sprpriority", &e.SprPriority}}
	for _, p := range ints {
		v, ok, err := evalOptional(ctrl, p.key, ctx)
		if err != nil {
			return err
		}
		if ok {
			*p.dst = int(v)
		}
	}
	for _, key := range [...]string{"supermovetime", "pausemovetime"} {
		v, ok, err := evalOptional(ctrl, key, ctx)
		if err != nil {
			return err
		}
		if ok && v != 0 {
			e.MovesInPause = true
		}
	}
	flags := []struct {
		key string
		dst *int
	}{{"facing", &e.Facing}, {"vfacing", &e.VFacing}}
	for _, p := range flags {
		v, ok, err := evalOptional(ctrl, p.key, ctx)
		if err != nil {
			return err
		}
		if ok && v < 0 {
			*p.dst = -1
		}
	}
	bools := []struct {
		key string
		dst *bool
	}{{"ontop", &e.OnTop}, {"removeongethit", &e.RemoveOnGetHit}, {"supermove", &e.MovesInPause}}
	for _, p := range bools {
		v, ok, err := evalOptional(ctrl, p.key, ctx)
		if err != nil {
			return err
		}
		if ok {
			*p.dst = v != 0
		}
	}
	if raw, ok := ctrl.Parameters["postype"]; ok {
		e.PosType = strings.ToLower(strings.TrimSpace(raw))
		if e.PosType == "none" {
			e.PosType = ""
		}
	}
	pairs := []struct {
		key string
		dst *[2]float64
	}{{"pos", &e.Pos}, {"vel", &e.Vel}, {"accel", &e.Accel}, {"scale", &e.Scale}}
	for _, p := range pairs {
		raw, ok := ctrl.Parameters[p.key]
		// vel, else velocity (compiler_functions.go:894-906).
		if !ok && p.key == "vel" {
			raw, ok = ctrl.Parameters["velocity"]
		}
		if !ok {
			continue
		}
		for i, part := range evaluator.SplitList(raw) {
			if i > 1 || part == "" {
				break
			}
			v, err := evalPart(ctrl, p.key, part, ctx)
			if err != nil {
				return err
			}
			p.dst[i] = v.Float()
		}
	}
	return nil
}

// applyRemoveExplod is RemoveExplod (bytecode.go:12321-12340): it removes
// the fighter's explods with that id, all without one.
// shortcut: the index parameter is not read.
func applyRemoveExplod(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	id := -1
	v, ok, err := evalOptional(ctrl, "id", &ctx)
	if err != nil {
		return ctx, err
	}
	if ok {
		id = int(v)
	}
	ctx.RemovedExplods = append(ctx.RemovedExplods, evaluator.ExplodRemoval{ID: id})
	return ctx, nil
}

// applyModifyExplod is ModifyExplod (bytecode.go:7158-7560): the fighter's
// explods with that id (all without one) take the parameters it writes; a
// MUGEN character's pos, vel, accel, facing and vfacing need a postype too;
// an Ikemen GO character's positive bindtime and removetime count from the
// explod's age.
// shortcut: index, redirectid and the parameters Explod does not read are
// ignored.
func applyModifyExplod(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	m := evaluator.ExplodModification{ID: -1, Relative: ctx.IkemenChar}
	m.Facing, m.VFacing, m.Scale = 1, 1, [2]float64{1, 1}
	if err := readExplod(ctrl, &ctx, &m.ExplodSpawn); err != nil {
		return ctx, err
	}
	m.ID = -1
	id, ok, err := evalOptional(ctrl, "id", &ctx)
	if err != nil {
		return ctx, err
	}
	if ok {
		m.ID = int(id)
	}
	has := func(key string) bool {
		_, ok := ctrl.Parameters[key]
		return ok
	}
	// A MUGEN character changes these only along with a postype
	// (bytecode.go:7172-7176).
	m.SetPosType = has("postype")
	unlocked := ctx.IkemenChar || m.SetPosType
	m.SetPos = unlocked && has("pos")
	m.SetVel = unlocked && (has("vel") || has("velocity"))
	m.SetAccel = unlocked && has("accel")
	m.SetFacing = unlocked && has("facing")
	m.SetVFacing = unlocked && has("vfacing")
	m.SetBindTime, m.SetRemoveTime, m.SetSprPriority = has("bindtime"), has("removetime"), has("sprpriority")
	m.SetOnTop, m.SetRemoveOnGetHit, m.SetAnim = has("ontop"), has("removeongethit"), has("anim")
	m.SetScale = has("scale")
	m.SetTrans = m.Trans != ""
	m.SetMovesInPause = has("supermove") || has("supermovetime") || has("pausemovetime")
	ctx.ModifiedExplods = append(ctx.ModifiedExplods, m)
	return ctx, nil
}

// applyTrans is the Trans controller (bytecode.go trans.Run): the fighter
// is drawn with that trans type and alpha this tick.
func applyTrans(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	trans, alpha, ok, err := transOf(ctrl, &ctx)
	if err != nil || !ok {
		return ctx, err
	}
	ctx.Trans, ctx.TransAlpha = trans, alpha
	return ctx, nil
}
