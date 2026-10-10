package ui

import (
	"slices"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// ButtonVariant selects a button's look.
type ButtonVariant uint8

const (
	Primary ButtonVariant = iota
	Secondary
	Ghost
	Danger
)

// ButtonElement is a themed, clickable button.
type ButtonElement struct {
	label    string
	icon     *icons.Icon
	variant  ButtonVariant
	small    bool
	disabled bool
	onClick  func()
	tooltip  string
	style    Style
	key      string
	name     string // the accessible name, see Label
	toggle   bool   // a toggle button, see Checked
	checked  bool
	inert    bool         // set by DropdownMenu: the wrapper is exposed instead (see accessRole)
	handle   *FocusHandle // set by Modal.Default, so the modal can focus the button
}

// Button creates a button with a text label. The default variant is Primary.
func Button(label string) *ButtonElement { return &ButtonElement{label: label} }

// IconButton creates a square, label-less Ghost button.
func IconButton(ic *icons.Icon) *ButtonElement {
	return &ButtonElement{icon: ic, variant: Ghost}
}

func (*ButtonElement) isElement()           {}
func (b *ButtonElement) elementKey() string { return b.key }

func (b *ButtonElement) Key(k string) *ButtonElement            { b.key = k; return b }
func (b *ButtonElement) OnClick(fn func()) *ButtonElement       { b.onClick = fn; return b }
func (b *ButtonElement) Icon(ic *icons.Icon) *ButtonElement     { b.icon = ic; return b }
func (b *ButtonElement) Variant(v ButtonVariant) *ButtonElement { b.variant = v; return b }
func (b *ButtonElement) Primary() *ButtonElement                { return b.Variant(Primary) }
func (b *ButtonElement) Secondary() *ButtonElement              { return b.Variant(Secondary) }
func (b *ButtonElement) Ghost() *ButtonElement                  { return b.Variant(Ghost) }
func (b *ButtonElement) Danger() *ButtonElement                 { return b.Variant(Danger) }
func (b *ButtonElement) Small() *ButtonElement                  { b.small = true; return b }
func (b *ButtonElement) Disabled(d bool) *ButtonElement         { b.disabled = d; return b }

// Label names the button for assistive technology. A button with text needs
// no label, since its text is its name; an icon-only button does.
func (b *ButtonElement) Label(s string) *ButtonElement { b.name = s; return b }

// accessName is the name the button is exposed with: its Label, then its text,
// then its tooltip.
func (b *ButtonElement) accessName() string {
	switch {
	case b.name != "":
		return b.name
	case b.label != "":
		return b.label
	}
	return b.tooltip
}

// Checked makes the button a toggle button, such as a pin that is on or off.
// Assistive technology reports its state, and the button's click handler
// changes it. Give an icon-only toggle a Label that names what it toggles.
func (b *ButtonElement) Checked(on bool) *ButtonElement { b.toggle, b.checked = true, on; return b }

// Tooltip shows s below the button after the pointer rests on it.
func (b *ButtonElement) Tooltip(s string) *ButtonElement { b.tooltip = s; return b }
func (b *ButtonElement) Style(s Style) *ButtonElement    { b.style = b.style.Merge(s); return b }

func (b *ButtonElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	h, px, fs, iconSize := float32(34), float32(14), float32(14), float32(16)
	if b.small {
		h, px, fs, iconSize = 28, 10, 13, 14
	}
	if b.label == "" {
		px = (h - iconSize) / 2
	}
	d := Div().Row().Align(Center).Justify(Center).Gap(7).
		MinHeight(Px(h)).PaddingX(px).Radius(t.Radius).
		FontSize(fs).FontWeight(FontMedium).
		Transition(110 * time.Millisecond).Shrink(0)
	switch b.variant {
	case Primary:
		d.Background(t.Accent).TextColor(t.AccentText).Shadow(ShadowSm).
			Hover(Style{Background: t.AccentHover}).Pressed(Style{Background: t.AccentActive})
	case Secondary:
		d.Background(t.Surface).Border(1, t.BorderStrong).TextColor(t.Text).Shadow(ShadowSm).
			Hover(Style{Background: t.SurfaceHover, BorderColor: t.TextMuted}).
			Pressed(Style{Background: t.SurfaceActive})
	case Ghost:
		d.TextColor(t.TextMuted).
			Hover(Style{Background: t.SurfaceHover, TextColor: t.Text}).
			Pressed(Style{Background: t.SurfaceActive})
	case Danger:
		label := Hex("#ffffff")
		if t.HighContrast {
			label = t.Background // the Danger fill is the window's text color there
		}
		d.Background(t.Danger).TextColor(label).Shadow(ShadowSm).
			Hover(Style{Background: t.Danger.Lighten(0.08)}).Pressed(Style{Background: t.Danger.Darken(0.1)})
	}
	if b.icon != nil {
		d.Child(Icon(b.icon).Size(iconSize))
	}
	if b.label != "" {
		d.Child(Text(b.label).NoWrap())
	}
	if b.disabled {
		d.Cursor(CursorNotAllowed)
		if t.HighContrast {
			// High contrast draws disabled controls in GrayText, not dimmed.
			d.Background(t.Surface).TextColor(t.Disabled).Border(1, t.Disabled)
		} else {
			d.Opacity(0.5)
		}
		d.hover, d.pressed = nil, nil
	} else {
		// Enabled buttons take part in Tab navigation and respond to Enter and
		// Space. The focus ring is a spread shadow, so it adds no layout shift.
		d.Focusable()
		if b.handle != nil {
			d.FocusHandle(b.handle)
		}
		if b.onClick != nil {
			d.OnClick(b.onClick)
		}
	}
	d.Style(b.style).Tooltip(b.tooltip)
	a := d.access()
	a.role, a.name, a.disabled, a.inert = access.RoleButton, b.name, b.disabled, b.inert
	if b.toggle {
		a.toggle, a.checked, a.checkedSet = true, b.checked, true
	}
	if !b.disabled {
		ring := Shadow{Spread: 2, Color: t.FocusRing}
		d.FocusVisible(Style{Shadows: append(slices.Clone(d.style.Shadows), ring)})
	}
	return d
}

// Divider is a 1px horizontal rule in the theme's border color.
func Divider() Element { return dividerElement{} }

type dividerElement struct{}

func (dividerElement) isElement() {}
func (dividerElement) expand(ctx *Context) Element {
	return Div().Height(Px(1)).Shrink(0).Background(ctx.Theme().Border)
}

// fieldOptions are shared by TextInput and TextArea.
type fieldOptions struct {
	value       *Signal[string]
	placeholder string
	name        string // the accessible name, see Label
	labelledBy  []string
	describedBy []string
	icon        *icons.Icon
	onSubmit    func(string)
	onChange    func(string)
	onBlur      func()        // set by NumberField, which checks its text on blur
	onStep      func(dir int) // set by NumberField, for Up and Down
	style       Style
	key         string
	borderless  bool
	small       bool
	autoFocus   bool
	handle      *FocusHandle
}

// TextInputElement is a single-line text field bound to a string signal.
// It supports selection (mouse, Shift+arrows, double-click), clipboard,
// undo/redo, word navigation (Ctrl+arrows) and any script.
type TextInputElement struct{ fieldOptions }

// TextInput creates a text field that reads and writes value.
func TextInput(value *Signal[string]) *TextInputElement {
	return &TextInputElement{fieldOptions{value: value}}
}

func (*TextInputElement) isElement()            {}
func (ti *TextInputElement) elementKey() string { return ti.key }

func (ti *TextInputElement) Key(k string) *TextInputElement         { ti.key = k; return ti }
func (ti *TextInputElement) Placeholder(s string) *TextInputElement { ti.placeholder = s; return ti }

// Label names the field for assistive technology. A field has no visible
// name of its own, so it should be given one; the placeholder is not used.
func (ti *TextInputElement) Label(s string) *TextInputElement { ti.name = s; return ti }

// LabelledBy names the field by the text of the elements with the given keys,
// such as a visible label above it (see DivElement.LabelledBy).
func (ti *TextInputElement) LabelledBy(keys ...string) *TextInputElement {
	ti.labelledBy = keys
	return ti
}

// DescribedBy describes the field by the text of the elements with the given keys.
func (ti *TextInputElement) DescribedBy(keys ...string) *TextInputElement {
	ti.describedBy = keys
	return ti
}
func (ti *TextInputElement) Icon(ic *icons.Icon) *TextInputElement      { ti.icon = ic; return ti }
func (ti *TextInputElement) OnSubmit(fn func(string)) *TextInputElement { ti.onSubmit = fn; return ti }
func (ti *TextInputElement) OnChange(fn func(string)) *TextInputElement { ti.onChange = fn; return ti }

// Style is merged into the field's frame: size (Width, Height, Min/Max),
// Padding, Margin, Radius, Background, BorderColor/BorderWidth and Grow
// apply to the frame; FontSize, FontWeight, FontFamily and TextColor are
// inherited by the text inside.
func (ti *TextInputElement) Style(s Style) *TextInputElement { ti.style = ti.style.Merge(s); return ti }

// Small makes a compact field (28px tall, 13px text), matching Button.Small.
func (ti *TextInputElement) Small() *TextInputElement         { ti.small = true; return ti }
func (ti *TextInputElement) Width(l Length) *TextInputElement { ti.style.Width = l; return ti }

// Borderless drops the field's frame and padding, for inputs that should
// look like plain text until clicked (titles, inline renames).
func (ti *TextInputElement) Borderless() *TextInputElement { ti.borderless = true; return ti }

// AutoFocus focuses the field when it first appears.
func (ti *TextInputElement) AutoFocus() *TextInputElement { ti.autoFocus = true; return ti }

// FocusHandle lets code focus the field (see NewFocusHandle).
func (ti *TextInputElement) FocusHandle(h *FocusHandle) *TextInputElement { ti.handle = h; return ti }

func (ti *TextInputElement) expand(ctx *Context) Element {
	return ti.fieldOptions.expand(ctx, false, 0)
}

// TextAreaElement is a multi-line text editor bound to a string signal. It
// wraps lines, grows with its content and keeps the caret visible inside
// the nearest scrolling ancestor.
type TextAreaElement struct {
	fieldOptions
	minRows int
}

// TextArea creates a multi-line editor that reads and writes value.
func TextArea(value *Signal[string]) *TextAreaElement {
	return &TextAreaElement{fieldOptions: fieldOptions{value: value}, minRows: 3}
}

func (*TextAreaElement) isElement()            {}
func (ta *TextAreaElement) elementKey() string { return ta.key }

func (ta *TextAreaElement) Key(k string) *TextAreaElement         { ta.key = k; return ta }
func (ta *TextAreaElement) Placeholder(s string) *TextAreaElement { ta.placeholder = s; return ta }

// Label names the field for assistive technology; see TextInputElement.Label.
func (ta *TextAreaElement) Label(s string) *TextAreaElement { ta.name = s; return ta }

// LabelledBy names the editor by the text of the elements with the given keys;
// see TextInputElement.LabelledBy.
func (ta *TextAreaElement) LabelledBy(keys ...string) *TextAreaElement {
	ta.labelledBy = keys
	return ta
}

// DescribedBy describes the editor by the text of the elements with the given keys.
func (ta *TextAreaElement) DescribedBy(keys ...string) *TextAreaElement {
	ta.describedBy = keys
	return ta
}
func (ta *TextAreaElement) OnChange(fn func(string)) *TextAreaElement { ta.onChange = fn; return ta }

// Style is merged into the editor's frame, as for TextInputElement.Style.
func (ta *TextAreaElement) Style(s Style) *TextAreaElement  { ta.style = ta.style.Merge(s); return ta }
func (ta *TextAreaElement) Width(l Length) *TextAreaElement { ta.style.Width = l; return ta }

// MinRows sets the minimum height in lines (default 3).
func (ta *TextAreaElement) MinRows(n int) *TextAreaElement { ta.minRows = n; return ta }

// Borderless drops the frame and padding, for full-pane editors.
func (ta *TextAreaElement) Borderless() *TextAreaElement { ta.borderless = true; return ta }

// AutoFocus focuses the editor when it first appears.
func (ta *TextAreaElement) AutoFocus() *TextAreaElement { ta.autoFocus = true; return ta }

// FocusHandle lets code focus the editor (see NewFocusHandle).
func (ta *TextAreaElement) FocusHandle(h *FocusHandle) *TextAreaElement { ta.handle = h; return ta }

func (ta *TextAreaElement) expand(ctx *Context) Element {
	return ta.fieldOptions.expand(ctx, true, ta.minRows)
}

func (f *fieldOptions) expand(ctx *Context, multiline bool, minRows int) Element {
	t := ctx.Theme()
	// Reading the value subscribes this field, so it re-renders (and
	// re-measures) when the text changes.
	f.value.Get()
	st := ctx.state()
	h := f.handle
	if h == nil {
		if st.focus == nil {
			st.focus = NewFocusHandle()
		}
		h = st.focus
	}
	ed := &editorElement{
		value: f.value, placeholder: f.placeholder, name: f.name, labelledBy: f.labelledBy, describedBy: f.describedBy, multiline: multiline,
		onSubmit: f.onSubmit, onChange: f.onChange, onBlur: f.onBlur, onStep: f.onStep, autoFocus: f.autoFocus, handle: h,
		style: Style{Grow: 1},
	}
	if multiline && minRows > 0 {
		size := t.FontSize
		if f.style.FontSize > 0 {
			size = f.style.FontSize
		}
		ed.style.MinHeight = Px(float32(minRows) * size * float32(ctx.TextScale()) / 100 * 1.4)
	}
	if f.borderless {
		box := Div().Row().Align(Start).Gap(8).Cursor(CursorText).
			OnMouseDown(func(*MouseEvent) { h.Focus() })
		if f.icon != nil {
			box.Child(Icon(f.icon).Size(16).Color(t.TextSubtle))
		}
		return box.Child(ed).Style(f.style)
	}
	box := Div().Row().Gap(8).PaddingX(10).
		Radius(t.Radius).Background(t.Surface).Border(1, t.BorderStrong).
		Cursor(CursorText).Transition(110 * time.Millisecond).
		Hover(Style{BorderColor: t.TextMuted}).
		FocusWithin(Style{BorderColor: t.Accent}).
		// Clicks on the frame (padding, icon) still focus the text.
		OnMouseDown(func(*MouseEvent) { h.Focus() })
	switch {
	case multiline:
		box.Align(Start).PaddingY(8)
	case f.small:
		box.Align(Center).MinHeight(Px(28)).PaddingX(8).FontSize(13)
	default:
		box.Align(Center).MinHeight(Px(34))
	}
	if f.icon != nil {
		box.Child(Icon(f.icon).Size(16).Color(t.TextSubtle))
	}
	return box.Child(ed).Style(f.style)
}
