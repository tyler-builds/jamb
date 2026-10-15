package charts

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

// sizes is the size that the sizing methods give an element's outer box: Width,
// Height, MinWidth, MinHeight, MaxWidth, MaxHeight, Grow and GridSpan, as the ui
// widgets have them. A field that is not set keeps the box's own value. A field that
// is set is written over the box's style, whatever the order of the calls.
type sizes struct {
	set                          sizeSet
	w, h, minW, minH, maxW, maxH ui.Length
	grow                         float32
	cols, rows                   int
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

// sizer is an element with the sizing methods of the ui package, such as a
// *ui.DivElement or a *ui.DataGridElement.
type sizer[E any] interface {
	Width(ui.Length) E
	Height(ui.Length) E
	MinWidth(ui.Length) E
	MinHeight(ui.Length) E
	MaxWidth(ui.Length) E
	MaxHeight(ui.Length) E
	Grow(float32) E
	GridSpan(cols, rows int) E
}

// applySizes calls the sizing methods of e for the fields of z that are set, and
// returns e. The element's own sizing then applies them as it does for the app.
func applySizes[E sizer[E]](z sizes, e E) E {
	if z.set&hasWidth != 0 {
		e.Width(z.w)
	}
	if z.set&hasHeight != 0 {
		e.Height(z.h)
	}
	if z.set&hasMinWidth != 0 {
		e.MinWidth(z.minW)
	}
	if z.set&hasMinHeight != 0 {
		e.MinHeight(z.minH)
	}
	if z.set&hasMaxWidth != 0 {
		e.MaxWidth(z.maxW)
	}
	if z.set&hasMaxHeight != 0 {
		e.MaxHeight(z.maxH)
	}
	if z.set&hasGrow != 0 {
		e.Grow(z.grow)
	}
	if z.set&hasSpan != 0 {
		e.GridSpan(z.cols, z.rows)
	}
	return e
}
