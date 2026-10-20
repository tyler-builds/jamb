package codeedit

import (
	"context"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// fakeCompleter is a completion provider for the tests: fn answers each query.
type fakeCompleter struct {
	triggers string
	fn       func(ctx context.Context, q CompletionQuery) (CompletionList, error)
}

func (f fakeCompleter) Triggers() string { return f.triggers }

func (f fakeCompleter) Complete(ctx context.Context, q CompletionQuery) (CompletionList, error) {
	return f.fn(ctx, q)
}

// words answers every query with the items labeled by names, at the word before the caret.
func words(names ...string) fakeCompleter {
	return fakeCompleter{triggers: ".", fn: func(ctx context.Context, q CompletionQuery) (CompletionList, error) {
		l := CompletionList{From: -1}
		for _, n := range names {
			l.Items = append(l.Items, CompletionItem{Label: n})
		}
		return l, nil
	}}
}

// A second keystroke cancels the request of the first, the first's answer is dropped, and the
// answer of the second is applied with a frame and filtered by the word typed.
func TestCompletionKeystrokeCancelsThePendingRequest(t *testing.T) {
	type call struct {
		ctx context.Context
		q   CompletionQuery
	}
	calls := make(chan call, 4)
	release := make(chan struct{})
	h := newHarness(t, "x", func(e *EditorElement) *EditorElement {
		return e.Completions(fakeCompleter{triggers: ".", fn: func(ctx context.Context, q CompletionQuery) (CompletionList, error) {
			calls <- call{ctx, q}
			<-release
			if q.Trigger == TriggerCharacter {
				return CompletionList{From: -1, Items: []CompletionItem{{Label: "stale"}}}, nil
			}
			return CompletionList{From: -1, Items: []CompletionItem{{Label: "alpha"}, {Label: "beta"}, {Label: "gamma"}}}, nil
		}})
	})
	vs := h.ed.view
	h.doc.SetCaret(1)
	h.typeText(".")
	first := <-calls
	if first.q.Trigger != TriggerCharacter || first.q.Char != '.' || first.q.Offset != 2 {
		t.Fatalf("the first query is %+v, want the trigger character at offset 2", first.q)
	}
	if !vs.completion.open || !vs.completion.loading {
		t.Fatal("the list does not show that it is loading")
	}
	h.typeText("g")
	second := <-calls
	if first.ctx.Err() == nil {
		t.Fatal("the second keystroke did not cancel the first request")
	}
	if second.q.Text != "x.g" {
		t.Errorf("the second query has the text %q, want the text after the keystroke", second.q.Text)
	}
	close(release)
	h.waitFor("the answer", func() bool { return !vs.completion.loading })
	if got := labels(vs); strings.Join(got, ",") != "gamma" {
		t.Errorf("the list shows %v, want the item that matches g", got)
	}
}

// labels returns the labels of the items the list shows.
func labels(vs *viewState) []string {
	var out []string
	for i := range vs.completion.shown {
		out = append(out, vs.completion.itemAt(i).Label)
	}
	return out
}

// openList opens the list with Ctrl+Space and waits for the answer.
func (h *harness) openList() {
	h.t.Helper()
	h.key(input.KeySpace, input.ModCtrl)
	h.waitFor("the completions", func() bool { return h.ed.view.completion.open && !h.ed.view.completion.loading })
}

// Up and Down move the highlight of the open list, Enter accepts it, and Escape closes the list;
// with the list closed the keys move the caret.
func TestCompletionKeysGoToTheList(t *testing.T) {
	h := newHarness(t, "al\nnext", func(e *EditorElement) *EditorElement {
		return e.Completions(words("alpha", "alfa", "beta"))
	})
	vs := h.ed.view
	h.doc.SetCaret(2)
	h.openList()
	if got := labels(vs); len(got) != 2 {
		t.Fatalf("the list shows %v, want alpha and alfa", got)
	}
	h.key(input.KeyDown, 0)
	if vs.completion.sel != 1 || h.doc.Caret() != 2 {
		t.Fatalf("down moved the highlight to %d and the caret to %d, want 1 and 2", vs.completion.sel, h.doc.Caret())
	}
	h.key(input.KeyEnter, 0)
	if got := h.doc.Text(); got != "alfa\nnext" || h.doc.Caret() != 4 || vs.completion.open {
		t.Fatalf("enter gave %q with the caret at %d (open %v), want alfa accepted", got, h.doc.Caret(), vs.completion.open)
	}
	h.doc.Undo()
	if got := h.doc.Text(); got != "al\nnext" {
		t.Fatalf("undo gave %q, want the accept undone in one step", got)
	}
	h.doc.SetCaret(2)
	h.openList()
	h.key(input.KeyEscape, 0)
	if vs.completion.open {
		t.Fatal("escape left the list open")
	}
	h.key(input.KeyDown, 0)
	if h.doc.RowOf(h.doc.Caret()) != 1 {
		t.Errorf("down with the list closed left the caret on row %d", h.doc.RowOf(h.doc.Caret()))
	}
	// A move off the word closes the list.
	h.doc.SetCaret(2)
	h.openList()
	h.key(input.KeyRight, 0)
	if vs.completion.open {
		t.Error("a move past the word left the list open")
	}
}

// Accepting replaces the word before every caret in one step, and the item's edits follow the
// text typed since the request.
func TestCompletionAcceptsAtEveryCaretWithMappedEdits(t *testing.T) {
	base := "fo\nfo\n"
	h := newHarness(t, base, func(e *EditorElement) *EditorElement {
		return e.Completions(fakeCompleter{fn: func(ctx context.Context, q CompletionQuery) (CompletionList, error) {
			return CompletionList{From: q.Offset - 2, Items: []CompletionItem{{
				Label: "foobar", Insert: "foobar()",
				Edits: []Change{{From: len(q.Text), To: len(q.Text), Text: "// end\n"}},
			}}}, nil
		}})
	})
	vs := h.ed.view
	h.doc.SetSelections(Selection{Anchor: 2, Head: 2}, Selection{Anchor: 5, Head: 5})
	h.openList()
	h.typeText("o") // typed at both carets, after the request
	if !vs.completion.open || len(vs.completion.shown) != 1 {
		t.Fatalf("typing closed or emptied the list: open %v, %v", vs.completion.open, labels(vs))
	}
	h.key(input.KeyTab, 0)
	if got, want := h.doc.Text(), "foobar()\nfoobar()\n// end\n"; got != want {
		t.Fatalf("accept gave %q, want %q", got, want)
	}
	if sels := h.doc.Selections(); len(sels) != 2 || sels[0].Head != 8 || sels[1].Head != 17 {
		t.Errorf("the carets are %+v, want after each insertion", sels)
	}
	h.doc.Undo()
	if got := h.doc.Text(); got != "foo\nfoo\n" {
		t.Errorf("undo gave %q, want the accept and its edit undone together", got)
	}
}

// The list is a list for assistive technology, with the highlighted item selected, and the editor
// keeps the focus.
func TestCompletionListIsAccessible(t *testing.T) {
	h := newHarness(t, "", func(e *EditorElement) *EditorElement {
		return e.Completions(words("alpha", "beta"))
	})
	updates := activateAccess(h)
	h.openList()
	h.key(input.KeyDown, 0)
	h.frame()
	tree := h.rt.AccessBridge().Tree()
	list := accesstest.Find(t, tree, access.RoleList, "Suggestions")
	var selected []string
	for _, c := range list.Children {
		if n := tree.Node(c); n != nil && n.Role == access.RoleListItem && n.States.Has(access.StateSelected) {
			selected = append(selected, n.Name)
		}
	}
	if len(selected) != 1 || selected[0] != "beta" {
		t.Errorf("the selected items are %v, want beta", selected)
	}
	if !h.ed.view.focused {
		t.Error("the list took the focus from the editor")
	}
	got := announcements(*updates) // the queue is the process's, so the last ones are this test's
	if len(got) < 2 || got[len(got)-2] != "2 suggestions" || got[len(got)-1] != "beta" {
		t.Errorf("the announcements are %q, want the count and then the highlighted item", got)
	}
}

// An incomplete list asks again as the word grows, after a pause.
func TestCompletionIncompleteAsksAgain(t *testing.T) {
	delays := instantTimers(t)
	asked := make(chan CompletionTrigger, 4)
	h := newHarness(t, "", func(e *EditorElement) *EditorElement {
		return e.Completions(fakeCompleter{fn: func(ctx context.Context, q CompletionQuery) (CompletionList, error) {
			asked <- q.Trigger
			return CompletionList{From: -1, Incomplete: true, Items: []CompletionItem{{Label: "ab"}}}, nil
		}})
	})
	h.openList()
	<-asked
	h.typeText("a")
	h.frame()
	h.waitFor("the second request", func() bool { return len(asked) > 0 })
	if got := <-asked; got != TriggerIncomplete {
		t.Errorf("the second request's trigger is %d, want TriggerIncomplete", got)
	}
	if len(*delays) == 0 || (*delays)[len(*delays)-1] != completionDebounce {
		t.Errorf("the delays are %v, want the debounce", *delays)
	}
}
