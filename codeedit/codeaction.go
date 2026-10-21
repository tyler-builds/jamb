package codeedit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// A CodeActionKind is the kind of a code action, in the hierarchy of the Language Server Protocol:
// "refactor.extract" is a kind of "refactor", and the menu groups the actions by their kind.
type CodeActionKind string

// The kinds of code actions. A provider may use any kind; these are the ones the menu groups.
const (
	CodeActionQuickFix              CodeActionKind = "quickfix"
	CodeActionRefactor              CodeActionKind = "refactor"
	CodeActionRefactorExtract       CodeActionKind = "refactor.extract"
	CodeActionRefactorInline        CodeActionKind = "refactor.inline"
	CodeActionRefactorRewrite       CodeActionKind = "refactor.rewrite"
	CodeActionSource                CodeActionKind = "source"
	CodeActionSourceOrganizeImports CodeActionKind = "source.organizeImports"
	CodeActionSourceFixAll          CodeActionKind = "source.fixAll"
)

// Within reports whether k is parent or a kind below it, so that "refactor.extract" is within
// "refactor", and "refactorExtra" is not.
func (k CodeActionKind) Within(parent CodeActionKind) bool {
	return k == parent || strings.HasPrefix(string(k), string(parent)+".")
}

// A CodeActionTrigger says why the editor asks for code actions.
type CodeActionTrigger uint8

const (
	CodeActionInvoked   CodeActionTrigger = iota + 1 // Ctrl+., a click on the lightbulb, or the hover's quick fix
	CodeActionAutomatic                              // the probe that decides whether the lightbulb shows
)

// A CodeActionQuery is a Query about the primary selection, with the diagnostics the actions
// may fix and why the editor asks.
type CodeActionQuery struct {
	// Query is the text and the offset of the primary caret.
	Query
	// From and To are the primary selection's range. From equals To for a caret.
	From, To int
	// Diagnostics are the diagnostics that overlap the range, their ends included. With none, they
	// are the diagnostics on the caret's row.
	Diagnostics []Diagnostic
	// Trigger is why the editor asks.
	Trigger CodeActionTrigger
}

// A CodeAction is one action the provider offers for the query: a quick fix, a refactoring or a
// source action. The editor applies its Edits itself and then calls its Run.
type CodeAction struct {
	// Title is the text the menu shows and announces.
	Title string
	// Kind is the kind of the action, which groups it in the menu.
	Kind CodeActionKind
	// Diagnostics are the diagnostics the action fixes. They are informational.
	Diagnostics []Diagnostic
	// Preferred sorts the action first in its group, and highlights it when the menu opens.
	Preferred bool
	// Disabled, when not empty, shows the action dimmed with this reason. It is not applied.
	Disabled string
	// Edits are the changes to this document, in the byte offsets of the query's text. They are
	// made as one step of the undo history.
	Edits []Change
	// Run runs on the UI thread after Edits: a command, a change in another file, or a workspace
	// edit. It may capture state that has since changed, so it should check what it acts on.
	Run func()
}

// A CodeActionProvider offers the code actions for a query, as a language server's code action
// request does. CodeActions runs on a goroutine of its own, with a context that is canceled when
// the answer is no longer wanted.
type CodeActionProvider interface {
	CodeActions(ctx context.Context, q CodeActionQuery) ([]CodeAction, error)
}

// CodeActionFunc is a function that is a CodeActionProvider.
type CodeActionFunc func(ctx context.Context, q CodeActionQuery) ([]CodeAction, error)

// CodeActions calls f.
func (f CodeActionFunc) CodeActions(ctx context.Context, q CodeActionQuery) ([]CodeAction, error) {
	return f(ctx, q)
}

// CodeActions sets the provider of the code actions. Ctrl+. and a click on the lightbulb that
// shows beside a line with a diagnostic open the menu of the actions at the caret; the editor asks
// the provider about each open, and the provider answers with the actions for the query.
func (e *EditorElement) CodeActions(p CodeActionProvider) *EditorElement { e.actionsP = p; return e }

// maxCodeActions is the most actions the menu keeps; actionRows is how many lines the menu shows
// at a time, around the highlighted one.
const (
	maxCodeActions = 100
	actionRows     = 10
)

