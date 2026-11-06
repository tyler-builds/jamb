package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/text"
)

const truncPath = "C:/Users/alex/source/jamb/examples/git/views/app.go"

// laidOut returns the layout the text with key has at its laid-out width.
func laidOut(t *testing.T, rt *Runtime, key string) *text.Layout {
	t.Helper()
	n := rt.byID[rt.keyID(t, key)]
	return n.textLayout(n.lay.W)
}

// TruncateMiddle and TruncateHead fit the text to its box and refit when the
// box changes, and a text that fits is drawn whole.
func TestTruncateMiddleAndHeadRefit(t *testing.T) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Width(Full).Column().
			Child(Text(truncPath).TruncateMiddle().Key("mid")).
			Child(Text(truncPath).TruncateHead().Key("head")).
			Child(Text("app.go").TruncateMiddle().Key("short"))
	}, 160, 300, 1)
	narrow := laidOut(t, rt, "mid")
	if !narrow.Truncated || narrow.Width > 160 || len(narrow.Lines) != 1 {
		t.Fatalf("middle at 160: truncated=%v width=%v lines=%d", narrow.Truncated, narrow.Width, len(narrow.Lines))
	}
	if h := laidOut(t, rt, "head"); !h.Truncated || h.Width > 160 {
		t.Errorf("head at 160: truncated=%v width=%v", h.Truncated, h.Width)
	}
	if s := laidOut(t, rt, "short"); s.Truncated {
		t.Error("a text that fits was truncated")
	}
	rt.SetViewport(260, 300, 1)
	frame(rt)
	wide := laidOut(t, rt, "mid")
	if wide.Width <= narrow.Width || wide.Width > 260 {
		t.Errorf("after widening to 260 the line is %v wide, was %v", wide.Width, narrow.Width)
	}
	rt.SetViewport(800, 300, 1)
	frame(rt)
	if l := laidOut(t, rt, "mid"); l.Truncated {
		t.Error("the path is still truncated when the window is wide enough for it")
	}
}

// Selecting and copying a middle-truncated text keeps the whole string, and so
// does its accessible name.
func TestTruncateMiddleCopiesAndNamesWholeText(t *testing.T) {
	r := textSelectRig(t, Text(truncPath).TruncateMiddle().Width(Px(160)).Selectable())
	n := r.rt.byID[r.rt.keyID(t, "sel")]
	if l := n.st.sel.layout; l == nil || !l.Truncated {
		t.Fatal("the selectable text was not truncated")
	}
	// A drag from the first character past the end selects across the ellipsis.
	x0, y := r.at(t, 0)
	r.drag(x0, y, n.abs.X+n.lay.W+20, y)
	r.key(input.KeyC, ctrl)
	if got := r.rt.memClipboard; got != truncPath {
		t.Errorf("dragging over the whole text copied %q", got)
	}
	r.rt.memClipboard = ""
	r.key(input.KeyA, ctrl)
	r.key(input.KeyC, ctrl)
	if got := r.rt.memClipboard; got != truncPath {
		t.Errorf("Ctrl+A, Ctrl+C copied %q", got)
	}

	a := newAccessRig(func(ctx *Context) Element {
		return Div().Child(Text(truncPath).TruncateHead().Width(Px(120)))
	})
	accesstest.Find(t, a.tree(), access.RoleStaticText, truncPath)
}

// The last of Truncate, TruncateMiddle, TruncateHead and MaxLines wins.
func TestTruncationLastCallWins(t *testing.T) {
	cases := []struct {
		el       *TextElement
		maxLines int
		at       text.Truncation
	}{
		{Text("x").TruncateMiddle().Truncate(), 1, text.TruncateEnd},
		{Text("x").Truncate().TruncateHead(), 1, text.TruncateHead},
		{Text("x").TruncateMiddle().MaxLines(3), 3, text.TruncateEnd},
		{Text("x").MaxLines(3).TruncateMiddle(), 1, text.TruncateMiddle},
	}
	for i, c := range cases {
		if c.el.maxLines != c.maxLines || c.el.truncation != c.at {
			t.Errorf("case %d: maxLines=%d truncation=%d, want %d and %d", i, c.el.maxLines, c.el.truncation, c.maxLines, c.at)
		}
	}
}

// MaxLines clamps a long text to its lines: it measures that many lines tall.
func TestMaxLinesMeasuresClampedHeight(t *testing.T) {
	long := strings.Repeat("a word or two ", 40)
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Column().Width(Px(200)).Child(Text(long).MaxLines(3).Key("clamp"))
	}, 400, 600, 1)
	n := rt.byID[rt.keyID(t, "clamp")]
	l := n.textLayout(n.lay.W)
	if len(l.Lines) != 3 || !l.Truncated {
		t.Fatalf("lines=%d truncated=%v, want 3 and true", len(l.Lines), l.Truncated)
	}
	if n.lay.H > 3*l.LineHeight+0.5 {
		t.Errorf("the text is %v tall, more than 3 lines of %v", n.lay.H, l.LineHeight)
	}
}
