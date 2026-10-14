package codeedit

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// barKind is which bar is open above the text: none, find, find and replace, or go to line.
type barKind uint8

const (
	barNone barKind = iota
	barFind
	barReplace
	barGoto
)

// finderKey is what a Finder was made for, so that the editor makes a new one when it changes.
type finderKey struct {
	query string
	opt   FindOptions
}

// openBar shows the bar k, and gives it the keyboard focus when it is built.
func (vs *viewState) openBar(k barKind) {
	vs.bar = k
	vs.focusBar = true
	vs.touch()
}

// closeBar hides the bar, and gives the keyboard back to the text.
func (vs *viewState) closeBar() {
	vs.bar = barNone
	vs.focusBar = false
	if vs.canvasFocus != nil {
		vs.canvasFocus.Focus()
	}
	vs.touch()
}

// findOptions returns the options of the find bar.
func (vs *viewState) findOptions() FindOptions {
	return FindOptions{MatchCase: vs.optCase.Peek(), WholeWord: vs.optWord.Peek(), Regex: vs.optRegex.Peek()}
}

// matches returns the matches of the find bar's query in the document, or nil when there
// is no query or the query is an invalid pattern.
func (vs *viewState) matches() []Match {
	key := finderKey{query: vs.query.Peek(), opt: vs.findOptions()}
	if vs.finder == nil || vs.finderKey != key {
		vs.finderKey = key
		f, err := NewFinder(key.query, key.opt)
		vs.finderErr = err != nil
		vs.finder = f
		if err != nil {
			vs.finder = nil
		}
	}
	if vs.finder == nil {
		return nil
	}
	return vs.finder.Matches(vs.doc)
}

// currentMatch returns the index of the match the bar counts as current: the match the
// primary selection is, or else the first match at or after the caret. It is 0 when the
// caret is past the last match, which wraps.
func (vs *viewState) currentMatch(matches []Match) int {
	p := vs.doc.Primary()
	from, to := p.Range()
	for i, m := range matches {
		if m.From == from && m.To == to {
			return i
		}
	}
	i := sort.Search(len(matches), func(i int) bool { return matches[i].From >= to })
	if i >= len(matches) {
		return 0
	}
	return i
}

// findStep selects the next match after the primary selection (forward) or the previous one
// before it, wrapping around the text, and scrolls it into view.
func (vs *viewState) findStep(forward bool) {
	matches := vs.matches()
	if len(matches) == 0 {
		return
	}
	from, to := vs.doc.Primary().Range()
	var i int
	if forward {
		i = sort.Search(len(matches), func(i int) bool { return matches[i].From >= to })
		if i >= len(matches) {
			i = 0
		}
	} else {
		i = sort.Search(len(matches), func(i int) bool { return matches[i].To > from }) - 1
		if i < 0 {
			i = len(matches) - 1
		}
	}
	m := matches[i]
	vs.doc.SetSelections(Selection{Anchor: m.From, Head: m.To})
	vs.reveal = true
	vs.touch()
}

// replaceCurrent replaces the match the primary selection is with the replacement, and then
// selects the next match. When the selection is not a match, it selects the next one first.
func (vs *viewState) replaceCurrent() {
	matches := vs.matches()
	if len(matches) == 0 {
		return
	}
	from, to := vs.doc.Primary().Range()
	for _, m := range matches {
		if m.From == from && m.To == to {
			_, _ = vs.doc.ReplaceMatches(vs.finder, []Match{m}, vs.repl.Peek())
			vs.findStep(true)
			return
		}
	}
	vs.findStep(true)
}

// replaceAll replaces every match of the find bar's query as one step of the undo history.
func (vs *viewState) replaceAll() {
	matches := vs.matches()
	if len(matches) == 0 {
		return
	}
	_, _ = vs.doc.ReplaceMatches(vs.finder, matches, vs.repl.Peek())
	vs.touch()
}

