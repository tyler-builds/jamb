package ui

import (
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// copyFeedbackDelay is how long a copy button shows its confirmation before it
// returns to its normal look.
const copyFeedbackDelay = 1500 * time.Millisecond

// copyFeedback is the pending return of a copy button to its normal look. It is
// kept in the button's Local state, so a second copy restarts the delay.
type copyFeedback struct {
	cancel     func()
	registered bool // the component's disposer that stops the delay is registered
}

// stop cancels the pending return, if there is one.
func (f *copyFeedback) stop() {
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
}

// CopyButtonElement is a button that copies text to the clipboard. It is an icon
// button by default, with a copy icon that turns into a check after a copy, and it
// is a text button when it has a Label. Its confirmation shows for 1.5 seconds and
// is announced to assistive technology politely.
type CopyButtonElement struct {
	text   string
	value  func() string
	label  string
	copied string // the confirmation text; empty for Strings.Copied
	toast  bool
	onCopy func(string)
	button ButtonElement // the button's look and key, see Key and Variant
}

// CopyButton creates a button that copies text to the clipboard. It is a Ghost
// icon button by default: a copy icon, which becomes a check for 1.5 seconds after a
// copy. Its accessible name is Strings.Copy. Use Value to compute the text at the
// click, and Label for a text button.
func CopyButton(text string) *CopyButtonElement {
	return &CopyButtonElement{text: text, button: ButtonElement{variant: ButtonGhost}}
}

func (*CopyButtonElement) isElement()           {}
func (c *CopyButtonElement) elementKey() string { return c.button.key }

// Value computes the text to copy at the click, instead of the text given to
// CopyButton. The function runs on the UI thread.
func (c *CopyButtonElement) Value(fn func() string) *CopyButtonElement { c.value = fn; return c }

// Label makes a text button, labeled s. After a copy its label reads the
// confirmation (see Copied). Without a label the button is an icon button.
func (c *CopyButtonElement) Label(s string) *CopyButtonElement { c.label = s; return c }

// Copied sets the confirmation text: the label of the button after a copy, and the
// text announced and shown in a toast. It defaults to Strings.Copied.
func (c *CopyButtonElement) Copied(s string) *CopyButtonElement { c.copied = s; return c }

// Toast sets whether a copy also shows a success toast in the window's Toaster
// (see Context.Toast). It is off by default: the button's own confirmation is
// enough for most copies, and the toast is announced in the same way.
func (c *CopyButtonElement) Toast(on bool) *CopyButtonElement { c.toast = on; return c }

// OnCopy sets the function that runs after a copy, with the text copied.
func (c *CopyButtonElement) OnCopy(fn func(string)) *CopyButtonElement { c.onCopy = fn; return c }

// Key identifies the button among its siblings (see DivElement.Key).
func (c *CopyButtonElement) Key(k string) *CopyButtonElement { c.button.key = k; return c }

// Tooltip shows s below the button after the pointer rests on it (see Button.Tooltip).
func (c *CopyButtonElement) Tooltip(s string) *CopyButtonElement { c.button.tooltip = s; return c }

// Variant sets the button's look (see ButtonVariant). The default is Ghost.
func (c *CopyButtonElement) Variant(v ButtonVariant) *CopyButtonElement {
	c.button.variant = v
	return c
}

// Primary sets the button's look to ButtonPrimary.
func (c *CopyButtonElement) Primary() *CopyButtonElement { return c.Variant(ButtonPrimary) }

// Secondary sets the button's look to ButtonSecondary.
func (c *CopyButtonElement) Secondary() *CopyButtonElement { return c.Variant(ButtonSecondary) }

// Ghost sets the button's look to ButtonGhost, its default.
func (c *CopyButtonElement) Ghost() *CopyButtonElement { return c.Variant(ButtonGhost) }

// Danger sets the button's look to ButtonDanger.
func (c *CopyButtonElement) Danger() *CopyButtonElement { return c.Variant(ButtonDanger) }

// Small makes the button use the small control size (see Button.Small).
func (c *CopyButtonElement) Small() *CopyButtonElement { c.button.ctlSize = sizeSmall; return c }

// Large makes the button larger (see Button.Large).
func (c *CopyButtonElement) Large() *CopyButtonElement { c.button.ctlSize = sizeLarge; return c }

// Disabled sets whether the button is disabled, so that it cannot copy (see Button.Disabled).
func (c *CopyButtonElement) Disabled(d bool) *CopyButtonElement { c.button.disabled = d; return c }

// Width sets the width of the button (see Button.Width).
func (c *CopyButtonElement) Width(l Length) *CopyButtonElement { c.button.Width(l); return c }

// Height sets the height of the button (see Button.Height).
func (c *CopyButtonElement) Height(l Length) *CopyButtonElement { c.button.Height(l); return c }

// Style sets the look of the button's Root part, for this instance only (see Button.Style).
func (c *CopyButtonElement) Style(s Style) *CopyButtonElement { c.button.Style(s); return c }

// Styles sets the look of the parts of the button, for this instance only (see Button.Styles).
func (c *CopyButtonElement) Styles(st ButtonStyle) *CopyButtonElement { c.button.Styles(st); return c }

func (c *CopyButtonElement) expand(*Context) Element {
	return Component(func(ctx *Context) Element {
		strs := ctx.Strings()
		done := Local(ctx, "copied", false)
		fb := Local(ctx, "feedback", &copyFeedback{}).Peek()
		if !fb.registered {
			fb.registered = true
			ctx.OnDispose(fb.stop)
		}
		confirm := c.copied
		if confirm == "" {
			confirm = strs.Copied
		}
		b := c.button // a copy: the render's state goes on the copy, not on the element
		if c.label != "" {
			b.label = c.label
			if done.Get() {
				b.label = confirm
			}
		} else {
			b.icon, b.name = icons.Copy, strs.Copy
			if done.Get() {
				b.icon = icons.Check
			}
		}
		b.onClick = func() {
			text := c.text
			if c.value != nil {
				text = c.value()
			}
			ctx.SetClipboard(text)
			fb.stop()
			done.Set(true)
			fb.cancel = ctx.After(copyFeedbackDelay, func() { done.Set(false) })
			if c.onCopy != nil {
				c.onCopy(text)
			}
			if c.toast {
				ctx.Toast(ToastSuccess, confirm)
			} else {
				Announce(confirm, access.Polite)
			}
		}
		return b.expand(ctx)
	})
}
