package codeedit

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/internal/rope"
)

// A Move is a caret movement, given to Document.Move.
type Move uint8

const (
	MoveLeft      Move = iota // one character left; a selection collapses to its start
	MoveRight                 // one character right; a selection collapses to its end
	MoveWordLeft              // to the start of the word before
	MoveWordRight             // to the end of the word after
	MoveUp                    // up one shown row, keeping the column
	MoveDown                  // down one shown row, keeping the column
	MoveLineStart             // to the first non-blank character, then to column 0
	MoveLineEnd               // to the end of the line
	MoveDocStart              // to the start of the text
	MoveDocEnd                // to the end of the text
	MovePageUp                // up a page of shown rows
	MovePageDown              // down a page of shown rows
)

// tabStop is the width that Tab indents to when the Document is not told another.
const tabStop = 4

// The brackets and quotes that are closed when typed, and the closers they pair with.
const (
	openers = "([{\"'`"
	closers = ")]}\"'`"
)

// charClass sorts the characters for word movement: white space, the characters of a
// word (letters, digits and the underscore), and the rest, which are punctuation.
func charClass(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return 0
	case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return 1
	}
	return 2
}

// runeBefore returns the character that ends at p, and its size in bytes. It returns
// 0, 0 at the start of the text.
func runeBefore(t textReader, p int) (rune, int) {
	if p <= 0 {
		return 0, 0
	}
	b := t.Slice(max(p-utf8.UTFMax, 0), p)
	r, n := utf8.DecodeLastRune(b)
	return r, n
}

// runeAt returns the character that starts at p, and its size in bytes. It returns 0, 0
// at the end of the text.
func runeAt(t textReader, p int) (rune, int) {
	if p >= t.Len() {
		return 0, 0
	}
	b := t.Slice(p, min(p+utf8.UTFMax, t.Len()))
	r, n := utf8.DecodeRune(b)
	return r, n
}

// textReader is the part of the rope's snapshot that the editing commands read.
type textReader interface {
	Len() int
	Slice(from, to int) []byte
	IsBoundary(off int) bool
}

// lineRuneCol returns how many characters lie between the start of p's row and p.
func (d *Document) lineRuneCol(p int) int {
	row := d.RowOf(p)
	return utf8.RuneCountInString(string(d.textSlice(d.LineStart(row), p)))
}

// textSlice returns the bytes of the text in [from, to).
func (d *Document) textSlice(from, to int) []byte {
	return d.text.Snapshot().Slice(from, to)
}

// cellAt returns the cell of the byte offset p, on the screen, in its row.
func (d *Document) cellAt(p int) int {
	row := d.RowOf(p)
	return cellOfCol(d.Line(row), p-d.LineStart(row), d.tab)
}

// offsetAtCell returns the byte offset nearest to the cell on row, or the end of the row when
// the row is shorter.
func (d *Document) offsetAtCell(row, cell int) int {
	return d.LineStart(row) + colOfCell(d.Line(row), cell, d.tab)
}

