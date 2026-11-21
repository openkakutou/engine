package evaluator

// HitBySlot is one HitBy or NotHitBy slot (Ikemen GO HitBy, char.go:1330-1339):
// the attributes it names, the ticks it stays active, whether it rejects
// (Not) or accepts them, and whether it stacks with the others.
// shortcut: the playerno, playerid and clsn parameters are not kept; a
// slot filters on attributes only.
type HitBySlot struct {
	Attr  int32
	Time  int
	Not   bool
	Stack bool
}

// HittableBy reports whether an attack of attributes attr (a HitDef attr,
// ParseAttr with hitdef true) may hit a fighter with these slots, Ikemen GO's
// checkHitByAllSlots (char.go:10997-11042): every active non-stacked slot
// must accept it, and when some slots stack, at least one of them must.
func HittableBy(slots [8]HitBySlot, attr int32) bool {
	stackHit, hasStack, nonStackHit := false, false, true
	for _, s := range slots {
		if s.Time == 0 {
			continue
		}
		ok := slotAccepts(s, attr)
		if s.Stack {
			hasStack = true
			stackHit = stackHit || ok
			continue
		}
		nonStackHit = nonStackHit && ok
	}
	if hasStack {
		return stackHit && nonStackHit
	}
	return nonStackHit
}

// slotAccepts is checkHitBySlot (char.go:10969-10993) on attributes: the
// state type and the attack class are compared one by one; a NotHitBy slot
// rejects an attack matching either, a HitBy slot accepts one matching both.
func slotAccepts(s HitBySlot, attr int32) bool {
	sca := s.Attr&attr&attrStateTypes != 0
	atk := s.Attr&attr&^attrStateTypes != 0
	if s.Not {
		return !(sca || atk)
	}
	return sca && atk
}

// OverrideMatches reports whether a HitOverride of attr catches a hit of
// hitAttr on a fighter of state type stateType (S, C, A, else none), as
// char.go:11540-11556 checks it: a shared attack class, and the fighter's
// state type among the override's.
func OverrideMatches(attr, hitAttr int32, stateType byte) bool {
	if attr&hitAttr&^attrStateTypes == 0 {
		return false
	}
	switch stateType {
	case 'S':
		return attr&attrS != 0
	case 'C':
		return attr&attrC != 0
	case 'A':
		return attr&attrA != 0
	}
	return false
}

// ReversalMatches reports whether a ReversalDef of attr reverses a HitDef of
// hitAttr (char.go:11079-11080): a shared state type and a shared attack
// class.
func ReversalMatches(attr, hitAttr int32) bool {
	return attr&hitAttr&attrStateTypes != 0 && attr&hitAttr&^attrStateTypes != 0
}
