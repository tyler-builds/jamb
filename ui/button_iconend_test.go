package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/icons"
)

// IconEnd draws the icon after the label, and Icon before it.
func TestButtonIconEnd(t *testing.T) {
	rt := newTestRuntime(func(*Context) Element {
		return Div().Row().
			Child(Button("See all").Key("end").IconEnd(icons.ChevronRight)).
			Child(Button("Back").Key("start").Icon(icons.ChevronLeft))
	}, 400, 100, 1)
	rt.Frame(time.Now())
	for _, c := range []struct {
		key       string
		iconFirst bool
	}{{"end", false}, {"start", true}} {
		b := keyNode(t, rt, c.key)
		icon := rt.nodeRect(partNodesUnder(b, "Button.Icon")[0])
		label := rt.nodeRect(partNodesUnder(b, "Button.Label")[0])
		if got := icon.X < label.X; got != c.iconFirst {
			t.Errorf("%s: icon at x=%v, label at x=%v; icon first = %v, want %v", c.key, icon.X, label.X, got, c.iconFirst)
		}
	}
}