// actionState is the menu of code actions of an editor: whether it is open and its answer, the
// line that is highlighted, the changes made since the request's text, and the request on its way.
type actionState struct {
	open    bool
	loading bool
	failed  bool
	at      int // the offset the menu hangs from: the caret's when it opened
	items   []CodeAction
	lines   []actionLine // the rows of the menu: a header for each group, and the actions
	sel     int          // the highlighted line, which is an action's
	top     int          // the first line the rows show, kept by follow
	since   []Change
	menu    asyncSlot[[]CodeAction]

	// bulb says whether the lightbulb shows, on the row bulbRow. key is what the lightbulb was asked
	// for, when keyed; again makes a debounced request stale, and auto is the request on its way.
	bulb    bool
	bulbRow int
	key     bulbKey
	keyed   bool
	again   uint64
	auto    asyncSlot[[]CodeAction]
}

// actionsDebounce is the pause after the text or the caret moves before the editor asks whether
// the line has a code action, so that the lightbulb does not ask on every key.
const actionsDebounce = 250 * time.Millisecond

// A bulbKey is what the lightbulb is asked about: the document, its text and its diagnostics as
// they are, and the primary selection.
type bulbKey struct {
	doc  *Document
	text uint64
	diag uint64
	sel  Selection
}

// An actionLine is a row of the menu: the header of a group, when item is -1, or the action
// items[item]. ord is the action's number among the actions, from 1, as the menu announces it.
type actionLine struct {
	item  int
	group int
	ord   int
}

// actionGroup returns the group of the menu that an action of kind k is in: 0 for the quick
// fixes, 1 for the refactorings, 2 for the source actions and 3 for the others.
func actionGroup(k CodeActionKind) int {
	switch {
	case k.Within(CodeActionQuickFix):
		return 0
	case k.Within(CodeActionRefactor):
		return 1
	case k.Within(CodeActionSource):
		return 2
	}
	return 3
}

// actionGroupName returns the name of the group g in the strings strs.
func actionGroupName(g int, strs *ui.Strings) string {
	switch g {
	case 0:
		return strs.CodeEditorQuickFix
	case 1:
		return strs.CodeEditorRefactor
	case 2:
		return strs.CodeEditorSourceAction
	}
	return strs.CodeEditorMoreActions
}

// actionLines returns the rows of the menu for items: the first maxCodeActions of them in their
// groups, the preferred ones first in each group and then in the provider's order, with a header
// row before each group.
func actionLines(items []CodeAction) []actionLine {
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int {
		if ga, gb := actionGroup(items[a].Kind), actionGroup(items[b].Kind); ga != gb {
			return ga - gb
		}
		switch pa, pb := items[a].Preferred, items[b].Preferred; {
		case pa && !pb:
			return -1
		case pb && !pa:
			return 1
		}
		return 0
	})
	idx = idx[:min(len(idx), maxCodeActions)]
	var lines []actionLine
	group := -1
	for n, i := range idx {
		if g := actionGroup(items[i].Kind); g != group {
			group = g
			lines = append(lines, actionLine{item: -1, group: g})
		}
		lines = append(lines, actionLine{item: i, group: group, ord: n + 1})
	}
	return lines
}

// highlightLine returns the line that the menu highlights when it opens: the first preferred
// action that is enabled, else the first enabled one, else the first action.
func highlightLine(lines []actionLine, items []CodeAction) int {
	first, enabled := -1, -1
	for i, l := range lines {
		if l.item < 0 {
			continue
		}
		a := items[l.item]
		switch {
		case a.Disabled == "" && a.Preferred:
			return i
		case a.Disabled == "" && enabled < 0:
			enabled = i
		case first < 0:
			first = i
		}
	}
	if enabled >= 0 {
		return enabled
	}
	return max(first, 0)
}

// actionCount returns how many actions the menu has.
func (as *actionState) actionCount() int {
	n := 0
	for _, l := range as.lines {
		if l.item >= 0 {
			n++
		}
	}
	return n
}

// step returns the action line n actions away from the line i: forward when n is positive. Headers
// are skipped, and a move past the first or the last action wraps around when wrap is set, and
// stops there otherwise.
func (as *actionState) step(i, n int, wrap bool) int {
	dir := 1
	if n < 0 {
		dir, n = -1, -n
	}
	j := i
	for ; n > 0; n-- {
		k := j
		for {
			k += dir
			if wrap {
				k = (k + len(as.lines)) % len(as.lines)
			} else if k < 0 || k >= len(as.lines) {
				return j
			}
			if as.lines[k].item >= 0 {
				break
			}
		}
		j = k
	}
	return j
}

// edgeAction returns the first action line of the menu, or the last one when last is set.
func (as *actionState) edgeAction(last bool) int {
	for i := range as.lines {
		j := i
		if last {
			j = len(as.lines) - 1 - i
		}
		if as.lines[j].item >= 0 {
			return j
		}
	}
	return as.sel
}

