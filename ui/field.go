package ui

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// FormFieldStyle is the look of each part of a Field. A part's style merges over
// the built-in defaults, then over the theme's FormField, then over the Styles of
// the instance; see docs/styling.md. Root is the field: a column of the label, the
// help text, the error and the control, or a row when the field is Inline. Label
// is the label, InlineLabel the label beside the control of an Inline field (merged
// over Label), Required the asterisk after the label, Description the help text,
// and Error the message row with the error. The error's icon and text take the
// Error part's TextColor, as a number input's message does.
type FormFieldStyle struct {
	Root        Style // the field: a column, or a row when Inline
	Label       Style // the label above the control
	InlineLabel Style // the label beside the control, when the field is Inline
	Required    Style // the asterisk after the label; hidden from assistive technology
	Description Style // the help text under the label
	Error       Style // the row with the alert icon and the error text
}

// merge returns s with the set fields of o applied on top, part by part.
func (s FormFieldStyle) merge(o FormFieldStyle) FormFieldStyle {
	return FormFieldStyle{
		Root:        s.Root.Merge(o.Root),
		Label:       s.Label.Merge(o.Label),
		InlineLabel: s.InlineLabel.Merge(o.InlineLabel),
		Required:    s.Required.Merge(o.Required),
		Description: s.Description.Merge(o.Description),
		Error:       s.Error.Merge(o.Error),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s FormFieldStyle) plain() FormFieldStyle {
	return FormFieldStyle{
		Root:        plainStyle(s.Root),
		Label:       plainStyle(s.Label),
		InlineLabel: plainStyle(s.InlineLabel),
		Required:    plainStyle(s.Required),
		Description: plainStyle(s.Description),
		Error:       plainStyle(s.Error),
	}
}

// messageStyle is the look of a message that says what is wrong with a control: a
// row with an alert icon and the text, in Danger, or in the text color under high
// contrast. A number input's message and a field's error take it, so the two look
// the same. The message is not a color cue alone: the icon marks it too.
func messageStyle(t *Theme) Style {
	c := t.Danger
	if t.HighContrast {
		c = t.Text
	}
	return Style{Direction: Row, Align: Center, Gap: 6, FontSize: 12, TextColor: c}
}

// messageIconColor is the color of a message's icon: the message's TextColor when
// it is set, and Danger otherwise (the text color under high contrast).
func messageIconColor(t *Theme, st Style) Color {
	if !st.TextColor.IsZero() {
		return st.TextColor
	}
	if t.HighContrast {
		return t.Text
	}
	return t.Danger
}

// formFieldDefaults returns the built-in look of a field, from the colors of t.
func formFieldDefaults(t *Theme) FormFieldStyle {
	required := t.Danger
	if t.HighContrast {
		required = t.Text
	}
	return FormFieldStyle{
		Label:       Style{FontSize: 13, FontWeight: FontMedium, TextColor: t.Text},
		InlineLabel: Style{Width: Px(180), Padding: Edges{Top: 9}},
		Required:    Style{TextColor: required, FontWeight: FontSemibold},
		Description: Style{FontSize: 12, TextColor: t.TextMuted},
		Error:       messageStyle(t),
	}
}

// resolveFormField returns the look of a field: the defaults, merged with the
// theme's FormField and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveFormField(ctx *Context, inst FormFieldStyle) FormFieldStyle {
	t := ctx.Theme()
	return layered(ctx, formFieldDefaults(t), FormFieldStyle.merge, FormFieldStyle.plain,
		themeLayer(t.FormField), instanceLayer(inst))
}

// fieldWire is what a Field gives the control it wraps. The control applies it in
// its own expand: Label is its accessible name, LabelledBy and DescribedBy relate
// it to the label, the help text and the error, Description is its help text, and
// required and invalid are its access states and its invalid frame. Handle is the
// element that takes focus for the field. onEdit runs when the user changes the
// value, onBlur when the control loses focus, and onEnter replaces the control's own
// Enter in a single-line field, so that Enter submits the form.
type fieldWire struct {
	label       string
	labelledBy  []string
	describedBy []string
	description string
	required    bool
	invalid     bool
	handle      *FocusHandle
	onEdit      func()
	onBlur      func()
	onEnter     func()
}

// formControl is a control that a Field can check and wire. The controls with a
// Validate method implement it.
type formControl interface {
	// formCheck is the error of the value now, or nil. It reads the value with Get,
	// so a Field re-renders when the value changes.
	formCheck() error
	// formWire returns a copy of the control with w as what the Field gives it. The
	// copy is a new element, so the runtime expands it again on every render of the
	// Field; the same element would be reused with its old wire.
	formWire(w fieldWire) Element
}

