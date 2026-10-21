package codeedit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
)

// withActions sets the provider of the code actions of an editor to fn.
func withActions(fn CodeActionFunc) func(*EditorElement) *EditorElement {
	return func(e *EditorElement) *EditorElement { return e.CodeActions(fn) }
}

// invokeActions opens the menu of code actions with Ctrl+. and waits for the answer.
func (h *harness) invokeActions() {
	h.t.Helper()
	h.key(input.KeyPeriod, input.ModCtrl)
	h.waitFor("the code actions", func() bool { return h.ed.view.actions.open && !h.ed.view.actions.loading })
}

// actionTitles returns the titles of the actions the menu shows, in their order.
func actionTitles(vs *viewState) []string {
	var out []string
	for _, l := range vs.actions.lines {
		if l.item >= 0 {
			out = append(out, vs.actions.items[l.item].Title)
		}
	}
	return out
}

// highlighted returns the title of the highlighted action of the menu.
func highlighted(vs *viewState) string {
	return vs.actions.items[vs.actions.lines[vs.actions.sel].item].Title
}

// Ctrl+. shows the menu with a loading row while the provider answers, and the answer's actions
// when it comes, with their count announced.
func TestCodeActionsShowLoadingThenTheAnswer(t *testing.T) {
	release := make(chan struct{})
	h := newHarness(t, "helo", func(e *EditorElement) *EditorElement {
		return e.CodeActions(CodeActionFunc(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
			<-release
			return []CodeAction{
				{Title: "Fix spelling", Kind: CodeActionQuickFix},
				{Title: "Extract function", Kind: CodeActionRefactorExtract},
			}, nil
		}))
	})
	updates := activateAccess(h)
	vs := h.ed.view
	h.key(input.KeyPeriod, input.ModCtrl)
	if !vs.actions.open || !vs.actions.loading {
		t.Fatalf("after Ctrl+. the menu is open %v and loading %v, want both", vs.actions.open, vs.actions.loading)
	}
	h.frame()
	if _, ok := h.rt.Find("codeedit.actions"); !ok {
		t.Error("the loading menu is not in the tree")
	}
	close(release)
	h.waitFor("the answer", func() bool { return !vs.actions.loading })
	if got := actionTitles(vs); !slices.Equal(got, []string{"Fix spelling", "Extract function"}) {
		t.Fatalf("the menu shows %v", got)
	}
	h.frame()
	if !slices.Contains(announcements(*updates), "2 actions") {
		t.Errorf("the count was not announced: %v", announcements(*updates))
	}
}

// A second Ctrl+. cancels the request of the first: the first provider's context is canceled, and
// its late answer is dropped, so that the menu shows the second answer.
func TestSecondInvokeCancelsTheFirstRequest(t *testing.T) {
	var calls atomic.Int32
	ctxs := make(chan context.Context, 2)
	release := make(chan struct{})
	h := newHarness(t, "x", func(e *EditorElement) *EditorElement {
		return e.CodeActions(CodeActionFunc(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
			i := calls.Add(1)
			ctxs <- ctx
			<-release
			return []CodeAction{{Title: fmt.Sprintf("answer %d", i)}}, nil
		}))
	})
	vs := h.ed.view
	h.key(input.KeyPeriod, input.ModCtrl)
	first := <-ctxs
	h.key(input.KeyPeriod, input.ModCtrl)
	<-ctxs
	if first.Err() == nil {
		t.Fatal("the second Ctrl+. did not cancel the first request")
	}
	close(release)
	h.waitFor("the answer", func() bool { return !vs.actions.loading })
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond)
		h.frame()
	}
	if got := actionTitles(vs); !slices.Equal(got, []string{"answer 2"}) {
		t.Fatalf("the menu shows %v, want the second answer only", got)
	}
}

