package ui

import "testing"

// Each side has its own setter, and the setters leave the other sides alone.
func TestDivSideSetters(t *testing.T) {
	d := Div().Padding(4).PaddingLeft(10).PaddingRight(12).PaddingTop(1).PaddingBottom(2).
		Margin(7).MarginLeft(3).MarginRight(5)
	if want := (Edges{Top: 1, Right: 12, Bottom: 2, Left: 10}); d.style.Padding != want {
		t.Errorf("padding = %+v, want %+v", d.style.Padding, want)
	}
	if want := (Edges{Top: 7, Right: 5, Bottom: 7, Left: 3}); d.style.Margin != want {
		t.Errorf("margin = %+v, want %+v", d.style.Margin, want)
	}
}

// PaddingEdges replaces every side, so a side left at zero loses the padding
// that Padding set.
func TestPaddingEdgesReplacesAllSides(t *testing.T) {
	d := Div().Padding(8).PaddingEdges(Edges{Top: 2})
	if want := (Edges{Top: 2}); d.style.Padding != want {
		t.Errorf("padding = %+v, want %+v", d.style.Padding, want)
	}
}
