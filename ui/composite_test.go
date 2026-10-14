package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// counter is an element defined outside package ui, as an add-on package
// defines one.
type counter struct {
	ui.Composite
	label  string
	source *ui.Signal[int]
}

func (c *counter) Render(ctx *ui.Context) ui.Element {
	renders := ui.Local(ctx, "renders", 0)
	renders.Set(renders.Peek() + 1)
	return ui.Text(fmt.Sprintf("%s %d (render %d)", c.label, c.source.Get(), renders.Peek()))
}

// A Composite renders like a component: it keeps its local state, renders
// again when a signal it reads changes, and answers to its key.
func TestCompositeFromAnotherPackage(t *testing.T) {
	n := ui.NewSignal(1)
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		c := &counter{label: "count", source: n}
		c.SetKey("counter")
		return ui.Div().Child(c)
	}, ui.DarkTheme(), nil)
	rt.SetViewport(300, 100, 1)
	dump := func() string {
		rt.Frame(time.Now())
		var b strings.Builder
		rt.Dump(&b, 0)
		return b.String()
	}
	if d := dump(); !strings.Contains(d, "count 1 (render 1)") {
		t.Fatalf("first render:\n%s", d)
	}
	n.Set(2)
	if d := dump(); !strings.Contains(d, "count 2 (render 2)") {
		t.Fatalf("after the signal changed, want the kept local state counting renders:\n%s", d)
	}
	if _, ok := rt.Find("counter"); !ok {
		t.Error("the composite does not answer to its key")
	}
}
