package zssexec

import (
	"reflect"
	"strings"
	"testing"

	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/match"
)

// stepBody runs one Step over a Statedef 0 whose body is the given lines.
func stepBody(t *testing.T, ctx evaluator.Context, lines ...string) Result {
	t.Helper()
	script := mustParseScript(t, "[Statedef 0]\n"+strings.Join(lines, "\n"))
	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	return result
}

func facingLeftCtx() evaluator.Context {
	return evaluator.Context{FighterState: match.FighterState{Facing: match.FacingLeft, Health: 500}}
}

func TestStep_VelSetVelAddVelMul_ApplyFacingRelativeVelocity(t *testing.T) {
	result := stepBody(t, facingLeftCtx(),
		"VelSet{x: 4; y: -6}",
		"VelAdd{x: 2}",
		"VelMul{x: 0.5; y: 2}",
	)
	// Facing left: VelSet x 4 -> world -4, VelAdd forward 2 -> -6, *0.5 -> -3; y: up 6, *2 -> 12.
	if got, want := result.Context.Velocity, (match.Velocity{X: -3, Y: 12}); got != want {
		t.Errorf("Velocity = %+v, want %+v", got, want)
	}
}

func TestStep_PosSetAndPosAdd_MoveTheFighter(t *testing.T) {
	result := stepBody(t, facingLeftCtx(),
		"PosSet{x: 100; y: 0}",
		"posAdd{x: 10; y: -5}",
	)
	// Set to 100, then forward 10 facing left: 90; y: up 5.
	if got, want := result.Context.Position, (match.Position{X: 90, Y: 5}); got != want {
		t.Errorf("Position = %+v, want %+v", got, want)
	}
}

func TestStep_ParameterNamesAreCaseInsensitive(t *testing.T) {
	result := stepBody(t, evaluator.Context{}, "VelSet{X: 3}")
	if result.Context.Velocity.X != 3 {
		t.Errorf("Velocity.X = %v, want 3", result.Context.Velocity.X)
	}
}

func TestStep_ChangeAnimWithElem_RecordsTheRequest(t *testing.T) {
	result := stepBody(t, evaluator.Context{Anim: 1, AnimTime: 9}, "ChangeAnim{value: 200; elem: 2; elemtime: 1}")
	c := result.Context
	if c.Anim != 200 || c.AnimTime != 0 || c.AnimElem != 2 || c.AnimElemTime != 1 {
		t.Errorf("Anim, AnimTime, AnimElem, AnimElemTime = %d, %d, %d, %d, want 200, 0, 2, 1", c.Anim, c.AnimTime, c.AnimElem, c.AnimElemTime)
	}
}

func TestStep_CtrlSetAndTurn_ChangeControlAndFacing(t *testing.T) {
	result := stepBody(t, evaluator.Context{Ctrl: true}, "CtrlSet{value: 0}", "Turn{}")
	if result.Context.Ctrl {
		t.Error("Ctrl = true, want false")
	}
	if result.Context.Facing != match.FacingLeft {
		t.Errorf("Facing = %v, want FacingLeft", result.Context.Facing)
	}
}

func TestStep_StateTypeSet_ChangesTypes(t *testing.T) {
	result := stepBody(t, evaluator.Context{StateType: 'S', MoveType: 'I'}, "StateTypeSet{statetype: A; movetype: H}")
	if result.Context.StateType != 'A' || result.Context.MoveType != 'H' {
		t.Errorf("StateType, MoveType = %c, %c, want A, H", result.Context.StateType, result.Context.MoveType)
	}
}

func TestStep_LifeAddAndLifeSet_ClampAndNeverRevive(t *testing.T) {
	result := stepBody(t, facingLeftCtx(), "LifeAdd{value: -600; kill: 0}")
	if result.Context.Health != 1 {
		t.Errorf("Health = %d, want 1 (kill: 0 cannot KO)", result.Context.Health)
	}
	result = stepBody(t, facingLeftCtx(), "LifeSet{value: 5000}")
	if result.Context.Health != 1000 {
		t.Errorf("Health = %d, want 1000 (max)", result.Context.Health)
	}
	ko := evaluator.Context{}
	result = stepBody(t, ko, "LifeSet{value: 300}")
	if result.Context.Health != 0 {
		t.Errorf("Health = %d, want 0 (a KO'd fighter is not revived)", result.Context.Health)
	}
}

