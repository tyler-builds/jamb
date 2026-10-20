package codeedit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// A CompletionTrigger says why the editor asks for completions.
type CompletionTrigger uint8

const (
	TriggerInvoked    CompletionTrigger = iota + 1 // Ctrl+Space, or CmdTriggerSuggest
	TriggerCharacter                               // one of the provider's Triggers was typed
	TriggerIncomplete                              // the text typed since an incomplete list changed
)

// A CompletionQuery is a Query about the caret, with why it is asked and, for TriggerCharacter,
// the character typed.
type CompletionQuery struct {
	Query
	// Trigger is why the editor asks.
	Trigger CompletionTrigger
	// Char is the character typed, for TriggerCharacter.
	Char rune
}

// A CompletionKind is what a completion is, as the list shows it.
type CompletionKind uint8

const (
	CompletionText     CompletionKind = iota // a word of the text
	CompletionKeyword                        // a keyword of the language
	CompletionFunction                       // a function
	CompletionMethod                         // a method
	CompletionVariable                       // a variable
	CompletionField                          // a field
	CompletionType                           // a type
	CompletionModule                         // a module or a package
	CompletionConstant                       // a constant
	CompletionSnippet                        // a snippet of code
)

// A CompletionItem is one entry of a completion list.
type CompletionItem struct {
	// Label is the text the list shows. Detail is shown after it, such as a signature, and Doc
	// is the item's documentation, shown under the list while the item is highlighted.
	Label, Detail, Doc string
	// Insert is the text that replaces the word before the caret; it is Label when empty.
	// Filter is the text that the typed word is matched against; it is Label when empty.
	Insert, Filter string
	// Kind is what the item is.
	Kind CompletionKind
	// Edits are more changes made with the item, such as an import, in the byte offsets of the
	// query's text. They follow the text typed since, and an edit that overlaps the replaced
	// word is dropped.
	Edits []Change
}

// A CompletionList is a provider's answer: the items, and where the text they replace starts.
type CompletionList struct {
	// From is the byte offset of the query's text where the replaced text starts, which ends
	// at the caret. -1 is the start of the word before the caret.
	From int
	// Items are the completions, in the order the list shows them. The editor shows at most
	// maxCompletions of them.
	Items []CompletionItem
	// Incomplete says that the list is not all there is: the editor asks again as the word
	// grows, instead of only filtering the items it has.
	Incomplete bool
}

// A CompletionProvider gives the completions at the caret, as a language server's completion
// does. Triggers returns the characters that open the list when typed, such as ".". Complete runs
// on a goroutine of its own, with a context that is canceled when the answer is no longer wanted.
type CompletionProvider interface {
	Triggers() string
	Complete(ctx context.Context, q CompletionQuery) (CompletionList, error)
}

// CmdTriggerSuggest opens the completion list at the caret.
var CmdTriggerSuggest = ui.NewAction("codeedit.trigger-suggest", "Trigger suggestions")

// maxCompletions is the most items a list keeps; completionRows is how many the popup shows at a
// time, around the highlighted one.
const (
	maxCompletions = 500
	completionRows = 10
)

// completionDebounce is the pause after typing before the editor asks again for an incomplete list.
const completionDebounce = 100 * time.Millisecond

// Completions sets the provider of the completion list. The list opens at the caret when one of
// the provider's trigger characters is typed, and on Ctrl+Space. Typing filters it, matching the
// typed word against each item's Filter or Label in order, ignoring case. Up, Down, PageUp and
// PageDown move the highlight, Enter and Tab accept it, and Escape, a click, the loss of focus and
// a move of the caret off the word close the list. The keyboard focus stays in the editor.
func (e *EditorElement) Completions(p CompletionProvider) *EditorElement { e.complete = p; return e }

// completionState is the completion list of an editor: whether it is open, the start of the word
// it replaces and the caret it was last left at, the items and the ones the typed word matches,
// the highlighted one, the changes made since the request's text, and the request on its way.
type completionState struct {
	open       bool
	loading    bool
	failed     bool
	incomplete bool
	from, end  int
	items      []CompletionItem
	shown      []int
	sel        int
	since      []Change
	slot       asyncSlot[CompletionList]
	again      uint64 // the debounced request for an incomplete list, which a later one makes stale
}

