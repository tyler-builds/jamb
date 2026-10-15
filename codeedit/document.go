package codeedit

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/internal/rope"
)

// undoGroupTime is how long after a typing or deleting step another one of the
// same kind joins its undo step.
const undoGroupTime = time.Second

// maxUndo is how many steps the undo history keeps.
const maxUndo = 1000

// textCopies counts the copies of the whole text that Document.Text makes. The tests read it
// to check that a query, such as a range read of the access model, does not make one.
var textCopies int

// A Document is the text of a code editor: its lines, its selections, the folded
// lines and the undo history. Several editors can show one Document, and each
// shows its selections, so that a change made in one appears in the others.
//
// Positions are byte offsets into the text, and rows are zero-based line numbers.
// A line ends at a line feed; a carriage return before it is kept as part of the
// line. The Document is not safe for concurrent use: it is changed on the UI
// thread, as the rest of the UI is.
type Document struct {
	text    *rope.Rope
	sels    []Selection // sorted by start, never empty, never overlapping
	primary int         // the index of the primary selection in sels
	goal    int         // the cell a run of vertical moves keeps, when goalOK
	goalOK  bool
	tab     int    // the width of a tab stop in cells, for the cells of the vertical moves (see SetTabSize)
	folds   []fold // sorted by start, never nested
	undo    []state
	redo    []state
	last    stepKind // the kind of the latest step, for grouping typing
	lastAt  time.Time

	version  uint64 // changes on every text, selection and fold change
	textVer  uint64 // changes on every text change
	cache    string // Text, as of textVer cacheVer
	cacheVer uint64
	cacheOK  bool

	now       func() time.Time
	listeners []listener
	nextID    int
	edits     []editHook
}

// A Selection is a range of the text with a caret. Anchor is where the selection
// was started and Head is the caret: the selection is the text between them, and
// an empty selection is a plain caret.
type Selection struct {
	Anchor, Head int
}

// Empty reports whether the selection is a caret, with no text selected.
func (s Selection) Empty() bool { return s.Anchor == s.Head }

// Range returns the start and the end of the selected text, in bytes.
func (s Selection) Range() (from, to int) { return min(s.Anchor, s.Head), max(s.Anchor, s.Head) }

// A Change replaces the bytes [From, To) of the text with Text. An insertion has
// From equal to To, and a deletion has an empty Text.
type Change struct {
	From, To int
	Text     string
}

// stepKind says what a step of the undo history did, so that steps of the same
// kind typed or deleted close together form one step.
type stepKind uint8

const (
	stepOther stepKind = iota
	stepTyping
	stepDeleting
)

// state is a point of the undo history: the text and the selections (and folds)
// at that point.
type state struct {
	text    rope.Text
	sels    []Selection
	primary int
	folds   []fold
}

// A listener is a function registered with OnChange, with the id that cancels it.
type listener struct {
	id int
	fn func()
}

// editHook is a function registered with OnEdit, with the id that cancels it.
type editHook struct {
	id int
	fn func(Change)
}

// ErrRange is returned by Edit and Replace for a change outside the text, or one
// that does not start and end on a character boundary, or that overlaps another.
var ErrRange = errors.New("codeedit: change out of range")

// NewDocument returns a document with the text. Invalid UTF-8 is replaced with
// U+FFFD, and a CR LF pair is read as a line feed, so Text returns the text with
// line feeds only. The caret starts at the beginning of the text.
func NewDocument(text string) *Document {
	d := &Document{now: time.Now, sels: []Selection{{}}, tab: 4}
	d.load(text)
	return d
}

// SetTabSize sets the width of a tab stop, in cells, that the commands use. A vertical move
// keeps its column in cells, as the editor shows them, so the tab stops decide where the caret
// lands after a tab. The editor sets it from its TabSize. A width below 1 is 1. It is 4 by default.
func (d *Document) SetTabSize(n int) {
	d.tab = max(n, 1)
}

// load replaces the text with s, normalized as NewDocument describes.
func (d *Document) load(s string) {
	s = strings.ReplaceAll(strings.ToValidUTF8(s, "�"), "\r\n", "\n")
	r, err := rope.NewString(s)
	if err != nil {
		panic(fmt.Sprintf("codeedit: normalized text is not valid UTF-8: %v", err))
	}
	d.text = r
}

