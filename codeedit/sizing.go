package codeedit

import "github.com/tyler-builds/jamb/ui"

// sizeSet is the set of sizing fields that the sizing methods have set.
type sizeSet uint8

const (
	hasWidth sizeSet = 1 << iota
	hasHeight
	hasMinWidth
	hasMinHeight
	hasMaxWidth
	hasMaxHeight
	hasGrow
	hasSpan
)

// sizes is the size that the sizing methods (Size, Width, Height, MinWidth, MinHeight,
// MaxWidth, MaxHeight, Grow and GridSpan) give the editor's box, as the ui widgets have
// them. A field that is not set keeps the box's own value: the default size, Full by 300
// pixels, or the Box style's. A field that is set is written over the box's style,
// whatever the order of the calls.
type sizes struct {
	set                    sizeSet
	w, h                   ui.Length
	minW, minH, maxW, maxH ui.Length
	grow                   float32
	cols, rows             int
}

func (z *sizes) setWidth(l ui.Length)     { z.w = l; z.set |= hasWidth }
func (z *sizes) setHeight(l ui.Length)    { z.h = l; z.set |= hasHeight }
func (z *sizes) setMinWidth(l ui.Length)  { z.minW = l; z.set |= hasMinWidth }
func (z *sizes) setMinHeight(l ui.Length) { z.minH = l; z.set |= hasMinHeight }
func (z *sizes) setMaxWidth(l ui.Length)  { z.maxW = l; z.set |= hasMaxWidth }
func (z *sizes) setMaxHeight(l ui.Length) { z.maxH = l; z.set |= hasMaxHeight }
func (z *sizes) setGrow(f float32)        { z.grow = f; z.set |= hasGrow }
func (z *sizes) setSpan(cols, rows int) {
	z.cols, z.rows = cols, rows
	z.set |= hasSpan
}

// apply writes the fields of z that are set over the style of d.
func (z *sizes) apply(d *ui.DivElement) {
	if z.set&hasWidth != 0 {
		d.Width(z.w)
	}
	if z.set&hasHeight != 0 {
		d.Height(z.h)
	}
	if z.set&hasMinWidth != 0 {
		d.MinWidth(z.minW)
	}
	if z.set&hasMinHeight != 0 {
		d.MinHeight(z.minH)
	}
	if z.set&hasMaxWidth != 0 {
		d.MaxWidth(z.maxW)
	}
	if z.set&hasMaxHeight != 0 {
		d.MaxHeight(z.maxH)
	}
	if z.set&hasGrow != 0 {
		d.Grow(z.grow)
	}
	if z.set&hasSpan != 0 {
		d.GridSpan(z.cols, z.rows)
	}
}
