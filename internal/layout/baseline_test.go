package layout

import "testing"

// lineText is a fake one-line text leaf: w wide, h tall, its baseline b below
// its top.
type lineText struct{ w, h, b float32 }

func (t lineText) Measure(float32) (float32, float32) { return t.w, t.h }
func (t lineText) Baseline(float32) (float32, bool)   { return t.b, true }
func (t lineText) MinContentWidth() (float32, bool)   { return t.w, false }

// Baseline items in a row move so that their first baselines meet; an item with
// no text sits on the line with its bottom edge; a container's baseline is that
// of its first text.
func TestRowAlignBaseline(t *testing.T) {
	big := &Node{Measure: lineText{100, 24, 18}}
	small := &Node{Measure: lineText{60, 16, 12}}
	icon := box(Style{Width: Px(10), Height: Px(10)})
	wrapped := box(Style{Direction: Column, Padding: Edges{Top: 4}}, &Node{Measure: lineText{50, 16, 12}})
	row := box(Style{Direction: Row, AlignItems: AlignBaseline}, big, small, icon, wrapped)
	root := box(Style{Direction: Column, AlignItems: AlignStart}, row)
	e := &Engine{}
	e.Compute(root, 400, 300)
	// The shared baseline is 18 from the top of the line.
	expect(t, big, 0, 0, 100, 24)
	expect(t, small, 100, 6, 60, 16)
	expect(t, icon, 160, 8, 10, 10)
	expect(t, wrapped, 170, 2, 50, 20)
	if row.H != 24 {
		t.Errorf("row height %v, want 24: the tallest part above and below the baseline", row.H)
	}
}

// In a column, Baseline is Start.
func TestColumnAlignBaselineIsStart(t *testing.T) {
	a := &Node{Measure: lineText{50, 16, 12}}
	col := box(Style{Direction: Column, AlignItems: AlignBaseline, Width: Px(200)}, a)
	e := &Engine{}
	e.Compute(box(Style{Direction: Column, AlignItems: AlignStart}, col), 400, 300)
	expect(t, a, 0, 0, 50, 16)
}