// goToLine moves the caret to the start of the line the go-to bar gives, unfolding the lines
// that hide it, and closes the bar. A number that is not a line goes to the nearest one.
func (vs *viewState) goToLine() {
	n, err := strconv.Atoi(strings.TrimSpace(vs.gotoLine.Peek()))
	if err != nil {
		return
	}
	row := min(max(n-1, 0), vs.doc.LineCount()-1)
	for vs.doc.hiddenRow(row) {
		vs.doc.Unfold(row)
	}
	vs.doc.SetCaret(vs.doc.LineStart(row))
	vs.reveal = true
	vs.closeBar()
}

// findBar returns the bar above the text: the find row, with the options, the count and the
// buttons, and for replace a second row with the replacement. Enter finds the next match,
// Shift+Enter the previous one, F3 and Shift+F3 do the same, and Escape closes the bar.
func (vs *viewState) findBar(ctx *ui.Context, st Style, readOnly bool) ui.Element {
	s := ctx.Strings()
	bar := ui.Div().Column().Style(st.Find)
	bar.OnKeyDown(func(ev *ui.KeyEvent) {
		switch {
		case ev.Key == input.KeyEscape:
			vs.closeBar()
		case ev.Key == input.KeyEnter && ev.Mods.Has(input.ModShift), ev.Key == input.KeyF3 && ev.Mods.Has(input.ModShift):
			vs.findStep(false)
		case ev.Key == input.KeyEnter, ev.Key == input.KeyF3:
			vs.findStep(true)
		default:
			return
		}
		ev.StopPropagation()
	})
	first := true
	focus := func(ti *ui.TextInputElement) *ui.TextInputElement {
		if first && vs.focusBar {
			first = false
			ti.AutoFocus()
		}
		return ti
	}
	if vs.bar == barGoto {
		row := ui.Div().Row().Gap(6).Align(ui.Center)
		row.Child(focus(ui.TextInput(vs.gotoLine).Label(s.CodeEditorGoToLine).Grow(1)))
		row.Child(ui.Button(s.CodeEditorGo).OnClick(vs.goToLine))
		row.Child(ui.Button(s.CodeEditorClose).OnClick(vs.closeBar))
		bar.Child(row)
		vs.focusBar = false
		return bar
	}
	count := vs.countLabel(s)
	row := ui.Div().Row().Gap(6).Align(ui.Center)
	row.Child(focus(ui.TextInput(vs.query).Label(s.CodeEditorFind).Grow(1)))
	row.Child(ui.Checkbox(s.CodeEditorMatchCase, vs.optCase))
	row.Child(ui.Checkbox(s.CodeEditorWholeWord, vs.optWord))
	row.Child(ui.Checkbox(s.CodeEditorRegex, vs.optRegex))
	row.Child(ui.Text(count).NoWrap())
	row.Child(ui.Button(s.CodeEditorPrevious).OnClick(func() { vs.findStep(false) }))
	row.Child(ui.Button(s.CodeEditorNext).OnClick(func() { vs.findStep(true) }))
	row.Child(ui.Button(s.CodeEditorClose).OnClick(vs.closeBar))
	bar.Child(row)
	if vs.bar == barReplace {
		rrow := ui.Div().Row().Gap(6).Align(ui.Center)
		rrow.Child(ui.TextInput(vs.repl).Label(s.CodeEditorReplace).Grow(1))
		if !readOnly {
			rrow.Child(ui.Button(s.CodeEditorReplaceOne).OnClick(vs.replaceCurrent))
			rrow.Child(ui.Button(s.CodeEditorReplaceAll).OnClick(vs.replaceAll))
		}
		bar.Child(rrow)
	}
	vs.focusBar = false
	return bar
}

// countLabel returns the count of the matches for the find bar: "3 of 12", "No results", or
// the error of an invalid pattern. The query is read from the signal, so the bar renders again
// when it changes.
func (vs *viewState) countLabel(s *ui.Strings) string {
	if vs.query.Get() == "" {
		return ""
	}
	matches := vs.matches()
	if vs.finderErr {
		return s.CodeEditorBadPattern
	}
	if len(matches) == 0 {
		return s.CodeEditorNoMatches
	}
	return s.CodeEditorCount(vs.currentMatch(matches)+1, len(matches))
}
