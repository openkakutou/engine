package evaluator

// ExplodSpawn is an Explod controller that ran (Ikemen GO explod.Run,
// bytecode.go:6773-7074, defaults initFromChar char.go:1885-1930): what the
// engine needs to make the explod. Positions and velocities are in the
// content's units (x forward, y down).
type ExplodSpawn struct {
	ID, Anim int
	// Common is an F anim, the common fightfx's.
	Common bool
	// PosType is postype as written, lower-cased ("p1" when not given, ""
	// for "none").
	PosType         string
	Pos, Vel, Accel [2]float64
	Facing, VFacing int
	BindTime        int
	RemoveTime      int
	SprPriority     int
	OnTop           bool
	RemoveOnGetHit  bool
	// MovesInPause is a supermove, supermovetime or pausemovetime: the explod
	// goes on during a pause.
	MovesInPause bool
}

// ExplodRemoval is a RemoveExplod that ran: the fighter's explods with that
// id, every one for -1.
type ExplodRemoval struct{ ID int }