// A change made through the editor while the answer is on its way closes the menu and drops the
// answer.
func TestEditWhilePendingDropsTheAnswer(t *testing.T) {
	release := make(chan struct{})
	returned := make(chan struct{}, 1)
	h := newHarness(t, "helo", func(e *EditorElement) *EditorElement {
		return e.CodeActions(CodeActionFunc(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
			<-release
			returned <- struct{}{}
			return []CodeAction{{Title: "Fix", Edits: []Change{{From: 0, To: 4, Text: "hello"}}}}, nil
		}))
	})
	vs := h.ed.view
	h.key(input.KeyPeriod, input.ModCtrl)
	h.typeText("!")
	if vs.actions.open {
		t.Fatal("typing left the menu open")
	}
	close(release)
	<-returned
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond)
		h.frame()
	}
	if vs.actions.open || vs.actions.items != nil {
		t.Errorf("the dropped answer reached the menu: open %v, items %v", vs.actions.open, vs.actions.items)
	}
	if got := h.doc.Text(); got != "helo!" {
		t.Errorf("the text is %q, want the typing alone", got)
	}
}

// An action applies its edits and then runs its Run; an edit that does not apply skips Run, and
// the menu announces that the action failed.
func TestActionRunsAfterItsEditsAndErrRangeSkipsIt(t *testing.T) {
	var ran []string
	h := newHarness(t, "helo", func(e *EditorElement) *EditorElement {
		return e.CodeActions(CodeActionFunc(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
			return []CodeAction{
				{Title: "Fix spelling", Kind: CodeActionQuickFix, Preferred: true,
					Edits: []Change{{From: 0, To: 4, Text: "hello"}},
					Run:   func() { ran = append(ran, "fix") }},
				{Title: "Broken", Kind: CodeActionQuickFix,
					Edits: []Change{{From: 0, To: 99, Text: "x"}},
					Run:   func() { ran = append(ran, "broken") }},
			}, nil
		}))
	})
	updates := activateAccess(h)
	h.invokeActions()
	h.key(input.KeyEnter, 0)
	if got := h.doc.Text(); got != "hello" {
		t.Fatalf("the text after the action is %q, want %q", got, "hello")
	}
	if !slices.Equal(ran, []string{"fix"}) {
		t.Fatalf("Run calls = %v, want the fix alone, after its edit", ran)
	}
	if h.ed.view.actions.open {
		t.Error("the menu is open after an action was applied")
	}
	h.invokeActions()
	h.key(input.KeyDown, 0)
	h.key(input.KeyEnter, 0)
	if got := h.doc.Text(); got != "hello" {
		t.Errorf("the text after a failed action is %q, want it unchanged", got)
	}
	if !slices.Equal(ran, []string{"fix"}) {
		t.Errorf("Run calls = %v after a failed action, want none", ran)
	}
	h.frame()
	if !slices.Contains(announcements(*updates), "Could not apply the action") {
		t.Errorf("the failure was not announced: %v", announcements(*updates))
	}
}

