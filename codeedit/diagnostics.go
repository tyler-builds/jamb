package codeedit

import (
	"slices"
	"sort"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// Diagnostics are the errors, warnings and notes that a language server or a linter reports on
// the text. They are kept by the Document, so every editor of the document shows them: a wavy
// underline under each range, and a marker in the gutter of the row where one starts, whose shape
// gives the severity. They follow the text through edits, undo and redo, as the selections do,
// until the app sets new ones.

// A Severity is how serious a diagnostic is.
type Severity uint8

const (
	SeverityError   Severity = iota + 1 // an error; the zero Severity is read as one
	SeverityWarning                     // a warning
	SeverityInfo                        // information
	SeverityHint                        // a hint, such as a suggestion to simplify
)

// A Diagnostic is a message about the bytes [From, To) of the text. An empty range marks the
// character at From.
type Diagnostic struct {
	// From and To are the byte range the diagnostic is about.
	From, To int
	// Severity is how serious it is.
	Severity Severity
	// Message is the text shown and announced. Source names what reported it, such as "gopls",
	// and Code is its code there; the editor shows them in the hover.
	Message, Source, Code string
}

// TextVersion returns a number that changes on every change of the text, and only of the text:
// a move of the caret or a fold leaves it as it was, unlike Version. A request to a language
// server records it, so that a result for an older text can be told apart (see
// SetDiagnosticsFor).
func (d *Document) TextVersion() uint64 { return d.textVer }

// SetDiagnostics replaces the diagnostics with ds. Each range is clamped to the text and to
// character boundaries, and a zero severity is SeverityError. The editors that show the document
// draw them at their next frame. The Document is not safe for concurrent use, so an app that
// receives diagnostics on another goroutine sets them on the UI thread, with ui.Context.Post.
func (d *Document) SetDiagnostics(ds []Diagnostic) {
	t := d.text.Snapshot()
	out := make([]Diagnostic, 0, len(ds))
	for _, g := range ds {
		g.From = d.clampBoundary(t, g.From)
		g.To = max(d.clampBoundary(t, g.To), g.From)
		if g.Severity == 0 {
			g.Severity = SeverityError
		}
		g.Severity = min(g.Severity, SeverityHint)
		out = append(out, g)
	}
	slices.SortStableFunc(out, func(a, b Diagnostic) int { return a.From - b.From })
	d.diags, d.diagSet = out, true
	d.redraw()
}

// SetDiagnosticsFor sets the diagnostics as SetDiagnostics does when the text is still the one of
// textVersion (see TextVersion), and reports whether it set them. A result computed for an older
// text is dropped: its offsets are of a text that has changed since.
func (d *Document) SetDiagnosticsFor(textVersion uint64, ds []Diagnostic) bool {
	if textVersion != d.textVer {
		return false
	}
	d.SetDiagnostics(ds)
	return true
}

// Diagnostics returns the diagnostics, sorted by their start.
func (d *Document) Diagnostics() []Diagnostic { return slices.Clone(d.diags) }

// DiagnosticsAt returns the diagnostics whose range holds the byte offset off, its ends included,
// sorted by their start.
func (d *Document) DiagnosticsAt(off int) []Diagnostic {
	var out []Diagnostic
	for _, g := range d.diags {
		if g.From > off {
			break
		}
		if off <= g.To {
			out = append(out, g)
		}
	}
	return out
}

// onRedraw calls fn when the document changes in a way that its editors draw but that is not a
// change of its version, such as new diagnostics, until the returned function is called.
func (d *Document) onRedraw(fn func()) (cancel func()) {
	d.nextID++
	id := d.nextID
	d.redraws = append(d.redraws, listener{id, fn})
	return func() {
		d.redraws = slices.DeleteFunc(d.redraws, func(l listener) bool { return l.id == id })
	}
}

// redraw tells the editors that draw the document to draw it again.
func (d *Document) redraw() {
	for _, l := range slices.Clone(d.redraws) {
		l.fn()
	}
}

// mapDiagnostics moves the diagnostics through the changes. A diagnostic's start stays with the
// text after it and its end with the text before it, so typing at either end does not grow it,
// and a diagnostic whose text is replaced becomes an empty one at the start of the replacement.
func mapDiagnostics(ds []Diagnostic, changes []Change) []Diagnostic {
	if len(ds) == 0 || len(changes) == 0 {
		return ds
	}
	out := ds[:0]
	for _, g := range ds {
		if g.From == g.To {
			g.From = mapThrough(changes, g.From, false)
			g.To = g.From
		} else {
			g.From = mapThrough(changes, g.From, true)
			g.To = max(mapThrough(changes, g.To, false), g.From)
		}
		out = append(out, g)
	}
	slices.SortStableFunc(out, func(a, b Diagnostic) int { return a.From - b.From })
	return out
}

// UTF16Pos returns the row and the column of the byte offset off, where the column counts UTF-16
// code units from the start of the row, as the Language Server Protocol counts positions by
// default. An offset outside the text is clamped to it.
func (d *Document) UTF16Pos(off int) (row, col int) {
	t := d.text.Snapshot()
	off = d.clampBoundary(t, off)
	row = t.PointAt(off).Row
	return row, t.ByteToUTF16(off) - t.ByteToUTF16(t.LineStart(row))
}

// OffsetUTF16 returns the byte offset of the row and the UTF-16 column, as UTF16Pos gives them. A
// row outside the text is clamped to it, a column past the end of the row is the end of the row,
// and a column between the two halves of a surrogate pair is the start of its character.
func (d *Document) OffsetUTF16(row, col int) int {
	t := d.text.Snapshot()
	row = min(max(row, 0), t.LineCount()-1)
	ls, le := t.LineStart(row), d.LineEnd(row)
	base := t.ByteToUTF16(ls)
	u := min(base+max(col, 0), t.ByteToUTF16(le))
	for ; u > base; u-- {
		if off, err := t.UTF16ToByte(u); err == nil {
			return off
		}
	}
	return ls
}

// severityName returns the name of the severity s in the strings strs, for an announcement.
func severityName(s Severity, strs *ui.Strings) string {
	switch s {
	case SeverityWarning:
		return strs.CodeEditorWarning
	case SeverityInfo:
		return strs.CodeEditorInfo
	case SeverityHint:
		return strs.CodeEditorHint
	}
	return strs.CodeEditorError
}

// diagnosticStyle returns the part of the style st that marks a diagnostic of severity s.
func diagnosticStyle(st Style, s Severity) ui.Style {
	switch s {
	case SeverityWarning:
		return st.DiagnosticWarning
	case SeverityInfo:
		return st.DiagnosticInfo
	case SeverityHint:
		return st.DiagnosticHint
	}
	return st.DiagnosticError
}

// onRow reports whether the diagnostic shows on the display row dr of a row that ends at le: its
// range overlaps the segment, or, empty, is at a character of it. On the last segment the line
// feed counts, so a diagnostic about the end of a line shows there.
func (g Diagnostic) onRow(dr displayRow, le int) bool {
	if g.From == g.To {
		return g.From >= dr.from && (g.From < dr.to || dr.last(le) && g.From <= le)
	}
	return g.From < dr.end(le) && g.To > dr.from
}

// diagnosticsIn returns the diagnostics that may show on the bytes [lo, hi], which are the rows a
// paint shows.
func (d *Document) diagnosticsIn(lo, hi int) []Diagnostic {
	end := sort.Search(len(d.diags), func(i int) bool { return d.diags[i].From > hi })
	var out []Diagnostic
	for _, g := range d.diags[:end] {
		if g.To >= lo {
			out = append(out, g)
		}
	}
	return out
}

// goToDiagnostic selects the next diagnostic after the primary selection (forward) or the one
// before it, wrapping around the text, scrolls it into view and announces it.
func (vs *viewState) goToDiagnostic(forward bool) {
	ds := vs.doc.diags
	if len(ds) == 0 {
		return
	}
	from, _ := vs.doc.Primary().Range()
	var i int
	if forward {
		i = sort.Search(len(ds), func(i int) bool { return ds[i].From > from })
		if i >= len(ds) {
			i = 0
		}
	} else {
		i = sort.Search(len(ds), func(i int) bool { return ds[i].From >= from }) - 1
		if i < 0 {
			i = len(ds) - 1
		}
	}
	g := ds[i]
	vs.doc.SetSelections(Selection{Anchor: g.From, Head: g.To})
	vs.reveal = true
	vs.restart = true
	vs.touch()
	vs.announceDiagnostic(g)
}

// announceDiagnostic says the severity, the line and the message of g, politely.
func (vs *viewState) announceDiagnostic(g Diagnostic) {
	strs := ui.EnglishStrings()
	if vs.ctx != nil {
		strs = vs.ctx.Strings()
	}
	ui.Announce(strs.CodeEditorDiagnostic(severityName(g.Severity, strs), vs.doc.RowOf(g.From)+1, g.Message), access.Polite)
}

// drawSquiggle draws a wavy underline from x0 to x1 along the bottom of a row that ends at y.
func drawSquiggle(p *ui.Painter, x0, x1, y float32, c ui.Color) {
	const step, height = 2, 2
	path := ui.NewPath().MoveTo(x0, y)
	up := true
	for x := x0; x < x1; {
		nx := min(x+step, x1)
		ny := y
		if up {
			ny = y - height
		}
		path.LineTo(nx, ny)
		up = !up
		x = nx
	}
	p.Stroke(path, ui.StrokeStyle{Width: 1, Color: c})
}

// drawDiagnosticMarker draws the marker of severity s in the gutter column at x, w wide, on the row
// at y: a filled circle for an error, a triangle for a warning, an "i" for information and a small
// dot for a hint, so that the severity does not rest on color alone.
func drawDiagnosticMarker(p *ui.Painter, s Severity, x, w, y, lineH float32, c ui.Color, fontSize float32) {
	cx, cy := x+w/2, y+lineH/2
	r := min(w, lineH) * 0.3
	switch s {
	case SeverityWarning:
		p.Fill(ui.NewPath().MoveTo(cx, cy-r).LineTo(cx+r, cy+r*0.8).LineTo(cx-r, cy+r*0.8).Close(), c)
	case SeverityInfo:
		p.Text(cx, y, "i", ui.TextFormat{FontSize: fontSize, Mono: true, Color: c, Align: ui.Center})
	case SeverityHint:
		p.Fill(ui.NewPath().Circle(cx, cy, r*0.45), c)
	default:
		p.Fill(ui.NewPath().Circle(cx, cy, r), c)
	}
}
