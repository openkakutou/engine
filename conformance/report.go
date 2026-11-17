package conformance

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// Status says what happened to one .def file.
type Status string

const (
	// StatusRan means the mirror match ran for the requested number of ticks.
	StatusRan Status = "ran"
	// StatusTickError means Tick returned an error; TicksRun is the number
	// of ticks completed before it and Error holds its message.
	StatusTickError Status = "tick-error"
	// StatusLoadError means the character could not be loaded or its state
	// files could not be parsed; Error holds the reason.
	StatusLoadError Status = "load-error"
	// StatusSkipped means the .def is not a character definition.
	StatusSkipped Status = "skipped"
	// StatusZssOnly means the character's states exist only in .zss, which
	// Tick does not execute yet.
	StatusZssOnly Status = "not executed: .zss-only"
)

// Count is one identifier (controller type or trigger name) and how many
// times the run was exposed to it.
type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// AggregateCount is one identifier's total exposure over all characters and
// the number of characters exposed to it.
type AggregateCount struct {
	Name       string `json:"name"`
	Exposure   int    `json:"exposure"`
	Characters int    `json:"characters"`
}

// CharacterReport is the result for one .def file. It holds paths,
// identifiers, numbers and error messages; an error message from character
// parsing may quote a line from the character's files.
type CharacterReport struct {
	Path                   string  `json:"path"`
	Status                 Status  `json:"status"`
	Error                  string  `json:"error,omitempty"`
	TicksRun               int     `json:"ticksRun"`
	StatesReached          int     `json:"statesReached"`
	StatesTotal            int     `json:"statesTotal"`
	ControllersSupported   int     `json:"controllersSupported"`
	ControllersTotal       int     `json:"controllersTotal"`
	UnsupportedControllers []Count `json:"unsupportedControllers"`
	UnknownTriggers        []Count `json:"unknownTriggers"`
	ControllersUnparsable  int     `json:"controllersWithUnparsableTrigger"`
	ZssStatedefBlocks      int     `json:"zssStatedefBlocks"`
}

// Report is the whole run: per-character results in discovery order plus the
// aggregate over every character that ran.
type Report struct {
	Ticks                  int               `json:"ticks"`
	Characters             []CharacterReport `json:"characters"`
	UnsupportedControllers []AggregateCount  `json:"unsupportedControllers"`
	UnknownTriggers        []AggregateCount  `json:"unknownTriggers"`
}

// sortedCounts turns a name -> count map into a slice ordered by count
// descending, then name ascending. It never returns nil.
func sortedCounts(m map[string]int) []Count {
	out := make([]Count, 0, len(m))
	for name, n := range m {
		out = append(out, Count{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// aggregate sums per-character counts: for each name, the total exposure and
// the number of characters that had any.
func aggregate(perCharacter [][]Count) []AggregateCount {
	exposure := map[string]int{}
	characters := map[string]int{}
	for _, counts := range perCharacter {
		for _, c := range counts {
			exposure[c.Name] += c.Count
			characters[c.Name]++
		}
	}
	out := make([]AggregateCount, 0, len(exposure))
	for name, n := range exposure {
		out = append(out, AggregateCount{Name: name, Exposure: n, Characters: characters[name]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Exposure != out[j].Exposure {
			return out[i].Exposure > out[j].Exposure
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// WriteJSON writes the report as indented JSON.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteText writes the report as plain text. The output is a pure function
// of the report, so the same report always renders byte-identically.
func (r Report) WriteText(w io.Writer) error {
	var err error
	printf := func(format string, args ...any) {
		if err == nil {
			_, err = fmt.Fprintf(w, format, args...)
		}
	}
	printf("Conformance report (%d ticks requested)\n", r.Ticks)
	for _, c := range r.Characters {
		printf("\n%s\n", c.Path)
		printf("  status: %s\n", c.Status)
		if c.Error != "" {
			printf("  error: %s\n", c.Error)
		}
		if c.Status == StatusSkipped {
			continue
		}
		printf("  ticks run: %d\n", c.TicksRun)
		printf("  states reached: %d/%d\n", c.StatesReached, c.StatesTotal)
		printf("  controllers supported (reached states): %d/%d\n", c.ControllersSupported, c.ControllersTotal)
		printf("  controllers with an unparsable trigger: %d\n", c.ControllersUnparsable)
		printf("  .zss Statedef blocks (not executed): %d\n", c.ZssStatedefBlocks)
		for _, u := range c.UnsupportedControllers {
			printf("  unsupported controller %s: %d\n", u.Name, u.Count)
		}
		for _, u := range c.UnknownTriggers {
			printf("  unknown trigger %s: %d\n", u.Name, u.Count)
		}
	}
	printf("\nAggregate\n")
	for _, a := range r.UnsupportedControllers {
		printf("  unsupported controller %s: exposure %d, %d character(s)\n", a.Name, a.Exposure, a.Characters)
	}
	for _, a := range r.UnknownTriggers {
		printf("  unknown trigger %s: exposure %d, %d character(s)\n", a.Name, a.Exposure, a.Characters)
	}
	return err
}
