package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/icons"
)

// shrinkRow lays out a row of a fixed-width bar and a short text group in a
// 96 pixel box, which is narrower than the content: the text must shrink.
func shrinkRow(scale int, width float32, label Element) (*Runtime, *node) {
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Div().Width(Px(width)).Row().Align(Center).Gap(12).
			Child(Div().Width(Px(60)).MinWidth(Px(0)).Height(Px(20))).
			Child(Div().Row().Align(Center).Gap(5).
				Child(Icon(icons.Check).Size(13)).
				Child(label)))
	}, 400, 300, 1)
	rt.SetSystemSettings(SystemSettings{TextScale: scale})
	rt.Frame(time.Now())
	return rt, rt.findKey(rt.tree, "label")
}

// widestWord is the width of the widest word of the text of n.
func widestWord(rt *Runtime, n *node) float32 {
	var widest float32
	for _, w := range strings.Fields(n.textEl.text) {
		widest = max(widest, rt.text.Layout(w, n.textStyle(), 1e6).Width)
	}
	return widest
}

// The text of a card keeps its words whole in a shrinking row: it wraps between
// words, and it is never narrower than its widest word. Before the min-content
// rule it shrank to the width of its widest letter.
func TestRowTextKeepsItsWordsWhenShrinking(t *testing.T) {
	for _, scale := range []int{100, 150, 225} {
		rt, n := shrinkRow(scale, 96, Text("18 open tasks").Size(12).Key("label"))
		if n == nil {
			t.Fatal("label not found")
		}
		r, _ := rt.Find("label")
		if widest := widestWord(rt, n); r.W < widest {
			t.Errorf("scale %d: label is %.1f wide, narrower than its widest word (%.1f)", scale, r.W, widest)
		}
		if lines := len(n.textLayout(r.W).Lines); lines < 2 {
			t.Errorf("scale %d: label fits in %d line at %.1f wide; the words should wrap", scale, lines, r.W)
		}
	}
}

// A word wider than its row breaks inside the word, so a URL stays in the row:
// nothing overflows the row, and the URL is on more than one line.
func TestWordWiderThanItsRowBreaksInsideTheWord(t *testing.T) {
	const url = "https://example.invalid/projects/0123456789abcdef0123456789abcdef0123456789abcdef"
	rt := newTestRuntime(func(ctx *Context) Element {
		return Div().Child(Div().Width(Px(96)).Row().
			Child(Text(url).Size(12).Key("label")))
	}, 400, 300, 1)
	rt.Frame(time.Now())
	r, ok := rt.Find("label")
	if !ok {
		t.Fatal("label not found")
	}
	if r.X+r.W > 96.5 {
		t.Errorf("label ends at %.1f, past the 96 row", r.X+r.W)
	}
	n := rt.findKey(rt.tree, "label")
	if lines := len(n.textLayout(r.W).Lines); lines < 2 {
		t.Errorf("URL is on %d line at %.1f wide; it should break inside the word", lines, r.W)
	}
}

// Truncate keeps one line with an ellipsis: it opts out of the minimum, so it may
// shrink below its widest word.
func TestTruncateOptsOutOfTheMinimum(t *testing.T) {
	rt, n := shrinkRow(100, 60, Text("18 open tasks").Size(12).Truncate().Key("label"))
	if n == nil {
		t.Fatal("label not found")
	}
	r, _ := rt.Find("label")
	if widest := widestWord(rt, n); r.W >= widest {
		t.Errorf("truncated label is %.1f wide, not narrower than its widest word (%.1f)", r.W, widest)
	}
}

// NoWrap is one line: it keeps its full width rather than shrink into its words.
func TestNoWrapKeepsItsFullWidth(t *testing.T) {
	rt, n := shrinkRow(100, 96, Text("18 open tasks").Size(12).NoWrap().Key("label"))
	if n == nil {
		t.Fatal("label not found")
	}
	r, _ := rt.Find("label")
	full := rt.text.Layout("18 open tasks", n.textStyle(), 1e6).Width
	if r.W < full-0.5 {
		t.Errorf("NoWrap label is %.1f wide, want its full width %.1f", r.W, full)
	}
}

// An explicit MinWidth replaces the automatic minimum.
func TestExplicitMinWidthOptsOutOfTheMinimum(t *testing.T) {
	rt, n := shrinkRow(100, 60, Text("18 open tasks").Size(12).Style(Style{MinWidth: Px(0)}).Key("label"))
	if n == nil {
		t.Fatal("label not found")
	}
	r, _ := rt.Find("label")
	if widest := widestWord(rt, n); r.W >= widest {
		t.Errorf("label with MinWidth 0 is %.1f wide, not narrower than its widest word (%.1f)", r.W, widest)
	}
}

// A rich text takes the minimum of its spans, so its words stay whole too.
func TestRichTextKeepsItsWordsWhenShrinking(t *testing.T) {
	rt, n := shrinkRow(225, 96, Rich(Seg("18 "), Seg("open tasks").Bold()).Size(12).Key("label"))
	if n == nil {
		t.Fatal("label not found")
	}
	r, _ := rt.Find("label")
	if widest := widestWord(rt, n); r.W < widest {
		t.Errorf("rich label is %.1f wide, narrower than its widest word (%.1f)", r.W, widest)
	}
}

// BenchmarkShrinkingTextRows is a frame of 100 rows, each a fixed bar and a text
// that shrinks, laid out again at a new width every iteration, so the minimum
// sizes are measured each time, with the text layouts served from the cache.
func BenchmarkShrinkingTextRows(b *testing.B) {
	rt := newTestRuntime(func(ctx *Context) Element {
		list := Div().Gap(8)
		for range 100 {
			list.Child(Div().Row().Align(Center).Gap(12).
				Child(Div().Width(Px(60)).MinWidth(Px(0)).Height(Px(20))).
				Child(Div().Row().Align(Center).Gap(5).
					Child(Icon(icons.Check).Size(13)).
					Child(Text("18 open tasks in a shrinking row").Size(12))))
		}
		return list
	}, 300, 300, 1)
	b.ResetTimer()
	for i := range b.N {
		rt.SetViewport(float32(300+i%2*40), 300, 1)
		rt.Frame(time.Now())
	}
}
