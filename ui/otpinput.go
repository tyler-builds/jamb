package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
)

// OTPInputStyle is the look of each part of a one-time code input. A part's style
// merges over the built-in defaults, then over the theme's OTPInput, then over the
// Styles of the instance; see docs/styling.md. Root is the row that holds the
// boxes, and Group each group of boxes (see Groups). Box is each box, the look of
// an empty one; BoxFilled is drawn over it for a box that holds a character, and
// BoxFocused for the box at the caret while the input has focus. BoxDisabled is a
// box of a disabled input, and BoxInvalid a box of an input whose value a Field
// shows an error for. Text is the character in a box, and Dot the dot that a
// Masked input draws in its place.
type OTPInputStyle struct {
	Root        Style // the row that holds the groups of boxes
	Group       Style // each group of boxes
	Box         Style // each box, as it is drawn empty
	BoxFilled   Style // a box that holds a character, over Box
	BoxFocused  Style // the box at the caret while the input has focus
	BoxDisabled Style // a box of a disabled input
	BoxInvalid  Style // a box of an input whose value is not valid
	Text        Style // the character in a box
	Dot         Style // the dot of a masked box that holds a character
}

// merge returns s with the set fields of o applied on top, part by part.
func (s OTPInputStyle) merge(o OTPInputStyle) OTPInputStyle {
	return OTPInputStyle{
		Root:        s.Root.Merge(o.Root),
		Group:       s.Group.Merge(o.Group),
		Box:         s.Box.Merge(o.Box),
		BoxFilled:   s.BoxFilled.Merge(o.BoxFilled),
		BoxFocused:  s.BoxFocused.Merge(o.BoxFocused),
		BoxDisabled: s.BoxDisabled.Merge(o.BoxDisabled),
		BoxInvalid:  s.BoxInvalid.Merge(o.BoxInvalid),
		Text:        s.Text.Merge(o.Text),
		Dot:         s.Dot.Merge(o.Dot),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s OTPInputStyle) plain() OTPInputStyle {
	return OTPInputStyle{
		Root:        plainStyle(s.Root),
		Group:       plainStyle(s.Group),
		Box:         plainStyle(s.Box),
		BoxFilled:   plainStyle(s.BoxFilled),
		BoxFocused:  plainStyle(s.BoxFocused),
		BoxDisabled: plainStyle(s.BoxDisabled),
		BoxInvalid:  plainStyle(s.BoxInvalid),
		Text:        plainStyle(s.Text),
		Dot:         plainStyle(s.Dot),
	}
}

// otpBoxWidth and otpBoxHeight are the size of a box at the Text size of 100
// percent, in logical pixels. They scale with the Text size.
const (
	otpBoxWidth  = 40
	otpBoxHeight = 44
)

// otpInputDefaults returns the built-in look of a one-time code input, from the
// colors of t, with its boxes scaled by scale (the Text size). The boxes are framed
// as a text field is; the box at the caret has the Accent border and the focus
// ring, and an invalid box the thicker Danger border. Under high contrast a filled
// box keeps the frame of an empty one, the invalid border is the text color, and a
// disabled box is GrayText, not dimmed.
func otpInputDefaults(t *Theme, scale float32, disabled bool) OTPInputStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := OTPInputStyle{
		Root:  Style{Direction: Row, Align: Center, Gap: 12, fitWidth: true},
		Group: Style{Direction: Row, Align: Center, Gap: 8},
		Box: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(otpBoxWidth * scale),
			Height: Px(otpBoxHeight * scale), NoShrink: true, Radius: t.Radius, BorderWidth: 1,
			BorderColor: t.BorderStrong, Background: t.Surface, FontSize: 18, TextColor: t.Text,
			Transition: 110 * time.Millisecond},
		BoxFilled:  Style{BorderColor: t.TextMuted},
		BoxFocused: Style{BorderColor: t.Accent, Shadows: []Shadow{ring}},
		BoxInvalid: Style{BorderWidth: 2, BorderColor: t.Danger},
		Dot:        Style{Width: Px(10 * scale), Height: Px(10 * scale), Radius: 5 * scale, Background: t.Text},
	}
	if !disabled {
		d.Box.Cursor = CursorText
	}
	if t.HighContrast {
		d.BoxFilled = Style{}
		d.BoxInvalid.BorderColor = t.Text
		d.BoxDisabled = Style{BorderColor: t.Disabled, TextColor: t.Disabled}
		if disabled {
			d.Dot.Background = t.Disabled
		}
	} else {
		d.BoxDisabled = Style{Opacity: 0.5, BorderColor: t.Border}
	}
	return d
}

