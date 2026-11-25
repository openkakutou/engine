package statemachine

import (
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
)

// ModifyExplod evaluates its id once (bytecode.go modifyExplod.Run): an id
// reading Random draws one number, not two.
func TestApplyController_ModifyExplod_EvaluatesItsIDOnce(t *testing.T) {
	ctx := evaluator.Context{RandSeed: 12345}
	once := ctx
	if _, err := evaluator.EvaluateMut("Random", &once); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyController(controller("ModifyExplod", "id", "Random"), &ctx, func(int) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if ctx.RandSeed != once.RandSeed {
		t.Errorf("seed after ModifyExplod = %d, want %d (one Random draw)", ctx.RandSeed, once.RandSeed)
	}
	if n := len(ctx.ModifiedExplods); n != 1 {
		t.Fatalf("%d modifications, want 1", n)
	}
}