// fieldSeq numbers the fields that are not in a form, so that each one has keys of
// its own in the window.
var fieldSeq atomic.Uint64

// FieldElement is a labelled field: a label above its control, with optional help
// text, a required marker and the error the field shows. The control is named by
// the label, described by the help text and the error, and reports whether it is
// required and whether its value is valid (see docs/accessibility.md, Phase K).
// Its validators (see Validate) are checked as the user works with the field, and
// on submit when the field belongs to a Form (see Form and the README, Forms).
type FieldElement struct {
	label    string
	control  Element
	help     string
	required bool
	inline   bool
	err      string
	check    func() error
	focus    *FocusHandle
	form     *Form
	key      string // the key of the field in its form, see Form.Field
	styles   FormFieldStyle
	size     sizing // the field's box sizes, from the sizing methods (see Sizing in docs/styling.md)
	elemKey  string // the element key, see Key
}

// Key identifies the field among its siblings, so it keeps its state when the
// siblings around it change (see DivElement.Key).
func (f *FieldElement) Key(k string) *FieldElement { f.elemKey = k; return f }

func (f *FieldElement) elementKey() string { return f.elemKey }

// Field creates a field with the label and the control. Add help text with
// Description, and mark it required with Required.
func Field(label string, control Element) *FieldElement {
	return &FieldElement{label: label, control: control}
}

func (*FieldElement) isElement() {}

// Description sets the help text under the label. It is read after the name, and
// it is part of the control's description, with the error.
func (f *FieldElement) Description(s string) *FieldElement { f.help = s; return f }

// Required marks the field as required: an asterisk follows the label, and the
// control reports IsRequiredForForm to assistive technology.
func (f *FieldElement) Required() *FieldElement { f.required = true; return f }

// Inline puts the label beside the control, as in a settings row, instead of above
// it. The help text and the error stay under the control.
func (f *FieldElement) Inline() *FieldElement { f.inline = true; return f }

// Error sets an error the app found, such as a name the server rejects. It is shown
// until the app clears it, and it wins over the control's validators. Like a
// validator's error, it keeps the form from being submitted.
func (f *FieldElement) Error(msg string) *FieldElement { f.err = msg; return f }

// Check sets a check of the whole field, for a group of controls that have no value
// of their own, such as "at least one channel". It returns the error, or nil. It runs
// after the control's validators. It reads its signals with Get, so the field
// re-renders when they change.
func (f *FieldElement) Check(fn func() error) *FieldElement { f.check = fn; return f }

// Focus sets the element that takes focus when a form with an error is submitted,
// for a field whose control is a group that the field does not wire (see Check). For
// a control, the control takes focus, and Focus is not needed.
func (f *FieldElement) Focus(h *FocusHandle) *FieldElement { f.focus = h; return f }

// The sizing methods give the field's outer box a size, so a form can size a labeled
// field as a unit. A field fills its container by default, as a column does. Its
// control keeps its own width: a control whose width is Auto or Full fills the field,
// and a fixed one keeps its size. See Sizing in docs/styling.md.
func (f *FieldElement) Width(l Length) *FieldElement    { f.size.setWidth(l); return f }
func (f *FieldElement) MinWidth(l Length) *FieldElement { f.size.setMinWidth(l); return f }
func (f *FieldElement) MaxWidth(l Length) *FieldElement { f.size.setMaxWidth(l); return f }
func (f *FieldElement) Grow(g float32) *FieldElement    { f.size.setGrow(g); return f }

// Styles sets the look of the parts of the field, for this instance only. Set fields
// win over the theme's FormField and the defaults; see FormFieldStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (f *FieldElement) Styles(st FormFieldStyle) *FieldElement {
	f.styles = f.styles.merge(st)
	return f
}

