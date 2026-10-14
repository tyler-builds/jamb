// Package inspect is the data of the visual inspector (see app/inspect.go): what
// it shows about an element of an inspected window, and the frame timings of
// the window. The ui package produces these values for the inspector, on the
// UI thread; this package does not depend on ui, so its tests run without a
// window.
package inspect

import "time"

// Box is a rectangle in logical pixels.
type Box struct{ X, Y, W, H float32 }

// Edges are per-side insets in logical pixels, in the order top, right, bottom,
// left.
type Edges struct{ Top, Right, Bottom, Left float32 }

// The sources of a style value, as the inspector names them. A field that a
// state changes says which state, such as "state: hover".
const (
	SourceElement = "element method" // set on an element that is not a widget part
	SourceWidget  = "widget part"    // set on a widget part: defaults, theme and Styles merged by the widget
	SourceTheme   = "theme part"     // a widget part whose value equals the theme's value for that part
	SourceSizing  = "sizing method"  // set by a sizing method of the widget, such as Width
	SourceEdit    = "live edit"      // set by the inspector, and dropped when the element is rebuilt
	SourceState   = "state: "        // followed by the state: hover, pressed, focus, focus visible, focus within, drag, drop target
)

// Element is one element of an inspected window.
type Element struct {
	ID       uint64
	Parent   uint64   // 0 for the root
	Children []uint64 // in tree order
	// Kind is "div", "text", "icon", "image" or "editor".
	Kind string
	// Key is the element's key, or "" when it has none.
	Key string
	// Part is the widget part the element draws, such as "Checkbox.Box", or "".
	Part string
	// Boundary is set when the element is the root of a component's output, and
	// Component names the component's render function then.
	Boundary  bool
	Component string
	// Hidden is set when the element is not displayed (Display(false)).
	Hidden bool
	// Text is the text of a text element, shortened to 40 characters.
	Text string
	// Box is the border box in window coordinates, and Padding, Margin and Border
	// the insets around the content. ContentW and ContentH are the size of the
	// laid-out content.
	Box                Box
	Padding, Margin    Edges
	Border             Edges
	ContentW, ContentH float32
	// OverflowX and OverflowY are "visible", "hidden" or "scroll".
	OverflowX, OverflowY string
}

// Field is one value of a resolved style: its name, its value as text, and the
// source the value came from (one of the Source constants).
type Field struct {
	Name   string
	Value  string
	Source string
}

// Access is the accessibility node of an element: its role and name, its states
// and actions, and the warnings that JAMB_A11Y_WARN reports for it.
type Access struct {
	Exposed     bool // the element is in the accessibility tree
	Role        string
	Name        string
	Description string
	Value       string
	States      []string
	Actions     []string
	Warnings    []string
}

// Target is what the inspector reads and changes in an inspected window. The
// ui package implements it; every method runs on the UI thread.
type Target interface {
	// Attach turns the inspector's state on in the window, and calls changed
	// (on the UI thread) when the selection or the picking changes. Detach
	// turns it off: the overlay goes, and the live edits are dropped.
	Attach(changed func())
	Detach()
	// Root is the ID of the window's root element, or 0 when there is none.
	Root() uint64
	// Element returns one element; false when it no longer exists.
	Element(id uint64) (Element, bool)
	// Fields returns the set fields of the element's resolved style, in
	// declaration order, with their sources.
	Fields(id uint64) []Field
	// Access returns the accessibility node of the element.
	Access(id uint64) Access
	// AccessNames returns the role and name of each element that has an
	// accessibility node, as text, by element ID. It is built for the whole
	// window, so the tree can show it on every row.
	AccessNames() map[uint64]string
	// EditableFields lists the style fields that Edit accepts, in declaration
	// order.
	EditableFields() []string
	// Edit sets one style field of the element to value, live, until the element
	// is rebuilt. The value is text: a color as hex, a length as a number of
	// pixels, "50%" or "auto", an edge or corner list as one or four numbers, a
	// number, or true or false.
	Edit(id uint64, field, value string) error
	// ClearEdit drops the element's live edits now.
	ClearEdit(id uint64)
	// Edited reports whether the element has live edits.
	Edited(id uint64) bool
	// GoLiteral returns the element's live edits as a ui.Style literal.
	GoLiteral(id uint64) string
	// Pick turns pick mode on or off. In pick mode the element under the pointer
	// is outlined, and a click selects it and ends the mode. Escape ends it too.
	Pick(on bool)
	Picking() bool
	// Select makes the element the selected one, which is outlined in the window.
	// Zero clears the selection.
	Select(id uint64)
	Selected() uint64
}

// Sample is the cost of one frame of an inspected window, as the frame reported it.
type Sample struct {
	Build, Layout, Paint time.Duration
	Nodes, Rendered      int
	Quads, Glyphs        int
}

// Total is the CPU time of the runtime's stages in the frame.
func (s Sample) Total() time.Duration { return s.Build + s.Layout + s.Paint }

// Perf keeps the last samples of a window, for the sparkline. The zero value
// keeps none; use NewPerf.
type Perf struct {
	buf  []Sample
	next int // the slot the next sample goes to
	n    int // how many samples are kept, at most len(buf)
}

// NewPerf returns a Perf that keeps the last size samples.
func NewPerf(size int) *Perf { return &Perf{buf: make([]Sample, size)} }

// Add records a sample, dropping the oldest when the buffer is full.
func (p *Perf) Add(s Sample) {
	if len(p.buf) == 0 {
		return
	}
	p.buf[p.next] = s
	p.next = (p.next + 1) % len(p.buf)
	p.n = min(p.n+1, len(p.buf))
}

// Samples returns the kept samples, oldest first.
func (p *Perf) Samples() []Sample {
	out := make([]Sample, 0, p.n)
	start := (p.next - p.n + len(p.buf)) % max(len(p.buf), 1)
	for i := 0; i < p.n; i++ {
		out = append(out, p.buf[(start+i)%len(p.buf)])
	}
	return out
}

// Last returns the newest sample, or the zero Sample when there is none.
func (p *Perf) Last() Sample {
	if p.n == 0 {
		return Sample{}
	}
	return p.buf[(p.next-1+len(p.buf))%len(p.buf)]
}

// MaxTotal returns the largest CPU time among the kept samples, so a sparkline
// can scale to it.
func (p *Perf) MaxTotal() time.Duration {
	var m time.Duration
	for _, s := range p.Samples() {
		m = max(m, s.Total())
	}
	return m
}
