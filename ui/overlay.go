package ui

import (
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/scene"
)

// Placement says where an Overlay sits relative to its parent (the anchor).
type Placement uint8

const (
	BelowStart  Placement = iota + 1 // under the anchor, left edges aligned
	BelowCenter                      // under the anchor, centered
	BelowEnd                         // under the anchor, right edges aligned
	AboveStart
	AboveCenter
	AboveEnd
	RightStart  // right of the anchor, top edges aligned
	LeftStart   // left of the anchor, top edges aligned
	OverCenter  // over the anchor, centered in both directions (a Modal's panel)
	RightCenter // right of the anchor, centered vertically
	RightEnd    // right of the anchor, bottom edges aligned
	LeftCenter  // left of the anchor, centered vertically
	LeftEnd     // left of the anchor, bottom edges aligned
)

// overlayMargin is how far inside the window an overlay is kept.
const overlayMargin = 8

// tooltipDelay is how long the pointer rests on an element before its
// tooltip appears.
const tooltipDelay = 500 * time.Millisecond

// overlayItem is an Overlay found while painting, to be painted after the
// rest of the window.
type overlayItem struct {
	n       *node
	anchor  scene.Rect // parent's border box, window coordinates
	opacity float32
	fg      Color
	ghost   bool // found inside an exiting element: cannot be hit
	exiting bool // the exiting overlay itself, painted where it was last placed
}

// placeOverlay returns the top-left corner for a w×h overlay at placement p
// next to anchor a. It flips to the opposite side when the requested one
// does not fit and the other has more room, then shifts the overlay to stay
// overlayMargin inside the view.
func placeOverlay(p Placement, gap float32, a scene.Rect, w, h float32, view scene.Rect) (x, y float32) {
	switch p {
	case edgeLeft, edgeTop:
		return a.X, a.Y
	case edgeRight:
		return a.X + a.W - w, a.Y
	case edgeBottom:
		return a.X, a.Y + a.H - h
	case OverCenter:
		x = a.X + (a.W-w)/2
		y = a.Y + (a.H-h)/2
	case BelowStart, BelowCenter, BelowEnd, AboveStart, AboveCenter, AboveEnd:
		below := p <= BelowEnd
		roomBelow := view.Y + view.H - (a.Y + a.H + gap)
		roomAbove := a.Y - gap - view.Y
		if below && h > roomBelow && roomAbove > roomBelow {
			below = false
		} else if !below && h > roomAbove && roomBelow > roomAbove {
			below = true
		}
		if below {
			y = a.Y + a.H + gap
		} else {
			y = a.Y - gap - h
		}
		switch p {
		case BelowStart, AboveStart:
			x = a.X
		case BelowCenter, AboveCenter:
			x = a.X + (a.W-w)/2
		default:
			x = a.X + a.W - w
		}
	default:
		right := p == RightStart || p == RightCenter || p == RightEnd
		roomRight := view.X + view.W - (a.X + a.W + gap)
		roomLeft := a.X - gap - view.X
		if right && w > roomRight && roomLeft > roomRight {
			right = false
		} else if !right && w > roomLeft && roomRight > roomLeft {
			right = true
		}
		if right {
			x = a.X + a.W + gap
		} else {
			x = a.X - gap - w
		}
		switch p {
		case RightCenter, LeftCenter:
			y = a.Y + (a.H-h)/2
		case RightEnd, LeftEnd:
			y = a.Y + a.H - h
		default:
			y = a.Y
		}
	}
	x = min(x, view.X+view.W-overlayMargin-w)
	x = max(x, view.X+overlayMargin)
	y = min(y, view.Y+view.H-overlayMargin-h)
	y = max(y, view.Y+overlayMargin)
	return x, y
}

// paintOverlays paints the overlays collected during the main paint, in
// order. Overlays found inside an overlay are appended and painted after it,
// so nested ones end up on top.
func (rt *Runtime) paintOverlays(view scene.Rect, dt float32) {
	for i := 0; i < len(rt.overlays); i++ {
		it := rt.overlays[i] // copy: painting may append to the slice
		n := it.n
		if it.exiting {
			rt.paintGhost(n, dt)
			continue
		}
		x, y := placeOverlay(n.div.overlay, n.div.overlayGap, it.anchor, n.lay.W, n.lay.H, view)
		prev := rt.inGhost
		rt.inGhost = it.ghost
		rt.paintNode(n, x-n.lay.X, y-n.lay.Y, view, it.opacity, it.fg, dt)
		rt.inGhost = prev
	}
}