func (f *FieldElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		s := ctx.Strings()
		st := resolveFormField(ctx, f.styles)
		fc, _ := f.control.(formControl)
		// base prefixes the keys of the label, the help text and the error. A field in
		// a form is keyed by its form key; a field on its own gets a number of its own.
		base := fmt.Sprintf("field-%d", Local(ctx, "id", fieldSeq.Add(1)).Peek())
		if f.form != nil {
			base = fmt.Sprintf("form-%d/%s", f.form.id, f.key)
		}
		// edited is set when the user changes the value, and touched when the user then
		// leaves the control: the error is shown from then on.
		edited := Local(ctx, "edited", false)
		touched := Local(ctx, "touched", false)
		// focus is the element that takes focus for the field: the one the field was
		// given, or the control's own.
		focus := f.focus
		if focus == nil && fc != nil {
			focus = Local(ctx, "focus", NewFocusHandle()).Peek()
		}
		// check is the error of the control, and then of the whole field. Reading it
		// subscribes the field to the value.
		check := func() error {
			if fc != nil {
				if err := fc.formCheck(); err != nil {
					return err
				}
			}
			if f.check != nil {
				return f.check()
			}
			return nil
		}
		// shown is the message the field shows: the app's error, else the check's
		// error once the user has changed the field and left it, or once a form has
		// been submitted.
		shown := f.err
		if shown == "" {
			if err := check(); err != nil && (touched.Get() || (f.form != nil && f.form.submitted.Get())) {
				shown = errorText(err, s)
			}
		}
		if f.form != nil {
			f.form.add(&formEntry{key: f.key, label: f.label, focus: focus, text: func() string {
				if f.err != "" {
					return f.err
				}
				if err := check(); err != nil {
					return errorText(err, s)
				}
				return ""
			}})
			f.form.post = ctx.Post
			f.form.strings = s
		}

		labelKey, helpKey, errKey := base+"/label", base+"/help", base+"/error"
		control := f.control
		if fc != nil {
			w := fieldWire{handle: focus, required: f.required, invalid: shown != "",
				description: describe(f.help, shown)}
			if f.label != "" {
				w.label, w.labelledBy = f.label, []string{labelKey}
			}
			if f.help != "" {
				w.describedBy = append(w.describedBy, helpKey)
			}
			if shown != "" {
				w.describedBy = append(w.describedBy, errKey)
			}
			w.onEdit = func() { edited.Set(true) }
			w.onBlur = func() {
				if edited.Peek() {
					touched.Set(true)
				}
			}
			if f.form != nil {
				w.onEnter = f.form.Submit
			}
			control = fc.formWire(w)
		}

		// The label, with the asterisk of a required field. The asterisk is drawn but
		// hidden from assistive technology, which reports the state instead.
		labelPart := "FormField.Label"
		if f.inline {
			labelPart = "FormField.InlineLabel"
		}
		label := Div().asPart(labelPart).Style(st.Label)
		if f.inline {
			label.Style(st.InlineLabel)
		}
		label.Row().Align(Center).Gap(4).Child(Text(f.label).Key(labelKey))
		if f.required {
			label.Child(Div().asPart("FormField.Required").AccessHidden().Style(st.Required).Child(Text("*")))
		}
		// The help text and the error are always in the tree, in the same places, and are
		// not displayed when they have nothing to say. A child's identity is its position
		// among its siblings, so a message that came and went would move the control and
		// rebuild it, and the control would lose its focus.
		help := Div().asPart("FormField.Description").Style(st.Description).Display(f.help != "").
			Child(Text(f.help).Key(helpKey))
		errRow := Div().asPart("FormField.Error").Style(st.Error).Display(shown != "").
			Child(Icon(icons.Alert).Size(14).Color(messageIconColor(t, st.Error))).
			Child(Text(shown).Key(errKey))
		// The error is drawn above the control, under the help text, so the user reads
		// what is wrong before the field they must correct.
		var root *DivElement
		if f.inline {
			// The label takes the left column, and the help, the error and the control
			// the right one.
			body := Div().Column().Gap(6).Grow(1).Child(help).Child(errRow).Child(control)
			root = Div().asPart("FormField.Root").Row().Align(Start).Gap(16).Style(st.Root).
				Child(label).Child(body)
		} else {
			root = Div().asPart("FormField.Root").Column().Gap(6).Style(st.Root).
				Child(label).Child(help).Child(errRow).Child(control)
		}
		if fc == nil {
			// A group of controls, such as check boxes, has no control of its own to wire.
			// The field is the group: it is named by its label, and its help text and error
			// describe it, as they describe a control.
			root.Role(access.RoleGroup).Label(f.label).LabelledBy(labelKey)
			a := root.access()
			a.required, a.invalid = f.required, shown != ""
			var described []string
			if f.help != "" {
				described = append(described, helpKey)
			}
			if shown != "" {
				described = append(described, errKey)
			}
			root.DescribedBy(described...)
			if desc := describe(f.help, shown); desc != "" {
				root.Description(desc)
			}
		}
		return f.size.apply(root)
	})
}

// describe joins the non-empty texts with a space, as the description of a control.
func describe(texts ...string) string {
	var keep []string
	for _, s := range texts {
		if s != "" {
			keep = append(keep, s)
		}
	}
	return strings.Join(keep, " ")
}
