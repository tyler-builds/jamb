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
	// ToastInfo is a toast for information.
	ToastInfo ToastVariant = iota
	// ToastSuccess is a toast for a completed action.
	ToastSuccess
	// ToastWarning is a toast for a situation that needs attention.
	ToastWarning
	// ToastError is a toast for a failure.
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

// ToastStyle is the look of each part of a toast. A part's style merges over the
// built-in defaults of its variant, then over the theme's Toast, then over the
// theme's Toasts for that variant, then over the Styles of the toast; see
// docs/styling.md. Container is the box of a toast, Icon the icon of its variant
// (its TextColor is the icon's color), Text the message, and Action and Close
// the wrappers of the action button and the dismiss button.
type ToastStyle struct {
	Container Style // the box of the toast
	Icon      Style // the icon, in the color of its variant
	Text      Style // the message
	Action    Style // the wrapper of the action button
	Close     Style // the wrapper of the dismiss button
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ToastStyle) merge(o ToastStyle) ToastStyle {
	return ToastStyle{
		Container: s.Container.Merge(o.Container),
		Icon:      s.Icon.Merge(o.Icon),
		Text:      s.Text.Merge(o.Text),
		Action:    s.Action.Merge(o.Action),
		Close:     s.Close.Merge(o.Close),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ToastStyle) plain() ToastStyle {
	return ToastStyle{
		Container: plainStyle(s.Container),
		Icon:      plainStyle(s.Icon),
		Text:      plainStyle(s.Text),
		Action:    plainStyle(s.Action),
		Close:     plainStyle(s.Close),
	}
}

// ToastVariants holds the style of each kind of toast, for the theme's Toasts.
// It merges over Theme.Toast, which applies to every kind.
type ToastVariants struct {
	// Info, Success, Warning and Error are the styles of the toast kinds, and each merges over Toast.
	Info, Success, Warning, Error ToastStyle
}

// of returns the style of variant v.
func (tv ToastVariants) of(v ToastVariant) ToastStyle {
	switch v {
	case ToastSuccess:
		return tv.Success
	case ToastWarning:
		return tv.Warning
	case ToastError:
		return tv.Error
	}
	return tv.Info
}

// toastDefaults returns the built-in look of a toast of variant v, from the
// colors of t. Under high contrast every icon is the text color, and the border
// is drawn in the text color, so the shape tells the variant.
func toastDefaults(t *Theme, v ToastVariant) ToastStyle {
	edge, width := t.Border, float32(1)
	if t.HighContrast {
		edge, width = t.Text, 2
	}
	_, c := toastIcon(t, v)
	return ToastStyle{
		Container: Style{Direction: Row, Align: Center, Gap: 10, MinWidth: Px(280), MaxWidth: Px(420),
			Padding: XY(14, 10), Radius: t.Radius, Background: t.Surface, BorderWidth: width, BorderColor: edge,
			Shadows: []Shadow{ShadowMd}},
		Icon: Style{Width: Px(18), Height: Px(18), NoShrink: true, TextColor: c},
	}
}

// resolveToast returns the look of a toast of variant v: the defaults, merged
// with the theme's Toast, the theme's Toasts for the variant, and the toast's own
// Styles. Under high contrast the overrides are plain (see plainStyle).
func resolveToast(ctx *Context, v ToastVariant, inst ToastStyle) ToastStyle {
	t := ctx.Theme()
	return layered(ctx, toastDefaults(t, v), ToastStyle.merge, ToastStyle.plain,
		themeLayer(t.Toast), themeLayer(t.Toasts.of(v)), instanceLayer(inst))
}

// ToastPlacement is where a Toaster shows its stack of toasts.
type ToastPlacement uint8

const (
	// ToastBottomRight stacks the toasts in the bottom-right corner. It is the default.
	ToastBottomRight ToastPlacement = iota
	// ToastTopRight stacks them in the top-right corner.
	ToastTopRight
	// ToastBottomCenter stacks them along the bottom, centered.
	ToastBottomCenter
)

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
	styles   ToastStyle    // the look of this toast alone (see Styles)
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

// Dismiss removes the toast now. If the focus is on the toast, it moves on as it
// does for the toast's close button.
func (t *Toast) Dismiss() { t.rt.closeToast(t.e) }

// Styles sets the look of the parts of this toast alone. Set fields win over the
// theme's Toast and Toasts and the defaults of its kind; see ToastStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (t *Toast) Styles(st ToastStyle) *Toast {
	t.e.styles = t.e.styles.merge(st)
	return t
}

