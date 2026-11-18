package conformance

import (
	"reflect"
	"testing"

	"github.com/openkakutou/character/cns"
)

func TestNeutralize_ControllersWithUnknownOrUnparsableTriggers_GetNeverTrueTriggers(t *testing.T) {
	original := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{
		{Type: "VarSet", Triggers: []string{"1", "nosuchtrigger = 1"}},                                               // unknown trigger
		{Type: "ChangeState", Triggers: []string{"time = 0"}, Parameters: map[string]string{"value": "nosuchparam"}}, // unknown in evaluated parameter
		{Type: "VarSet", Triggers: []string{"1 +* ("}},                                                               // unparsable
		{Type: "VarSet", Triggers: []string{"time = 0"}},                                                             // fine
		{Type: "HitDef", Triggers: []string{"1"}, Parameters: map[string]string{"animtype": "light"}},                // unsupported type: params not evaluated
		{Type: "VarSet"}, // no triggers: untouched
	}}}

	got, unparsable := neutralize(original)

	ctrls := got[0].Controllers
	never := []string{"0"}
	for _, i := range []int{0, 1, 2} {
		if !reflect.DeepEqual(ctrls[i].Triggers, never) {
			t.Errorf("controller %d Triggers = %v, want %v", i, ctrls[i].Triggers, never)
		}
	}
	for _, i := range []int{3, 4, 5} {
		if !reflect.DeepEqual(ctrls[i].Triggers, original[0].Controllers[i].Triggers) {
			t.Errorf("controller %d Triggers = %v, want unchanged %v", i, ctrls[i].Triggers, original[0].Controllers[i].Triggers)
		}
	}
	if unparsable != 1 {
		t.Errorf("unparsable = %d, want 1", unparsable)
	}
	if !reflect.DeepEqual(original[0].Controllers[0].Triggers, []string{"1", "nosuchtrigger = 1"}) {
		t.Error("neutralize modified the original table")
	}
}

func TestExposureOf_RepeatedTypeAndName_CountedOncePerState(t *testing.T) {
	def := cns.StateDef{Controllers: []cns.Controller{
		{Type: "Bogus", Triggers: []string{"zzz = 1"}},
		{Type: "bogus", Triggers: []string{"zzz = 2 || yyy(1) = 3"}},
		{Type: "VarSet", Triggers: []string{"1"}},
	}}
	e := exposureOf(def)
	if !reflect.DeepEqual(e.unsupportedTypes, []string{"bogus"}) {
		t.Errorf("unsupportedTypes = %v, want [bogus]", e.unsupportedTypes)
	}
	if !reflect.DeepEqual(e.unknownTriggers, []string{"yyy()", "zzz"}) {
		t.Errorf("unknownTriggers = %v, want [yyy() zzz]", e.unknownTriggers)
	}
	if e.controllers != 3 || e.supported != 1 {
		t.Errorf("controllers/supported = %d/%d, want 3/1", e.controllers, e.supported)
	}
}

func TestExposureOf_HitDef_CountedAsSupported(t *testing.T) {
	def := cns.StateDef{Controllers: []cns.Controller{
		{Type: "hitdef", Triggers: []string{"1"}},
		{Type: "Bogus", Triggers: []string{"1"}},
	}}
	e := exposureOf(def)
	if !reflect.DeepEqual(e.unsupportedTypes, []string{"bogus"}) {
		t.Errorf("unsupportedTypes = %v, want [bogus]", e.unsupportedTypes)
	}
	if e.supported != 1 {
		t.Errorf("supported = %d, want 1", e.supported)
	}
}

func TestInspectController_UnsupportedGetHitVarVariable_IsUnknownTrigger(t *testing.T) {
	ctrl := cns.Controller{Type: "ChangeState", Triggers: []string{"GetHitVar(yvel) > 0 && GetHitVar(hittime) > 0"}}
	unknown, unparsable := inspectController(ctrl)
	if unparsable || !reflect.DeepEqual(unknown, []string{"gethitvar(yvel)"}) {
		t.Errorf("unknown = %v unparsable = %v, want [gethitvar(yvel)] false", unknown, unparsable)
	}
}

func TestInspectController_PlaySndHalves_AreInspectedSeparately(t *testing.T) {
	ctrl := cns.Controller{Type: "PlaySnd", Triggers: []string{"1"}, Parameters: map[string]string{"value": "0, P2StateNo % 3"}}
	unknown, _ := inspectController(ctrl)
	if !reflect.DeepEqual(unknown, []string{"p2stateno"}) {
		t.Errorf("unknown = %v, want [p2stateno]", unknown)
	}
	ctrl.Parameters["value"] = "MoveContact, 1"
	unknown, _ = inspectController(ctrl)
	if !reflect.DeepEqual(unknown, []string{"movecontact"}) {
		t.Errorf("unknown = %v, want [movecontact]", unknown)
	}
}

func TestNeutralize_UnknownTriggerInMovementParameter_NeutralizesTheController(t *testing.T) {
	original := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{
		{Type: "VelSet", Triggers: []string{"1"}, Parameters: map[string]string{"x": "nosuchtrigger"}},
		{Type: "ChangeAnim", Triggers: []string{"1"}, Parameters: map[string]string{"value": "200", "elem": "nosuchtrigger"}},
		{Type: "LifeAdd", Triggers: []string{"1"}, Parameters: map[string]string{"value": "-1", "kill": "nosuchtrigger"}},
		{Type: "VelSet", Triggers: []string{"1"}, Parameters: map[string]string{"x": "3"}},
	}}}

	got, _ := neutralize(original)

	for i, want := range [][]string{{"0"}, {"0"}, {"0"}, {"1"}} {
		if !reflect.DeepEqual(got[0].Controllers[i].Triggers, want) {
			t.Errorf("controller %d Triggers = %v, want %v", i, got[0].Controllers[i].Triggers, want)
		}
	}
}
