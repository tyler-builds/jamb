package ui

import (
	"image"
	"slices"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// ButtonVariant selects a button's look.
type ButtonVariant uint8

const (
	ButtonPrimary ButtonVariant = iota
	ButtonSecondary
	ButtonGhost
	ButtonDanger
)

// ButtonStyle is the look of each part of a button. A part's style merges over
// the built-in defaults of its variant, then over the theme's Button, then over
// the theme's Buttons for that variant, then over the Styles of the instance;
// see docs/styling.md. The states are parts of their own: RootHover while the
// pointer is over an enabled button, RootPressed while it is pressed,
// RootDisabled when it is disabled, and RootFocusVisible while it has keyboard
// focus (its focus ring is added to the shadows of Root).
type ButtonStyle struct {
	Root             Style // the button: its size, shape, fill, border, font and text color
	RootHover        Style // the button while the pointer is over it, if enabled
	RootPressed      Style // the button while it is pressed
	RootDisabled     Style // the button when it is disabled
	RootFocusVisible Style // the button while it has keyboard focus
	Label            Style // the text of the label
	Icon             Style // the icon
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ButtonStyle) merge(o ButtonStyle) ButtonStyle {
	return ButtonStyle{
		Root:             s.Root.Merge(o.Root),
		RootHover:        s.RootHover.Merge(o.RootHover),
		RootPressed:      s.RootPressed.Merge(o.RootPressed),
		RootDisabled:     s.RootDisabled.Merge(o.RootDisabled),
		RootFocusVisible: s.RootFocusVisible.Merge(o.RootFocusVisible),
		Label:            s.Label.Merge(o.Label),
		Icon:             s.Icon.Merge(o.Icon),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ButtonStyle) plain() ButtonStyle {
	return ButtonStyle{
		Root:             plainStyle(s.Root),
		RootHover:        plainStyle(s.RootHover),
		RootPressed:      plainStyle(s.RootPressed),
		RootDisabled:     plainStyle(s.RootDisabled),
		RootFocusVisible: plainStyle(s.RootFocusVisible),
		Label:            plainStyle(s.Label),
		Icon:             plainStyle(s.Icon),
	}
}

// ButtonVariants holds the style of each variant of a button, for the theme's
// Buttons. It merges over Theme.Button, which applies to every variant.
type ButtonVariants struct {
	Primary   ButtonStyle
	Secondary ButtonStyle
	Ghost     ButtonStyle
	Danger    ButtonStyle
}

// of returns the style of variant v.
func (bv ButtonVariants) of(v ButtonVariant) ButtonStyle {
	switch v {
	case ButtonSecondary:
		return bv.Secondary
	case ButtonGhost:
		return bv.Ghost
	case ButtonDanger:
		return bv.Danger
	}
	return bv.Primary
}

// controlSize is the size of a control: the default, Small or Large. The
// controls that have Small and Large share these sizes, so they line up in a row.
type controlSize uint8

const (
	sizeDefault controlSize = iota
	sizeSmall
	sizeLarge
)

// field returns a field's height, horizontal padding and text size at s, at the
// Text size of 100 percent.
func (s controlSize) field() (h, padX, fontSize float32) {
	switch s {
	case sizeSmall:
		return 28, 8, 13
	case sizeLarge:
		return 40, 12, 16
	}
	return 34, 10, 14
}

// buttonIconSize is the size of a button's icon at size s.
func buttonIconSize(s controlSize) float32 {
	switch s {
	case sizeSmall:
		return 14
	case sizeLarge:
		return 18
	}
	return 16
}

// buttonDefaults returns the built-in look of a button of variant v, from the
// colors of t. size is its size, iconOnly a button with no label, and
// disabled the state the disabled style is chosen for. Under high contrast a
// disabled button is GrayText and not dimmed.
func buttonDefaults(t *Theme, v ButtonVariant, size controlSize, iconOnly bool) ButtonStyle {
	h, _, fs := size.field()
	px := float32(14)
	switch size {
	case sizeSmall:
		px = 10
	case sizeLarge:
		px = 18
	}
	if iconOnly {
		px = (h - buttonIconSize(size)) / 2
	}
	d := ButtonStyle{
		Root: Style{Direction: Row, Align: Center, Justify: Center, Gap: 7, MinHeight: Px(h),
			Padding: XY(px, 0), Radius: t.Radius, FontSize: fs, FontWeight: FontMedium,
			Transition: 110 * time.Millisecond, NoShrink: true, fitWidth: true},
		RootDisabled: Style{Opacity: 0.5},
	}
	switch v {
	case ButtonPrimary:
		d.Root.Background, d.Root.TextColor, d.Root.Shadows = t.Accent, t.AccentText, []Shadow{ShadowSm}
		d.RootHover = Style{Background: t.AccentHover}
		d.RootPressed = Style{Background: t.AccentActive}
	case ButtonSecondary:
		d.Root.Background, d.Root.BorderWidth, d.Root.BorderColor = t.Surface, 1, t.BorderStrong
		d.Root.TextColor, d.Root.Shadows = t.Text, []Shadow{ShadowSm}
		d.RootHover = Style{Background: t.SurfaceHover, BorderColor: t.TextMuted}
		d.RootPressed = Style{Background: t.SurfaceActive}
	case ButtonGhost:
		d.Root.TextColor = t.TextMuted
		d.RootHover = Style{Background: t.SurfaceHover, TextColor: t.Text}
		d.RootPressed = Style{Background: t.SurfaceActive}
	case ButtonDanger:
		label := Hex("#ffffff")
		if t.HighContrast {
			label = t.Background // the Danger fill is the window's text color there
		}
		d.Root.Background, d.Root.TextColor, d.Root.Shadows = t.Danger, label, []Shadow{ShadowSm}
		d.RootHover = Style{Background: t.Danger.Lighten(0.08)}
		d.RootPressed = Style{Background: t.Danger.Darken(0.1)}
	}
	if t.HighContrast {
		// High contrast draws a disabled button in GrayText, not dimmed.
		d.RootDisabled = Style{Background: t.Surface, TextColor: t.Disabled, BorderWidth: 1, BorderColor: t.Disabled}
	}
	return d
}

// resolveButton returns the look of a button: the defaults of its variant,
// merged with the theme's Button, the theme's Buttons for the variant, and then
// the instance's Styles. Under high contrast the overrides are plain.
func resolveButton(t *Theme, v ButtonVariant, size controlSize, iconOnly bool, inst ButtonStyle) ButtonStyle {
	st := buttonDefaults(t, v, size, iconOnly)
	for _, o := range []ButtonStyle{t.Button, t.Buttons.of(v), inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// ButtonElement is a themed, clickable button.
type ButtonElement struct {
	label    string
	icon     *icons.Icon
	variant  ButtonVariant
	ctlSize  controlSize // see Small and Large
	disabled bool
	onClick  func()
	tooltip  string
	styles   ButtonStyle
	key      string
	name     string // the accessible name, see Label
	toggle   bool   // a toggle button, see AccessChecked
	checked  bool
	inert    bool         // set by DropdownMenu: the wrapper is exposed instead (see accessRole)
	handle   *FocusHandle // see FocusHandle; Modal.Default sets one if there is none
	size     sizing
}

// Button creates a button with a text label. The default variant is Primary.
func Button(label string) *ButtonElement { return &ButtonElement{label: label} }

// IconButton creates a square, label-less Ghost button.
func IconButton(ic *icons.Icon) *ButtonElement {
	return &ButtonElement{icon: ic, variant: ButtonGhost}
}

func (*ButtonElement) isElement()           {}
func (b *ButtonElement) elementKey() string { return b.key }

func (b *ButtonElement) Key(k string) *ButtonElement            { b.key = k; return b }
func (b *ButtonElement) OnClick(fn func()) *ButtonElement       { b.onClick = fn; return b }
func (b *ButtonElement) Icon(ic *icons.Icon) *ButtonElement     { b.icon = ic; return b }
func (b *ButtonElement) Variant(v ButtonVariant) *ButtonElement { b.variant = v; return b }
func (b *ButtonElement) Primary() *ButtonElement                { return b.Variant(ButtonPrimary) }
func (b *ButtonElement) Secondary() *ButtonElement              { return b.Variant(ButtonSecondary) }
func (b *ButtonElement) Ghost() *ButtonElement                  { return b.Variant(ButtonGhost) }
func (b *ButtonElement) Danger() *ButtonElement                 { return b.Variant(ButtonDanger) }
func (b *ButtonElement) Small() *ButtonElement                  { b.ctlSize = sizeSmall; return b }

// Large makes the button larger: 40 px high with 16 px text, for prominent or
// touch-friendly actions. Small and Large replace each other.
func (b *ButtonElement) Large() *ButtonElement          { b.ctlSize = sizeLarge; return b }
func (b *ButtonElement) Disabled(d bool) *ButtonElement { b.disabled = d; return b }

// The sizing methods give the button's outer box a size. A button is as wide as
// its label by default, and Width(Full) fills its container (see Sizing in
// docs/styling.md).
func (b *ButtonElement) Width(l Length) *ButtonElement          { b.size.setWidth(l); return b }
func (b *ButtonElement) Height(l Length) *ButtonElement         { b.size.setHeight(l); return b }
func (b *ButtonElement) MinWidth(l Length) *ButtonElement       { b.size.setMinWidth(l); return b }
func (b *ButtonElement) MinHeight(l Length) *ButtonElement      { b.size.setMinHeight(l); return b }
func (b *ButtonElement) MaxWidth(l Length) *ButtonElement       { b.size.setMaxWidth(l); return b }
func (b *ButtonElement) MaxHeight(l Length) *ButtonElement      { b.size.setMaxHeight(l); return b }
func (b *ButtonElement) Grow(f float32) *ButtonElement          { b.size.setGrow(f); return b }
func (b *ButtonElement) GridSpan(cols, rows int) *ButtonElement { b.size.setSpan(cols, rows); return b }

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

// FocusHandle lets code focus the button (see NewFocusHandle). A modal's
// Default button keeps the handle it was given.
func (b *ButtonElement) FocusHandle(h *FocusHandle) *ButtonElement { b.handle = h; return b }

// AccessChecked makes the button a toggle button, such as a pin that is on or off.
// Assistive technology reports its state, and the button's click handler
// changes it. Give an icon-only toggle a Label that names what it toggles.
func (b *ButtonElement) AccessChecked(on bool) *ButtonElement {
	b.toggle, b.checked = true, on
	return b
}

// Tooltip shows s below the button after the pointer rests on it.
func (b *ButtonElement) Tooltip(s string) *ButtonElement { b.tooltip = s; return b }

// Style sets the look of the button's Root part, for this instance only. It is
// the same as Styles(ButtonStyle{Root: s}); calling it again merges.
func (b *ButtonElement) Style(s Style) *ButtonElement {
	b.styles.Root = b.styles.Root.Merge(s)
	return b
}

// Styles sets the look of the parts of the button, for this instance only. Set
// fields win over the theme's Button and Buttons and the defaults of the
// variant; see ButtonStyle and docs/styling.md. Calling Styles again merges with
// the earlier values.
func (b *ButtonElement) Styles(st ButtonStyle) *ButtonElement {
	b.styles = b.styles.merge(st)
	return b
}

func (b *ButtonElement) expand(ctx *Context) Element {
	t := ctx.Theme()
	iconSize := buttonIconSize(b.ctlSize)
	st := resolveButton(t, b.variant, b.ctlSize, b.label == "", b.styles)
	d := Div().asPart("Button.Root").Style(st.Root)
	if b.disabled {
		d.Style(st.RootDisabled).Cursor(CursorNotAllowed)
		d.hover, d.pressed = nil, nil
	} else {
		d.Hover(st.RootHover).Pressed(st.RootPressed)
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
	if b.icon != nil {
		d.Child(Div().asPart("Button.Icon").Style(st.Icon).Child(Icon(b.icon).Size(iconSize)))
	}
	if b.label != "" {
		d.Child(Div().asPart("Button.Label").Style(st.Label).Child(Text(b.label).NoWrap()))
	}
	d.Tooltip(b.tooltip)
	a := d.access()
	a.role, a.name, a.disabled, a.inert = access.RoleButton, b.name, b.disabled, b.inert
	if b.toggle {
		a.toggle, a.checked, a.checkedSet = true, b.checked, true
	}
	if !b.disabled {
		// The focus ring is added to the shadows of the button, so it is drawn
		// with the button's own shadow. RootFocusVisible can replace it.
		ring := Shadow{Spread: 2, Color: t.FocusRing}
		focus := Style{Shadows: append(slices.Clone(st.Root.Shadows), ring)}.Merge(st.RootFocusVisible)
		d.FocusVisible(focus)
	}
	return b.size.apply(d)
}

// Divider is a 1px horizontal rule in the theme's border color.
func Divider() Element { return dividerElement{} }

type dividerElement struct{}

func (dividerElement) isElement() {}
func (dividerElement) expand(ctx *Context) Element {
	return Div().Height(Px(1)).Shrink(0).Background(ctx.Theme().Border)
}

// FieldStyle is the look of the text frame of a text field: TextInput, TextArea
// and NumberInput. A part's style merges over the built-in defaults, then over
// the theme's Field, then over the Styles of the instance. A NumberInput also
// takes its theme's and instance's NumberInput parts; see docs/styling.md.
//
// Frame is the box around the text. Its states are FrameHover, while the pointer
// is over an editable field, and FrameFocusWithin, while the focus is inside it.
// FrameDisabled is the frame of a disabled field, FrameReadOnly that of a
// read-only one (see TextInputElement.ReadOnly), and FrameInvalid that of a field
// whose value is not valid.
// Text is the text inside the frame, with its TextColor and font, and Selection
// is the background of the selected text. Placeholder is the text shown while a
// TextInput or TextArea is empty. Leading and Trailing are the containers of the
// content before and after the text: the icon (see Icon) and the element of
// Leading, and the element of Trailing. A number input has no placeholder, icon
// or element, so Placeholder, Leading and Trailing do not apply to it. The caret
// is drawn in the text color, and its width and blink follow the system settings.
type FieldStyle struct {
	Frame            Style // the box around the text; not drawn when a field is Borderless
	FrameHover       Style // the frame while the pointer is over it, if the field is enabled (paint only)
	FrameFocusWithin Style // the frame while the focus is inside it (paint only)
	FrameDisabled    Style // the frame of a disabled field
	FrameReadOnly    Style // the frame of a read-only field
	FrameInvalid     Style // the frame of a field whose value is not valid
	Text             Style // the text inside the frame: TextColor and font
	Placeholder      Style // the placeholder, while the field is empty
	Selection        Style // the background of the selected text
	Leading          Style // the container of the icon and the Leading element, before the text
	Trailing         Style // the container of the Trailing element, after the text
}

// merge returns s with the set fields of o applied on top, part by part.
func (s FieldStyle) merge(o FieldStyle) FieldStyle {
	return FieldStyle{
		Frame:            s.Frame.Merge(o.Frame),
		FrameHover:       s.FrameHover.Merge(o.FrameHover),
		FrameFocusWithin: s.FrameFocusWithin.Merge(o.FrameFocusWithin),
		FrameDisabled:    s.FrameDisabled.Merge(o.FrameDisabled),
		FrameReadOnly:    s.FrameReadOnly.Merge(o.FrameReadOnly),
		FrameInvalid:     s.FrameInvalid.Merge(o.FrameInvalid),
		Text:             s.Text.Merge(o.Text),
		Placeholder:      s.Placeholder.Merge(o.Placeholder),
		Selection:        s.Selection.Merge(o.Selection),
		Leading:          s.Leading.Merge(o.Leading),
		Trailing:         s.Trailing.Merge(o.Trailing),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s FieldStyle) plain() FieldStyle {
	return FieldStyle{
		Frame:            plainStyle(s.Frame),
		FrameHover:       plainStyle(s.FrameHover),
		FrameFocusWithin: plainStyle(s.FrameFocusWithin),
		FrameDisabled:    plainStyle(s.FrameDisabled),
		FrameReadOnly:    plainStyle(s.FrameReadOnly),
		FrameInvalid:     plainStyle(s.FrameInvalid),
		Text:             plainStyle(s.Text),
		Placeholder:      plainStyle(s.Placeholder),
		Selection:        plainStyle(s.Selection),
		Leading:          plainStyle(s.Leading),
		Trailing:         plainStyle(s.Trailing),
	}
}

// fieldLayout is the layout a text frame is built for: a multi-line editor, a
// compact one (Small), or a disabled field.
type fieldLayout struct {
	multiline, disabled bool
	size                controlSize
	width               Length // the frame's width by default (see textFieldWidth)
}

// textFieldWidth is the default width of a text field at the Text size of 100
// percent, like a text input's default size in CSS. A field is fixed at this width
// by default, so it does not stretch; Width(Full) stretches it. It scales with the
// Text size, as the number input's width does.
const textFieldWidth = 240

// fieldDefaults returns the built-in look of a text frame with layout lay, from
// the colors of t. Under high contrast a disabled frame is GrayText and not
// dimmed, and an invalid frame is drawn in the text color.
func fieldDefaults(t *Theme, lay fieldLayout) FieldStyle {
	d := FieldStyle{
		Frame: Style{Direction: Row, Align: Center, Gap: 8, Padding: XY(10, 0), MinHeight: Px(34),
			Width: lay.width, contentMin: true, Radius: t.Radius, Background: t.Surface, BorderWidth: 1, BorderColor: t.BorderStrong,
			Transition: 110 * time.Millisecond},
		FrameHover:       Style{BorderColor: t.TextMuted},
		FrameFocusWithin: Style{BorderColor: t.Accent},
		FrameInvalid:     Style{BorderWidth: 2, BorderColor: t.Danger},
		Text:             Style{TextColor: t.Text},
		Placeholder:      Style{TextColor: t.TextSubtle},
		Selection:        Style{Background: t.Accent.Alpha(0.32)},
		Leading:          Style{Direction: Row, Align: Center, NoShrink: true, TextColor: t.TextSubtle},
		Trailing:         Style{Direction: Row, Align: Center, NoShrink: true},
	}
	if t.HighContrast {
		// The selection is the Highlight color with HighlightText on it, as Windows draws it.
		d.Selection = Style{Background: t.Accent, TextColor: t.AccentText}
	}
	// A read-only field keeps its border and full-contrast text, on a fill that
	// sets it apart from an editable one. The step between surfaces is smaller in
	// the dark theme, so it takes the next one.
	// Under high contrast it keeps the window color, as every field does there:
	// the theme's surfaces are system colors such as Highlight.
	switch {
	case t.HighContrast:
	case t.Dark:
		d.FrameReadOnly = Style{Background: t.SurfaceActive}
	default:
		d.FrameReadOnly = Style{Background: t.SurfaceHover}
	}
	if !lay.disabled {
		d.Frame.Cursor = CursorText
	}
	switch {
	case lay.multiline:
		d.Frame.Align, d.Frame.MinHeight, d.Frame.Padding = Start, Auto, XY(10, 8)
	case lay.size != sizeDefault:
		h, px, fs := lay.size.field()
		d.Frame.MinHeight, d.Frame.Padding, d.Frame.FontSize = Px(h), XY(px, 0), fs
	}
	if t.HighContrast {
		d.FrameInvalid.BorderColor = t.Text
		d.FrameDisabled = Style{BorderColor: t.Disabled, TextColor: t.Disabled}
		if lay.disabled {
			d.Text.TextColor = t.Disabled
		}
	} else {
		d.FrameDisabled = Style{Opacity: 0.5, BorderColor: t.Border}
	}
	return d
}

// resolveField returns the look of a text frame with layout lay: the defaults,
// merged with the theme's Field and then the instance's Styles. Under high
// contrast the two overrides are plain (see plainStyle).
func resolveField(t *Theme, lay fieldLayout, inst FieldStyle) FieldStyle {
	st := fieldDefaults(t, lay)
	for _, o := range []FieldStyle{t.Field, inst} {
		if t.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// fieldOptions are shared by TextInput, TextArea and NumberInput.
type fieldOptions struct {
	value       *Signal[string]
	placeholder string
	name        string // the accessible name, see Label
	labelledBy  []string
	describedBy []string
	icon        *icons.Icon
	leadingEl   Element // drawn in the Leading part, after the icon (see Leading)
	trailingEl  Element // drawn in the Trailing part, after the text (see Trailing)
	onSubmit    func(string)
	onChange    func(string)
	onPasteImg  func(image.Image) // see OnPasteImage
	onBlur      func()            // set by NumberInput, which checks its text on blur
	onCancel    func()            // set by DataGrid, which ends an edit on Escape in place of Cancel
	description string            // set by DataGrid, to say why the last save failed (a literal, not a key)
	onStep      func(dir int)     // set by NumberInput, for Up and Down
	readOnly    bool              // see ReadOnly
	tooltip     string            // shown over the frame; the field's description when it has none
	disabled    bool              // see Disabled
	styles      FieldStyle        // the instance's parts: from Styles, and the frame's Style
	key         string
	borderless  bool
	ctlSize     controlSize // see Small and Large
	autoFocus   bool
	handle      *FocusHandle
	widget      string              // the widget this field is, for the part names (see asPart)
	align       Alignment           // where a single-line text sits in the field, zero for Start (NumberInput's Align)
	trailing    Element             // drawn inside the frame's right edge, after the text (NumberInput's stacked spin buttons)
	overlays    []Element           // laid over the frame's edges, after the text, out of its flow (NumberInput's inline spin buttons)
	rules       []Validator[string] // the validators of the value (see Validate)
	wire        fieldWire           // what a Field gives the control (see Field)
	size        sizing              // the frame's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// formCheck and formWire make a text field a formControl (see Field).
func (f *fieldOptions) formCheck() error { return firstError(f.value.Get(), f.rules) }

func (ti *TextInputElement) formWire(w fieldWire) Element {
	c := *ti
	c.wire = w
	return &c
}

func (ta *TextAreaElement) formWire(w fieldWire) Element {
	c := *ta
	c.wire = w
	return &c
}

// TextInputElement is a single-line text field bound to a string signal.
// It supports selection (mouse, Shift+arrows, double-click), clipboard,
// undo/redo, word navigation (Ctrl+arrows) and any script.
type TextInputElement struct{ fieldOptions }

// TextInput creates a text field that reads and writes value.
func TextInput(value *Signal[string]) *TextInputElement {
	return &TextInputElement{fieldOptions{value: value, widget: "TextInput"}}
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

// ReadOnly shows the text without letting it change: the field can still be
// focused, and its text selected and copied. Assistive technology reports it as
// read-only. Its frame is the FrameReadOnly part.
func (ti *TextInputElement) ReadOnly(on bool) *TextInputElement { ti.readOnly = on; return ti }

// OnPasteImage runs fn with the image on the clipboard when the user pastes
// (Ctrl+V or the Paste action) and the clipboard holds an image but no text,
// as when a screenshot is pasted into a message box. Without it, such a paste
// does nothing. A read-only or disabled field ignores it.
func (ti *TextInputElement) OnPasteImage(fn func(image.Image)) *TextInputElement {
	ti.onPasteImg = fn
	return ti
}

// Tooltip shows s below the field after the pointer rests on it. Assistive
// technology reads it as the field's description, unless it has another.
func (ti *TextInputElement) Tooltip(s string) *TextInputElement { ti.tooltip = s; return ti }

// Disabled shows the text but takes the field out of use: it can't be focused,
// selected or changed. Its frame is the FrameDisabled part.
func (ti *TextInputElement) Disabled(d bool) *TextInputElement { ti.disabled = d; return ti }

// Style is merged into the Frame part of the field, over the Styles of the
// instance: Padding, Margin, Radius, Background and BorderColor/BorderWidth apply
// to the frame, and FontSize, FontWeight, FontFamily and TextColor are inherited
// by the text inside. A size set here is overridden by the sizing methods (Width
// and the others); see Sizing in docs/styling.md. It is the same as
// Styles(FieldStyle{Frame: s}).
func (ti *TextInputElement) Style(s Style) *TextInputElement {
	ti.styles.Frame = ti.styles.Frame.Merge(s)
	return ti
}

// Styles sets the look of the parts of the field, for this instance only. Set
// fields win over the theme's Field and the defaults; see FieldStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (ti *TextInputElement) Styles(s FieldStyle) *TextInputElement {
	ti.styles = ti.styles.merge(s)
	return ti
}

// Leading puts el inside the frame before the text, after the icon (see Icon).
// It is drawn in the Leading part.
func (ti *TextInputElement) Leading(el Element) *TextInputElement { ti.leadingEl = el; return ti }

// Trailing puts el inside the frame after the text, such as a button. It is
// drawn in the Trailing part.
func (ti *TextInputElement) Trailing(el Element) *TextInputElement { ti.trailingEl = el; return ti }

// Small makes a compact field (28px tall, 13px text), matching Button.Small.
func (ti *TextInputElement) Small() *TextInputElement { ti.ctlSize = sizeSmall; return ti }

// Large makes the field larger: 40 px high with 16 px text, like Button.Large.
// Small and Large replace each other.
func (ti *TextInputElement) Large() *TextInputElement { ti.ctlSize = sizeLarge; return ti }

// The sizing methods give the field's frame a size. A field is textFieldWidth wide
// by default, so Width(Full) fills its container, and the frame never shrinks below
// the room for a few characters (see MinWidth). See Sizing in docs/styling.md.
func (ti *TextInputElement) Width(l Length) *TextInputElement     { ti.size.setWidth(l); return ti }
func (ti *TextInputElement) Height(l Length) *TextInputElement    { ti.size.setHeight(l); return ti }
func (ti *TextInputElement) MinWidth(l Length) *TextInputElement  { ti.size.setMinWidth(l); return ti }
func (ti *TextInputElement) MinHeight(l Length) *TextInputElement { ti.size.setMinHeight(l); return ti }
func (ti *TextInputElement) MaxWidth(l Length) *TextInputElement  { ti.size.setMaxWidth(l); return ti }
func (ti *TextInputElement) MaxHeight(l Length) *TextInputElement { ti.size.setMaxHeight(l); return ti }
func (ti *TextInputElement) Grow(f float32) *TextInputElement     { ti.size.setGrow(f); return ti }
func (ti *TextInputElement) GridSpan(cols, rows int) *TextInputElement {
	ti.size.setSpan(cols, rows)
	return ti
}

// Borderless drops the field's frame and padding, for inputs that should
// look like plain text until clicked (titles, inline renames). The Frame part
// is not drawn, so its styles do not apply.
func (ti *TextInputElement) Borderless() *TextInputElement { ti.borderless = true; return ti }

// AutoFocus focuses the field when it first appears.
func (ti *TextInputElement) AutoFocus() *TextInputElement { ti.autoFocus = true; return ti }

// FocusHandle lets code focus the field (see NewFocusHandle).
func (ti *TextInputElement) FocusHandle(h *FocusHandle) *TextInputElement { ti.handle = h; return ti }

// Validate sets the validators of the text: the field shows the first error of
// them once the user has changed the text and left the field, and on submit when
// it is in a Form (see Field and Validator). An empty text passes MinLength and
// Pattern, so pair those with Required when the field must be filled in.
func (ti *TextInputElement) Validate(rules ...Validator[string]) *TextInputElement {
	ti.rules = rules
	return ti
}

func (ti *TextInputElement) expand(ctx *Context) Element {
	lay := fieldLayout{size: ti.ctlSize, disabled: ti.disabled, width: Px(textFieldWidth * textScale(ctx))}
	st := resolveField(ctx.Theme(), lay, ti.styles)
	return ti.fieldOptions.expand(ctx, st, false, 0)
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
	return &TextAreaElement{fieldOptions: fieldOptions{value: value, widget: "TextArea"}, minRows: 3}
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

// ReadOnly shows the text without letting it change: the field can still be
// focused, and its text selected and copied. Assistive technology reports it as
// read-only. Its frame is the FrameReadOnly part.
func (ta *TextAreaElement) ReadOnly(on bool) *TextAreaElement { ta.readOnly = on; return ta }

// OnPasteImage runs fn with the image on the clipboard when the user pastes
// and the clipboard holds an image but no text (see TextInputElement.OnPasteImage).
func (ta *TextAreaElement) OnPasteImage(fn func(image.Image)) *TextAreaElement {
	ta.onPasteImg = fn
	return ta
}

// Tooltip shows s below the field after the pointer rests on it. Assistive
// technology reads it as the field's description, unless it has another.
func (ta *TextAreaElement) Tooltip(s string) *TextAreaElement { ta.tooltip = s; return ta }

// Disabled shows the text but takes the field out of use: it can't be focused,
// selected or changed. Its frame is the FrameDisabled part.
func (ta *TextAreaElement) Disabled(d bool) *TextAreaElement { ta.disabled = d; return ta }

// Style is merged into the Frame part of the editor, as for TextInputElement.Style.
func (ta *TextAreaElement) Style(s Style) *TextAreaElement {
	ta.styles.Frame = ta.styles.Frame.Merge(s)
	return ta
}

// The sizing methods give the editor's frame a size, as for TextInput. A text area
// is textFieldWidth wide by default.
func (ta *TextAreaElement) Width(l Length) *TextAreaElement     { ta.size.setWidth(l); return ta }
func (ta *TextAreaElement) Height(l Length) *TextAreaElement    { ta.size.setHeight(l); return ta }
func (ta *TextAreaElement) MinWidth(l Length) *TextAreaElement  { ta.size.setMinWidth(l); return ta }
func (ta *TextAreaElement) MinHeight(l Length) *TextAreaElement { ta.size.setMinHeight(l); return ta }
func (ta *TextAreaElement) MaxWidth(l Length) *TextAreaElement  { ta.size.setMaxWidth(l); return ta }
func (ta *TextAreaElement) MaxHeight(l Length) *TextAreaElement { ta.size.setMaxHeight(l); return ta }
func (ta *TextAreaElement) Grow(f float32) *TextAreaElement     { ta.size.setGrow(f); return ta }
func (ta *TextAreaElement) GridSpan(cols, rows int) *TextAreaElement {
	ta.size.setSpan(cols, rows)
	return ta
}

// MinRows sets the minimum height in lines (default 3).
func (ta *TextAreaElement) MinRows(n int) *TextAreaElement { ta.minRows = n; return ta }

// Borderless drops the frame and padding, for full-pane editors. The Frame part
// is not drawn, so its styles do not apply.
func (ta *TextAreaElement) Borderless() *TextAreaElement { ta.borderless = true; return ta }

// Styles sets the look of the parts of the editor, for this instance only; see
// TextInputElement.Styles.
func (ta *TextAreaElement) Styles(s FieldStyle) *TextAreaElement {
	ta.styles = ta.styles.merge(s)
	return ta
}

// Leading puts el inside the frame before the text. It is drawn in the Leading
// part; see TextInputElement.Leading.
func (ta *TextAreaElement) Leading(el Element) *TextAreaElement { ta.leadingEl = el; return ta }

// Trailing puts el inside the frame after the text. It is drawn in the Trailing
// part; see TextInputElement.Trailing.
func (ta *TextAreaElement) Trailing(el Element) *TextAreaElement { ta.trailingEl = el; return ta }

// AutoFocus focuses the editor when it first appears.
func (ta *TextAreaElement) AutoFocus() *TextAreaElement { ta.autoFocus = true; return ta }

// FocusHandle lets code focus the editor (see NewFocusHandle).
func (ta *TextAreaElement) FocusHandle(h *FocusHandle) *TextAreaElement { ta.handle = h; return ta }

// Validate sets the validators of the text; see TextInputElement.Validate.
func (ta *TextAreaElement) Validate(rules ...Validator[string]) *TextAreaElement {
	ta.rules = rules
	return ta
}

func (ta *TextAreaElement) expand(ctx *Context) Element {
	lay := fieldLayout{multiline: true, disabled: ta.disabled, width: Px(textFieldWidth * textScale(ctx))}
	st := resolveField(ctx.Theme(), lay, ta.styles)
	return ta.fieldOptions.expand(ctx, st, true, ta.minRows)
}

// expand returns the field's frame with its editor, in the look st. The frame
// is the Frame part, and the content before and after the text is in the
// Leading and Trailing parts. A borderless field draws no Frame part.
func (f *fieldOptions) expand(ctx *Context, st FieldStyle, multiline bool, minRows int) *DivElement {
	t := ctx.Theme()
	// What a Field gives the field replaces or adds to what the field was given. An
	// invalid frame also stays while the field has focus, so the danger color is not
	// hidden by the focus color (as in the grid's editor).
	w := f.wire
	if w.invalid {
		st.Frame = st.Frame.Merge(st.FrameInvalid)
		st.FrameFocusWithin = st.FrameInvalid
	}
	name, labelledBy, description := f.name, f.labelledBy, f.description
	if w.label != "" {
		name, labelledBy = w.label, w.labelledBy
	}
	if w.description != "" {
		description = w.description
	}
	if description == "" {
		// A tooltip says more about the field, so assistive technology reads it as
		// the description, as it does for a Div's tooltip.
		description = f.tooltip
	}
	describedBy := slices.Concat(f.describedBy, w.describedBy)
	onChange, onBlur, onSubmit := f.onChange, f.onBlur, f.onSubmit
	if w.onEdit != nil {
		change := f.onChange
		onChange = func(s string) {
			if change != nil {
				change(s)
			}
			w.onEdit()
		}
	}
	if w.onBlur != nil {
		blur := f.onBlur
		onBlur = func() {
			if blur != nil {
				blur()
			}
			w.onBlur()
		}
	}
	if w.onEnter != nil {
		submit := f.onSubmit
		onSubmit = func(s string) {
			if submit != nil {
				submit(s)
			}
			w.onEnter()
		}
	}
	// Reading the value subscribes this field, so it re-renders (and
	// re-measures) when the text changes.
	f.value.Get()
	ctxState := ctx.state()
	h := f.handle
	if w.handle != nil {
		h = w.handle
	}
	if h == nil {
		if ctxState.focus == nil {
			ctxState.focus = NewFocusHandle()
		}
		h = ctxState.focus
	}
	ed := &editorElement{
		value: f.value, placeholder: f.placeholder, name: name, labelledBy: labelledBy, describedBy: describedBy, multiline: multiline,
		onSubmit: onSubmit, onChange: onChange, onBlur: onBlur, onCancel: f.onCancel, description: description,
		onStep: f.onStep, autoFocus: f.autoFocus, handle: h, invalid: w.invalid, required: w.required, align: f.align,
		style: Style{Grow: 1}.Merge(st.Text), selection: st.Selection.Background, selectionText: st.Selection.TextColor,
		placeholderColor: st.Placeholder.TextColor, part: f.widget + ".Text",
		readOnly: f.readOnly, disabled: f.disabled, onPasteImage: f.onPasteImg,
	}
	if multiline && minRows > 0 {
		size := t.FontSize
		if st.Frame.FontSize > 0 {
			size = st.Frame.FontSize
		}
		if st.Text.FontSize > 0 {
			size = st.Text.FontSize
		}
		ed.style.MinHeight = Px(float32(minRows) * size * float32(ctx.TextScale()) / 100 * 1.4)
	}
	var box *DivElement
	switch {
	case f.borderless && f.disabled:
		box = Div().Row().Align(Start).Gap(8)
	case f.borderless:
		box = Div().Row().Align(Start).Gap(8).Cursor(CursorText).
			OnMouseDown(func(*MouseEvent) { h.Focus() })
	case f.disabled:
		// A disabled field takes no pointer input, so it has no hover state.
		box = Div().asPart(f.widget + ".Frame").Style(st.Frame).Style(st.FrameDisabled).Cursor(CursorDefault)
	case f.readOnly:
		// A read-only field is not edited, so it has no hover state either.
		box = Div().asPart(f.widget + ".Frame").Style(st.Frame).Style(st.FrameReadOnly).
			FocusWithin(st.FrameFocusWithin).
			OnMouseDown(func(*MouseEvent) { h.Focus() })
	default:
		// Clicks on the frame (padding, icon) still focus the text.
		box = Div().asPart(f.widget + ".Frame").Style(st.Frame).
			Hover(st.FrameHover).FocusWithin(st.FrameFocusWithin).
			OnMouseDown(func(*MouseEvent) { h.Focus() })
	}
	box.Tooltip(f.tooltip)
	if f.icon != nil || f.leadingEl != nil {
		lead := Div().asPart(f.widget + ".Leading").Style(st.Leading)
		if f.icon != nil {
			// The icon takes the Leading part's text color, as text does.
			lead.Child(Icon(f.icon).Size(16))
		}
		if f.leadingEl != nil {
			lead.Child(f.leadingEl)
		}
		box.Child(lead)
	}
	box.Child(ed)
	if f.trailingEl != nil {
		box.Child(Div().asPart(f.widget + ".Trailing").Style(st.Trailing).Child(f.trailingEl))
	}
	if f.trailing != nil {
		box.Child(f.trailing)
	}
	for _, o := range f.overlays {
		box.Child(o)
	}
	return f.size.apply(box)
}
