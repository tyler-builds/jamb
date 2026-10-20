package ui

import (
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// formSeq numbers the forms, so that each form's summary has keys of its own.
var formSeq atomic.Uint64

// Form groups the fields of a form and submits them. Build the form once, with
// NewForm, and keep it across renders (in a Local, or in the app's state); its
// fields are made with Field, and its error summary is placed with Summary:
//
//	f := ui.NewForm().OnSubmit(func() error { return save() })
//	ui.Column(
//		f.Summary(),
//		f.Field("name", "Name", ui.TextInput(name).Validate(ui.Required[string]())).Required(),
//		ui.Button("Save").Primary().OnClick(f.Submit),
//	)
//
// On submit, the form checks every field. If one has an error, focus moves to the
// first such field, the number of fields that need attention is announced
// assertively, and the summary lists each error as a link that moves focus to its
// field. If none has, the submit function runs. It returns an error, which the form
// shows as its message, or it runs its work in the background (OnSubmitAsync), and
// Submitting reports while it does.
//
// Enter in a single-line field of the form submits it. A check box, switch or radio
// button keeps Enter for the modal's default button, as it does outside a form.
type Form struct {
	id        uint64
	fields    []*formEntry
	submitted *Signal[bool]
	busy      *Signal[bool]
	formErr   *Signal[string]
	onSubmit  func() error
	onAsync   func() <-chan error
	post      func(func()) // runs a function on the UI thread, set by the form's fields
	strings   *Strings     // the window's text, set by the form's fields (see messages)
}

// formEntry is a field of a form, as it was last built.
type formEntry struct {
	key   string
	label string
	focus *FocusHandle  // the element that takes focus for the field, or nil
	text  func() string // the field's message now: the app's error, else the check's; "" is valid
}

// NewForm creates a form with no fields and no submit function. Set one with
// OnSubmit or OnSubmitAsync.
func NewForm() *Form {
	return &Form{id: formSeq.Add(1), submitted: NewSignal(false), busy: NewSignal(false), formErr: NewSignal("")}
}

// Field makes a field of the form, with the label and the control. The key names
// the field in the form and in the window's keys, so it must be unique in the form.
// The field is checked when it is built: a field that is no longer built is dropped
// by Reset.
func (f *Form) Field(key, label string, control Element) *FieldElement {
	return &FieldElement{label: label, control: control, form: f, key: key}
}

// OnSubmit sets the function that runs when the form is valid. It returns nil when
// the work succeeded, or an error whose text the form shows as its message.
func (f *Form) OnSubmit(fn func() error) *Form {
	f.onSubmit = fn
	return f
}

// OnSubmitAsync sets a submit function that takes time. It starts the work and
// returns a channel that receives the result: nil on success, or an error whose text
// the form shows. Until the result arrives, Submitting reports true, and Submit does
// nothing. The work runs on its own goroutine; the form receives the result on the
// UI thread.
func (f *Form) OnSubmitAsync(fn func() <-chan error) *Form {
	f.onAsync = fn
	return f
}

// add records a field as it was built. A field built again replaces its entry.
func (f *Form) add(e *formEntry) {
	if i := slices.IndexFunc(f.fields, func(x *formEntry) bool { return x.key == e.key }); i >= 0 {
		f.fields[i] = e
		return
	}
	f.fields = append(f.fields, e)
}

// Submit checks the form and, when it is valid, runs its submit function. It is the
// action of a submit button (OnClick(f.Submit)) and of Enter in a field. Submit
// does nothing while an async submit is running.
func (f *Form) Submit() {
	if f.busy.Peek() {
		return
	}
	f.submitted.Set(true)
	f.formErr.Set("")
	if bad := f.failing(); len(bad) > 0 {
		if bad[0].focus != nil {
			bad[0].focus.Focus()
		}
		Announce(f.messages().FieldsNeedAttention(len(bad)), access.Assertive)
		return
	}
	switch {
	case f.onSubmit != nil:
		if err := f.onSubmit(); err != nil {
			f.fail(err.Error())
		}
	case f.onAsync != nil:
		f.busy.Set(true)
		done := f.onAsync()
		post := f.post
		if post == nil {
			post = func(fn func()) { fn() }
		}
		go func() {
			err := <-done
			post(func() {
				f.busy.Set(false)
				if err != nil {
					f.fail(err.Error())
				}
			})
		}()
	}
}

// Submitting reports whether an async submit is running. Read it while building the
// form, so the submit button can be disabled and show progress.
func (f *Form) Submitting() bool { return f.busy.Get() }

// Reset clears the submitted state and the message of the form, and forgets its
// fields, which are added again when they are built. Call it when the fields leave
// the window, such as when the modal that holds them closes.
func (f *Form) Reset() {
	f.submitted.Set(false)
	f.formErr.Set("")
	f.fields = nil
}

// failing returns the fields that have a message, in the order they were built.
func (f *Form) failing() []*formEntry {
	var out []*formEntry
	for _, e := range f.fields {
		if e.text() != "" {
			out = append(out, e)
		}
	}
	return out
}

// fail shows msg as the form's message and announces it.
func (f *Form) fail(msg string) {
	f.formErr.Set(msg)
	Announce(msg, access.Assertive)
}

// messages returns the window's text that the form's fields and summary gave it, or the
// English text before any of them has been built.
func (f *Form) messages() *Strings {
	if f.strings != nil {
		return f.strings
	}
	return englishStrings()
}

// FormStyle is the look of each part of a form's error summary. A part's style
// merges over the built-in defaults, then over the theme's Form, then over the
// Styles of the instance; see docs/styling.md. Summary is the box, with its title
// (SummaryTitle) and its items. Each item is a link to its field, or the form's own
// message. SummaryItem's hover and focus states are SummaryItemHover and
// SummaryItemFocusVisible.
type FormStyle struct {
	Summary                 Style // the box of the summary, above the fields
	SummaryTitle            Style // the title: the number of fields that need attention
	SummaryItem             Style // each item: a link to its field, or the form's message
	SummaryItemHover        Style // an item the pointer is over, if it is a link
	SummaryItemFocusVisible Style // an item with keyboard focus
}

// merge returns s with the set fields of o applied on top, part by part.
func (s FormStyle) merge(o FormStyle) FormStyle {
	return FormStyle{
		Summary:                 s.Summary.Merge(o.Summary),
		SummaryTitle:            s.SummaryTitle.Merge(o.SummaryTitle),
		SummaryItem:             s.SummaryItem.Merge(o.SummaryItem),
		SummaryItemHover:        s.SummaryItemHover.Merge(o.SummaryItemHover),
		SummaryItemFocusVisible: s.SummaryItemFocusVisible.Merge(o.SummaryItemFocusVisible),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s FormStyle) plain() FormStyle {
	return FormStyle{
		Summary:                 plainStyle(s.Summary),
		SummaryTitle:            plainStyle(s.SummaryTitle),
		SummaryItem:             plainStyle(s.SummaryItem),
		SummaryItemHover:        plainStyle(s.SummaryItemHover),
		SummaryItemFocusVisible: plainStyle(s.SummaryItemFocusVisible),
	}
}

// formDefaults returns the built-in look of a form's summary, from the colors of t.
// The summary's border is two pixels wide and its title has an alert icon, so the
// error is not carried by color alone.
func formDefaults(t *Theme) FormStyle {
	border := t.Danger
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	if t.HighContrast {
		border = t.Text
	}
	return FormStyle{
		Summary: Style{Direction: Column, Gap: 6, Padding: All(12), Radius: t.Radius, Background: t.Surface,
			BorderWidth: 2, BorderColor: border},
		SummaryTitle: Style{Direction: Row, Align: Center, Gap: 6, FontSize: 14, FontWeight: FontSemibold, TextColor: t.Text},
		SummaryItem: Style{FontSize: 13, TextColor: t.Accent, Padding: XY(4, 2), Radius: t.Radius,
			Transition: t.Motion.hover(), Spring: t.Motion.Spring},
		SummaryItemHover:        Style{Background: t.SurfaceHover},
		SummaryItemFocusVisible: Style{Shadows: []Shadow{ring}},
	}
}

// resolveForm returns the look of a form's summary: the defaults, merged with the
// theme's Form and then the instance's Styles. Under high contrast the two overrides
// are plain (see plainStyle).
func resolveForm(ctx *Context, inst FormStyle) FormStyle {
	t := ctx.Theme()
	return layered(ctx, formDefaults(t), FormStyle.merge, FormStyle.plain,
		themeLayer(t.Form), instanceLayer(inst))
}

// FormSummaryElement is the error summary of a form. It is empty until the form is
// submitted, and it lists the fields with a message, or the form's own message, while
// there are any. Place it above the fields.
type FormSummaryElement struct {
	form   *Form
	styles FormStyle
}

// Summary returns the error summary of the form (see FormSummaryElement).
func (f *Form) Summary() *FormSummaryElement { return &FormSummaryElement{form: f} }

func (*FormSummaryElement) isElement() {}

// Styles sets the look of the parts of the summary, for this instance only. Set
// fields win over the theme's Form and the defaults; see FormStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (s *FormSummaryElement) Styles(st FormStyle) *FormSummaryElement {
	s.styles = s.styles.merge(st)
	return s
}

func (s *FormSummaryElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		f := s.form
		f.post = ctx.Post
		strs := ctx.Strings()
		f.strings = strs
		if !f.submitted.Get() {
			return nil
		}
		t := ctx.Theme()
		st := resolveForm(ctx, s.styles)
		key := fmt.Sprintf("form-%d/summary-title", f.id)
		var items []Element
		count := 0
		for _, e := range f.fields {
			msg := e.text()
			if msg == "" {
				continue
			}
			count++
			items = append(items, Div().asPart("Form.SummaryItem").Style(st.SummaryItem).
				Hover(st.SummaryItemHover).FocusVisible(st.SummaryItemFocusVisible).
				Role(access.RoleLink).Focusable().Key(fmt.Sprintf("form-%d/item:%s", f.id, e.key)).
				OnClick(func() {
					if e.focus != nil {
						e.focus.Focus()
					}
				}).
				Child(Text(summaryText(e.label, msg))))
		}
		if msg := f.formErr.Get(); msg != "" {
			items = append(items, Div().asPart("Form.SummaryItem").Style(st.SummaryItem).Child(Text(msg)))
		}
		if len(items) == 0 {
			return nil
		}
		title := strs.FormSubmitFailed
		if count > 0 {
			title = strs.FieldsNeedAttention(count)
		}
		head := Div().asPart("Form.SummaryTitle").Style(st.SummaryTitle).
			Child(Icon(icons.Alert).Size(16).Color(messageIconColor(t, st.Summary))).
			Child(Text(title).Key(key))
		return Div().asPart("Form.Summary").Style(st.Summary).Column().Gap(6).
			Role(access.RoleGroup).Label(title).LabelledBy(key).
			Child(head).Children(items...)
	})
}

// summaryText is the text of a summary item: the field's label and its message.
func summaryText(label, msg string) string {
	if label == "" {
		return msg
	}
	return label + ": " + msg
}