// TooltipStyle is the look of each part of a tooltip. A part's style merges over
// the built-in defaults, then over the theme's Tooltip; a tooltip has no
// instance. Panel is the box that shows the text, and Text the text in it. See
// docs/styling.md.
type TooltipStyle struct {
	Panel Style // the box of the tooltip
	Text  Style // the text in it
}

// merge returns s with the set fields of o applied on top, part by part.
func (s TooltipStyle) merge(o TooltipStyle) TooltipStyle {
	return TooltipStyle{Panel: s.Panel.Merge(o.Panel), Text: s.Text.Merge(o.Text)}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s TooltipStyle) plain() TooltipStyle {
	return TooltipStyle{Panel: plainStyle(s.Panel), Text: plainStyle(s.Text)}
}

// tooltipDefaults returns the built-in look of a tooltip, from the colors of t.
func tooltipDefaults(t *Theme) TooltipStyle {
	return TooltipStyle{Panel: Style{Padding: XY(8, 4), Radius: 6, Background: t.Surface, BorderWidth: 1,
		BorderColor: t.Border, Shadows: []Shadow{ShadowMd}, TextColor: t.Text, FontSize: 12}}
}

// resolveTooltip returns the look of a tooltip: the defaults, merged with the
// theme's Tooltip. Under high contrast the override is plain (see plainStyle).
func resolveTooltip(t *Theme) TooltipStyle {
	st := tooltipDefaults(t)
	o := t.Tooltip
	if t.HighContrast {
		o = o.plain()
	}
	return st.merge(o)
}

// tooltipElement builds the label shown for a tooltip. It fades in and out.
func (rt *Runtime) tooltipElement(s string) Element {
	st := resolveTooltip(rt.theme)
	return Div().asPart("Tooltip.Panel").Key("jamb.tooltip").Overlay(BelowCenter).OverlayGap(6).Style(st.Panel).
		Enter(Fade(), Animation{Duration: 120 * time.Millisecond}).
		Exit(Fade(), Animation{Duration: 80 * time.Millisecond}).
		Child(Div().asPart("Tooltip.Text").Style(st.Text).Child(Text(s).NoWrap())).Role(access.RoleTooltip)
}

// showTip makes id the element whose tooltip is displayed (0 hides it),
// rebuilding the components that own the old and new elements.
func (rt *Runtime) showTip(id uint64) {
	old := rt.tipShown
	if old == id {
		return
	}
	rt.tipShown = id
	for _, x := range [2]uint64{old, id} {
		if n := rt.byID[x]; n != nil && n.owner != nil {
			n.owner.invalidate()
		}
	}
}

// escapeOverlay returns the dismiss handler (OnClickOutside) of the topmost
// open overlay, or nil. Overlays paint in order, nested ones last, so the
// last one with a handler is on top. Leaving and unhittable overlays are
// skipped.
func (rt *Runtime) escapeOverlay() func() {
	for i := len(rt.overlays) - 1; i >= 0; i-- {
		it := rt.overlays[i]
		if it.exiting || it.ghost {
			continue
		}
		if fn := it.n.div.h.onClickOutside; fn != nil {
			return fn
		}
	}
	return nil
}

// escapeDismisses reports whether Escape would close an overlay or hide a
// tooltip. While it would, the field's own Cancel does not run.
func (rt *Runtime) escapeDismisses() bool { return rt.escapeOverlay() != nil || rt.tipShown != 0 }

// dismissEscape closes the topmost overlay, or else hides the tooltip, and
// reports whether it did. A hidden tooltip stays hidden while the pointer
// rests on the same element.
func (rt *Runtime) dismissEscape() bool {
	if fn := rt.escapeOverlay(); fn != nil {
		fn()
		return true
	}
	if rt.tipShown != 0 {
		rt.tipDismissed = rt.tipCandidate
		rt.showTip(0)
		return true
	}
	return false
}

// anchorVisible reports whether an overlay's anchor is at least partly inside
// clip. Edges count, so a zero-size anchor (a popover pinned to a point)
// is visible wherever its point is.
func anchorVisible(a, clip scene.Rect) bool {
	return a.X <= clip.Right() && clip.X <= a.Right() && a.Y <= clip.Bottom() && clip.Y <= a.Bottom() &&
		!clip.Empty()
}
