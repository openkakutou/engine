package evaluator

// HelperSpawn is a Helper controller that ran: what the engine needs to
// create the helper (Ikemen GO helper.Run, bytecode.go:6161-6300; newHelper
// and helperInit, char.go:7156-7247). Pos is in the content's units (x
// forward, y down).
type HelperSpawn struct {
	ID, StateNo int
	// PosType is postype as written, lower-cased ("p1" when not given).
	PosType string
	Pos     [2]float64
	// Facing is -1 to face away from the spawner, else 1.
	Facing int
	Name   string
}