func TestStep_Gravity_UsesTheContextsYAccel(t *testing.T) {
	result := stepBody(t, evaluator.Context{YAccel: 0.5}, "Gravity{}")
	if result.Context.Velocity.Y != -0.5 {
		t.Errorf("Velocity.Y = %v, want -0.5", result.Context.Velocity.Y)
	}
}

func TestStep_Null_IsAcceptedAndChangesNothing(t *testing.T) {
	ctx := facingLeftCtx()
	result := stepBody(t, ctx, "Null{}")
	if !reflect.DeepEqual(result.Context, ctx) || result.Unsupported != nil {
		t.Errorf("Null changed the context or was reported unsupported: %+v, %v", result.Context, result.Unsupported)
	}
}

func TestStep_ConditionalMovementController_RunsOnlyWhenItsConditionHolds(t *testing.T) {
	body := []string{"if Time = 0 {", "\tVelSet{x: 3}", "}"}
	if got := stepBody(t, evaluator.Context{Time: 0}, body...).Context.Velocity.X; got != 3 {
		t.Errorf("Velocity.X at Time 0 = %v, want 3", got)
	}
	if got := stepBody(t, evaluator.Context{Time: 1}, body...).Context.Velocity.X; got != 0 {
		t.Errorf("Velocity.X at Time 1 = %v, want 0", got)
	}
}

func TestStep_MovementControllerWithBadParameter_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nVelSet{x: 1 +}")
	_, err := Step(evaluator.Context{}, script)
	if err == nil || !strings.Contains(err.Error(), "VelSet") {
		t.Fatalf("Step error = %v, want one naming VelSet", err)
	}
}

func TestStep_UnsupportedController_IsReportedByNameAndDoesNotLeak(t *testing.T) {
	ctx := evaluator.Context{Unsupported: []string{"stale"}}
	result := stepBody(t, ctx, "Explod{id: 1}", "explod{id: 2}", "Helper{id: 3}")
	if want := []string{"explod", "helper"}; !reflect.DeepEqual(result.Unsupported, want) {
		t.Errorf("Unsupported = %v, want %v", result.Unsupported, want)
	}
	if len(result.Context.Unsupported) != 0 {
		t.Errorf("Context.Unsupported = %v, want empty", result.Context.Unsupported)
	}
}

func TestStep_LifeAddOnKOdFighter_DoesNotRevive(t *testing.T) {
	result := stepBody(t, evaluator.Context{}, "LifeAdd{value: 50}")
	if result.Context.Health != 0 {
		t.Errorf("Health = %d, want 0", result.Context.Health)
	}
}

func TestStep_LifeAddHealingPastMax_ClampsAtMax(t *testing.T) {
	result := stepBody(t, evaluator.Context{FighterState: match.FighterState{Health: 990}}, "LifeAdd{value: 50}")
	if result.Context.Health != 1000 {
		t.Errorf("Health = %d, want 1000", result.Context.Health)
	}
}

func TestStep_MixedCaseMultiLetterKey_IsRecognized(t *testing.T) {
	result := stepBody(t, evaluator.Context{}, "ChangeAnim{Value: 200; Elem: 2; ElemTime: 1}")
	c := result.Context
	if c.Anim != 200 || c.AnimElem != 2 || c.AnimElemTime != 1 {
		t.Errorf("Anim, AnimElem, AnimElemTime = %d, %d, %d, want 200, 2, 1", c.Anim, c.AnimElem, c.AnimElemTime)
	}
}

func TestStep_StateTypeSetQuotedLowerCaseLetters_AreAccepted(t *testing.T) {
	result := stepBody(t, evaluator.Context{}, `StateTypeSet{statetype: "a"; movetype: h; physics: n}`)
	c := result.Context
	if c.StateType != 'A' || c.MoveType != 'H' || c.Physics != 'N' {
		t.Errorf("StateType, MoveType, Physics = %c, %c, %c, want A, H, N", c.StateType, c.MoveType, c.Physics)
	}
}
