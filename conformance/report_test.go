package conformance

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSortedCounts_MixedCounts_OrdersByCountDescendingThenName(t *testing.T) {
	got := sortedCounts(map[string]int{"b": 2, "a": 2, "c": 9, "d": 1})
	want := []Count{{"c", 9}, {"a", 2}, {"b", 2}, {"d", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortedCounts = %+v, want %+v", got, want)
	}
}

func TestSortedCounts_Empty_ReturnsNonNilEmptySlice(t *testing.T) {
	if got := sortedCounts(nil); got == nil || len(got) != 0 {
		t.Errorf("sortedCounts(nil) = %#v, want empty non-nil slice", got)
	}
}

func sampleReport() Report {
	return Report{
		Ticks: 100,
		Characters: []CharacterReport{{
			Path: "x/x.def", Status: StatusTickError, Error: "engine: boom", TicksRun: 7,
			StatesReached: 2, StatesTotal: 5, ControllersSupported: 3, ControllersTotal: 4,
			UnsupportedControllers: []Count{{"Foo", 12}}, UnknownTriggers: []Count{{"bar()", 3}},
			ControllersUnparsable: 1, ZssStatedefBlocks: 6,
		}},
		UnsupportedControllers: []AggregateCount{{Name: "Foo", Exposure: 12, Characters: 1}},
		UnknownTriggers:        []AggregateCount{{Name: "bar()", Exposure: 3, Characters: 1}},
	}
}

func TestReport_WriteText_ContainsEveryReportedFigure(t *testing.T) {
	var buf bytes.Buffer
	if err := sampleReport().WriteText(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"x/x.def", "engine: boom", "7", "2/5", "3/4", "Foo", "12", "bar()", "6", "controllers with an unparsable trigger: 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
}

func TestReport_WriteJSON_RoundTripsToTheSameReport(t *testing.T) {
	var buf bytes.Buffer
	if err := sampleReport().WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !reflect.DeepEqual(back, sampleReport()) {
		t.Errorf("round trip differs:\n%+v\n%+v", back, sampleReport())
	}
}