// toast adds a toast to the stack and announces it. When the stack is over its
// maximum, the oldest toast that is not hovered or focused goes.
func (rt *Runtime) toast(v ToastVariant, text string) *Toast {
	if !rt.toastsTimed() {
		rt.toastAt = rt.now() // the first timed toast's time starts now, not at the last frame
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
	rt.wakeToasts(rt.now()) // a toast's end is a wake-up, even when no frame is asked for
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

// toastsTimed reports whether a toast with a timeout is shown. Its time runs one
// frame at a time, so the runtime asks for frames while it is. A sticky toast,
// such as an error, has no time to run, so it asks for none.
func (rt *Runtime) toastsTimed() bool {
	for _, e := range rt.toasts {
		if !e.sticky {
			return true
		}
	}
	return false
}

// wakeToasts asks for a frame at the moment the next running toast runs out, so
// that a shown toast needs no frames while its time runs. A paused toast (hovered,
// or with focus inside) asks for none: the pause ending changes what is drawn, and
// that asks for a frame of its own.
func (rt *Runtime) wakeToasts(now time.Time) {
	for _, e := range rt.toasts {
		if !e.sticky && !e.hovered && !rt.focusWithin(toastKey(e.id)) {
			rt.wakeAt(now.Add(e.left))
		}
	}
}

// tickToasts runs the time of each toast that is not paused, and dismisses the
// ones that run out. It runs at the start of each frame.
func (rt *Runtime) tickToasts(now time.Time) {
	if !rt.toastsTimed() {
		// No toast is running its time, so the clock starts again when one is shown:
		// the frames in between were not asked for, and their time is not the toast's.
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
	key       string
	max       int
	placement ToastPlacement
}

// Toaster creates the stack of toasts. Its region is focusable with F6, as
// Windows' notification regions are, and it is named with the Notifications text of
// the window's Strings.
func Toaster() *ToasterElement { return &ToasterElement{} }

func (*ToasterElement) isElement()            {}
func (tv *ToasterElement) elementKey() string { return tv.key }

// Key identifies the toaster among its siblings (see DivElement.Key).
func (tv *ToasterElement) Key(k string) *ToasterElement { tv.key = k; return tv }

// Max sets how many toasts are shown at once. The oldest is dismissed to make
// room for a new one. The default is 3.
func (tv *ToasterElement) Max(n int) *ToasterElement { tv.max = n; return tv }

// Placement sets the corner or edge the stack is shown at: ToastBottomRight (the
// default), ToastTopRight or ToastBottomCenter.
func (tv *ToasterElement) Placement(p ToastPlacement) *ToasterElement { tv.placement = p; return tv }

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
	strs := ctx.Strings()
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	rt.toastRegion = expandedID(ctx.id)
	region := Div().Key("jamb.toasts").Absolute().Column().Gap(8).
		Role(access.RoleGroup).Label(strs.Notifications).
		Focusable().FocusVisible(Style{Shadows: []Shadow{ring}}).
		OnKeyDown(func(e *KeyEvent) {
			if e.Key == input.KeyEscape && e.Mods == 0 {
				e.StopPropagation()
				rt.leaveToastRegion()
			}
		})
	switch tv.placement {
	case ToastTopRight:
		region.Top(16).Right(16).Align(End)
	case ToastBottomCenter:
		region.Left(0).Right(0).Bottom(16).Align(Center)
	default:
		region.Right(16).Bottom(16).Align(End)
	}
	for _, e := range rt.toasts {
		region.Child(rt.toastView(ctx, strs, t, e))
	}
	return region
}

// toastView is one toast in the stack: an icon, the text, an optional action
// and a close button. It is a group named by its text.
func (rt *Runtime) toastView(ctx *Context, strs *Strings, t *Theme, e *toastEntry) *DivElement {
	st := resolveToast(ctx, e.variant, e.styles)
	ic, _ := toastIcon(t, e.variant)
	box := Div().asPart("Toast.Container").Key(toastKey(e.id)).Style(st.Container).
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
	box.Child(Div().asPart("Toast.Icon").Style(st.Icon).Child(Icon(ic).Size(18)))
	box.Child(Div().asPart("Toast.Text").Style(st.Text).Grow(1).Child(Text(e.text)))
	if e.action != "" {
		box.Child(Div().asPart("Toast.Action").Style(st.Action).Child(Button(e.action).Small().Ghost().OnClick(func() {
			if e.onAction != nil {
				e.onAction()
			}
			rt.dismissToast(e)
		})))
	}
	box.Child(Div().asPart("Toast.Close").Style(st.Close).Child(
		IconButton(icons.X).Label(strs.DismissNotification).Ghost().OnClick(func() { rt.closeToast(e) })))
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
