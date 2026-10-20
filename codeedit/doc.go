// Package codeedit is a code editor for jamb: a multi-line text area for source code,
// with a line-indexed text buffer, undo and redo, several selections, find and replace,
// folding by indentation, and syntax highlighting.
//
// A Document holds the text, the selections and the history. An editor shows one, and is
// placed in a tree as its element:
//
//	doc := codeedit.NewDocument(source)
//	ui.Div().Child(codeedit.Editor(doc).Label("main.go").Language("go"))
//
// The keys are actions (see Bindings and the Cmd actions) with default bindings, so they
// work without setup, and an app can bind them to other keys. Several editors may show one Document: the
// selections and the folds belong to the Document, and the scrolling to each editor.
//
// Highlighting uses @pierre/highlights, a highlighter with a lexer for each of 76 languages,
// built into the package (see codeedit/highlight). EditorElement.Language names the language, as
// a code fence does, such as "go" or "python", and starts a highlight session for the document;
// the editor sends it each change as it is made and paints with the spans it has. A document over
// the highlight limit is shown without colors. EditorElement.Highlighter takes a highlighter of
// the app's own instead, such as the tree-sitter one of github.com/tyler-builds/jamb-grammars.
package codeedit
