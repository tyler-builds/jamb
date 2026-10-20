package ui

import (
	"slices"
	"strconv"
	"time"

	"github.com/tyler-builds/jamb/access"
)

// The default delays of a hover card: how long the pointer rests on the trigger
// before the card opens, and how long the card stays after the pointer leaves,
// which lets the pointer cross the gap between the trigger and the card.
const (
	hoverCardOpenDelay  = 500 * time.Millisecond
	hoverCardCloseDelay = 300 * time.Millisecond
)

// HoverCardStyle is the look of a hover card's panel. It merges over the built-in
// defaults, which are the look of a popover's panel, then over the theme's
// HoverCard, then over the Styles of the instance; see docs/styling.md.
type HoverCardStyle struct {
	Panel Style // the floating panel that holds the content
}

// merge returns s with the set fields of o applied on top.
func (s HoverCardStyle) merge(o HoverCardStyle) HoverCardStyle {
	return HoverCardStyle{Panel: s.Panel.Merge(o.Panel)}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s HoverCardStyle) plain() HoverCardStyle { return HoverCardStyle{Panel: plainStyle(s.Panel)} }

// hoverCardDefaults returns the built-in look of a hover card: the panel of a
// popover (see popoverDefaults), so the two floating panels match.
func hoverCardDefaults(t *Theme) HoverCardStyle {
	return HoverCardStyle{Panel: popoverDefaults(t).Panel}
}

// resolveHoverCard returns the look of a hover card: the defaults, merged with the
// theme's HoverCard and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveHoverCard(ctx *Context, inst HoverCardStyle) HoverCardStyle {
	t := ctx.Theme()
	return layered(ctx, hoverCardDefaults(t), HoverCardStyle.merge, HoverCardStyle.plain,
		themeLayer(t.HoverCard), instanceLayer(inst))
}

// HoverCardElement is a trigger that shows a floating card of rich content, such
// as the profile behind a name or the preview behind a link, while the pointer
// rests on it. The card opens after the pointer has rested on the trigger for the
// open delay, or after keyboard focus has moved to the trigger for that delay. It
// stays open while the pointer is over the trigger or the card, and for the close
// delay after the pointer leaves both, so the pointer can cross the gap between
// them. It closes at once on Escape, a click outside, or focus leaving the trigger
// and the card while the pointer is elsewhere.
//
// The card does not take focus. Assistive technology reads it as the description
// of the trigger while it is open, as it reads a tooltip; content that only a
// pointer user should see belongs in the card, and anything needed belongs in the
// page too.
type HoverCardElement struct {
	trigger    Element
	content    func() Element
	placement  Placement
	openDelay  time.Duration
	closeDelay time.Duration
	styles     HoverCardStyle
	key        string
}

// HoverCard creates a hover card: resting the pointer on trigger, or focusing it
// with the keyboard, shows a card with what content returns. content runs while
// the card is open, like a component's render.
//
//	ui.HoverCard(ui.Button("Dana Lee").Ghost().Small(), func() ui.Element {
//		return profileCard("Dana Lee")
//	}).Placement(ui.BelowStart)
func HoverCard(trigger Element, content func() Element) *HoverCardElement {
	return &HoverCardElement{trigger: trigger, content: content, placement: BelowStart,
		openDelay: hoverCardOpenDelay, closeDelay: hoverCardCloseDelay}
}

func (*HoverCardElement) isElement()           {}
func (h *HoverCardElement) elementKey() string { return h.key }

// Key identifies the hover card among its siblings, so it keeps its state when the
// siblings around it change (see DivElement.Key).
func (h *HoverCardElement) Key(k string) *HoverCardElement { h.key = k; return h }

// Placement sets where the card opens next to the trigger, BelowStart by default.
// It flips to the other side when it does not fit (see Overlay).
func (h *HoverCardElement) Placement(pl Placement) *HoverCardElement { h.placement = pl; return h }

// OpenDelay sets how long the pointer rests on the trigger, or keyboard focus stays
// on it, before the card opens: 500 milliseconds by default.
func (h *HoverCardElement) OpenDelay(d time.Duration) *HoverCardElement { h.openDelay = d; return h }

