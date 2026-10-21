package views

import (
	"context"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/codeedit"
)

// The sample's providers answer for the names of the sample file: the hover describes
// greet, and the definition of greet is its declaration in the same text.
func TestCodeEditorSampleProvidersAnswerForItsNames(t *testing.T) {
	at := strings.Index(codeEditorSample, `greet("world")`)
	q := codeedit.Query{Text: codeEditorSample, Offset: at + 2}
	h, err := codeEditorHover.Hover(context.Background(), q)
	if err != nil || h.Text != codeEditorNames["greet"] {
		t.Fatalf("the hover of greet is %+v, %v; want %q", h, err, codeEditorNames["greet"])
	}
	locs, err := codeEditorDefinitions{}.Definition(context.Background(), q)
	decl := strings.Index(codeEditorSample, "greet(name")
	if err != nil || len(locs) != 1 || locs[0].URI != "" || locs[0].From != decl || locs[0].To != decl+len("greet") {
		t.Fatalf("the definition of greet is %+v, %v; want its declaration at %d", locs, err, decl)
	}
}

// The sample's four problems are marked at the text they name.
func TestCodeEditorDocumentMarksItsProblems(t *testing.T) {
	ds := codeEditorDocument().Diagnostics()
	if len(ds) != 4 {
		t.Fatalf("the document has %d diagnostics, want 4", len(ds))
	}
	for _, d := range ds {
		if d.From < 0 || d.To > len(codeEditorSample) || d.From >= d.To {
			t.Errorf("the diagnostic %+v is not a range of the sample", d)
		}
	}
}

// The sample's code actions fix the misspelt name with a preferred quick fix that replaces it, list
// a refactoring disabled with its reason, and run organize imports into the status line.
func TestCodeEditorSampleOffersCodeActions(t *testing.T) {
	var status string
	at := strings.Index(codeEditorSample, "nmae")
	q := codeedit.CodeActionQuery{
		Query:       codeedit.Query{Text: codeEditorSample, Offset: at},
		From:        at,
		To:          at,
		Diagnostics: codeEditorDocument().DiagnosticsAt(at),
		Trigger:     codeedit.CodeActionInvoked,
	}
	acts, err := codeEditorActions{report: func(s string) { status = s }}.CodeActions(context.Background(), q)
	if err != nil || len(acts) != 3 {
		t.Fatalf("the actions are %+v, %v; want three", acts, err)
	}
	fix := acts[0]
	if fix.Kind != codeedit.CodeActionQuickFix || !fix.Preferred || len(fix.Edits) != 1 ||
		fix.Edits[0] != (codeedit.Change{From: at, To: at + len("nmae"), Text: "name"}) {
		t.Errorf("the quick fix is %+v, want the misspelt name replaced", fix)
	}
	if acts[1].Disabled == "" || !acts[1].Kind.Within(codeedit.CodeActionRefactor) {
		t.Errorf("the refactoring is %+v, want it disabled with a reason", acts[1])
	}
	if acts[2].Kind != codeedit.CodeActionSourceOrganizeImports || acts[2].Run == nil {
		t.Fatalf("the source action is %+v, want organize imports with a Run", acts[2])
	}
	acts[2].Run()
	if status == "" {
		t.Error("organize imports did not report into the status line")
	}
}

// wordAt finds the identifier around an offset, and the empty range between words.
func TestWordAtFindsTheIdentifier(t *testing.T) {
	s := "func greet(name string)"
	if from, to := wordAt(s, 7); s[from:to] != "greet" {
		t.Errorf("the word at 7 is %q, want greet", s[from:to])
	}
	if from, to := wordAt("x = y", 2); from != 2 || to != 2 {
		t.Errorf("the range at 2 of \"x = y\" is %d to %d, want empty", from, to)
	}
}
