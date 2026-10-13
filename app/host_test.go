package app

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// The host side of a runtime has every method app calls; hostOf panics if not.
func TestRuntimeHasHostMethods(t *testing.T) {
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element { return ui.Div() }, nil, nil)
	if hostOf(rt) == nil {
		t.Fatal("hostOf returned nil")
	}
}
