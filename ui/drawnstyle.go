package ui

// DrawnStyle is how the latest paint drew an element: its style with the
// hover, press, focus and other state styles merged in, and its transitions at
// the point they had reached. Runtime.StyleOf returns it, for tests and tools
// that check what is on screen; the ui/uitest package wraps it.
//
// Colors are the element's own, before its opacity is applied; Opacity says
// how transparent it was drawn.
type DrawnStyle struct {
	// Painted reports whether the latest paint drew the element. An element
	// culled as off screen, or hidden by Display(false), was not, and the other
	// fields are then zero.
	Painted bool
	// Bounds is where the element was drawn, in logical pixels, window
	// coordinates, as Find reports it.
	Bounds Rect
	// Background is the fill. It is transparent when the element has none, and
	// the first color of a gradient when Gradient is set.
	Background Color
	// Gradient reports whether the fill is a gradient. High contrast draws a
	// gradient as its first color, and then it is not set.
	Gradient bool
	// BorderColor and BorderWidth are the border, widths in logical pixels.
	// Both are zero when no border is drawn.
	BorderColor Color
	BorderWidth Edges
	// TextColor is the color of the element's text and icons, and the color
	// its children inherit; an icon reports its own color.
	TextColor Color
	// Opacity is the opacity the element was drawn with, its ancestors'
	// included, and an entrance motion's too: 1 is opaque.
	Opacity float32
	// Ring and RingWidth are a ring drawn around the element: a shadow with a
	// spread and no offset or blur, which is how the built-in controls draw
	// their focus ring. RingWidth is in logical pixels, and 0 when there is no
	// ring.
	Ring      Color
	RingWidth float32
}

// StyleOf returns how the latest paint drew the first element in tree order
// whose key is key (see Find, and DrawnStyle), and false when there is no such
// element. Call it after Frame. It is meant for tests and tools, and reads
// what paint recorded without changing anything.
func (rt *Runtime) StyleOf(key string) (DrawnStyle, bool) {
	n := rt.findKey(rt.tree, key)
	if n == nil {
		return DrawnStyle{}, false
	}
	if n.paintGen != rt.paintGen {
		return DrawnStyle{}, true
	}
	s := &n.style
	d := DrawnStyle{Painted: true, Bounds: n.abs, Opacity: n.opacity}
	bg, border, fg := s.Background, s.BorderColor, n.inFg
	if !s.TextColor.IsZero() {
		fg = s.TextColor
	}
	if n.st != nil && s.Transition > 0 && n.st.animReady {
		bg, border, fg = n.st.shown.bg, n.st.shown.border, n.st.shown.fg
	}
	d.TextColor = fg
	switch n.kind {
	case kindDiv, kindEditor:
		// As paintNode draws them.
		d.Background = bg
		if grad := s.Gradient; !grad.isZero() {
			d.Background, d.Gradient = grad.From, !rt.sys.HighContrast
		}
		if be := s.borderEdges(); !be.isZero() && border.A > 0 {
			d.BorderColor, d.BorderWidth = border, be
		}
		for _, sh := range s.Shadows {
			if sh.Blur == 0 && sh.X == 0 && sh.Y == 0 && sh.Spread > 0 {
				d.Ring, d.RingWidth = sh.Color, sh.Spread
				break
			}
		}
	case kindIcon:
		if !n.icon.color.IsZero() && !rt.sys.HighContrast {
			d.TextColor = n.icon.color
		}
	case kindImage:
		if o := n.img.style.Opacity; o > 0 {
			d.Opacity *= o
		}
	}
	return d, true
}
