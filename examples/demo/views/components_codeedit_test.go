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

// The sample's three problems are marked at the text they name.
func TestCodeEditorDocumentMarksItsProblems(t *testing.T) {
	ds := codeEditorDocument().Diagnostics()
	if len(ds) != 3 {
		t.Fatalf("the document has %d diagnostics, want 3", len(ds))
	}
	for _, d := range ds {
		if d.From < 0 || d.To > len(codeEditorSample) || d.From >= d.To {
			t.Errorf("the diagnostic %+v is not a range of the sample", d)
		}
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
