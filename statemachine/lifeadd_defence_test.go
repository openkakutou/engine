package statemachine

import (
	"strings"
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
)

// defended is an Ikemen GO character at 100 life with [Data] defence.
func defended(t *testing.T, defence string) evaluator.Context {
	t.Helper()
	c, err := evaluator.ParseConstants(strings.NewReader("[Data]\ndefence = " + defence + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	return evaluator.Context{Consts: c, FighterState: match.FighterState{Health: 100}, IkemenChar: true}
}

// char.go:8921-8931 lifeAdd: the value is divided by the final defence
// unless absolute.
func TestApplyController_LifeAdd_IsScaledByDefenceUnlessAbsolute(t *testing.T) {
	if got := apply(t, controller("LifeAdd", "value", "-30"), defended(t, "200")).Health; got != 85 {
		t.Errorf("defence 200: Health = %d, want 85 (-30 / 2)", got)
	}
	if got := apply(t, controller("LifeAdd", "value", "-30", "absolute", "1"), defended(t, "200")).Health; got != 70 {
		t.Errorf("absolute: Health = %d, want 70", got)
	}
	ctx := defended(t, "100")
	ctx.CustomDefence = 2
	if got := apply(t, controller("LifeAdd", "value", "-30"), ctx).Health; got != 85 {
		t.Errorf("DefenceMulSet 2: Health = %d, want 85", got)
	}
}

// char.go:8928-8930: a loss between 0 and -1 after the defence is -1.
func TestApplyController_LifeAdd_ScaledLossIsAtLeastOne(t *testing.T) {
	if got := apply(t, controller("LifeAdd", "value", "-5"), defended(t, "1000")).Health; got != 99 {
		t.Errorf("Health = %d, want 99 (-5 / 10 is -0.5, at least -1)", got)
	}
}

// bytecode.go:10804-10810: a MUGEN character's healing is always absolute.
func TestApplyController_LifeAdd_MugenHealingIsAbsolute(t *testing.T) {
	ctx := defended(t, "200")
	ctx.Health = 50
	if got := apply(t, controller("LifeAdd", "value", "20"), ctx).Health; got != 60 {
		t.Errorf("Ikemen GO character: Health = %d, want 60 (20 / 2)", got)
	}
	ctx.IkemenChar = false
	if got := apply(t, controller("LifeAdd", "value", "20"), ctx).Health; got != 70 {
		t.Errorf("MUGEN character: Health = %d, want 70", got)
	}
}
