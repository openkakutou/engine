package evaluator

import (
	"reflect"
	"testing"
)

func TestSplitList_KeepsCommasOfParenthesesAndRedirections(t *testing.T) {
	for raw, want := range map[string][]string{
		"10, 5":                                  {"10", "5"},
		"floor(90*root,fvar(0)*root,fvar(3)),10": {"floor(90*root,fvar(0)*root,fvar(3))", "10"},
		"root,fvar(0), 10":                       {"root,fvar(0)", "10"},
		"enemy(1), life , ifelse(var(1), 2, 3)":  {"enemy(1), life", "ifelse(var(1), 2, 3)"},
		"helper(9999) ,var(8)":                   {"helper(9999) ,var(8)"},
		"(1+2), 3":                               {"(1+2)", "3"},
		"7":                                      {"7"},
		"":                                       {""},
		"ceil(13*(2-(var(16)>0))/2),ceil(13*(2-(var(16)>0))/2)": {"ceil(13*(2-(var(16)>0))/2)", "ceil(13*(2-(var(16)>0))/2)"},
	} {
		if got := SplitList(raw); !reflect.DeepEqual(got, want) {
			t.Errorf("SplitList(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestSplitList_RepeatsWithoutAllocating(t *testing.T) {
	raw := "floor(90*root,fvar(0)),10"
	SplitList(raw)
	if n := testing.AllocsPerRun(20, func() { SplitList(raw) }); n != 0 {
		t.Errorf("allocs = %v, want 0", n)
	}
}