// Up and Down move the highlight of the menu and not the caret, Enter applies the highlighted
// action without a line break, Tab still indents, and Escape closes the menu and keeps the
// selection.
func TestActionKeysRouteToTheMenu(t *testing.T) {
	ran := 0
	provider := withActions(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
		return []CodeAction{
			{Title: "Fix spelling", Kind: CodeActionQuickFix, Preferred: true,
				Edits: []Change{{From: 0, To: 4, Text: "hello"}}},
			{Title: "Other", Kind: CodeActionQuickFix, Run: func() { ran++ }},
		}, nil
	})
	h := newHarness(t, "helo\nnext", provider)
	vs := h.ed.view
	h.doc.SetCaret(2)
	h.invokeActions()
	h.key(input.KeyDown, 0)
	if highlighted(vs) != "Other" || h.doc.Caret() != 2 {
		t.Fatalf("down highlights %q and the caret is at %d, want Other and 2", highlighted(vs), h.doc.Caret())
	}
	h.key(input.KeyUp, 0)
	if highlighted(vs) != "Fix spelling" {
		t.Fatalf("up highlights %q, want the preferred action", highlighted(vs))
	}
	h.key(input.KeyEnter, 0)
	if got := h.doc.Text(); got != "hello\nnext" {
		t.Fatalf("enter gave %q, want the action applied with no line break", got)
	}

	h.doc.SetCaret(0)
	h.invokeActions()
	h.key(input.KeyTab, 0)
	if got := h.doc.Text(); !strings.HasPrefix(got, " ") && !strings.HasPrefix(got, "\t") {
		t.Errorf("tab with the menu open gave %q, want the line indented", got)
	}
	if vs.actions.open {
		t.Error("typing a tab left the menu open")
	}

	h.doc.SetSelections(Selection{Anchor: 0, Head: 4})
	h.invokeActions()
	h.key(input.KeyEscape, 0)
	if vs.actions.open {
		t.Fatal("escape left the menu open")
	}
	if p := h.doc.Primary(); p.Anchor != 0 || p.Head != 4 {
		t.Errorf("escape changed the selection to %+v, want it kept", p)
	}
	if ran != 0 {
		t.Errorf("Run ran %d times without an action applied", ran)
	}
}

// The menu lists the quick fixes first, the preferred ones first in each group, with the first
// preferred enabled action highlighted. A disabled action can be highlighted; Enter announces its
// reason and keeps the menu open.
func TestActionOrderAndDisabledActions(t *testing.T) {
	provider := withActions(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
		return []CodeAction{
			{Title: "Organize", Kind: CodeActionSourceOrganizeImports},
			{Title: "Extract", Kind: CodeActionRefactorExtract},
			{Title: "Quick B", Kind: CodeActionQuickFix},
			{Title: "Quick A", Kind: CodeActionQuickFix, Preferred: true},
			{Title: "Import", Kind: CodeActionQuickFix, Preferred: true, Disabled: "no import path"},
		}, nil
	})
	h := newHarness(t, "text", provider)
	updates := activateAccess(h)
	vs := h.ed.view
	h.invokeActions()
	want := []string{"Quick A", "Import", "Quick B", "Extract", "Organize"}
	if got := actionTitles(vs); !slices.Equal(got, want) {
		t.Fatalf("the menu shows %v, want %v", got, want)
	}
	if highlighted(vs) != "Quick A" {
		t.Fatalf("the menu highlights %q, want the first preferred enabled action", highlighted(vs))
	}
	h.key(input.KeyDown, 0)
	if highlighted(vs) != "Import" {
		t.Fatalf("down highlights %q, want the disabled action", highlighted(vs))
	}
	h.key(input.KeyEnter, 0)
	if !vs.actions.open || h.doc.Text() != "text" {
		t.Fatalf("enter on a disabled action: open %v, text %q; want the menu open and the text unchanged", vs.actions.open, h.doc.Text())
	}
	h.frame()
	if !slices.Contains(announcements(*updates), "Import, unavailable: no import path") {
		t.Errorf("the reason was not announced: %v", announcements(*updates))
	}
}

// A change made elsewhere while the menu is open does not close it: the action's edits are mapped
// through it when the action is applied.
func TestExternalEditWhileOpenMapsTheEdits(t *testing.T) {
	h := newHarness(t, "helo world", withActions(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
		return []CodeAction{{Title: "Fix spelling", Edits: []Change{{From: 0, To: 4, Text: "hello"}}}}, nil
	}))
	h.invokeActions()
	if err := h.doc.Edit(Change{From: 10, To: 10, Text: "!"}); err != nil {
		t.Fatal(err)
	}
	if !h.ed.view.actions.open {
		t.Fatal("a change elsewhere closed the menu")
	}
	h.key(input.KeyEnter, 0)
	if got := h.doc.Text(); got != "hello world!" {
		t.Fatalf("the text is %q, want the action applied to the mapped range", got)
	}
}