// indentText returns the leading white space of row.
func (d *Document) indentText(row int) string {
	line := d.Line(row)
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// wordLeft returns the start of the word before p, on p's row. At the start of a row it
// is the end of the row above.
func (d *Document) wordLeft(p int) int {
	t := d.text.Snapshot()
	row := t.PointAt(p).Row
	if p == d.LineStart(row) {
		if row == 0 {
			return 0
		}
		return d.LineEnd(row - 1)
	}
	line := string(t.Slice(d.LineStart(row), p))
	runes := []rune(line)
	i := len(runes)
	for i > 0 && charClass(runes[i-1]) == 0 {
		i--
	}
	if i > 0 {
		c := charClass(runes[i-1])
		for i > 0 && charClass(runes[i-1]) == c {
			i--
		}
	}
	return d.LineStart(row) + len(string(runes[:i]))
}

// wordRight returns the end of the word after p, on p's row. At the end of a row it is the
// start of the row below.
func (d *Document) wordRight(p int) int {
	t := d.text.Snapshot()
	row := t.PointAt(p).Row
	end := d.LineEnd(row)
	if p == end {
		if row+1 >= t.LineCount() {
			return p
		}
		return d.LineStart(row + 1)
	}
	runes := []rune(string(t.Slice(p, end)))
	i := 0
	for i < len(runes) && charClass(runes[i]) == 0 {
		i++
	}
	if i < len(runes) {
		c := charClass(runes[i])
		for i < len(runes) && charClass(runes[i]) == c {
			i++
		}
	}
	return p + len(string(runes[:i]))
}

// SelectWord selects the word at byte offset p, or the white space or punctuation there.
func (d *Document) SelectWord(p int) {
	t := d.text.Snapshot()
	p = d.clampBoundary(t, p)
	row := t.PointAt(p).Row
	start, end := d.LineStart(row), d.LineEnd(row)
	before, nb := runeBefore(t, p)
	after, na := runeAt(t, p)
	// The class of the word or the run of white space or punctuation at p. A caret at the end
	// of a word belongs to that word.
	c := -1
	switch {
	case na > 0 && p < end:
		c = charClass(after)
	case nb > 0 && p > start:
		c = charClass(before)
	}
	if c < 0 {
		d.setSels([]Selection{{Anchor: p, Head: p}}, 0)
		return
	}
	from, to := p, p
	for from > start {
		r, n := runeBefore(t, from)
		if charClass(r) != c {
			break
		}
		from -= n
	}
	for to < end {
		r, n := runeAt(t, to)
		if charClass(r) != c {
			break
		}
		to += n
	}
	d.setSels([]Selection{{Anchor: from, Head: to}}, 0)
}

// SelectRow selects the whole of row, with its line feed, as a triple click does.
func (d *Document) SelectRow(row int) {
	start, end := d.LineStart(row), d.LineEnd(row)
	if row+1 < d.LineCount() {
		end = d.LineStart(row + 1)
	}
	d.setSels([]Selection{{Anchor: start, Head: end}}, 0)
}

// setSels sets the selections, with primary the index of the primary one, and tells the
// listeners. The vertical goal cell is forgotten.
func (d *Document) setSels(sels []Selection, primary int) {
	d.sels, d.primary = d.normalize(sels, primary)
	d.clampFolds()
	d.goalOK = false
	d.changed()
}

// Move moves every caret by m. With extend the selections grow from their anchors, and
// without it they collapse to the caret. page is how many shown rows a page holds, for
// MovePageUp and MovePageDown. The screen column of a vertical move, in cells, is kept from
// one move to the next, as far as the lines allow, when there is one selection.
func (d *Document) Move(m Move, extend bool, page int) {
	t := d.text.Snapshot()
	sels := slices.Clone(d.sels)
	vertical := m == MoveUp || m == MoveDown || m == MovePageUp || m == MovePageDown
	if !vertical && !extend && (m == MoveLeft || m == MoveRight) {
		for i, s := range sels {
			if !s.Empty() {
				from, to := s.Range()
				p := from
				if m == MoveRight {
					p = to
				}
				sels[i] = Selection{p, p}
			}
		}
	}
	goal := -1
	if len(sels) == 1 && vertical && d.goalOK {
		goal = d.goal
	}
	for i, s := range sels {
		if !extend && (m == MoveLeft || m == MoveRight) && !d.sels[i].Empty() {
			sels[i] = Selection{sels[i].Head, sels[i].Head} // collapsed above
			continue
		}
		h := d.moveHead(t, s.Head, m, page, goal)
		if extend {
			sels[i] = Selection{s.Anchor, h}
		} else {
			sels[i] = Selection{h, h}
		}
	}
	if len(sels) == 1 && vertical {
		// The cell of a run of vertical moves is the one the first move started from.
		if !d.goalOK {
			d.goal, d.goalOK = d.cellAt(d.sels[0].Head), true
		}
		d.sels, d.primary = d.normalize(sels, 0)
		d.clampFolds()
		d.changed()
		return
	}
	d.setSels(sels, d.primary)
}

// moveHead returns where a caret at p goes for m. goal is the cell a vertical move
// keeps, or -1 to use p's cell.
func (d *Document) moveHead(t textReader, p int, m Move, page, goal int) int {
	row := d.RowOf(p)
	switch m {
	case MoveLeft:
		_, n := runeBefore(t, p)
		return p - n
	case MoveRight:
		_, n := runeAt(t, p)
		return p + n
	case MoveWordLeft:
		return d.wordLeft(p)
	case MoveWordRight:
		return d.wordRight(p)
	case MoveLineStart:
		line := d.Line(row)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if p == d.LineStart(row)+indent {
			return d.LineStart(row)
		}
		return d.LineStart(row) + indent
	case MoveLineEnd:
		return d.LineEnd(row)
	case MoveDocStart:
		return 0
	case MoveDocEnd:
		return t.Len()
	case MoveUp, MoveDown, MovePageUp, MovePageDown:
		steps := 1
		if m == MovePageUp || m == MovePageDown {
			steps = max(page, 1)
		}
		cell := d.cellAt(p)
		if goal >= 0 {
			cell = goal
		}
		target := row
		up := m == MoveUp || m == MovePageUp
		for i := 0; i < steps; i++ {
			next := d.nextRow(target)
			if up {
				next = d.prevRow(target)
			}
			if next < 0 {
				break
			}
			target = next
		}
		if target == row {
			if up {
				return 0
			}
			return t.Len()
		}
		return d.offsetAtCell(target, cell)
	}
	return p
}

// plan is one edit of a multi-caret command: the change, and where the selection
// goes after it, as offsets from the start of the change in the text after it.
// A command has one plan for each selection, in document order.
type plan struct {
	ch           Change
	anchor, head int
}

// applyPlans applies the plans as one step of kind k. Each plan's selection is the
// one the plan's change replaces, and the selections after the step are the plans'
// selections, with the primary selection kept.
func (d *Document) applyPlans(k stepKind, plans []plan) error {
	if len(plans) != len(d.sels) {
		panic("codeedit: a command needs one plan for each selection")
	}
	changes := make([]Change, len(plans))
	sels := make([]Selection, len(plans))
	shift := 0
	for i, p := range plans {
		changes[i] = p.ch
		from := p.ch.From + shift
		sels[i] = Selection{Anchor: from + p.anchor, Head: from + p.head}
		shift += len(p.ch.Text) - (p.ch.To - p.ch.From)
	}
	return d.apply(k, changes, &selectionPost{sels: sels, primary: d.primary})
}

// TypeText types s over every selection: each selection is replaced by s. Typing a
// bracket or a quote closes it, and typing a closer that is already next steps over it.
// A bracket or quote typed over a selection wraps the selection in the pair, and keeps
// the selection. Typing white space ends an undo step.
func (d *Document) TypeText(s string) error {
	if s == "" {
		return nil
	}
	k := stepTyping
	if strings.ContainsAny(s, " \t\n") {
		k = stepOther
	}
	if len(s) == 1 && strings.IndexByte(openers+closers, s[0]) >= 0 {
		return d.typePair(s[0])
	}
	plans := make([]plan, len(d.sels))
	for i, sel := range d.sels {
		from, to := sel.Range()
		plans[i] = plan{ch: Change{From: from, To: to, Text: s}, anchor: len(s), head: len(s)}
	}
	return d.applyPlans(k, plans)
}

// typePair handles a typed bracket or quote, as TypeText describes.
func (d *Document) typePair(c byte) error {
	t := d.text.Snapshot()
	quote := c == '"' || c == '\'' || c == '`'
	closer := c
	isOpen := strings.IndexByte(openers, c) >= 0
	if isOpen {
		closer = closers[strings.IndexByte(openers, c)]
	}
	plans := make([]plan, len(d.sels))
	for i, sel := range d.sels {
		from, to := sel.Range()
		if !sel.Empty() && isOpen {
			// Wrap the selection, and keep it selected.
			a := 1
			plans[i] = plan{ch: Change{From: from, To: to, Text: string(c) + string(d.textSlice(from, to)) + string(closer)},
				anchor: a, head: a + (to - from)}
			continue
		}
		next, _ := runeAt(t, from)
		prev, _ := runeBefore(t, from)
		if !isOpen && next == rune(c) && sel.Empty() {
			// Step over the closer that is already there.
			_, n := runeAt(t, from)
			plans[i] = plan{ch: Change{From: from, To: from}, anchor: n, head: n}
			continue
		}
		if isOpen && sel.Empty() && pairsHere(next, prev, quote, c) {
			plans[i] = plan{ch: Change{From: from, To: from, Text: string(c) + string(closer)}, anchor: 1, head: 1}
			continue
		}
		plans[i] = plan{ch: Change{From: from, To: to, Text: string(c)}, anchor: 1, head: 1}
	}
	return d.applyPlans(stepTyping, plans)
}

// pairsHere reports whether an opener typed before next, after prev, closes itself: a
// bracket does so before white space, a closer or the end of the line, and a quote also
// not after a word character, so that an apostrophe in a word stays as typed.
func pairsHere(next, prev rune, quote bool, c byte) bool {
	if next != 0 && !unicode.IsSpace(next) && strings.IndexRune(closers, next) < 0 {
		return false
	}
	if quote {
		return prev == 0 || (!unicode.IsLetter(prev) && !unicode.IsDigit(prev) && prev != '_' && prev != rune(c))
	}
	return true
}

// Newline replaces each selection with a line feed and the indentation of its line. After
// an opening bracket the new line is indented one more tab stop, and a closer typed
// next goes on a line of its own, below the caret, at the indentation of its line.
func (d *Document) Newline(tab int) error {
	t := d.text.Snapshot()
	plans := make([]plan, len(d.sels))
	unit := strings.Repeat(" ", max(tab, 1))
	for i, sel := range d.sels {
		from, to := sel.Range()
		row := t.PointAt(from).Row
		indent := d.indentText(row)
		prev, _ := runeBefore(t, from)
		next, _ := runeAt(t, to)
		text := "\n" + indent
		local := len(text)
		switch {
		case strings.ContainsRune("([{", prev) && next != 0 && strings.ContainsRune(")]}", next):
			// Between a pair: the caret goes on an indented line of its own, above the closer.
			text = "\n" + indent + unit + "\n" + indent
			local = len("\n" + indent + unit)
		case strings.ContainsRune("([{", prev):
			text = "\n" + indent + unit
			local = len(text)
		}
		plans[i] = plan{ch: Change{From: from, To: to, Text: text}, anchor: local, head: local}
	}
	return d.applyPlans(stepOther, plans)
}

// DeleteBackward deletes the selected text, or the character before each caret. Between an
// empty pair of brackets or quotes it deletes both.
func (d *Document) DeleteBackward() error {
	return d.deleteEach(func(t textReader, sel Selection) (int, int) {
		from, to := sel.Range()
		if !sel.Empty() {
			return from, to
		}
		prev, n := runeBefore(t, from)
		if n == 0 {
			return from, from
		}
		next, m := runeAt(t, from)
		if strings.ContainsRune(openers, prev) && m > 0 && strings.ContainsRune(closers, next) && pairOf(prev) == next {
			return from - n, from + m
		}
		return from - n, from
	}, stepDeleting)
}

// DeleteForward deletes the selected text, or the character after each caret.
func (d *Document) DeleteForward() error {
	return d.deleteEach(func(t textReader, sel Selection) (int, int) {
		from, to := sel.Range()
		if !sel.Empty() {
			return from, to
		}
		_, n := runeAt(t, to)
		return to, to + n
	}, stepDeleting)
}

// DeleteWordBackward deletes the selected text, or the text back to the start of the
// word before each caret.
func (d *Document) DeleteWordBackward() error {
	return d.deleteEach(func(_ textReader, sel Selection) (int, int) {
		from, to := sel.Range()
		if !sel.Empty() {
			return from, to
		}
		return d.wordLeft(from), from
	}, stepDeleting)
}

// DeleteWordForward deletes the selected text, or the text to the end of the word after
// each caret.
func (d *Document) DeleteWordForward() error {
	return d.deleteEach(func(_ textReader, sel Selection) (int, int) {
		from, to := sel.Range()
		if !sel.Empty() {
			return from, to
		}
		return to, d.wordRight(to)
	}, stepDeleting)
}

// pairOf returns the closer that pairs with the opener o.
func pairOf(o rune) rune {
	if i := strings.IndexRune(openers, o); i >= 0 {
		return rune(closers[i])
	}
	return 0
}

// deleteEach deletes, for each selection, the range that span returns.
func (d *Document) deleteEach(span func(textReader, Selection) (int, int), k stepKind) error {
	t := d.text.Snapshot()
	plans := make([]plan, len(d.sels))
	for i, sel := range d.sels {
		from, to := span(t, sel)
		plans[i] = plan{ch: Change{From: from, To: to}}
	}
	return d.applyPlans(k, plans)
}

// Paste replaces each selection with text. When there are as many selections as lines
// of text, and there is more than one of each, each selection gets one line, in order.
func (d *Document) Paste(text string) error {
	lines := strings.Split(text, "\n")
	distribute := len(d.sels) > 1 && len(lines) == len(d.sels)
	plans := make([]plan, len(d.sels))
	for i, sel := range d.sels {
		from, to := sel.Range()
		s := text
		if distribute {
			s = lines[i]
		}
		plans[i] = plan{ch: Change{From: from, To: to, Text: s}, anchor: len(s), head: len(s)}
	}
	return d.applyPlans(stepOther, plans)
}

// Copy returns the selected text, one selection after another, each on a line of its own.
// It returns "" when every selection is a caret.
func (d *Document) Copy() string {
	var parts []string
	for _, s := range d.sels {
		if from, to := s.Range(); from != to {
			parts = append(parts, string(d.textSlice(from, to)))
		}
	}
	return strings.Join(parts, "\n")
}

// SelectAll selects the whole text, as one selection.
func (d *Document) SelectAll() {
	d.setSels([]Selection{{Anchor: 0, Head: d.text.Snapshot().Len()}}, 0)
}

// Collapse makes the primary caret the only selection, as Escape does.
func (d *Document) Collapse() {
	p := d.Primary().Head
	d.setSels([]Selection{{p, p}}, 0)
}

// AddCaret adds a caret at the byte offset p, which becomes the primary selection.
func (d *Document) AddCaret(p int) {
	sels := append(slices.Clone(d.sels), Selection{p, p})
	d.setSels(sels, len(sels)-1)
}

// AddCaretVertical adds a caret one shown row above (dir < 0) or below (dir > 0) the
// topmost or bottommost caret, at the same screen cell, and makes it the primary selection.
// It does nothing when there is no such row.
func (d *Document) AddCaretVertical(dir int) {
	base := d.sels[0].Head
	if dir > 0 {
		base = d.sels[len(d.sels)-1].Head
	}
	row := d.RowOf(base)
	target := d.nextRow(row)
	if dir < 0 {
		target = d.prevRow(row)
	}
	if target < 0 {
		return
	}
	p := d.offsetAtCell(target, d.cellAt(base))
	sels := append(slices.Clone(d.sels), Selection{p, p})
	d.setSels(sels, len(sels)-1)
}

// AddNextOccurrence adds the next occurrence of the primary selection's text as a
// selection, after the last selection and wrapping around, and makes it the primary. With
// a caret, it selects the word there instead. It reports whether it selected anything new.
func (d *Document) AddNextOccurrence() bool {
	p := d.Primary()
	from, to := p.Range()
	if from == to {
		d.SelectWord(from)
		return true
	}
	text := d.Text()
	needle := text[from:to]
	start := 0
	for _, s := range d.sels {
		start = max(start, s.Anchor, s.Head)
	}
	at := strings.Index(text[start:], needle)
	if at >= 0 {
		at += start
	} else {
		at = strings.Index(text, needle)
	}
	if at < 0 {
		return false
	}
	for _, s := range d.sels {
		if f, _ := s.Range(); f == at {
			return false
		}
	}
	sels := append(slices.Clone(d.sels), Selection{at, at + len(needle)})
	d.setSels(sels, len(sels)-1)
	return true
}

// SetCaret sets the selections to one caret at byte offset p.
func (d *Document) SetCaret(p int) {
	p = d.clampBoundary(d.text.Snapshot(), p)
	d.setSels([]Selection{{p, p}}, 0)
}

// rowsOf returns the rows that the selections touch, in order, once each. A selection
// that ends at the start of a row does not include that row.
func (d *Document) rowsOf() []int {
	var rows []int
	for _, s := range d.sels {
		from, to := s.Range()
		first, last := d.RowOf(from), d.RowOf(to)
		if to > from && to == d.LineStart(last) {
			last--
		}
		for r := first; r <= last; r++ {
			if n := len(rows); n == 0 || rows[n-1] != r {
				rows = append(rows, r)
			}
		}
	}
	return rows
}

// Indent indents the rows of the selections by one tab stop, or, for a caret on one line,
// inserts spaces to the next tab stop. tab is the width of a tab stop. It is a
// multi-line command: each row gets its indent, and the selections move with the text.
func (d *Document) Indent(tab int) error {
	tab = max(tab, 1)
	if len(d.sels) == 1 && d.sels[0].Empty() {
		p := d.sels[0].Head
		col := d.lineRuneCol(p)
		text := strings.Repeat(" ", tab-col%tab)
		return d.Edit(Change{From: p, To: p, Text: text})
	}
	var changes []Change
	for _, r := range d.rowsOf() {
		changes = append(changes, Change{From: d.LineStart(r), To: d.LineStart(r), Text: strings.Repeat(" ", tab)})
	}
	return d.apply(stepOther, changes, nil)
}

// Outdent removes one tab stop of indentation from the rows of the selections: one tab, or
// up to tab spaces.
func (d *Document) Outdent(tab int) error {
	tab = max(tab, 1)
	var changes []Change
	for _, r := range d.rowsOf() {
		line := d.Line(r)
		n := 0
		switch {
		case strings.HasPrefix(line, "\t"):
			n = 1
		default:
			for n < tab && n < len(line) && line[n] == ' ' {
				n++
			}
		}
		if n > 0 {
			start := d.LineStart(r)
			changes = append(changes, Change{From: start, To: start + n})
		}
	}
	return d.apply(stepOther, changes, nil)
}

// ToggleComment comments or uncomments the rows of the selections with prefix, such as
// "// ". The rows are uncommented when every non-blank one starts with the prefix, after its
// indentation (the prefix's trailing space is optional when uncommenting). Otherwise the
// prefix goes in at the indentation of the least indented non-blank row.
func (d *Document) ToggleComment(prefix string) error {
	if prefix == "" {
		return nil
	}
	rows := d.rowsOf()
	trimmed := strings.TrimRight(prefix, " ")
	all := true
	least := -1
	for _, r := range rows {
		line := d.Line(r)
		if blankLine(line) {
			continue
		}
		body := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(body, trimmed) {
			all = false
		}
		if w := len(line) - len(body); least < 0 || w < least {
			least = w
		}
	}
	var changes []Change
	for _, r := range rows {
		line := d.Line(r)
		if blankLine(line) {
			continue
		}
		start := d.LineStart(r)
		if all {
			body := strings.TrimLeft(line, " \t")
			at := start + len(line) - len(body)
			n := len(trimmed)
			switch {
			case strings.HasPrefix(body, prefix):
				n = len(prefix)
			case strings.HasPrefix(body, trimmed+" "):
				n = len(trimmed) + 1
			}
			changes = append(changes, Change{From: at, To: at + n})
			continue
		}
		changes = append(changes, Change{From: start + least, To: start + least, Text: prefix})
	}
	return d.apply(stepOther, changes, nil)
}

// MatchingBracket returns the byte offset of the bracket that pairs with the one at the
// caret, or before it, and reports whether there is one. It looks at most 20,000 bytes
// away, so that a long text is not searched to its end.
func (d *Document) MatchingBracket(p int) (int, bool) {
	t := d.text.Snapshot()
	if q, ok := bracketAt(t, p); ok {
		return q, true
	}
	if p > 0 {
		return bracketAt(t, p-1)
	}
	return 0, false
}

// bracketWindow is how far, in bytes, MatchingBracket looks for the bracket that pairs with the
// one at the caret. It bounds the copy a search makes, so a long text is never searched to its end.
const bracketWindow = 20000

// bracketAt returns the bracket that pairs with the bracket at byte at, if there is one. It reads
// only the bytes at and near at: the byte is checked first, and a window around it is copied only
// when it is a bracket.
func bracketAt(t rope.Text, at int) (int, bool) {
	if at < 0 || at >= t.Len() {
		return 0, false
	}
	if strings.IndexByte(openers[:3]+closers[:3], t.Slice(at, at+1)[0]) < 0 {
		return 0, false
	}
	lo, hi := max(at-bracketWindow, 0), min(at+bracketWindow, t.Len())
	w := string(t.Slice(lo, hi))
	i := at - lo
	c := w[i]
	if o := strings.IndexByte(openers[:3], c); o >= 0 {
		depth := 0
		for k := i; k < len(w); k++ {
			switch w[k] {
			case openers[o]:
				depth++
			case closers[o]:
				depth--
				if depth == 0 {
					return lo + k, true
				}
			}
		}
		return 0, false
	}
	j := strings.IndexByte(closers[:3], c)
	depth := 0
	for k := i; k >= 0; k-- {
		switch w[k] {
		case closers[j]:
			depth++
		case openers[j]:
			depth--
			if depth == 0 {
				return lo + k, true
			}
		}
	}
	return 0, false
}