// SetText replaces the text and the selections, and clears the undo history, as
// when a file is loaded. The caret goes to the beginning of the text, and the
// folds are removed. It is not an undo step.
func (d *Document) SetText(s string) {
	old := d.Text()
	d.load(s)
	from, to, text := textDiff(old, d.Text())
	if from != to || text != "" {
		for _, h := range slices.Clone(d.edits) {
			h.fn(Change{From: from, To: to, Text: text})
		}
	}
	d.sels, d.primary, d.folds = []Selection{{}}, 0, nil
	d.undo, d.redo, d.last = nil, nil, stepOther
	d.textVer++
	d.changed()
}

// Text returns the text, with line feeds only.
func (d *Document) Text() string {
	if !d.cacheOK || d.cacheVer != d.textVer {
		textCopies++
		d.cache = d.text.Snapshot().String()
		d.cacheVer, d.cacheOK = d.textVer, true
	}
	return d.cache
}

// Len returns the length of the text in bytes.
func (d *Document) Len() int { return d.text.Snapshot().Len() }

// LineCount returns the number of lines. A text with no line feed has one line,
// and a text that ends with a line feed has an empty last line.
func (d *Document) LineCount() int { return d.text.Snapshot().LineCount() }

// LineStart returns the byte offset of the first byte of row. It panics if row is
// out of range.
func (d *Document) LineStart(row int) int { return d.text.Snapshot().LineStart(row) }

// LineEnd returns the byte offset of the line feed that ends row, or the length of
// the text for the last line.
func (d *Document) LineEnd(row int) int {
	t := d.text.Snapshot()
	if row+1 < t.LineCount() {
		return t.LineStart(row+1) - 1
	}
	return t.Len()
}

// Line returns the text of row, without its line feed.
func (d *Document) Line(row int) string {
	t := d.text.Snapshot()
	return string(t.Slice(t.LineStart(row), d.LineEnd(row)))
}

// RowOf returns the row that holds the byte offset off.
func (d *Document) RowOf(off int) int { return d.text.Snapshot().PointAt(off).Row }

// Version returns a number that changes on every change of the text, the
// selections or the folds. Compare it with an earlier value to see whether the
// document changed since.
func (d *Document) Version() uint64 { return d.version }

// OnChange calls fn after each change of the text, the selections or the folds,
// until the returned function is called. fn runs on the UI thread.
func (d *Document) OnChange(fn func()) (cancel func()) {
	d.nextID++
	id := d.nextID
	d.listeners = append(d.listeners, listener{id, fn})
	return func() {
		d.listeners = slices.DeleteFunc(d.listeners, func(l listener) bool { return l.id == id })
	}
}

// OnEdit calls fn for each change of the text, as the change is made, with the
// bytes it replaces in the text as it was just before it. A multi-part edit is
// reported one change at a time, last part first, so that each change's offsets
// hold for the text before it. Undo and SetText report one change for the whole
// difference. The highlighter follows the text through these changes. The
// returned function cancels fn.
func (d *Document) OnEdit(fn func(Change)) (cancel func()) {
	d.nextID++
	id := d.nextID
	d.edits = append(d.edits, editHook{id, fn})
	return func() {
		d.edits = slices.DeleteFunc(d.edits, func(h editHook) bool { return h.id == id })
	}
}

// changed records a change of the text, the selections or the folds, and tells
// the listeners.
func (d *Document) changed() {
	d.version++
	for _, l := range slices.Clone(d.listeners) {
		l.fn()
	}
}

// Selections returns the selections in document order. The primary selection is
// the one the caret and the scrolling follow.
func (d *Document) Selections() []Selection { return slices.Clone(d.sels) }

// PrimaryIndex returns the index of the primary selection in Selections.
func (d *Document) PrimaryIndex() int { return d.primary }

// Primary returns the primary selection.
func (d *Document) Primary() Selection { return d.sels[d.primary] }

// SetSelections replaces the selections, with the last one as the primary. Each
// selection is clamped to the text and to a character boundary; overlapping
// selections are merged. It panics if no selection is given.
func (d *Document) SetSelections(sels ...Selection) {
	if len(sels) == 0 {
		panic("codeedit: SetSelections needs at least one selection")
	}
	d.sels, d.primary = d.normalize(sels, len(sels)-1)
	d.changed()
}