// CloseDelay sets how long the card stays open after the pointer leaves the trigger
// and the card: 300 milliseconds by default. Coming back within it keeps the card.
func (h *HoverCardElement) CloseDelay(d time.Duration) *HoverCardElement {
	h.closeDelay = d
	return h
}

// Styles sets the look of the card, for this instance only. Set fields win over
// the theme's HoverCard and the defaults. Calling Styles again merges.
func (h *HoverCardElement) Styles(st HoverCardStyle) *HoverCardElement {
	h.styles = h.styles.merge(st)
	return h
}

// hoverCardState is what a hover card keeps between its renders.
type hoverCardState struct {
	cancel   func() // cancels the open or the close that is waiting, if any
	hovered  bool   // the pointer is over the trigger or the card
	focused  bool   // focus is in the trigger or the card
	disposes bool   // the card cancels its timer when it is removed
}

// stop cancels the open or the close that is waiting.
func (s *hoverCardState) stop() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (h *HoverCardElement) expand(ctx *Context) Element {
	trigger, content, placement, styles := h.trigger, h.content, h.placement, h.styles
	openDelay, closeDelay := h.openDelay, h.closeDelay
	return Component(func(ctx *Context) Element {
		rt := ctx.rt
		open := Local(ctx, "open", false)
		st := Local(ctx, "state", &hoverCardState{}).Peek()
		if !st.disposes {
			st.disposes = true
			ctx.OnDispose(st.stop)
		}
		// after opens or closes the card once d has passed, in place of the change
		// that was waiting. The window's clock times it, as it does a tooltip.
		after := func(on bool, d time.Duration) {
			st.stop()
			if open.Peek() == on {
				return
			}
			st.cancel = ctx.After(d, func() {
				st.cancel = nil
				open.Set(on)
			})
		}
		closeNow := func() {
			st.stop()
			open.Set(false)
		}
		// The panel's key relates it to the trigger it describes (see describeTrigger).
		key := "jamb.hovercard." + strconv.FormatUint(ctx.id, 36)
		// The overlay is a child of the wrapper, so the pointer over the card is
		// over the wrapper too, and the wrapper's enter and leave cover both.
		d := Div().AlignSelf(Start).Shrink(0).
			OnMouseEnter(func() {
				st.hovered = true
				after(true, openDelay)
			}).
			OnMouseLeave(func() {
				st.hovered = false
				if !st.focused {
					after(false, closeDelay)
				}
			}).
			Child(trigger)
		d.focusIn = func(in bool) {
			st.focused = in
			switch {
			case in && rt.keyboardModality:
				after(true, openDelay)
			case !in && !st.hovered:
				closeNow()
			}
		}
		d.access().details = key
		if open.Get() {
			s := resolveHoverCard(ctx, styles)
			d.Child(Div().asPart("HoverCard.Panel").Key(key).Overlay(placement).OverlayGap(6).Style(s.Panel).
				Role(access.RoleTooltip).
				// Escape and a click outside close the card; it never takes focus.
				OnClickOutside(closeNow).
				Enter(Fade().Slide(0, -4), Animation{Duration: ctx.Theme().Motion.enter(120 * time.Millisecond)}).
				Exit(Fade(), Animation{Duration: ctx.Theme().Motion.exit(80 * time.Millisecond)}).
				Child(content()))
		}
		return d
	})
}

// focusInChanged tells the elements that watch focus within them (see
// DivElement.focusIn) that focus moved from the element oldID to id: those around
// the old element but not the new one that it left, and those around the new one
// but not the old one that it entered.
func (rt *Runtime) focusInChanged(oldID, id uint64) {
	var in, out []*node
	for a := rt.byID[id]; a != nil; a = a.parent {
		in = append(in, a)
	}
	for a := rt.byID[oldID]; a != nil; a = a.parent {
		out = append(out, a)
	}
	for _, a := range out {
		if a.div != nil && a.div.focusIn != nil && !slices.Contains(in, a) {
			a.div.focusIn(false)
		}
	}
	for _, a := range in {
		if a.div != nil && a.div.focusIn != nil && !slices.Contains(out, a) {
			a.div.focusIn(true)
		}
	}
}
