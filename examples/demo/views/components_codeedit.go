package views

import (
	"context"
	"strings"
	"time"

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

// farewell says goodbye to name.
func farewell(name string) string {
	return fmt.Sprintf("bye, %s", nmae)
}

func main() {
	fmt.Println(greet("world"))
}
`

// codeEditorContext is the key context around the code editor and its find bar,
// in which the demo's Ctrl+F is turned off (see Bindings).
const codeEditorContext = "CodeEditorSample"

// codeEditorSection shows a code editor on a sample Go file, highlighted as Go. The
// document is kept in the section, so the text and the caret survive the section's re-renders.
// It has diagnostics, a hover, completions, go to definition and code actions from fake providers,
// and soft wrap set by the controls above it. Try Ctrl+F, Ctrl+H, Ctrl+G, Ctrl+D and Alt+click.
func codeEditorSection() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		doc := ui.Local(ctx, "codeedit.doc", codeEditorDocument()).Peek()
		wrap := ui.Local(ctx, "codeedit.wrap", codeedit.WrapNone)
		indentOne := ui.Local(ctx, "codeedit.indentOne", false)
		opened := ui.Local(ctx, "codeedit.opened", "")

		indent := codeedit.IndentSame
		if indentOne.Get() {
			indent = codeedit.IndentOneLevel
		}
		ed := codeedit.Editor(doc).Label("Sample Go file").Language("go").CommentPrefix("// ").Height(ui.Px(260)).
			SoftWrap(wrap.Get()).WrapColumn(60).WrappingIndent(indent).
			Hover(codeEditorHover).Completions(codeEditorCompletions{}).Definitions(codeEditorDefinitions{}).
			CodeActions(codeEditorActions{report: func(s string) { opened.Set(s) }}).
			OnOpenLocation(func(l codeedit.Location) { opened.Set("Opened in another tab: " + l.URI) })

		controls := ui.Div().Row().Wrap().Align(ui.Center).Gap(18).
			Child(ui.Tabs(wrap,
				ui.Tab[codeedit.WrapMode]{Value: codeedit.WrapNone, Label: "Off"},
				ui.Tab[codeedit.WrapMode]{Value: codeedit.WrapViewport, Label: "Viewport"},
				ui.Tab[codeedit.WrapMode]{Value: codeedit.WrapColumn, Label: "Column 60"},
			).Key("codeedit-wrap").Label("Soft wrap").Variant(ui.TabsSegmented)).
			Child(ui.Checkbox("Indent wrapped lines one level", indentOne).Key("codeedit-indent"))
		status := opened.Get()
		if status == "" {
			status = "Go to definition of greet stays in this file. Go to definition of Sprintf opens another file."
		}
		return section(t, "Code editor", ui.Div().Column().Gap(10).Width(ui.Full).
			Child(controls).
			Child(ui.Div().Key("code-editor").Width(ui.Full).KeyContext(codeEditorContext).Child(ed)).
			Child(ui.Text("F8 and Shift+F8 go to the next and previous problem, underlined and marked in the gutter. "+
				"Ctrl+. opens the code actions for a problem. "+
				"Ctrl+Space, or a dot, asks for completions. F12 or Ctrl+click goes to a definition. "+
				"Rest the pointer on a name, or press Ctrl+K Ctrl+I, for its hover.").FontSize(12).TextColor(t.TextMuted)).
			Child(ui.Text(status).FontSize(12).TextColor(t.TextMuted)))
	})
}

// codeEditorDocument is the sample file with three problems marked in it, at the
// places the sample names. The marks are for the demo: they follow the text as it is
// edited, as a language server's would.
func codeEditorDocument() *codeedit.Document {
	doc := codeedit.NewDocument(codeEditorSample)
	mark := func(s string, sev codeedit.Severity, msg string) codeedit.Diagnostic {
		from := strings.Index(codeEditorSample, s)
		return codeedit.Diagnostic{From: from, To: from + len(s), Severity: sev, Message: msg, Source: "demo"}
	}
	doc.SetDiagnostics([]codeedit.Diagnostic{
		mark(`return "hello"`, codeedit.SeverityWarning, "Demo warning: this greeting could be a constant."),
		mark(`name == ""`, codeedit.SeverityHint, "Demo hint: a hint is a suggestion, such as a simpler check."),
		mark(`fmt.Println`, codeedit.SeverityError, "Demo error: an error marks a line that does not build."),
		mark(`nmae`, codeedit.SeverityError, "Demo error: undefined: nmae"),
	})
	return doc
}

// codeEditorNames describes the names of the sample file, as a language server's hover
// does, by the word under the pointer.
var codeEditorNames = map[string]string{
	"greet":   "func greet(name string) string",
	"name":    "name string, the person to greet",
	"fmt":     "package fmt",
	"Sprintf": "func Sprintf(format string, a ...any) string",
	"Println": "func Println(a ...any) (n int, err error)",
	"main":    "func main()",
}

// codeEditorHover answers the hover of the word at the pointer, from codeEditorNames.
var codeEditorHover = codeedit.HoverFunc(func(_ context.Context, q codeedit.Query) (codeedit.Hover, error) {
	from, to := wordAt(q.Text, q.Offset)
	text, ok := codeEditorNames[q.Text[from:to]]
	if !ok {
		return codeedit.Hover{}, nil
	}
	return codeedit.Hover{From: from, To: to, Text: text}, nil
})

// codeEditorItems are the completions the sample offers: Go's keywords and some of its
// names, with their kinds.
var codeEditorItems = []codeedit.CompletionItem{
	{Label: "Println", Detail: "func(a ...any) (int, error)", Doc: "Prints to standard output.", Kind: codeedit.CompletionFunction},
	{Label: "Sprintf", Detail: "func(format string, a ...any) string", Doc: "Formats according to a format specifier.", Kind: codeedit.CompletionFunction},
	{Label: "Errorf", Detail: "func(format string, a ...any) error", Doc: "Formats an error message.", Kind: codeedit.CompletionFunction},
	{Label: "greet", Detail: "func(name string) string", Kind: codeedit.CompletionFunction},
	{Label: "name", Detail: "string", Kind: codeedit.CompletionVariable},
	{Label: "fmt", Detail: "package", Kind: codeedit.CompletionModule},
	{Label: "strings", Detail: "package", Kind: codeedit.CompletionModule},
	{Label: "string", Detail: "predeclared type", Kind: codeedit.CompletionType},
	{Label: "error", Detail: "predeclared type", Kind: codeedit.CompletionType},
	{Label: "len", Detail: "func(v Type) int", Kind: codeedit.CompletionFunction},
	{Label: "nil", Detail: "predeclared", Kind: codeedit.CompletionConstant},
	{Label: "return", Kind: codeedit.CompletionKeyword},
	{Label: "func", Kind: codeedit.CompletionKeyword},
	{Label: "if", Kind: codeedit.CompletionKeyword},
}

// codeEditorCompletions is a CompletionProvider with a list of codeEditorItems. A
// dot asks for the list too, and it takes a moment, as a language server does, so the
// editor shows its loading state first.
type codeEditorCompletions struct{}

// Triggers returns the dot, which opens the list when typed.
func (codeEditorCompletions) Triggers() string { return "." }

// Complete answers with the items after a short wait. It stops early when the editor
// has asked again, as ctx is then canceled.
func (codeEditorCompletions) Complete(ctx context.Context, _ codeedit.CompletionQuery) (codeedit.CompletionList, error) {
	select {
	case <-ctx.Done():
		return codeedit.CompletionList{}, ctx.Err()
	case <-time.After(400 * time.Millisecond):
	}
	return codeedit.CompletionList{From: -1, Items: codeEditorItems}, nil
}

// codeEditorDefinitions is a DefinitionProvider for the names of the sample file: greet
// goes to its declaration in the same text, and Sprintf to a file that is not open, which
// the editor hands to OnOpenLocation.
type codeEditorDefinitions struct{}

// Definition finds the definition of the word at the query's offset.
func (codeEditorDefinitions) Definition(_ context.Context, q codeedit.Query) ([]codeedit.Location, error) {
	from, to := wordAt(q.Text, q.Offset)
	switch q.Text[from:to] {
	case "greet":
		decl := strings.Index(q.Text, "func greet")
		if decl < 0 {
			return nil, nil
		}
		decl += len("func ")
		return []codeedit.Location{{From: decl, To: decl + len("greet")}}, nil
	case "Sprintf":
		return []codeedit.Location{{URI: "file:///sample/fmt/print.go"}}, nil
	}
	return nil, nil
}

// codeEditorActions is a CodeActionProvider for the sample. A misspelt name has a quick fix, which
// is preferred; a refactoring is listed disabled, with its reason; and organize imports is a source
// action whose Run reports into the status line of the section, through report.
type codeEditorActions struct {
	report func(string)
}

// CodeActions answers for the diagnostics of the query: the quick fix for a misspelt name, and the
// two actions every query gets.
func (a codeEditorActions) CodeActions(_ context.Context, q codeedit.CodeActionQuery) ([]codeedit.CodeAction, error) {
	var out []codeedit.CodeAction
	for _, d := range q.Diagnostics {
		if d.From >= 0 && d.From <= d.To && d.To <= len(q.Text) && q.Text[d.From:d.To] == "nmae" {
			out = append(out, codeedit.CodeAction{
				Title:       "Change nmae to name",
				Kind:        codeedit.CodeActionQuickFix,
				Diagnostics: []codeedit.Diagnostic{d},
				Preferred:   true,
				Edits:       []codeedit.Change{{From: d.From, To: d.To, Text: "name"}},
			})
		}
	}
	out = append(out,
		codeedit.CodeAction{
			Title:    "Extract function",
			Kind:     codeedit.CodeActionRefactorExtract,
			Disabled: "the demo does not extract functions",
		},
		codeedit.CodeAction{
			Title: "Organize imports",
			Kind:  codeedit.CodeActionSourceOrganizeImports,
			Run:   func() { a.report("Organized imports: the demo has no imports to sort.") },
		})
	return out, nil
}

// wordAt returns the byte range of the identifier that holds the offset off in s. An
// offset between two words gives the empty range at it.
func wordAt(s string, off int) (from, to int) {
	from, to = off, off
	for from > 0 && isIdentByte(s[from-1]) {
		from--
	}
	for to < len(s) && isIdentByte(s[to]) {
		to++
	}
	return from, to
}

// isIdentByte reports whether c can be in an identifier of the sample's Go.
func isIdentByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
