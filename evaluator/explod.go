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
	// Scale is the drawing scale, 1,1 by default.
	Scale           [2]float64
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

// ExplodModification is a ModifyExplod that ran (bytecode.go modifyExplod.Run,
// 7158-7560): the fighter's explods with ID (every one for -1) take the
// parameters it wrote. Set* tells which.
type ExplodModification struct {
	ID int
	ExplodSpawn
	SetPosType, SetFacing, SetVFacing, SetPos, SetVel, SetAccel bool
	SetBindTime, SetRemoveTime, SetSprPriority, SetOnTop        bool
	SetRemoveOnGetHit, SetAnim, SetMovesInPause, SetScale       bool
	// Relative makes bindtime and removetime count from the explod's age
	// (an Ikemen GO character).
	Relative bool
}
