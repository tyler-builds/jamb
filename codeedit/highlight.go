package codeedit

import (
	"slices"

	"github.com/tyler-builds/jamb/codeedit/highlight"
	"github.com/tyler-builds/jamb/ui"
)

// A Highlighter gives an editor its syntax colors. It is satisfied by *highlight.Session. The
// editor sends it each change of the text as it is made, and paints with the spans it has.
//
// The contract, which the session keeps:
//   - Edit is called on the UI thread for each change, with the bytes it replaces, and returns
//     at once. The spans shift with the text until the highlighter has new ones, so a keystroke
//     never waits for the highlighter.
//   - SetViewport tells the highlighter which bytes are shown, so that it colors those first.
//   - Spans returns the spans as they are now, and a version that changes when they do. The
//     editor draws whatever it has.
//   - CaptureNames returns the names that highlight.Span.Capture indexes.
//
// Methods may be added to Highlighter in later versions.
type Highlighter interface {
	// Edit tells the highlighter about one change of the text, with the bytes it replaces.
	Edit(e highlight.EditRecord) error
	// SetViewport tells the highlighter which bytes are shown, so that it colors those first.
	SetViewport(lo, hi int) error
	// Spans returns the spans as they are now, and a version that changes when they do.
	Spans() ([]highlight.Span, uint64)
	// CaptureNames returns the names that highlight.Span.Capture indexes.
	CaptureNames() []string
	// Close stops the highlighter and releases what it holds.
	Close() error
}

// spanIndex is the spans of a highlighter sorted by start, for lookups by line. Each span
// keeps its place in the list, which decides which one wins where spans overlap.
type spanIndex struct {
	ver    uint64
	ok     bool
	list   []highlight.Span
	order  []int // the index in the highlighter's list of each sorted span
	maxLen int   // the longest span, so a search can start early enough
}

// build sorts the spans, which is done again only when their version changes.
func (x *spanIndex) build(spans []highlight.Span, ver uint64) {
	if x.ok && x.ver == ver {
		return
	}
	x.ver, x.ok = ver, true
	x.order = x.order[:0]
	for i := range spans {
		x.order = append(x.order, i)
	}
	slices.SortStableFunc(x.order, func(a, b int) int { return spans[a].Start - spans[b].Start })
	x.list = spans
	x.maxLen = 0
	for _, s := range spans {
		x.maxLen = max(x.maxLen, s.End-s.Start)
	}
}

// around returns the spans that may overlap [from, to), in the order of the list, so that the
// later one wins.
func (x *spanIndex) around(from, to int) []int {
	need := from - x.maxLen
	lo, _ := slices.BinarySearchFunc(x.order, need, func(i int, target int) int {
		return x.list[i].Start - target
	})
	var out []int
	for k := lo; k < len(x.order); k++ {
		s := x.list[x.order[k]]
		if s.Start >= to {
			break
		}
		if s.End > from && s.End > s.Start {
			out = append(out, x.order[k])
		}
	}
	slices.Sort(out)
	return out
}

// captureStyles returns the look of each capture name in the theme's syntax styles, mapped with
// highlight.Closest, and whether each has one. A capture with no style keeps the text color.
func captureStyles(names []string, syntax map[string]ui.SyntaxStyle) ([]ui.SyntaxStyle, []bool) {
	st := make([]ui.SyntaxStyle, len(names))
	ok := make([]bool, len(names))
	for i, n := range names {
		_, st[i], ok[i] = highlight.Closest(syntax, n)
	}
	return st, ok
}
