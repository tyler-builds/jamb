package views

import (
	"github.com/tyler-builds/jamb-grammars/golang"
	"github.com/tyler-builds/jamb/codeedit"
	"github.com/tyler-builds/jamb/ui"
)

// codeEditorSample is the text the code editor section opens with: a short Go file.
const codeEditorSample = `package main

import "fmt"

// greet returns the greeting for name.
func greet(name string) string {
	if name == "" {
		return "hello"
	}
	return fmt.Sprintf("hello, %s", name)
}

func main() {
	fmt.Println(greet("world"))
}
`

// codeEditorSection shows a code editor on a sample Go file, highlighted with the Go grammar. The
// document is kept in the section, so the text and the caret survive the section's re-renders.
// Try Ctrl+F, Ctrl+H, Ctrl+G, Ctrl+D and Alt+click.
func codeEditorSection() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		doc := ui.Local(ctx, "codeedit.doc", codeedit.NewDocument(codeEditorSample)).Peek()
		ed := codeedit.Editor(doc).Label("Sample Go file").Language(golang.Grammar).CommentPrefix("// ").Height(ui.Px(260))
		return section(t, "Code editor", ed)
	})
}
