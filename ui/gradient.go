package ui

// Gradient is a linear gradient background. The zero value is unset. A gradient
// replaces the element's Background when both are set. Build one with
// LinearGradient and refine it with Stops and InOklab:
//
//	ui.Div().BackgroundGradient(ui.LinearGradient(135, ui.Hex("#6366f1"), ui.Hex("#ec4899")))
//
// Gradient colors do not animate: a Transition does not ease them, so a hover
// that changes the gradient snaps to the new one.
type Gradient struct {
	Angle        float32 // degrees: 0 points up, 90 right, 180 down (as in CSS)
	From, To     Color
	FromAt, ToAt float32 // stop positions 0..1; both zero means 0 and 1
	Oklab        bool    // interpolate in Oklab (smoother, no muddy middle); default sRGB like GPUI
}

// LinearGradient returns a gradient from one color to another at angle degrees
// (0 points up, 90 right, 180 down). The colors sit at the ends of the
// gradient line, so the corners of the box get them.
func LinearGradient(angle float32, from, to Color) Gradient {
	return Gradient{Angle: angle, From: from, To: to}
}

// Stops returns g with its colors at the given positions along the gradient
// line, 0 to 1. Outside them the end colors continue.
func (g Gradient) Stops(fromAt, toAt float32) Gradient {
	g.FromAt, g.ToAt = fromAt, toAt
	return g
}

// InOklab returns g interpolated in Oklab. Mixes between saturated colors stay
// bright where sRGB mixing turns gray: blue to yellow passes through green,
// not gray.
func (g Gradient) InOklab() Gradient {
	g.Oklab = true
	return g
}

func (g Gradient) isZero() bool { return g == Gradient{} }

// stops returns the stop positions, with the zero value meaning 0 and 1.
func (g Gradient) stops() (float32, float32) {
	if g.FromAt == 0 && g.ToAt == 0 {
		return 0, 1
	}
	return g.FromAt, g.ToAt
}

// BackgroundGradient fills the element with a linear gradient instead of its
// Background color. It is merged like the other visual fields, so a Hover or
// Focused style may set one too.
func (d *DivElement) BackgroundGradient(g Gradient) *DivElement { d.style.Gradient = g; return d }
