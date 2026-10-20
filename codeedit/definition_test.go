package codeedit

import (
	"context"
	"testing"

	"github.com/tyler-builds/jamb/input"
)

// definedAt is a provider of definitions that answers every query with locs.
type definedAt []Location

func (l definedAt) Definition(ctx context.Context, q Query) ([]Location, error) { return l, nil }

// F12 goes to a definition in the document: it is selected.
func TestDefinitionInTheDocumentIsSelected(t *testing.T) {
	opened := 0
	h := newHarness(t, "func f() {}\nf()\n", func(e *EditorElement) *EditorElement {
		return e.Definitions(definedAt{{From: 5, To: 6}}).OnOpenLocation(func(Location) { opened++ })
	})
	h.doc.SetCaret(13)
	h.key(input.KeyF12, 0)
	h.waitFor("the definition", func() bool { return h.doc.Primary() == Selection{Anchor: 5, Head: 6} })
	if opened != 0 {
		t.Errorf("a definition in the document was opened %d times as another file", opened)
	}
}

// A definition in another file goes to OnOpenLocation, and the selection stays; a Ctrl+click runs
// it too, and Ctrl under the pointer underlines the word.
func TestDefinitionInAnotherFileIsOpened(t *testing.T) {
	var got []Location
	h := newHarness(t, "fmt.Println()\n", func(e *EditorElement) *EditorElement {
		return e.Definitions(definedAt{{URI: "file:///go/src/fmt/print.go", Row: 272, Col: 5}}).
			OnOpenLocation(func(l Location) { got = append(got, l) })
	})
	vs := h.ed.view
	x, y := h.lineXY(0, 6)
	h.rt.PointerMove(x, y, input.ModCtrl)
	h.frame()
	if !vs.link.on || vs.link.from != 4 || vs.link.to != 11 {
		t.Errorf("Ctrl over Println underlines %+v, want [4, 11)", vs.link)
	}
	h.click(x, y, input.ModCtrl, 1)
	h.waitFor("the location", func() bool { return len(got) == 1 })
	if got[0].URI != "file:///go/src/fmt/print.go" || got[0].Row != 272 {
		t.Errorf("OnOpenLocation got %+v", got[0])
	}
	if p := h.doc.Primary(); !p.Empty() {
		t.Errorf("the selection is %+v, want the caret the click placed", p)
	}
}