// resolveOTPInput returns the look of a one-time code input: the defaults, merged
// with the theme's OTPInput and then the instance's Styles. Under high contrast the
// two overrides are plain (see plainStyle).
func resolveOTPInput(ctx *Context, scale float32, disabled bool, inst OTPInputStyle) OTPInputStyle {
	t := ctx.Theme()
	return layered(ctx, otpInputDefaults(t, scale, disabled), OTPInputStyle.merge, OTPInputStyle.plain,
		themeLayer(t.OTPInput), instanceLayer(inst))
}

// OTPInputElement is an input for a one-time code, such as a verification code
// sent by text message: a row of boxes that each take one character, bound to a
// string signal. It is one control with a caret, not a field per box. Typing fills
// the box at the caret and moves to the next one, Backspace clears the box before
// the caret and moves back, and Delete clears the box at the caret. The code has no
// gaps: the characters after a cleared box move up. Left, Right, Home and End move
// the caret, and a click on a box moves it there (no further than the first empty
// box). Ctrl+V, or the Paste action, pastes the clipboard's text from the caret,
// keeping only the characters the input takes. Its keys are the actions of a text
// field (see DefaultBindings), so a binding that changes them for fields changes
// them here too.
//
// It takes digits only, unless it is Alphanumeric. An input method's composition is
// not shown in the boxes: the text it confirms is typed as typed text is.
//
// Assistive technology sees one text field named by its label, whose value is the
// code, or a dot for each character when it is Masked, with the password state.
type OTPInputElement struct {
	label        string
	value        *Signal[string]
	length       int
	groups       []int // see Groups
	masked       bool
	alphanumeric bool
	labelledBy   []string
	key          string
	disabled     bool
	onChange     func(string)
	onComplete   func(string)
	rules        []Validator[string] // the validators of the value (see Validate)
	wire         fieldWire           // what a Field gives the input (see Field)
	styles       OTPInputStyle
	size         sizing       // the row's sizes, from the sizing methods (see Sizing in docs/styling.md)
	focusHandle  *FocusHandle // see FocusHandle
	tooltip      string       // see Tooltip
}

// OTPInput creates an input of length boxes that reads and writes value. label
// names it for assistive technology; a Field that wraps it names it by the
// field's label instead. A value longer than length shows its first length
// characters. OTPInput panics when length is less than 1.
func OTPInput(label string, value *Signal[string], length int) *OTPInputElement {
	if length < 1 {
		panic(fmt.Sprintf("ui.OTPInput(%q, %d): the length must be 1 or more", label, length))
	}
	return &OTPInputElement{label: label, value: value, length: length}
}

func (*OTPInputElement) isElement()           {}
func (o *OTPInputElement) elementKey() string { return o.key }

// Key identifies the input among its siblings (see DivElement.Key).
func (o *OTPInputElement) Key(k string) *OTPInputElement { o.key = k; return o }

// LabelledBy names the input by the text of the elements with the given keys;
// see DivElement.LabelledBy.
func (o *OTPInputElement) LabelledBy(keys ...string) *OTPInputElement {
	o.labelledBy = keys
	return o
}

// Masked draws a dot in place of each character, as a password field does, and
// reports the input to assistive technology as a password field whose value is a
// dot for each character.
func (o *OTPInputElement) Masked() *OTPInputElement { o.masked = true; return o }

// Alphanumeric lets the input take the letters A to Z and a to z as well as the
// digits 0 to 9. A letter is kept as it is typed.
func (o *OTPInputElement) Alphanumeric() *OTPInputElement { o.alphanumeric = true; return o }

// Groups splits the boxes into groups of the given sizes, with a wider gap between
// two groups, such as Groups(3, 3) for a six-character code shown as 123 456. The
// sizes must add up to the length; Groups panics otherwise, or when a size is less
// than 1. Without Groups the boxes are one group.
func (o *OTPInputElement) Groups(sizes ...int) *OTPInputElement {
	sum := 0
	for _, n := range sizes {
		if n < 1 {
			panic(fmt.Sprintf("ui.OTPInput.Groups(%v): each size must be 1 or more", sizes))
		}
		sum += n
	}
	if sum != o.length {
		panic(fmt.Sprintf("ui.OTPInput.Groups(%v): the sizes add up to %d, not the length %d", sizes, sum, o.length))
	}
	o.groups = sizes
	return o
}

// Disabled shows the code but takes the input out of use: it can't be focused or
// changed. Its boxes are the BoxDisabled part.
func (o *OTPInputElement) Disabled(d bool) *OTPInputElement { o.disabled = d; return o }

// OnChange runs fn with the new code after the user changes it, by typing, by
// deleting, by pasting or by assistive technology. The signal is set first.
func (o *OTPInputElement) OnChange(fn func(string)) *OTPInputElement { o.onChange = fn; return o }