// A change elsewhere that touches the range of an action's edit drops the action: the menu asks
// again, and the text is left as it is.
func TestOverlappingExternalEditDropsAndAsksAgain(t *testing.T) {
	var calls atomic.Int32
	h := newHarness(t, "helo world", withActions(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
		calls.Add(1)
		return []CodeAction{{Title: "Fix spelling", Edits: []Change{{From: 0, To: 4, Text: "hello"}}}}, nil
	}))
	vs := h.ed.view
	h.invokeActions()
	if err := h.doc.Edit(Change{From: 0, To: 2, Text: "XY"}); err != nil {
		t.Fatal(err)
	}
	h.key(input.KeyEnter, 0)
	h.waitFor("the second answer", func() bool { return calls.Load() == 2 && !vs.actions.loading })
	if got := h.doc.Text(); got != "XYlo world" {
		t.Errorf("the text is %q, want the external edit alone", got)
	}
	if !vs.actions.open {
		t.Error("the menu closed instead of asking again")
	}
}

// The edits of an action are one step of the undo history, however many changes they make.
func TestMultiChangeActionIsOneUndoStep(t *testing.T) {
	h := newHarness(t, "helo world", withActions(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
		return []CodeAction{{Title: "Fix", Edits: []Change{{From: 0, To: 4, Text: "hello"}, {From: 5, To: 10, Text: "there"}}}}, nil
	}))
	h.invokeActions()
	h.key(input.KeyEnter, 0)
	if got := h.doc.Text(); got != "hello there" {
		t.Fatalf("the text is %q, want both edits made", got)
	}
	h.doc.Undo()
	if got := h.doc.Text(); got != "helo world" {
		t.Fatalf("one undo left %q, want the action undone whole", got)
	}
}

// deferredTimers makes the editor's timers wait, until the test runs them with the returned
// functions' list, so that the debounce of the lightbulb can be seen.
func deferredTimers(t *testing.T) *[]func() {
	var fns []func()
	old := afterFunc
	afterFunc = func(d time.Duration, f func()) { fns = append(fns, f) }
	t.Cleanup(func() { afterFunc = old })
	return &fns
}

// The lightbulb shows on the row of a diagnostic once an automatic request has an enabled action
// for it. It hides when the caret leaves the row, when the text changes, and when the editor loses
// focus, and asks again after the debounce. Its click opens the menu.
func TestLightbulbShowsOnADiagnosticRow(t *testing.T) {
	timers := deferredTimers(t)
	queries := make(chan CodeActionQuery, 8)
	h := newHarness(t, "alpha beta\ngamma", withActions(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
		queries <- q
		return []CodeAction{{Title: "Fix", Kind: CodeActionQuickFix, Edits: []Change{{From: 0, To: 5, Text: "omega"}}}}, nil
	}))
	vs := h.ed.view
	h.doc.SetDiagnostics([]Diagnostic{{From: 0, To: 5, Message: "undefined: alpha"}})
	runTimers := func() {
		fns := *timers
		*timers = nil
		for _, f := range fns {
			f()
		}
		h.frame()
	}

	h.doc.SetCaret(2)
	h.frame()
	runTimers()
	h.waitFor("the lightbulb", func() bool { return vs.actions.bulb })
	if vs.actions.bulbRow != 0 {
		t.Fatalf("the lightbulb is on row %d, want the diagnostic's row 0", vs.actions.bulbRow)
	}
	q := <-queries
	if q.Trigger != CodeActionAutomatic || q.From != 2 || q.To != 2 {
		t.Errorf("the automatic query is %+v", q)
	}
	if len(q.Diagnostics) != 1 || q.Diagnostics[0].Message != "undefined: alpha" {
		t.Errorf("the query's diagnostics are %+v, want the one on the caret", q.Diagnostics)
	}

	h.doc.SetCaret(len(h.doc.Text()))
	h.frame()
	if vs.actions.bulb {
		t.Fatal("the lightbulb is still shown on a row without a diagnostic")
	}
	runTimers()
	if len(queries) != 0 {
		t.Errorf("the provider was asked about a row without a diagnostic")
	}

	h.doc.SetCaret(2)
	h.frame()
	runTimers()
	h.waitFor("the lightbulb again", func() bool { return vs.actions.bulb })
	<-queries

	h.typeText("x")
	h.frame()
	if vs.actions.bulb {
		t.Fatal("the lightbulb is still shown after an edit")
	}
	runTimers()
	h.waitFor("the lightbulb after the edit", func() bool { return vs.actions.bulb })
	<-queries

	vs.focused = false // the blur handler's effect on the editor
	vs.closePopups()
	vs.touch()
	h.frame()
	if vs.actions.bulb {
		t.Error("the lightbulb is still shown after the editor lost focus")
	}
}

