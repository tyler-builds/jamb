package ui

import (
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// PopoverElement is a trigger that opens a floating panel of any content, such
// as filters or a small form, next to it. It opens and closes itself, as a
// DropdownMenu does: a click (or Enter or Space) on the trigger toggles it, and
// Escape or a click outside closes it. When it opens, keyboard focus moves to
// the first field or button of the panel; when Escape closes it, focus returns
// to the trigger. Assistive technology sees the trigger as a button that opens
// a dialog, and the panel as that dialog.
type PopoverElement struct {
	trigger   Element
	content   func() Element
	placement Placement
	label     string
	styles    PopoverStyle
	key       string
	onChange  func(open bool)
}

// Popover creates a popover: trigger opens a panel showing what content
// returns. content runs while the panel is open, like a component's render.
//
//	ui.Popover(ui.Button("Filters").Icon(icons.Filter), func() ui.Element {
//		return filterForm()
//	}).Placement(ui.BelowStart)
func Popover(trigger Element, content func() Element) *PopoverElement {
	return &PopoverElement{trigger: trigger, content: content, placement: BelowStart}
}

func (*PopoverElement) isElement()           {}
func (p *PopoverElement) elementKey() string { return p.key }

// Key identifies the popover among its siblings, so it keeps its state when the
// siblings around it change (see DivElement.Key).
func (p *PopoverElement) Key(k string) *PopoverElement { p.key = k; return p }

// Placement sets where the panel opens next to the trigger, BelowStart by
// default. It flips to the other side when it does not fit (see Overlay).
func (p *PopoverElement) Placement(pl Placement) *PopoverElement { p.placement = pl; return p }

// Label names the panel for assistive technology. It defaults to the trigger's
// name, such as the button's label.
func (p *PopoverElement) Label(name string) *PopoverElement { p.label = name; return p }

// OnChange runs fn when the popover opens or closes.
func (p *PopoverElement) OnChange(fn func(open bool)) *PopoverElement { p.onChange = fn; return p }

// PopoverStyle is the look of a popover's panel. It merges over the built-in
// defaults, then over the theme's Popover, then over the Styles of the
// instance; see docs/styling.md.
type PopoverStyle struct {
	Panel Style // the floating panel that holds the content
}

func (s PopoverStyle) merge(o PopoverStyle) PopoverStyle {
	return PopoverStyle{Panel: s.Panel.Merge(o.Panel)}
}

func (s PopoverStyle) plain() PopoverStyle { return PopoverStyle{Panel: plainStyle(s.Panel)} }

// popoverDefaults returns the built-in look of a popover, from the colors of t:
// the surface and border of a menu, with room around the content.
func popoverDefaults(t *Theme) PopoverStyle {
	return PopoverStyle{Panel: Style{Direction: Column, Gap: 10, Padding: All(14), Background: t.Surface,
		BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius, Shadows: []Shadow{ShadowMd}}}
}

func resolvePopover(ctx *Context, inst PopoverStyle) PopoverStyle {
	t := ctx.Theme()
	return layered(ctx, popoverDefaults(t), PopoverStyle.merge, PopoverStyle.plain,
		themeLayer(t.Popover), instanceLayer(inst))
}

// Styles sets the look of the panel, for this instance only. Set fields win
// over the theme's Popover and the defaults. Calling Styles again merges.
func (p *PopoverElement) Styles(st PopoverStyle) *PopoverElement {
	p.styles = p.styles.merge(st)
	return p
}

func (p *PopoverElement) expand(ctx *Context) Element {
	trigger, content, placement, label, styles, onChange := p.trigger, p.content, p.placement, p.label, p.styles, p.onChange
	return Component(func(ctx *Context) Element {
		rt := ctx.rt
		open := Local(ctx, "open", false)
		// entering is set while the panel opens, so its first focus moves on to
		// the first field or button inside it.
		entering := Local(ctx, "entering", false)
		panelH := Local(ctx, "panel", NewFocusHandle()).Peek()
		// back is where focus returns when Escape closes the panel: the element that
		// had it when the panel opened, the trigger's button.
		back := Local(ctx, "back", (*FocusHandle)(nil))
		set := func(on bool) {
			if open.Peek() == on {
				return
			}
			if on {
				back.Set(rt.FocusedHandle())
			}
			open.Set(on)
			entering.Set(on)
			if onChange != nil {
				onChange(on)
			}
		}
		// closeFn closes the panel. Focus inside it returns to the trigger; after a
		// click elsewhere, focus is already where the user put it.
		closeFn := func() {
			if b := back.Peek(); b != nil && panelH.id != 0 && rt.focusInside(panelH.id) {
				b.FocusNow()
			}
			set(false)
		}
		d := Div().AlignSelf(Start).Shrink(0).AccessExpanded(open.Get()).
			OnClick(func() {
				if open.Peek() {
					closeFn()
				} else {
					set(true)
				}
			}).
			OnKeyDown(func(e *KeyEvent) {
				if open.Peek() || e.Repeat || e.Mods != 0 {
					return
				}
				if e.Key == input.KeyEnter || e.Key == input.KeySpace {
					set(true)
					e.StopPropagation()
				}
			}).
			Child(trigger)
		name := label
		if b, ok := trigger.(*ButtonElement); ok {
			// As in DropdownMenu, the wrapper is the control that assistive
			// technology sees, named by the button, and focus goes to the button.
			b.inert = true
			d.Role(access.RoleButton).Label(b.accessName())
			d.access().focusInside = !b.disabled
			if name == "" {
				name = b.accessName()
			}
		}
		if open.Get() {
			st := resolvePopover(ctx, styles)
			panel := Div().asPart("Popover.Panel").Overlay(placement).OverlayGap(6).Style(st.Panel).
				Role(access.RoleDialog).Label(name).
				// The panel takes focus when it opens, and hands it to its first field
				// or button; a click on its background keeps focus inside it.
				Focusable().TabStop(false).AutoFocus().FocusHandle(panelH).
				OnFocus(func() {
					if entering.Peek() {
						entering.Set(false)
						rt.focusFirstIn(panelH.id)
					}
				}).
				OnClickOutside(closeFn).
				Enter(Fade().Slide(0, -4), Animation{Duration: 120 * time.Millisecond}).
				Exit(Fade(), Animation{Duration: 80 * time.Millisecond}).
				Child(content())
			d.Child(panel)
		}
		return d
	})
}

// focusFirstIn moves focus to the first element inside id that Tab reaches,
// if there is one.
func (rt *Runtime) focusFirstIn(id uint64) {
	n := rt.byID[id]
	if n == nil {
		return
	}
	var found uint64
	var walk func(c *node)
	walk = func(c *node) {
		if found != 0 || c.flags&flagNoDisplay != 0 {
			return
		}
		if d := c.div; d != nil && c.id != id && d.focusable && !d.noTabStop && d.tabIndex >= 0 {
			found = c.id
			return
		}
		for _, k := range c.children {
			walk(k)
		}
	}
	walk(n)
	if found != 0 {
		rt.setFocus(found)
	}
}