// OnComplete runs fn with the code when a change by the user fills every box, after
// OnChange.
func (o *OTPInputElement) OnComplete(fn func(code string)) *OTPInputElement {
	o.onComplete = fn
	return o
}

// Validate sets the validators of the code, which a Field checks as it checks a
// TextInput's text (see Field and Validator). MinLength with the input's length
// asks for a complete code, and Required for a code that is not empty.
func (o *OTPInputElement) Validate(rules ...Validator[string]) *OTPInputElement {
	o.rules = rules
	return o
}

// formCheck and formWire make a one-time code input a formControl (see Field).
func (o *OTPInputElement) formCheck() error { return firstError(o.value.Get(), o.rules) }

func (o *OTPInputElement) formWire(w fieldWire) Element {
	c := *o
	c.wire = w
	return &c
}

// FocusHandle lets code focus the input (see NewFocusHandle). A Field that wraps
// the input gives it its own handle, which takes precedence.
func (o *OTPInputElement) FocusHandle(h *FocusHandle) *OTPInputElement {
	o.focusHandle = h
	return o
}

// Tooltip shows s below the input after the pointer rests on it. Assistive
// technology reads it as the input's description, unless it has another.
func (o *OTPInputElement) Tooltip(s string) *OTPInputElement { o.tooltip = s; return o }

// Styles sets the look of the parts of the input, for this instance only. Set
// fields win over the theme's OTPInput and the defaults; see OTPInputStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (o *OTPInputElement) Styles(st OTPInputStyle) *OTPInputElement {
	o.styles = o.styles.merge(st)
	return o
}

// The sizing methods give the input's row a size. The row is as wide as its boxes
// by default, and a box is 40 px wide at the Text size of 100 percent. See Sizing
// in docs/styling.md.
func (o *OTPInputElement) Width(l Length) *OTPInputElement { o.size.setWidth(l); return o }

// MinWidth sets the smallest width of the input.
func (o *OTPInputElement) MinWidth(l Length) *OTPInputElement { o.size.setMinWidth(l); return o }

// MaxWidth sets the largest width of the input.
func (o *OTPInputElement) MaxWidth(l Length) *OTPInputElement { o.size.setMaxWidth(l); return o }

// Grow makes the input take a share of the free space on its parent's main axis.
func (o *OTPInputElement) Grow(f float32) *OTPInputElement { o.size.setGrow(f); return o }

// GridSpan sets how many columns and rows the input covers in its parent's grid.
func (o *OTPInputElement) GridSpan(cols, rows int) *OTPInputElement {
	o.size.setSpan(cols, rows)
	return o
}

// allowed reports whether the input takes the character r.
func (o *OTPInputElement) allowed(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case o.alphanumeric:
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
	}
	return false
}

// code returns the characters of s that the input shows: its first length runes.
func (o *OTPInputElement) code(s string) []rune {
	rs := []rune(s)
	return rs[:min(len(rs), o.length)]
}

// fill types the characters of text that the input takes into the code rs, from
// the caret c: over the boxes that are filled, then into the empty ones, and no
// further than the last box. It returns the code, the caret after the last
// character typed, and whether any character was typed.
func (o *OTPInputElement) fill(rs []rune, c int, text string) ([]rune, int, bool) {
	typed := false
	for _, r := range text {
		if c >= o.length {
			break
		}
		if !o.allowed(r) {
			continue
		}
		if c < len(rs) {
			rs[c] = r
		} else {
			rs = append(rs, r)
		}
		c++
		typed = true
	}
	return rs, c, typed
}

