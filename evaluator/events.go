package evaluator

// ControllerEvent is a visual or feedback controller a fighter's states ran
// this tick (PalFX, Explod, EnvShake, ...), reported for the game to draw;
// the engine only records it (backlog item 052).
type ControllerEvent struct {
	Type   string       `json:"type"`
	Params []EventParam `json:"params"`
}

// EventParam is one parameter of a ControllerEvent: its text as written and,
// when every comma-separated part evaluated, the values.
type EventParam struct {
	Name   string    `json:"name"`
	Raw    string    `json:"raw"`
	Values []float64 `json:"values"`
}

// Param is the parameter name, the zero EventParam when the event has none.
func (e ControllerEvent) Param(name string) EventParam {
	for _, p := range e.Params {
		if p.Name == name {
			return p
		}
	}
	return EventParam{}
}

// EnvShake is a screen shake, Ikemen GO's EnvShake time, freq, ampl and
// phase (a HitDef's fall.envshake.*).
type EnvShake struct {
	Time       int
	Freq, Ampl float64
	Phase      float64
}
