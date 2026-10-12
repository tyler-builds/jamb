package ui

// sizeSet is the set of sizing fields an app has given a widget.
type sizeSet uint8

const (
	hasWidth sizeSet = 1 << iota
	hasHeight
	hasMinWidth
	hasMinHeight
	hasMaxWidth
	hasMaxHeight
	hasGrow
)

// sizing is the size a widget's outer box is given by its sizing methods: Width,
// Height, MinWidth, MinHeight, MaxWidth, MaxHeight and Grow. A widget embeds the
// fields it has methods for, so every widget gives its sizes the same way. A field
// the app has not set keeps the widget's default. A field it has set is written
// over the theme, the Styles of the instance and the defaults, whatever the order
// of the calls, and the other fields of the box are left as they were. See
// Sizing in docs/styling.md.
type sizing struct {
	set   sizeSet
	style Style // the fields in set; the others are unused
}

func (z *sizing) setWidth(l Length)     { z.style.Width = l; z.set |= hasWidth }
func (z *sizing) setHeight(l Length)    { z.style.Height = l; z.set |= hasHeight }
func (z *sizing) setMinWidth(l Length)  { z.style.MinWidth = l; z.set |= hasMinWidth }
func (z *sizing) setMinHeight(l Length) { z.style.MinHeight = l; z.set |= hasMinHeight }
func (z *sizing) setMaxWidth(l Length)  { z.style.MaxWidth = l; z.set |= hasMaxWidth }
func (z *sizing) setMaxHeight(l Length) { z.style.MaxHeight = l; z.set |= hasMaxHeight }
func (z *sizing) setGrow(f float32)     { z.style.Grow = f; z.set |= hasGrow }

// applyTo writes the fields the app has set over st.
func (z *sizing) applyTo(st *Style) {
	if z.set&hasWidth != 0 {
		st.Width = z.style.Width
	}
	if z.set&hasHeight != 0 {
		st.Height = z.style.Height
	}
	if z.set&hasMinWidth != 0 {
		st.MinWidth = z.style.MinWidth
	}
	if z.set&hasMinHeight != 0 {
		st.MinHeight = z.style.MinHeight
	}
	if z.set&hasMaxWidth != 0 {
		st.MaxWidth = z.style.MaxWidth
	}
	if z.set&hasMaxHeight != 0 {
		st.MaxHeight = z.style.MaxHeight
	}
	if z.set&hasGrow != 0 {
		st.Grow = z.style.Grow
	}
}

// textScale is the Text size setting as a factor: 1.5 at 150 percent. The default
// sizes of controls that have a width of their own are multiplied by it.
func textScale(ctx *Context) float32 { return float32(ctx.TextScale()) / 100 }

// apply writes the sizes over d's style and returns d, for the end of an expand.
func (z *sizing) apply(d *DivElement) *DivElement {
	z.applyTo(&d.style)
	return d
}