// follow scrolls the rows the menu shows the least it takes to keep the highlighted line among
// them. The rows of the menu do not change height while the highlight moves.
func (as *actionState) follow() {
	as.top = min(max(as.top, 0), max(len(as.lines)-actionRows, 0))
	switch {
	case as.sel < as.top:
		as.top = as.sel
	case as.sel >= as.top+actionRows:
		as.top = as.sel - actionRows + 1
	}
}

// actionDiagnostics returns the diagnostics of a query about the range [from, to]: those that
// overlap it, its ends included, or, with none, those on the row of the caret.
func (vs *viewState) actionDiagnostics(from, to int) []Diagnostic {
	var out []Diagnostic
	for _, g := range vs.doc.diags {
		if g.From <= to && from <= g.To {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		row := vs.doc.RowOf(vs.doc.Caret())
		ls, le := vs.doc.LineStart(row), vs.doc.LineEnd(row)
		for _, g := range vs.doc.diags {
			if g.From <= le && g.To >= ls {
				out = append(out, g)
			}
		}
	}
	return out
}

// actionQuery returns the query of the code actions for the primary selection.
func (vs *viewState) actionQuery(trigger CodeActionTrigger) CodeActionQuery {
	from, to := vs.doc.Primary().Range()
	return CodeActionQuery{Query: vs.request(vs.doc.Caret()), From: from, To: to,
		Diagnostics: vs.actionDiagnostics(from, to), Trigger: trigger}
}

// openActions opens the menu of code actions at the primary caret, with a loading row, and asks
// the provider for the actions of the query. The hover and the completion list close: the menu
// takes their place at the caret.
func (vs *viewState) openActions(trigger CodeActionTrigger) {
	if vs.actionsP == nil {
		return
	}
	vs.closeHover()
	vs.closeCompletion()
	st := &vs.actions
	st.at = vs.doc.Caret()
	st.open, st.loading, st.failed = true, true, false
	st.items, st.lines, st.since, st.sel, st.top = nil, nil, nil, 0, 0
	prov, q := vs.actionsP, vs.actionQuery(trigger)
	st.menu.run(vs, func(ctx context.Context) ([]CodeAction, error) { return prov.CodeActions(ctx, q) },
		func(items []CodeAction, err error) { vs.actionsAnswer(items, err) })
	vs.touch()
}

// actionsAnswer shows the provider's answer: the highlight goes to the first preferred action that
// is enabled, and the count of the actions is announced, or that there are none.
func (vs *viewState) actionsAnswer(items []CodeAction, err error) {
	st := &vs.actions
	strs := vs.uiStrings()
	st.loading = false
	if err != nil {
		st.failed = true
		ui.Announce(strs.CodeEditorActionFailed, access.Polite)
		vs.touch()
		return
	}
	st.items = items
	st.lines = actionLines(items)
	st.sel = highlightLine(st.lines, items)
	st.top = 0
	st.follow()
	if n := st.actionCount(); n > 0 {
		ui.Announce(strs.CodeEditorActionCount(n), access.Polite)
	} else {
		ui.Announce(strs.CodeEditorNoCodeActions, access.Polite)
	}
	vs.touch()
}

// closeActions closes the menu of code actions and drops its request, and reports whether it was
// open.
func (vs *viewState) closeActions() bool {
	st := &vs.actions
	st.menu.stop()
	if !st.open {
		return false
	}
	st.open, st.loading, st.failed = false, false, false
	st.items, st.lines, st.since, st.sel, st.top = nil, nil, nil, 0, 0
	vs.touch()
	return true
}

// actionsEdited follows a change of the text. A change made through the editor closes the menu,
// as typing does, and so does a change made elsewhere while the answer is on its way. Any other
// change is kept, to map the edits of the actions and the offset the menu hangs from.
func (vs *viewState) actionsEdited(c Change) {
	st := &vs.actions
	switch {
	case !st.open:
	case vs.selfEdit || st.menu.pending():
		vs.closeActions()
	default:
		st.since = append(st.since, c)
		st.at = mapThrough([]Change{c}, st.at, false)
	}
}

// movedActions closes the menu when the caret has left the row the menu hangs from.
func (vs *viewState) movedActions() {
	st := &vs.actions
	if st.open && vs.doc.RowOf(vs.doc.Caret()) != vs.doc.RowOf(st.at) {
		vs.closeActions()
	}
}

// mapAction maps an edit of an action through the changes made since the request's text, in the
// order they were made. It reports false when one of them touches the edit's range, so that the
// edit no longer applies to the text.
func mapAction(since []Change, c Change) (Change, bool) {
	from, to := c.From, c.To
	for _, s := range since {
		if s.From < to && from < s.To {
			return Change{}, false
		}
		from = mapThrough([]Change{s}, from, true)
		to = mapThrough([]Change{s}, to, false)
	}
	return Change{From: from, To: max(to, from), Text: c.Text}, true
}

// actionKey moves the highlight of the open menu for the move m, and reports whether it did: Up and
// Down move it by one action, wrapping around, and PageUp and PageDown by the rows shown.
func (vs *viewState) actionKey(m Move) bool {
	st := &vs.actions
	if !st.open || len(st.lines) == 0 {
		return false
	}
	switch m {
	case MoveUp:
		st.sel = st.step(st.sel, -1, true)
	case MoveDown:
		st.sel = st.step(st.sel, 1, true)
	case MovePageUp:
		st.sel = st.step(st.sel, -actionRows, false)
	case MovePageDown:
		st.sel = st.step(st.sel, actionRows, false)
	case MoveLineStart:
		st.sel = st.edgeAction(false)
	case MoveLineEnd:
		st.sel = st.edgeAction(true)
	default:
		return false
	}
	st.follow()
	vs.announceAction(st.sel)
	vs.touch()
	return true
}

// announceAction says the title of the action on the line i, and the reason when it is disabled.
func (vs *viewState) announceAction(i int) {
	strs := vs.uiStrings()
	a := vs.actions.items[vs.actions.lines[i].item]
	text := a.Title
	if a.Disabled != "" {
		text = strs.CodeEditorActionUnavailable(a.Title, a.Disabled)
	}
	ui.Announce(text, access.Polite)
}

// applySelectedAction applies the highlighted action of the open menu, for Enter, and reports
// whether there was one.
func (vs *viewState) applySelectedAction() bool {
	st := &vs.actions
	if !st.open || len(st.lines) == 0 {
		return false
	}
	return vs.applyAction(st.sel)
}

// applyAction applies the action on the line i of the menu. A disabled action is not applied: its
// reason is announced, and the menu stays open. Otherwise the menu closes, the edits of the action
// are mapped through the changes made since, and they are made as one step of the undo history.
// When a change made since touches an edit, the action is dropped and the menu asks again. When
// the edits do not apply, the failure is announced and Run does not run. Run runs last.
func (vs *viewState) applyAction(i int) bool {
	st := &vs.actions
	strs := vs.uiStrings()
	a := st.items[st.lines[i].item]
	if a.Disabled != "" {
		ui.Announce(strs.CodeEditorActionUnavailable(a.Title, a.Disabled), access.Polite)
		return true
	}
	edits := make([]Change, 0, len(a.Edits))
	for _, c := range a.Edits {
		m, ok := mapAction(st.since, c)
		if !ok {
			vs.openActions(CodeActionInvoked)
			return true
		}
		edits = append(edits, m)
	}
	vs.closeActions()
	var err error
	vs.edit(func(d *Document) error {
		err = d.Edit(edits...)
		return err
	})
	if err != nil {
		ui.Announce(strs.CodeEditorActionFailed, access.Polite)
		return true
	}
	if a.Run != nil {
		a.Run()
	}
	return true
}

// actionsPopup returns the open menu of code actions: an overlay under the caret, with the group
// headers and the actions, the highlighted one selected, or a row that says the menu is loading,
// could not load, or has no actions.
func (vs *viewState) actionsPopup(st Style) ui.Element {
	as := &vs.actions
	strs := vs.uiStrings()
	panel := ui.Div().Key("codeedit.actions").Column().Style(st.Popup).
		Overlay(ui.BelowStart).OverlayGap(2).MinWidth(ui.Px(200)).MaxWidth(ui.Px(560)).
		Role(access.RoleList).SelectionContainer().Label(strs.CodeEditorCodeActions).
		OnClickOutside(func() { vs.closeActions() }).
		KeepFocus(). // a press in the menu leaves the editor focused, as a completion list does
		OnMouseDown(func(*ui.MouseEvent) { vs.canvasFocus.Focus() })
	switch {
	case as.failed:
		panel.Child(ui.Div().Style(st.PopupItem).Child(ui.Text(strs.CodeEditorActionFailed)))
	case as.loading:
		panel.Child(ui.Div().Style(st.PopupItem).Child(ui.Text(strs.CodeEditorLoadingActions)))
	case len(as.items) == 0:
		panel.Child(ui.Div().Style(st.PopupItem).Child(ui.Text(strs.CodeEditorNoCodeActions)))
	}
	first := min(as.top, max(len(as.lines)-actionRows, 0))
	for i := first; i < min(first+actionRows, len(as.lines)); i++ {
		if l := as.lines[i]; l.item < 0 {
			panel.Child(ui.Div().Style(st.PopupItem).TextColor(st.PopupGroup.TextColor).AccessHidden().
				Child(ui.Text(actionGroupName(l.group, strs)).NoWrap()))
		} else {
			panel.Child(vs.actionRow(st, i, l))
		}
	}
	return vs.popupAnchor("codeedit.actions.anchor", as.at, panel)
}

// syncLightbulb is called at each render. The lightbulb shows on the row of the caret when a
// diagnostic is there and the provider has an enabled action for it. The lightbulb is hidden when
// the editor is not focused, when the provider is not set, when there is no diagnostic at the
// selection or on the caret's row, and when the text, the diagnostics or the selection change; a
// change asks again after the debounce.
func (vs *viewState) syncLightbulb() {
	st := &vs.actions
	key := bulbKey{doc: vs.doc, text: vs.doc.TextVersion(), diag: vs.doc.diagVer, sel: vs.doc.Primary()}
	from, to := key.sel.Range()
	if vs.actionsP == nil || !vs.focused || len(vs.actionDiagnostics(from, to)) == 0 {
		vs.hideLightbulb()
		return
	}
	if st.keyed && st.key == key {
		return // asked about already, and its answer is shown or on its way
	}
	st.bulb = false
	st.auto.stop()
	st.again++
	st.key, st.keyed = key, true
	st.bulbRow = vs.doc.RowOf(key.sel.Head)
	again := st.again
	afterFunc(actionsDebounce, func() {
		if c := vs.latest.Load(); c != nil {
			c.Post(func() {
				if st.again == again {
					vs.probeLightbulb()
				}
			})
		}
	})
}

// hideLightbulb hides the lightbulb and drops its request, and a debounced one that is waiting.
func (vs *viewState) hideLightbulb() {
	st := &vs.actions
	st.bulb, st.keyed = false, false
	st.auto.stop()
	st.again++
}

// probeLightbulb asks the provider for the actions of the primary selection, as an automatic
// request, and shows the lightbulb when one of them is enabled.
func (vs *viewState) probeLightbulb() {
	st := &vs.actions
	prov, q := vs.actionsP, vs.actionQuery(CodeActionAutomatic)
	st.auto.run(vs, func(ctx context.Context) ([]CodeAction, error) { return prov.CodeActions(ctx, q) },
		func(items []CodeAction, err error) {
			st.bulb = err == nil && slices.ContainsFunc(items, func(a CodeAction) bool { return a.Disabled == "" })
			vs.touch()
		})
}

// drawLightbulb draws a lightbulb in the gutter column at x, w wide, on the row at y: a filled bulb
// with its base under it, so that the mark shows that there is an action by its shape, not by its
// color alone.
func drawLightbulb(p *ui.Painter, x, w, y, lineH float32, c ui.Color) {
	r := min(w, lineH) * 0.3
	cx, cy := x+w/2, y+lineH/2-r*0.3
	p.Fill(ui.NewPath().Circle(cx, cy, r), c)
	p.FillRect(cx-r*0.6, cy+r*0.9, r*1.2, r*0.5, c)
}

// actionRow returns the row of the action on the line i of the menu. A disabled action shows its
// reason after its title, dimmed, and its name for assistive technology includes the reason.
func (vs *viewState) actionRow(st Style, i int, l actionLine) ui.Element {
	as := &vs.actions
	a := as.items[l.item]
	style := st.PopupItem
	label := a.Title
	if a.Disabled != "" {
		style = style.Merge(st.PopupItemDisabled)
		label += " — " + a.Disabled
	}
	if i == as.sel {
		style = style.Merge(st.PopupItemSelected)
	}
	row := ui.Div().Key(fmt.Sprintf("codeedit.action/%d", l.item)).Row().Gap(12).Style(style).
		Role(access.RoleListItem).AccessSelected(i == as.sel).AccessPosition(l.ord, as.actionCount()).Label(label).
		// The pointer moves the highlight only when it moves, as the completion list's does.
		OnMouseMove(func(*ui.MouseEvent) {
			if as.sel != i {
				as.sel = i
				vs.touch()
			}
		}).
		OnClick(func() { as.sel = i; vs.applyAction(i) }).
		Child(ui.Text(a.Title).NoWrap())
	if a.Disabled != "" {
		row.Child(ui.Div().AccessHidden().Opacity(0.7).Child(ui.Text(" — " + a.Disabled).NoWrap()))
	}
	return row
}