// normalize clamps the selections to the text, sorts them, merges the overlapping
// ones and returns them with the index of the one that was primary.
func (d *Document) normalize(in []Selection, primary int) ([]Selection, int) {
	t := d.text.Snapshot()
	type ranked struct {
		s       Selection
		primary bool
	}
	rs := make([]ranked, len(in))
	for i, s := range in {
		s.Anchor = d.clampBoundary(t, s.Anchor)
		s.Head = d.clampBoundary(t, s.Head)
		rs[i] = ranked{s, i == primary}
	}
	slices.SortStableFunc(rs, func(a, b ranked) int {
		fa, _ := a.s.Range()
		fb, _ := b.s.Range()
		return fa - fb
	})
	out := make([]Selection, 0, len(rs))
	prim := 0
	for _, r := range rs {
		from, to := r.s.Range()
		if n := len(out); n > 0 {
			pf, pt := out[n-1].Range()
			if from < pt || (from == pt && from == to && pf == pt) {
				// Overlapping, or two carets at one point: the merged selection covers both.
				if to > pt {
					merged := Selection{Anchor: pf, Head: to}
					if out[n-1].Head < out[n-1].Anchor {
						merged = Selection{Anchor: to, Head: pf}
					}
					out[n-1] = merged
				}
				if r.primary {
					prim = n - 1
				}
				continue
			}
		}
		out = append(out, r.s)
		if r.primary {
			prim = len(out) - 1
		}
	}
	return out, prim
}

// clampBoundary returns off limited to the text and moved back to a character
// boundary.
func (d *Document) clampBoundary(t rope.Text, off int) int {
	off = min(max(off, 0), t.Len())
	for off > 0 && off < t.Len() && !t.IsBoundary(off) {
		off--
	}
	return off
}

// Caret returns the byte offset of the caret of the primary selection.
func (d *Document) Caret() int { return d.sels[d.primary].Head }

// Edit applies the changes as one step of the undo history. The changes may be in
// any order, but they must not overlap, and each must lie in the text and start
// and end on character boundaries; otherwise Edit makes no change and returns
// ErrRange. The selections move with the text: a selection's end that is inside a
// replaced range goes to the start of the replacement. The caret stays where the
// text around it stays.
func (d *Document) Edit(changes ...Change) error {
	return d.apply(stepOther, changes, nil)
}

// Replace replaces the bytes [from, to) of the text with s, as Edit does for one
// change.
func (d *Document) Replace(from, to int, s string) error {
	return d.Edit(Change{From: from, To: to, Text: s})
}

// apply applies the changes, sorted and checked, as one step of kind k. If sels is
// nil the selections move with the text; otherwise they are sels, with primary
// the index of the primary one, already in the coordinates after the changes.
func (d *Document) apply(k stepKind, changes []Change, post *selectionPost) error {
	changes = slices.Clone(changes)
	slices.SortStableFunc(changes, func(a, b Change) int { return a.From - b.From })
	t := d.text.Snapshot()
	prev := 0
	for _, c := range changes {
		if c.From < prev || c.From > c.To || c.To > t.Len() || !t.IsBoundary(c.From) || !t.IsBoundary(c.To) {
			return ErrRange
		}
		prev = c.To
	}
	if len(changes) == 0 && post == nil {
		return nil
	}
	d.recordStep(k, d.snapshot())
	// The selections are the ones given, or the current ones moved through the changes.
	sels, primary := d.mapSelections(changes), d.primary
	if post != nil {
		sels, primary = post.sels, post.primary
	}
	d.goalOK = false
	// The changes are applied last part first, so that each one's offsets hold for the text
	// before it, and each is reported to the edit hooks as it is made.
	for i := len(changes) - 1; i >= 0; i-- {
		c := changes[i]
		if c.From == c.To && c.Text == "" {
			continue
		}
		if _, err := d.text.Replace(c.From, c.To, []byte(c.Text)); err != nil {
			panic(fmt.Sprintf("codeedit: edit of checked range failed: %v", err))
		}
		for _, h := range slices.Clone(d.edits) {
			h.fn(c)
		}
	}
	d.sels, d.primary = d.normalize(sels, primary)
	d.folds = mapFolds(d.folds, changes)
	d.clampFolds()
	d.textVer++
	d.changed()
	return nil
}

// selectionPost is the selections that an edit leaves, for the edits that set them.
type selectionPost struct {
	sels    []Selection
	primary int
}

