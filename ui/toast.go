package ui

import (
	"fmt"
	"slices"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// ToastVariant is the kind of a toast. It sets the icon and the color, and how
// the toast is announced.
type ToastVariant uint8

const (
	ToastInfo ToastVariant = iota
	ToastSuccess
	ToastWarning
	ToastError
)

// toastMaxDefault is the number of toasts shown at once, when the Toaster does
// not set another. The oldest is dismissed to make room.
const toastMaxDefault = 3

// toastDefaultTime is how long a toast stays by default. An error stays until
// it is dismissed.
func toastDefaultTime(v ToastVariant) time.Duration {
	switch v {
	case ToastSuccess:
		return 4 * time.Second
	case ToastWarning:
		return 8 * time.Second
	}
	return 5 * time.Second
}

// Toast is a notification in the Toaster. Context.Toast makes one; the methods
// below set its action, its time and its dismissal.
type Toast struct {
	rt *Runtime
	e  *toastEntry
}

// toastEntry is one toast on the stack. The runtime keeps it, so that its time
// runs whether or not the Toaster is being rendered.
type toastEntry struct {
	id       uint64
	variant  ToastVariant
	text     string
	action   string
	onAction func()
	left     time.Duration // time before the toast is dismissed, not counted while paused
	sticky   bool          // no timeout: the toast stays until it is dismissed
	hovered  bool          // the pointer is over the toast
}

// Toast shows a notification with text in the window's Toaster. It is announced
// to assistive technology: politely for info and success, assertively for
// warning and error. It does not take focus, so typing carries on where it was.
// Without a Toaster in the window it is not shown. Call it from a click or
// another handler on the UI thread, or from a goroutine through Post.
func (c *Context) Toast(v ToastVariant, text string) *Toast { return c.rt.toast(v, text) }

// Action adds a button labelled label to the toast, such as "Undo". Running it
// calls fn and dismisses the toast.
func (t *Toast) Action(label string, fn func()) *Toast {
	t.e.action, t.e.onAction = label, fn
	return t
}

// Duration sets how long the toast stays. The time it is hovered or has
// keyboard focus does not count.
func (t *Toast) Duration(d time.Duration) *Toast {
	t.e.left, t.e.sticky = d, false
	return t
}

// Sticky keeps the toast on screen until the user dismisses it.
func (t *Toast) Sticky() *Toast {
	t.e.sticky = true
	return t
}

// Dismiss removes the toast now.
func (t *Toast) Dismiss() { t.rt.dismissToast(t.e) }

// toast adds a toast to the stack and announces it. When the stack is over its
// maximum, the oldest toast that is not hovered or focused goes.
func (rt *Runtime) toast(v ToastVariant, text string) *Toast {
	if len(rt.toasts) == 0 {
		rt.toastAt = rt.now() // the first toast's time starts now, not at the last frame
	}
	rt.toastNext++
	e := &toastEntry{id: rt.toastNext, variant: v, text: text, left: toastDefaultTime(v), sticky: v == ToastError}
	rt.toasts = append(rt.toasts, e)
	for len(rt.toasts) > rt.toastMaxShown() {
		i := rt.evictable()
		if i < 0 {
			break // every toast is in use: the stack grows rather than drop one
		}
		rt.toasts = slices.Delete(rt.toasts, i, i+1)
	}
	rt.bumpToasts()
	p := access.Polite
	if v >= ToastWarning {
		p = access.Assertive
	}
	Announce(text, p)
	return &Toast{rt: rt, e: e}
}

// evictable returns the index of the oldest toast that is neither hovered nor
// holding keyboard focus, or -1 when there is none. A toast a user is reading or
// using is not dropped to make room.
func (rt *Runtime) evictable() int {
	for i, e := range rt.toasts {
		if !e.hovered && !rt.focusWithin(toastKey(e.id)) {
			return i
		}
	}
	return -1
}

func (rt *Runtime) toastMaxShown() int {
	if rt.toastMax > 0 {
		return rt.toastMax
	}
	return toastMaxDefault
}

// bumpToasts re-renders the Toaster after the stack changed.
func (rt *Runtime) bumpToasts() { rt.toastVer.Set(rt.toastVer.Peek() + 1) }

// dismissToast removes e from the stack, if it is still there.
func (rt *Runtime) dismissToast(e *toastEntry) {
	i := slices.Index(rt.toasts, e)
	if i < 0 {
		return
	}
	rt.toasts = slices.Delete(rt.toasts, i, i+1)
	rt.bumpToasts()
}

// tickToasts runs the time of each toast that is not paused, and dismisses the
// ones that run out. It runs at the start of each frame.
func (rt *Runtime) tickToasts(now time.Time) {
	if len(rt.toasts) == 0 {
		rt.toastAt = now
		return
	}
	dt := now.Sub(rt.toastAt)
	rt.toastAt = now
	if dt <= 0 {
		return
	}
	var ran []*toastEntry
	for _, e := range rt.toasts {
		if e.sticky || e.hovered || rt.focusWithin(toastKey(e.id)) {
			continue
		}
		e.left -= dt
		if e.left <= 0 {
			ran = append(ran, e)
		}
	}
	for _, e := range ran {
		rt.dismissToast(e)
	}
}

// focusWithin reports whether the focused element is the element with key, or
// is inside it.
func (rt *Runtime) focusWithin(key string) bool {
	for n := rt.byID[rt.focusID]; rt.focusID != 0 && n != nil; n = n.parent {
		if n.key == key {
			return true
		}
	}
	return false
}

// inToastRegion reports whether the focused element is the toast region or is
// inside it.
func (rt *Runtime) inToastRegion() bool {
	for n := rt.byID[rt.focusID]; rt.focusID != 0 && n != nil; n = n.parent {
		if n.id == rt.toastRegion {
			return true
		}
	}
	return false
}

// toggleToastFocus moves focus into the toast region, and back to the element
// that had it. It is bound to F6 and Shift+F6, which Windows uses to move
// between the regions of a window. With no toast on screen it does nothing.
func (rt *Runtime) toggleToastFocus() {
	if len(rt.toasts) == 0 || rt.toastRegion == 0 || rt.byID[rt.toastRegion] == nil {
		return
	}
	if rt.inToastRegion() {
		rt.leaveToastRegion()
		return
	}
	rt.toastReturn = rt.focusID
	rt.setFocus(rt.toastRegion)
}

// leaveToastRegion gives focus back to the element that had it before the
// region was entered. If that element has gone, focus is cleared.
func (rt *Runtime) leaveToastRegion() {
	ret := rt.toastReturn
	rt.toastReturn = 0
	if ret != 0 && rt.byID[ret] == nil {
		ret = 0
	}
	rt.setFocus(ret)
}

// closeToast dismisses a toast from its own close button or from Escape. When
// the focus was on the toast, it moves to the next toast in the region, or back
// to where it was when no toast remains. The focus moves after the next build,
// when the removed toast has left the tree.
func (rt *Runtime) closeToast(e *toastEntry) {
	had := rt.focusWithin(toastKey(e.id))
	rt.dismissToast(e)
	if !had {
		return
	}
	if len(rt.toasts) > 0 {
		rt.pendingFocus = rt.toastRegion
		return
	}
	rt.pendingFocus = rt.toastReturn
	rt.toastReturn = 0
}

// toastKey is the key of the element of a toast.
func toastKey(id uint64) string { return fmt.Sprintf("jamb.toast:%d", id) }

// ToasterElement shows the window's toasts as a stack in the bottom-right
// corner. Place one in the window, as the last child of the root element, so
// that it is laid out over the content. Only one is needed; it shows every
// toast made with Context.Toast.
type ToasterElement struct {
	key string
	max int
}

// Toaster creates the stack of toasts. Its region is focusable with F6, as
// Windows' notification regions are, and it is named "Notifications".
func Toaster() *ToasterElement { return &ToasterElement{} }

func (*ToasterElement) isElement()            {}
func (tv *ToasterElement) elementKey() string { return tv.key }

func (tv *ToasterElement) Key(k string) *ToasterElement { tv.key = k; return tv }

// Max sets how many toasts are shown at once. The oldest is dismissed to make
// room for a new one. The default is 3.
func (tv *ToasterElement) Max(n int) *ToasterElement { tv.max = n; return tv }

func (tv *ToasterElement) expand(ctx *Context) Element {
	rt := ctx.rt
	rt.toastVer.Get() // re-render when the stack changes
	if tv.max > 0 {
		rt.toastMax = tv.max
	}
	if len(rt.toasts) == 0 {
		rt.toastRegion = 0
		return nil
	}
	t := ctx.Theme()
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	rt.toastRegion = expandedID(ctx.id)
	region := Div().Key("jamb.toasts").Absolute().Right(16).Bottom(16).Column().Gap(8).Align(End).
		Role(access.RoleGroup).Label("Notifications").
		Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
		OnKeyDown(func(e *KeyEvent) {
			if e.Key == input.KeyEscape && e.Mods == 0 {
				e.StopPropagation()
				rt.leaveToastRegion()
			}
		})
	for _, e := range rt.toasts {
		region.Child(rt.toastView(t, e))
	}
	return region
}

// toastView is one toast in the stack: an icon, the text, an optional action
// and a close button. It is a group named by its text.
func (rt *Runtime) toastView(t *Theme, e *toastEntry) *DivElement {
	ic, c := toastIcon(t, e.variant)
	edge, width := t.Border, float32(1)
	if t.HighContrast {
		edge, width = t.Text, 2
	}
	box := Div().Key(toastKey(e.id)).Row().Align(Center).Gap(10).
		MinWidth(Px(280)).MaxWidth(Px(420)).PaddingX(14).PaddingY(10).
		Radius(t.Radius).Background(t.Surface).Border(width, edge).Shadow(ShadowMd).
		Enter(Fade().Slide(0, 8), Animation{Duration: 160 * time.Millisecond}).
		Exit(Fade(), Animation{Duration: 120 * time.Millisecond}).
		Role(access.RoleGroup).Label(e.text).
		OnMouseEnter(func() { e.hovered = true }).
		OnMouseLeave(func() { e.hovered = false }).
		OnKeyDown(func(k *KeyEvent) {
			if k.Key == input.KeyEscape && k.Mods == 0 {
				k.StopPropagation()
				rt.closeToast(e)
			}
		})
	box.Child(Div().Size(Px(18), Px(18)).Shrink(0).Child(Icon(ic).Size(18).Color(c)))
	box.Child(Text(e.text).Grow(1))
	if e.action != "" {
		box.Child(Button(e.action).Small().Ghost().OnClick(func() {
			if e.onAction != nil {
				e.onAction()
			}
			rt.dismissToast(e)
		}))
	}
	box.Child(IconButton(icons.X).Label("Dismiss notification").Ghost().OnClick(func() { rt.closeToast(e) }))
	return box
}

// toastIcon returns the icon and its color for a variant. Under high contrast
// every icon is the text color, and the shape tells the variant.
func toastIcon(t *Theme, v ToastVariant) (*icons.Icon, Color) {
	switch v {
	case ToastSuccess:
		return icons.Check, pick(t, t.Success)
	case ToastWarning:
		return icons.Alert, pick(t, t.Warning)
	case ToastError:
		return icons.X, pick(t, t.Danger)
	}
	return icons.Info, pick(t, t.Accent)
}

// pick returns c, or the text color under high contrast.
func pick(t *Theme, c Color) Color {
	if t.HighContrast {
		return t.Text
	}
	return c
}
