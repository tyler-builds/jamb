package ui

// PartState is the state of one part of a widget. A render slot (such as
// TabsElement.RenderTab) is called with it, so the content it returns can
// follow the state. Hovered and Pressed are kept current: the slot's
// component re-renders when they change.
//
// Focused is keyboard focus on the part. For a select option, which is never
// focused itself, it means the option is the highlighted one in the open list.
type PartState struct {
	Hovered      bool // the pointer is over the part
	Pressed      bool // the part is being pressed
	Focused      bool // the part has focus (for a select option: it is highlighted)
	Selected     bool // the part is the selected one
	Disabled     bool // the part cannot be used
	HighContrast bool // high contrast is on; colors are the system colors
}

// plainStyle returns s without the parts that high contrast takes from the
// system colors: colors, gradients, shadows and opacity. Sizes, spacing, font
// size and weight, radius, border widths and transitions are kept. Theme and
// instance overrides pass through it under high contrast, so an app cannot
// replace the system colors or remove a built-in focus ring.
func plainStyle(s Style) Style {
	s.Background, s.Gradient, s.BorderColor, s.TextColor = Color{}, Gradient{}, Color{}, Color{}
	s.Shadows, s.Opacity = nil, 0
	// A clear of a color, a border or a shadow would leave the system color, the
	// border or the focus ring out, so it is dropped with them.
	s.Clear &^= ClearBackground | ClearTextColor | ClearBorder | ClearShadows
	return s
}

// asPart names the part of a widget that this element draws. The name is
// how the tests find the element a part's style reached (see
// TestEveryPartIsApplied); it is not an accessibility property.
func (d *DivElement) asPart(name string) *DivElement {
	d.part = name
	return d
}