// triggerCompletion asks the provider for the completions at the caret, and opens the list, which
// shows that it is loading until the answer comes.
func (vs *viewState) triggerCompletion(why CompletionTrigger, char rune) {
	if vs.completeP == nil {
		return
	}
	cs := &vs.completion
	caret := vs.doc.Caret()
	if !cs.open {
		cs.from, cs.items, cs.shown, cs.sel = vs.wordStart(caret), nil, nil, 0
	}
	cs.open, cs.loading, cs.failed, cs.end, cs.since = true, true, false, caret, nil
	prov := vs.completeP
	q := CompletionQuery{Query: vs.request(caret), Trigger: why, Char: char}
	cs.slot.run(vs, func(ctx context.Context) (CompletionList, error) { return prov.Complete(ctx, q) },
		func(l CompletionList, err error) { vs.completionAnswer(l, err) })
	vs.touch()
}

// completionAnswer shows the provider's answer, and announces how many items match.
func (vs *viewState) completionAnswer(l CompletionList, err error) {
	cs := &vs.completion
	strs := vs.uiStrings()
	cs.loading = false
	if err != nil {
		cs.failed = true
		ui.Announce(strs.CouldNotLoadSuggestions, access.Polite)
		vs.touch()
		return
	}
	if l.From >= 0 {
		cs.from = min(mapSince(cs.since, l.From), vs.doc.Caret())
	}
	cs.items = l.Items[:min(len(l.Items), maxCompletions)]
	cs.incomplete = l.Incomplete
	vs.filterCompletions()
	ui.Announce(strs.SuggestionCount(len(cs.shown)), access.Polite)
	vs.touch()
}

// mapSince returns the byte offset p of a request's text in the text now, through the changes
// made since, in the order they were made.
func mapSince(since []Change, p int) int {
	for _, c := range since {
		p = mapThrough([]Change{c}, p, false)
	}
	return p
}

// completionEdited follows a change of the text while the list is open: the change is kept, to
// map the answer's offsets, and the request on its way is canceled, since its text is gone.
func (vs *viewState) completionEdited(c Change) {
	cs := &vs.completion
	if !cs.open {
		return
	}
	cs.since = append(cs.since, c)
	cs.from = mapThrough([]Change{c}, cs.from, false)
	if cs.slot.pending() {
		cs.slot.stop()
	}
}

// followCompletion brings the open list up to date after an edit: it closes when the caret has
// left the word, asks again when the answer was canceled by the edit or is incomplete, and is
// filtered with the word as it is now.
func (vs *viewState) followCompletion() {
	cs := &vs.completion
	if !cs.open {
		return
	}
	caret := vs.doc.Caret()
	if caret < cs.from || vs.doc.RowOf(caret) != vs.doc.RowOf(cs.from) {
		vs.closeCompletion()
		return
	}
	cs.end = caret
	switch {
	case cs.loading:
		vs.triggerCompletion(TriggerIncomplete, 0)
	case cs.incomplete:
		cs.again++
		again := cs.again
		afterFunc(completionDebounce, func() {
			if c := vs.latest.Load(); c != nil {
				c.Post(func() {
					if cs.open && cs.again == again {
						vs.triggerCompletion(TriggerIncomplete, 0)
					}
				})
			}
		})
	}
	vs.filterCompletions()
	vs.touch()
}