// mapSelections returns the selections moved through the changes.
func (d *Document) mapSelections(changes []Change) []Selection {
	out := make([]Selection, len(d.sels))
	for i, s := range d.sels {
		out[i] = Selection{Anchor: mapThrough(changes, s.Anchor, false), Head: mapThrough(changes, s.Head, false)}
	}
	return out
}

// mapThrough returns the byte offset p after the changes, which are sorted and do
// not overlap. A change that ends at p moves p past its text when after is true;
// a change that starts at p, or an insertion at p when after is false, leaves p
// before its text. A position inside a replaced range goes to the start of the
// replacement.
func mapThrough(changes []Change, p int, after bool) int {
	delta := 0
	for _, c := range changes {
		if c.To < p || (c.To == p && (c.From < c.To || after)) {
			delta += len(c.Text) - (c.To - c.From)
			continue
		}
		if c.From < p && p < c.To {
			return c.From + delta
		}
		break
	}
	return p + delta
}

// snapshot returns the state of the document now, for the undo history.
func (d *Document) snapshot() state {
	return state{
		text:    d.text.Snapshot(),
		sels:    slices.Clone(d.sels),
		primary: d.primary,
		folds:   slices.Clone(d.folds),
	}
}

// recordStep puts the state before a step on the undo history, unless the step joins
// the previous one: typing or deleting of the same kind, within undoGroupTime, forms one
// step. Any new step clears the redo history.
func (d *Document) recordStep(k stepKind, before state) {
	now := d.now()
	grouped := k != stepOther && k == d.last && now.Sub(d.lastAt) < undoGroupTime && len(d.undo) > 0
	if !grouped {
		d.undo = append(d.undo, before)
		if len(d.undo) > maxUndo {
			d.undo = d.undo[1:]
		}
	}
	d.redo = nil
	d.last, d.lastAt = k, now
}

// Undo undoes the latest step and reports whether there was one to undo.
func (d *Document) Undo() bool {
	if len(d.undo) == 0 {
		return false
	}
	st := d.undo[len(d.undo)-1]
	d.undo = d.undo[:len(d.undo)-1]
	d.restore(st, &d.redo)
	return true
}

// Redo redoes the latest undone step and reports whether there was one to redo.
func (d *Document) Redo() bool {
	if len(d.redo) == 0 {
		return false
	}
	st := d.redo[len(d.redo)-1]
	d.redo = d.redo[:len(d.redo)-1]
	d.restore(st, &d.undo)
	return true
}

// restore makes st the current state, and puts the state it replaces on other, the
// history it came from. The text difference is reported to the edit hooks as one change.
func (d *Document) restore(st state, other *[]state) {
	cur := d.snapshot()
	*other = append(*other, cur)
	old := d.Text()
	target := st.text.String()
	from, to, text := textDiff(old, target)
	if from != to || text != "" {
		if _, err := d.text.Replace(from, to, []byte(text)); err != nil {
			panic(fmt.Sprintf("codeedit: undo of a checked text failed: %v", err))
		}
		c := Change{From: from, To: to, Text: text}
		for _, h := range slices.Clone(d.edits) {
			h.fn(c)
		}
	}
	d.sels, d.primary, d.folds = st.sels, st.primary, st.folds
	d.last = stepOther
	d.textVer++
	d.changed()
}

// textDiff returns the one change that turns old into target: the bytes of old
// from the common prefix to the common suffix, and their replacement. The
// boundaries are on character boundaries.
func textDiff(old, target string) (from, to int, text string) {
	// start reports whether both texts have a character boundary at byte i.
	start := func(i, j int) bool {
		return (i >= len(old) || utf8.RuneStart(old[i])) && (j >= len(target) || utf8.RuneStart(target[j]))
	}
	p := 0
	for p < len(old) && p < len(target) && old[p] == target[p] {
		p++
	}
	for p > 0 && !start(p, p) {
		p--
	}
	s := 0
	for s < len(old)-p && s < len(target)-p && old[len(old)-1-s] == target[len(target)-1-s] {
		s++
	}
	for s > 0 && !start(len(old)-s, len(target)-s) {
		s--
	}
	return p, len(old) - s, target[p : len(target)-s]
}

// CanUndo reports whether there is a step to undo.
func (d *Document) CanUndo() bool { return len(d.undo) > 0 }

// CanRedo reports whether there is a step to redo.
func (d *Document) CanRedo() bool { return len(d.redo) > 0 }
