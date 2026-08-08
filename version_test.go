package engine

import "testing"

func TestVersion_IsNotEmpty(t *testing.T) {
	if Version == "" {
		t.Error("expected Version to be non-empty")
	}
}