// typedCompletion opens the list when the text s typed ends with one of the provider's trigger
// characters, in place of the list that was open.
func (vs *viewState) typedCompletion(s string) {
	if vs.completeP == nil || s == "" {
		return
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	if strings.ContainsRune(vs.completeP.Triggers(), r) {
		vs.closeCompletion()
		vs.triggerCompletion(TriggerCharacter, r)
	}
}

// movedCompletion closes the list when a move took the caret off the word it completes.
func (vs *viewState) movedCompletion() {
	cs := &vs.completion
	if caret := vs.doc.Caret(); cs.open && (caret < cs.from || caret > cs.end) {
		vs.closeCompletion()
	}
}

// closeCompletion closes the list and drops its request, and reports whether it was open.
func (vs *viewState) closeCompletion() bool {
	cs := &vs.completion
	cs.slot.stop()
	cs.again++
	if !cs.open {
		return false
	}
	cs.open, cs.loading, cs.failed, cs.items, cs.shown, cs.since = false, false, false, nil, nil, nil
	vs.touch()
	return true
}

// wordStart returns the start of the word that ends at the byte offset p.
func (vs *viewState) wordStart(p int) int {
	from, _ := vs.wordAround(p)
	return min(from, p)
}

// filterCompletions keeps the items that the word typed matches: its characters appear in the
// item's Filter or Label in order, whatever their case. The items that start with the word come
// first, in the provider's order, and then the others.
func (vs *viewState) filterCompletions() {
	cs := &vs.completion
	word := strings.ToLower(string(vs.doc.textSlice(cs.from, max(vs.doc.Caret(), cs.from))))
	var prefix, rest []int
	for i, it := range cs.items {
		f := it.Filter
		if f == "" {
			f = it.Label
		}
		f = strings.ToLower(f)
		switch {
		case strings.HasPrefix(f, word):
			prefix = append(prefix, i)
		case subsequence(word, f):
			rest = append(rest, i)
		}
	}
	cs.shown = append(prefix, rest...)
	cs.sel = min(cs.sel, max(len(cs.shown)-1, 0))
}

// subsequence reports whether the characters of word appear in s in order.
func subsequence(word, s string) bool {
	for _, r := range word {
		i := strings.IndexRune(s, r)
		if i < 0 {
			return false
		}
		s = s[i+utf8.RuneLen(r):]
	}
	return true
}

// completionKey moves the highlight of the open list for the move m, and reports whether it did:
// Up and Down move it by one, wrapping around, and PageUp and PageDown by the rows shown.
func (vs *viewState) completionKey(m Move) bool {
	cs := &vs.completion
	if !cs.open || len(cs.shown) == 0 {
		return false
	}
	n := len(cs.shown)
	switch m {
	case MoveUp:
		cs.sel = (cs.sel - 1 + n) % n
	case MoveDown:
		cs.sel = (cs.sel + 1) % n
	case MovePageUp:
		cs.sel = max(cs.sel-completionRows, 0)
	case MovePageDown:
		cs.sel = min(cs.sel+completionRows, n-1)
	default:
		return false
	}
	ui.Announce(cs.itemAt(cs.sel).Label, access.Polite)
	vs.touch()
	return true
}

// itemAt returns the i-th item shown.
func (cs *completionState) itemAt(i int) CompletionItem { return cs.items[cs.shown[i]] }

// acceptCompletion accepts the highlighted item of the open list, and reports whether there was
// one: the word before each caret is replaced by the item's text, with the item's edits, as one
// step of the undo history.
func (vs *viewState) acceptCompletion() bool {
	cs := &vs.completion
	if !cs.open || len(cs.shown) == 0 {
		return false
	}
	it := cs.itemAt(cs.sel)
	insert := it.Insert
	if insert == "" {
		insert = it.Label
	}
	var extra []Change
	for _, c := range it.Edits {
		c.From, c.To = mapSince(cs.since, c.From), mapSince(cs.since, c.To)
		extra = append(extra, Change{From: c.From, To: max(c.To, c.From), Text: c.Text})
	}
	n := vs.doc.Caret() - cs.from
	vs.closeCompletion()
	vs.edit(func(d *Document) error { return d.complete(n, insert, extra) })
	return true
}

// complete replaces the n bytes before each caret, or as many as its row has, and each selection,
// with insert, and makes the changes extra in the same step. The carets go after the inserted
// text. An extra change that overlaps another change is dropped.
func (d *Document) complete(n int, insert string, extra []Change) error {
	t := d.text.Snapshot()
	var changes []Change
	for _, s := range d.sels {
		a, b := s.Range()
		from := max(a-n, d.LineStart(d.RowOf(a)))
		changes = append(changes, Change{From: d.clampBoundary(t, from), To: b, Text: insert})
	}
	carets := slices.Clone(changes)
	for _, c := range extra {
		if c.From < 0 || c.To > t.Len() || !t.IsBoundary(c.From) || !t.IsBoundary(c.To) {
			continue
		}
		overlaps := false
		for _, o := range changes {
			if c.From < o.To && o.From < c.To || c.From == o.From {
				overlaps = true
				break
			}
		}
		if !overlaps {
			changes = append(changes, c)
		}
	}
	slices.SortStableFunc(changes, func(a, b Change) int { return a.From - b.From })
	sels := make([]Selection, len(carets))
	for i, c := range carets {
		p := mapThrough(changes, c.From, false) + len(insert)
		sels[i] = Selection{Anchor: p, Head: p}
	}
	if err := d.apply(stepOther, changes, &selectionPost{sels: sels, primary: d.primary}); err != nil && len(extra) > 0 {
		return d.complete(n, insert, nil) // the extra changes did not fit; the item goes in alone
	}
	return nil
}

// completionPopup returns the open completion list: an overlay under the start of the word, with
// the items around the highlighted one, the highlighted one selected, or a row that says the list
// is loading or could not load.
func (vs *viewState) completionPopup(st Style) ui.Element {
	cs := &vs.completion
	strs := vs.uiStrings()
	panel := ui.Div().Key("codeedit.completions").Column().Style(st.Popup).
		Overlay(ui.BelowStart).OverlayGap(2).MinWidth(ui.Px(200)).MaxWidth(ui.Px(560)).
		Role(access.RoleList).SelectionContainer().Label(strs.Suggestions).
		OnClickOutside(func() { vs.closeCompletion() }).
		KeepFocus(). // a press in the list leaves the editor focused, as a combobox's list does
		OnMouseDown(func(*ui.MouseEvent) { vs.canvasFocus.Focus() })
	switch {
	case cs.failed:
		panel.Child(ui.Div().Style(st.PopupItem).Child(ui.Text(strs.CouldNotLoadSuggestions)))
	case cs.loading && len(cs.shown) == 0:
		panel.Child(ui.Div().Style(st.PopupItem).Child(ui.Text(strs.LoadingSuggestions)))
	case len(cs.shown) == 0:
		panel.Child(ui.Div().Style(st.PopupItem).Child(ui.Text(strs.NoResults)))
	}
	first := min(max(cs.sel-completionRows/2, 0), max(len(cs.shown)-completionRows, 0))
	for i := first; i < min(first+completionRows, len(cs.shown)); i++ {
		i, it := i, cs.itemAt(i)
		style := st.PopupItem
		if i == cs.sel {
			style = style.Merge(st.PopupItemSelected)
		}
		row := ui.Div().Key(fmt.Sprintf("codeedit.completion/%d", cs.shown[i])).Row().Gap(12).Style(style).
			Role(access.RoleListItem).AccessSelected(i == cs.sel).AccessPosition(i+1, len(cs.shown)).Label(it.Label).
			OnMouseEnter(func() { cs.sel = i; vs.touch() }).
			OnClick(func() { cs.sel = i; vs.acceptCompletion() }).
			Child(ui.Text(it.Label).NoWrap())
		if it.Detail != "" {
			row.Child(ui.Div().AccessHidden().Opacity(0.7).Child(ui.Text(it.Detail).NoWrap()))
		}
		panel.Child(row)
	}
	if len(cs.shown) > 0 {
		if doc := cs.itemAt(cs.sel).Doc; doc != "" {
			panel.Child(ui.Div().Style(st.PopupItem).Opacity(0.8).Child(ui.Text(doc)))
		}
	}
	return vs.popupAnchor(cs.from, panel)
}
