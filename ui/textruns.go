package ui

import (
	"github.com/tyler-builds/jamb/internal/text"
	"github.com/tyler-builds/jamb/internal/uihost"
)

// A window whose input methods find places in a field's text by where they
// are on screen, as iOS's keyboard trackpad does, lays the field's text out
// itself: in the runs the field reports (see platform.TextContext.Runs). They
// are the pieces of the field's layout (see text.System.Pieces), in the
// field's value, with the syntax that a field with marks hides as runs of no
// width, and lengths in logical pixels at the window's zoom.

// fieldRuns is what textRuns made of an editor's layout, kept in its state.
type fieldRuns struct {
	layout *text.Layout
	hid    [][2]int
	zoom   float32
	runs   []uihost.TextRun
	faces  []text.FaceData
}

// textRuns returns the runs of the text of the editor n as its last paint laid
// it out, and their faces: the same slices while the layout is. They are nil
// while an input method composes, when the layout has the composition in it.
func (rt *Runtime) textRuns(n *node) ([]uihost.TextRun, []text.FaceData) {
	es := n.st.edit
	l := es.layout
	if l == nil || rt.composing == n.id && es.comp.text != "" {
		return nil, nil
	}
	var hid [][2]int
	if es.drawn.hides {
		hid = es.hid
	}
	fr := &es.runs
	if fr.layout == l && fr.zoom == rt.zoom && equalRanges(fr.hid, hid) {
		return fr.runs, fr.faces
	}
	pieces, faces := rt.text.Pieces(l)
	runs := make([]uihost.TextRun, 0, len(pieces)+len(hid))
	z := rt.zoom
	vp, h := 0, 0 // where in the value the runs are, and the next hidden range
	hidden := func(line int) {
		for h < len(hid) && hid[h][0] <= vp {
			if hid[h][1] > vp {
				runs = append(runs, uihost.TextRun{Start: vp, End: hid[h][1], Line: line, Face: -1, Kind: uint8(text.PieceNone)})
				vp = hid[h][1]
			}
			h++
		}
	}
	for _, p := range pieces {
		for s := p.Start; s < p.End; {
			hidden(p.Line)
			next := vp + (p.End - s) // with no hidden range ahead, the rest of the piece
			if h < len(hid) {
				next = hid[h][0]
			}
			take := min(p.End-s, next-vp)
			r := uihost.TextRun{Start: vp, End: vp + take, Line: p.Line, Face: p.Face, Kind: uint8(p.Kind),
				Width: p.Width * z, Spacing: p.Spacing * z}
			if take < p.End-p.Start {
				r.Width = 0 // a piece the hidden syntax splits: its parts' widths are the browser's
			}
			runs = append(runs, r)
			vp += take
			s += take
		}
	}
	hidden(max(len(l.Lines)-1, 0))
	*fr = fieldRuns{layout: l, hid: hid, zoom: z, runs: runs, faces: faces}
	return runs, faces
}

func equalRanges(a, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fieldCarets returns where the focused editor puts its caret at each
// character boundary of its value from the byte start to end, in logical
// pixels at the zoom from the start of its lines, as its last paint laid it
// out (see uihost.TextContext.Carets).
func (rt *Runtime) fieldCarets(start, end int) []uihost.Caret {
	n, es, v := rt.editorOf(rt.focusID)
	if n == nil || es.layout == nil {
		return nil
	}
	l := es.layout
	start, end = clampToRune(v, start), clampToRune(v, end)
	var out []uihost.Caret
	for i := start; i <= end; {
		s := es.drawn.toShown(i)
		x, top := l.Caret(s)
		line := 0
		if l.LineHeight > 0 {
			line = int(top/l.LineHeight + 0.5)
		}
		out = append(out, uihost.Caret{Offset: i, X: x * rt.zoom, Line: line})
		if i == end {
			break
		}
		i++
		for i < end && !runeStartAt(v, i) {
			i++
		}
	}
	return out
}

// runeStartAt reports whether the byte i of v starts a character.
func runeStartAt(v string, i int) bool { return i >= len(v) || v[i]&0xC0 != 0x80 }
