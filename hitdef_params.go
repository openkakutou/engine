package engine

import (
	"maps"
	"strconv"
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

// hitDefNumbers are the HitDef parameters whose values are numbers, which
// Ikemen GO evaluates as expressions when the HitDef runs (hitDef.runSub,
// bytecode.go:8631-8650). Words (attr, hitflag, animtype, priority...) and
// the prefixed sparkno and hitsound are read elsewhere.
var hitDefNumbers = map[string]bool{
	"damage": true, "pausetime": true, "guard.pausetime": true,
	"ground.velocity": true, "air.velocity": true, "guard.velocity": true, "airguard.velocity": true, "down.velocity": true,
	"ground.hittime": true, "ground.slidetime": true, "air.hittime": true, "down.hittime": true,
	"guard.hittime": true, "guard.slidetime": true, "guard.ctrltime": true, "airguard.ctrltime": true,
	"yaccel": true, "fall": true, "air.fall": true, "down.bounce": true,
	"fall.yvelocity": true, "fall.xvelocity": true, "fall.recover": true, "fall.recovertime": true,
	"fall.damage": true, "fall.kill": true, "down.recovertime": true,
	"fall.envshake.time": true, "fall.envshake.freq": true, "fall.envshake.ampl": true, "fall.envshake.phase": true,
	"envshake.time": true, "envshake.freq": true, "envshake.ampl": true, "envshake.phase": true,
	"id": true, "chainid": true, "nochainid": true, "guard.dist": true,
	"givepower": true, "getpower": true, "p1stateno": true, "p2stateno": true,
	"kill": true, "guard.kill": true, "numhits": true, "hitonce": true, "air.juggle": true,
	"p1sprpriority": true, "p2sprpriority": true, "p1facing": true, "p2facing": true,
	"p1getp2facing": true, "p2getp1facing": true, "forcestand": true, "forcenofall": true,
	"ground.cornerpush.veloff": true, "air.cornerpush.veloff": true, "down.cornerpush.veloff": true,
	"guard.cornerpush.veloff": true, "airguard.cornerpush.veloff": true,
	"mindist": true, "maxdist": true, "snap": true,
}

// resolveHitDef is hitDef with its number parameters evaluated in the
// fighter's context, as when it runs (item 065): each value becomes the
// literal list the combat package reads. A HitDef written in literals is
// returned as is, allocating nothing; a part that does not evaluate is kept
// as written (its default then applies).
func resolveHitDef(hitDef cns.Controller, ctx *evaluator.Context) cns.Controller {
	resolved := hitDef.Parameters
	cloned := false
	for key, raw := range hitDef.Parameters {
		if !hitDefNumbers[key] {
			continue
		}
		parts := evaluator.SplitList(raw)
		if allLiterals(parts) {
			continue
		}
		values := make([]string, len(parts))
		for i, part := range parts {
			values[i] = part
			if isLiteral(part) {
				continue
			}
			v, err := evaluator.EvaluateMut(part, ctx)
			if err != nil {
				continue
			}
			values[i] = strconv.FormatFloat(v.Number, 'f', -1, 64)
		}
		if !cloned {
			resolved, cloned = maps.Clone(hitDef.Parameters), true
		}
		resolved[key] = strings.Join(values, ",")
	}
	hitDef.Parameters = resolved
	return hitDef
}

func allLiterals(parts []string) bool {
	for _, p := range parts {
		if !isLiteral(p) {
			return false
		}
	}
	return true
}

// isLiteral reports an empty part or a plain number.
func isLiteral(part string) bool {
	if part == "" {
		return true
	}
	_, err := strconv.ParseFloat(part, 64)
	return err == nil
}
