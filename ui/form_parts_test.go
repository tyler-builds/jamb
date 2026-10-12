package ui

import "testing"

// testFormParts checks the parts of a Field, the error summary of a form, and the
// invalid states that a Field gives the controls. Each case sets its part to a marker
// style, makes the state in effect, and checks that the node that draws the part has
// the style.
func testFormParts(t *testing.T, applied map[string]bool) {
	mark := Style{Background: markBG, TextColor: markFG}
	type formCase struct {
		part   string
		text   bool // the part is colored by its text color
		submit bool // the form is submitted, so its summary shows
		build  func(f *Form, m Style, name *Signal[string]) Element
		act    func(*testing.T, *accessRig)
	}
	cases := []formCase{
		{part: "FormField.Root", build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Name", TextInput(name).Key("name")).Styles(FormFieldStyle{Root: m})
		}},
		{part: "FormField.Label", text: true, build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Name", TextInput(name).Key("name")).Styles(FormFieldStyle{Label: m})
		}},
		{part: "FormField.InlineLabel", text: true, build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Name", TextInput(name).Key("name")).Inline().Styles(FormFieldStyle{InlineLabel: m})
		}},
		{part: "FormField.Required", text: true, build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Name", TextInput(name).Key("name")).Required().Styles(FormFieldStyle{Required: m})
		}},
		{part: "FormField.Description", text: true, build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Name", TextInput(name).Key("name")).Description("Help text.").Styles(FormFieldStyle{Description: m})
		}},
		{part: "FormField.Error", text: true, build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Name", TextInput(name).Key("name")).Error("Taken.").Styles(FormFieldStyle{Error: m})
		}},
		{part: "TextInput.FrameInvalid", build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Name", TextInput(name).Key("name").Styles(FieldStyle{FrameInvalid: m})).Error("Taken.")
		}},
		{part: "TextArea.FrameInvalid", build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Notes", TextArea(name).Key("notes").Styles(FieldStyle{FrameInvalid: m})).Error("Too short.")
		}},
		{part: "Select.TriggerInvalid", build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Channel", Select(NewSignal(""), Option[string]{Value: "a", Label: "A"}).Key("channel").
				Styles(SelectStyle{TriggerInvalid: m})).Error("Choose a channel.")
		}},
		{part: "Checkbox.BoxInvalid", build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Terms", Checkbox("I accept", NewSignal(false)).Key("accept").
				Styles(CheckboxStyle{BoxInvalid: m})).Error("Accept the terms.")
		}},
		{part: "CheckboxTri.BoxInvalid", build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Some", CheckboxTri("Some channels", NewSignal(CheckMixed)).Key("some").
				Styles(CheckboxStyle{BoxInvalid: m})).Error("Choose one.")
		}},
		{part: "RadioGroup.RingInvalid", build: func(f *Form, m Style, name *Signal[string]) Element {
			return Field("Frequency", RadioGroup(NewSignal(""), Option[string]{Value: "a", Label: "A"}).Key("freq").
				Styles(RadioGroupStyle{RingInvalid: m})).Error("Choose one.")
		}},
		{part: "Form.Summary", submit: true, build: func(f *Form, m Style, name *Signal[string]) Element {
			return Div().Column().Child(f.Field("name", "Name", TextInput(name).Key("name")).Error("Taken.")).
				Child(f.Summary().Styles(FormStyle{Summary: m}))
		}},
		{part: "Form.SummaryTitle", submit: true, build: func(f *Form, m Style, name *Signal[string]) Element {
			return Div().Column().Child(f.Field("name", "Name", TextInput(name).Key("name")).Error("Taken.")).
				Child(f.Summary().Styles(FormStyle{SummaryTitle: m}))
		}},
		{part: "Form.SummaryItem", submit: true, build: func(f *Form, m Style, name *Signal[string]) Element {
			return Div().Column().Child(f.Field("name", "Name", TextInput(name).Key("name")).Error("Taken.")).
				Child(f.Summary().Styles(FormStyle{SummaryItem: m}))
		}},
		{part: "Form.SummaryItemHover", submit: true,
			build: func(f *Form, m Style, name *Signal[string]) Element {
				return Div().Column().Child(f.Field("name", "Name", TextInput(name).Key("name")).Error("Taken.")).
					Child(f.Summary().Styles(FormStyle{SummaryItemHover: m}))
			},
			act: func(t *testing.T, r *accessRig) { hoverNode(r, partNode(t, r.rt, "Form.SummaryItem", "")) }},
		{part: "Form.SummaryItemFocusVisible", submit: true,
			build: func(f *Form, m Style, name *Signal[string]) Element {
				return Div().Column().Child(f.Field("name", "Name", TextInput(name).Key("name")).Error("Taken.")).
					Child(f.Summary().Styles(FormStyle{SummaryItemFocusVisible: m}))
			},
			act: func(t *testing.T, r *accessRig) { focusNode(r, partNode(t, r.rt, "Form.SummaryItem", "")) }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			name := NewSignal("")
			f := NewForm()
			if c.submit {
				f.submitted.Set(true)
			}
			r := newAccessRig(func(ctx *Context) Element { return c.build(f, mark, name) })
			r.frame()
			if c.act != nil {
				c.act(t, r)
			}
			n := partNode(t, r.rt, drawnBy(c.part), "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
			} else {
				wantBackground(t, n, c.part, markBG)
			}
		})
	}
}