// An editor whose actions are all disabled shows no lightbulb.
func TestLightbulbNeedsAnEnabledAction(t *testing.T) {
	timers := deferredTimers(t)
	asked := make(chan struct{}, 4)
	h := newHarness(t, "alpha", withActions(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
		asked <- struct{}{}
		return []CodeAction{{Title: "Import", Disabled: "no import path"}}, nil
	}))
	vs := h.ed.view
	h.doc.SetDiagnostics([]Diagnostic{{From: 0, To: 5, Message: "undefined: alpha"}})
	h.doc.SetCaret(1)
	h.frame()
	fns := *timers
	*timers = nil
	for _, f := range fns {
		f()
	}
	h.frame() // runs the request the timer posted
	<-asked
	h.waitFor("the answer", func() bool { return !vs.actions.auto.pending() })
	if vs.actions.bulb || !vs.actions.keyed {
		t.Errorf("the lightbulb is %v after disabled actions only, want hidden and not asked again", vs.actions.bulb)
	}
}

// The lightbulb's click opens the menu at the caret, asked as an invoked request.
func TestLightbulbClickOpensTheMenu(t *testing.T) {
	timers := deferredTimers(t)
	triggers := make(chan CodeActionTrigger, 4)
	h := newHarness(t, "alpha", withActions(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
		triggers <- q.Trigger
		return []CodeAction{{Title: "Fix", Kind: CodeActionQuickFix}}, nil
	}))
	vs := h.ed.view
	h.doc.SetDiagnostics([]Diagnostic{{From: 0, To: 5, Message: "undefined: alpha"}})
	h.doc.SetCaret(1)
	h.frame()
	fns := *timers
	*timers = nil
	for _, f := range fns {
		f()
	}
	h.frame() // runs the request the timer posted
	h.waitFor("the lightbulb", func() bool { return vs.actions.bulb })
	if got := <-triggers; got != CodeActionAutomatic {
		t.Fatalf("the lightbulb was asked with trigger %v, want automatic", got)
	}
	r, _ := h.rt.Find("wrap")
	g := vs.geo
	h.click(r.X+1+g.markW/2, r.Y+1+g.lineH/2, 0, 1)
	h.waitFor("the menu", func() bool { return vs.actions.open && !vs.actions.loading })
	if got := <-triggers; got != CodeActionInvoked {
		t.Errorf("the click asked with trigger %v, want invoked", got)
	}
	if got := actionTitles(vs); !slices.Equal(got, []string{"Fix"}) {
		t.Errorf("the menu shows %v", got)
	}
}

// Read-only editors have no code actions: Ctrl+. does nothing.
func TestReadOnlyEditorHasNoCodeActions(t *testing.T) {
	asked := false
	h := newHarness(t, "text", func(e *EditorElement) *EditorElement {
		return e.ReadOnly(true).CodeActions(CodeActionFunc(func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
			asked = true
			return []CodeAction{{Title: "Fix"}}, nil
		}))
	})
	h.key(input.KeyPeriod, input.ModCtrl)
	h.frame()
	if h.ed.view.actions.open || asked {
		t.Errorf("a read-only editor opened the menu (open %v) or asked the provider (%v)", h.ed.view.actions.open, asked)
	}
}
