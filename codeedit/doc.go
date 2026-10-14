// Package codeedit is a code editor for jamb: a multi-line text area for source code,
// with a line-indexed text buffer, undo and redo, several selections, find and replace,
// folding by indentation, and syntax highlighting.
//
// A Document holds the text, the selections and the history. An Editor shows one, and is
// placed in a tree as its element:
//
//	doc := codeedit.NewDocument(source)
//	ui.Div().Child(codeedit.New(doc).Label("main.go").Language(golang.Grammar))
//
// The keys are actions (see Bindings and the Cmd actions) with default bindings, so they
// work without setup, and an app can bind them to other keys. Several editors may show one Document: the
// selections and the folds belong to the Document, and the scrolling to each editor.
//
// Highlighting uses tree-sitter. The highlight package holds the Session, which parses in a
// worker and follows the text, and the packages of github.com/tyler-builds/jamb-grammars
// (golang, python and the others) each give a highlight.Grammar. Editor.Language starts a
// session for the document; the editor sends it each change as it is made and paints with the
// spans it has. A document over the highlight limit is shown without colors.
package codeedit
