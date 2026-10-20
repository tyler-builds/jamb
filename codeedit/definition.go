package codeedit

import (
	"context"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// A Location is where a definition is: in this document, when URI is empty, at the bytes
// [From, To), or in another file, which the app opens (see OnOpenLocation). Row and Col are the
// position in that file, for the app, as its provider gives them; the editor reads them only for
// another file.
type Location struct {
	// URI names the file, such as "file:///home/me/main.go", or is empty for this document.
	URI string
	// From and To are the byte range of the definition in this document.
	From, To int
	// Row and Col are the position of the definition in another file.
	Row, Col int
}

// A DefinitionProvider finds where the symbol at an offset is defined, as a language server's
// go-to-definition does. Its Definition runs on a goroutine of its own, with a context that is
// canceled when the answer is no longer wanted. The editor goes to the first location.
type DefinitionProvider interface {
	Definition(ctx context.Context, q Query) ([]Location, error)
}

// CmdGoToDefinition goes to the definition of the symbol at the caret.
var CmdGoToDefinition = ui.NewAction("codeedit.go-to-definition", "Go to definition")

// Definitions sets the provider of go-to-definition, which F12 and Ctrl+click run. A definition in
// this document is selected and scrolled into view, and one in another file is handed to the
// function set with OnOpenLocation. While Ctrl is held, the word under the pointer is underlined.
func (e *EditorElement) Definitions(p DefinitionProvider) *EditorElement { e.define = p; return e }

// OnOpenLocation sets the function that opens a definition in another file, such as in a new tab
// of the app. Without one, such a definition is not opened.
func (e *EditorElement) OnOpenLocation(fn func(Location)) *EditorElement { e.openLoc = fn; return e }

// linkState is the word under the pointer while Ctrl is held, which a Ctrl+click goes to the
// definition of.
type linkState struct {
	on       bool
	from, to int
}

// goToDefinition asks the provider where the symbol at the byte offset p is defined, and goes to
// the first location it gives.
func (vs *viewState) goToDefinition(p int) {
	if vs.defineP == nil {
		return
	}
	prov, q := vs.defineP, vs.request(p)
	vs.define.run(vs, func(ctx context.Context) ([]Location, error) { return prov.Definition(ctx, q) },
		func(locs []Location, err error) {
			if err != nil || len(locs) == 0 {
				return
			}
			loc := locs[0]
			if loc.URI != "" {
				if vs.openLoc != nil {
					vs.openLoc(loc)
				}
				return
			}
			vs.doc.SetSelections(Selection{Anchor: loc.From, Head: max(loc.To, loc.From)})
			vs.reveal = true
			vs.restart = true
			vs.touch()
		})
}

// pointerLink underlines the word under the pointer while Ctrl is held and there is a provider of
// definitions, and removes the underline otherwise.
func (vs *viewState) pointerLink(ev *ui.MouseEvent) {
	link := linkState{}
	if vs.defineP != nil && ev.Mods.Has(input.ModCtrl) && ev.LocalX >= vs.geo.textX {
		if _, p, _, ok := vs.hitAt(ev.LocalX, ev.LocalY); ok {
			if from, to := vs.wordAround(p); to > from {
				link = linkState{on: true, from: from, to: to}
			}
		}
	}
	if link != vs.link {
		vs.link = link
		vs.touch()
	}
}