func (o *OTPInputElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		scale := textScale(ctx)
		st := resolveOTPInput(ctx, scale, o.disabled, o.styles)
		w := o.wire
		v := o.code(o.value.Get())
		caret := Local(ctx, "caret", 0)
		at := min(max(caret.Get(), 0), len(v)) // the caret is never past the first empty box
		focused := !o.disabled && ctx.isFocused()

		// apply makes rs the code, with the caret at c, and runs the callbacks when the
		// code changed.
		apply := func(rs []rune, c int) {
			caret.Set(c)
			s := string(rs)
			if s == o.value.Peek() {
				return
			}
			o.value.Set(s)
			if o.onChange != nil {
				o.onChange(s)
			}
			if w.onEdit != nil {
				w.onEdit()
			}
			if len(rs) == o.length && o.onComplete != nil {
				o.onComplete(s)
			}
		}
		// current returns the code and the caret as they are now, in a handler.
		current := func() ([]rune, int) {
			rs := o.code(o.value.Peek())
			return rs, min(max(caret.Peek(), 0), len(rs))
		}
		// insert types the characters of text that the input takes, from the caret.
		insert := func(text string) {
			rs, c := current()
			if rs, c, ok := o.fill(rs, c, text); ok {
				apply(rs, c)
			}
		}
		move := func(to func(rs []rune, c int) int) func() {
			return func() {
				rs, c := current()
				caret.Set(min(max(to(rs, c), 0), len(rs)))
			}
		}

		name, labelledBy := o.label, o.labelledBy
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		// The row keeps the input's key when the input renders again on its own, as
		// when the caret moves.
		root := o.size.apply(Div().asPart("OTPInput.Root").Key(o.key).Style(st.Root).Role(access.RoleTextField).
			Tooltip(o.tooltip))
		if name != "" {
			root.Label(name)
		}
		if len(labelledBy) > 0 {
			root.LabelledBy(labelledBy...)
		}
		if len(w.describedBy) > 0 {
			root.DescribedBy(w.describedBy...)
		}
		if w.description != "" {
			root.Description(w.description)
		}
		a := root.access()
		a.value = string(v)
		if o.masked {
			a.value, a.password = strings.Repeat("•", len(v)), true
		}
		a.invalid, a.required = w.invalid, w.required

		// The boxes, in their groups. The groups are hidden from assistive technology,
		// which reads the code as the field's value.
		sizes := o.groups
		if len(sizes) == 0 {
			sizes = []int{o.length}
		}
		i := 0
		for _, size := range sizes {
			g := Div().asPart("OTPInput.Group").Style(st.Group).AccessHidden()
			for range size {
				g.Child(o.box(ctx, st, v, i, focused && i == min(at, o.length-1), caret))
				i++
			}
			root.Child(g)
		}
		if o.disabled {
			a.disabled = true
			return root
		}

		h := o.focusHandle
		if w.handle != nil {
			h = w.handle
		}
		if h != nil {
			root.FocusHandle(h)
		}
		if w.onBlur != nil {
			root.OnBlur(w.onBlur)
		}
		// Focus puts the caret at the first empty box. A press on a box then moves it to
		// that box, since the press focuses the input before the box takes it.
		root.OnFocus(func() {
			rs, _ := current()
			caret.Set(len(rs))
		})
		// The keys are a single-line field's actions, so the bindings of fields apply.
		root.KeyContext("Editor mode=single_line").Focusable().OnTextInput(insert).
			OnAction(moveLeft, move(func(_ []rune, c int) int { return c - 1 })).
			OnAction(moveRight, move(func(_ []rune, c int) int { return c + 1 })).
			OnAction(moveLineStart, move(func([]rune, int) int { return 0 })).
			OnAction(moveToStart, move(func([]rune, int) int { return 0 })).
			OnAction(moveLineEnd, move(func(rs []rune, _ int) int { return len(rs) })).
			OnAction(moveToEnd, move(func(rs []rune, _ int) int { return len(rs) })).
			OnAction(deleteBack, func() {
				if rs, c := current(); c > 0 {
					apply(slices.Delete(rs, c-1, c), c-1)
				}
			}).
			OnAction(deleteForward, func() {
				if rs, c := current(); c < len(rs) {
					apply(slices.Delete(rs, c, c+1), c)
				}
			}).
			OnAction(paste, func() {
				if s, ok := ctx.Clipboard(); ok {
					insert(s)
				}
			})
		if w.onEnter != nil {
			// In a form, Enter submits it, as in a text field.
			root.OnAction(confirm, w.onEnter)
		}
		// Assistive technology sets the code from the first box.
		a.setValue = func(s string) {
			rs, c, _ := o.fill(nil, 0, s)
			apply(rs, c)
		}
		return root
	})
}

// box returns the box at index i of the code v. The box at the caret is drawn
// focused while the input has focus, and a press on a box moves the caret to it, or
// to the first empty box when it is past that.
func (o *OTPInputElement) box(ctx *Context, st OTPInputStyle, v []rune, i int, atCaret bool, caret *Signal[int]) *DivElement {
	b := Div().asPart("OTPInput.Box").Style(st.Box)
	filled := i < len(v)
	if filled {
		b.Style(st.BoxFilled)
		if o.masked {
			b.Child(Div().asPart("OTPInput.Dot").Style(st.Dot))
		} else {
			b.Child(Div().asPart("OTPInput.Text").Style(st.Text).Child(Text(string(v[i])).NoWrap()))
		}
	}
	if o.wire.invalid {
		b.Style(st.BoxInvalid)
	}
	if o.disabled {
		return b.Style(st.BoxDisabled)
	}
	if atCaret {
		// The caret's box is the focus state of the input, which is the element that
		// has focus, so the inspector names it as a focus state.
		b.Style(ctx.rt.inState(st.BoxFocused, "focus"))
	}
	return b.OnMouseDown(func(*MouseEvent) { caret.Set(min(i, len(o.code(o.value.Peek())))) })
}
