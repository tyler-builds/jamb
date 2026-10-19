# Form widgets

Form controls, the color picker, and forms with validation and submission.

## Form controls

Checkboxes, switches, toggles, toggle groups, radio groups, selects, sliders and
number inputs are bound to signals, as `TextInput` is. Each reads its signal when
it renders and writes it when the user changes it. They use the theme's colors,
follow high contrast and the Text size, and show their state by a shape as well as
a color: a check, a dash, a dot, the place of the knob, or a fill with a border.
Their looks are `ui.CheckboxStyle`, `ui.RadioGroupStyle`, `ui.SwitchStyle`,
`ui.ToggleStyle` and `ui.ToggleGroupStyle`, set with `Theme.Checkbox`,
`Theme.RadioGroup`, `Theme.Switch`, `Theme.Toggle`, `Theme.ToggleGroup` or
`.Styles(...)`, and a button's look is `ui.ButtonStyle` (see Styling).

```go
ui.Checkbox("Email alerts", email)                // *ui.Signal[bool]
ui.CheckboxTri("All alerts", all)                 // *ui.Signal[ui.CheckState]: CheckOff, CheckOn or CheckMixed
ui.Switch("Dark mode", dark).OnChange(func(on bool) { ... })
ui.Toggle("Pin", pinned).Icon(icons.Pin).IconOnly().Small() // *ui.Signal[bool]; Ghost (default) or Secondary
ui.ToggleGroup(view,                              // *ui.Signal[T]; the options of a RadioGroup, with an Icon
	ui.Option[string]{Value: "list", Label: "List", Icon: icons.Menu},
	ui.Option[string]{Value: "grid", Label: "Grid", Icon: icons.Dashboard},
).Label("View").IconOnly().OnChange(func(v string) { ... })
ui.RadioGroup(freq,                               // *ui.Signal[T], for any comparable T
	ui.Option[string]{Value: "daily", Label: "Daily digest"},
	ui.Option[string]{Value: "weekly", Label: "Weekly digest", Disabled: true},
).Label("Frequency").OnChange(func(v string) { ... })
ui.Select(channel, ui.Option[string]{Value: "email", Label: "Email"}).Label("Channel").Placeholder("Choose").Width(ui.Px(240)).OnChange(func(v string) { ... })
ui.Slider(volume, 0, 100, 5).Label("Volume").Format(percent).OnChange(fn).Width(ui.Full)
ui.NumberInput(hour, 0, 23, 1).Label("Quiet hours from").SpinButtons(ui.SpinStacked).Precision(2) // int or float64
// every control also takes .Key("id") and .Disabled(true), and every control with a value takes
// .OnChange(fn), which runs after the user changes it (the signal is already set);
// select, slider, number input, text input and text area
// take .Styles(...); a number input's frame takes a FieldStyle, and its spin buttons and message their own parts.
// Select takes RenderOption and RenderValue. See the Styling section.
// The controls take the sizing methods (Width, MinWidth, MaxWidth and Grow, and Height where it
// applies) with a ui.Length; a progress ring keeps Size, and a form field (`Field`) takes them on its outer box. The defaults and the precedence are in
// the Sizing section of docs/styling.md.
```

- A check box toggles with Space, or with a click on the box or its label. A
  tri-state box goes from mixed to on, as Windows check boxes do. Enter does not
  toggle it: Enter goes on to the modal's default button or the form, as on Windows.
- A switch is drawn as a track and a knob, with no text: its label is its
  accessible name, so put the title beside it. Space toggles it; Enter does not.
- A toggle is a button that stays pressed while it is on, such as a Bold button in a
  toolbar. It has the size and look of a Ghost button (or, with `Secondary()`, a
  Secondary one), so it lines up with buttons, and `Small()` and `Large()` are the
  button sizes. On, it has a soft accent fill and an accent border; the weight of
  its text does not change. `Icon` adds an icon before the label, and `IconOnly()`
  draws the icon alone, with the label as its accessible name; give an icon-only
  toggle a `Tooltip` for the pointer. A click or Space toggles it; Enter does not.
- A toggle group is a row of joined toggles of which one is selected: the items
  share their borders, and the outer corners are rounded. It takes the options of a
  radio group, with an `Icon` each if you like, and `IconOnly()` draws the icons
  alone. Its keys are a radio group's: one Tab stop, the arrows select and move,
  Space chooses, and Enter goes on to the default button. A `Field` names it and
  checks it with `Validate`, as for a radio group.
- A radio group is one Tab stop, at the selected option. Up and Left go to the
  previous option, and Down and Right to the next. The arrows select the option
  they reach and move the focus to it. They wrap around and skip disabled options.
  Space chooses the focused option; Enter goes on to the modal's default button or
  the form, as for a check box.
  `OnChange` runs with the new value when the user selects an option, by a click,
  Space or the arrows.
- A select opens with a click, Enter, Space, Alt+Down or F4, and its list opens
  below the field, or above it when there is no room. While the list is closed,
  Up and Down choose the previous and next option, Home and End the first and
  last, and a typed letter the first option that starts with what has been typed.
  In the list, Up and Down move the highlight without wrapping, Home and End go
  to the ends, and typing moves to a matching option. Enter or Space chooses the
  highlighted option and closes the list.
  `OnChange` runs with the new value whenever the user changes the selection, by the
  keys or by the list. Escape and a press outside the list change nothing, so they
  do not run it. Escape, Tab and a press outside the list
  close it without a change. Focus returns to the field.
- A slider moves by one `step` with the arrows, by a tenth of the range with Page
  Up and Page Down, and to the ends with Home and End. A click or drag sets the
  value to the nearest step. `OnChange` runs when the value changes.
- A number input takes typed text, or steps with Up and Down (and its spin buttons,
  added with `SpinButtons(ui.SpinInline)`, a - button at its left edge and a + button at
  its right, inside the frame, or `SpinButtons(ui.SpinStacked)`, an up arrow above a down
  arrow at its right). Enter, or
  leaving the field, checks the text. A number
  outside the range is clamped to the nearest bound. Text that is not a number (or
  not a whole number, for an `int`) is not applied: Enter shows the field as
  invalid, with a message that gives the range, and keeps the text so it can be
  corrected. Leaving the field then reverts the text to the value that is set.
  `Precision` sets the decimals of a float; the default is the decimals of the step.
  The value is centered in the field by default. `Align(ui.Start)` or `Align(ui.End)`
  sets it to the left or the right, such as `End` for figures in a column. The spin
  buttons' hover and pressed fills are inset from the border by a small gap, so they
  never touch or cover it.

- A one-time code input, `ui.OTPInput(label, code, length)`, is a row of `length`
  boxes that each take one character, bound to a `*ui.Signal[string]`. It is one
  control with a caret, not a field per box. Typing fills the box at the caret and
  moves to the next one, Backspace clears the box before the caret and moves back,
  and Delete clears the box at the caret; the code has no gaps, so the characters
  after a cleared box move up. Left, Right, Home and End move the caret, never past
  the first empty box, and a click on a box moves it there. Focus puts the caret at
  the first empty box. Ctrl+V, or the Paste action, pastes the clipboard's text from
  the caret, keeping only the characters the input takes, so "Code: 482 916" fills
  six boxes. These keys are the actions of a text field, so a binding that changes
  them for fields changes them here too. It takes the digits 0 to 9, and with
  `Alphanumeric()` the letters A to Z and a to z as well, kept as typed. An input
  method's composition is not shown in the boxes; the text it confirms is typed.
  `Masked()` draws a dot in place of each character. `Groups(3, 3)` splits the boxes
  into groups with a wider gap between them; the sizes must add up to the length.
  `OnChange` runs after each change, and `OnComplete(fn)` when a change fills every
  box, after it. In a `Field` it is named by the field's label, shows the field's
  error with invalid boxes, and Enter submits the form; `Validate(ui.Required[string](),
  ui.MinLength(6))` asks for a complete code. Its look is `ui.OTPInputStyle`, set
  with `Theme.OTPInput` or `.Styles(...)`.

```go
ui.OTPInput("Verification code", code, 6).Groups(3, 3).
	OnComplete(func(code string) { verify(code) })
ui.OTPInput("PIN", pin, 4).Masked()                 // dots, reported as a password field
ui.OTPInput("Recovery code", rec, 8).Alphanumeric() // letters and digits
```

![Inline spin buttons inside the field, with the pointer over the plus button](screenshots/spin-buttons-inline.png)

![Stacked spin buttons, with the pointer over the up button](screenshots/spin-buttons-stacked.png)

Assistive technology reads a check box with a mixed state as mixed, a switch as a
check box with the Toggle pattern (as Phase B chose), a toggle as a button with
the Toggle pattern (as a `Button` with `AccessChecked` is), a radio group and a
toggle group as a selection container of radio buttons named by its label, and a select as a combo box whose Value is the
chosen label. A slider and a number input report their range (RangeValue). A
number input is a spin box that holds its text field, as Microsoft's Spinner
control type describes
([Spinner control type](https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-supportspinnercontroltype)).
A one-time code input is one text field named by its label, whose value is the
code; its boxes are not separate nodes. A masked one reports IsPassword, and its
value is a dot for each character, so the code is never exposed. Assistive
technology can set the code, which is filled from the first box.

## Color picker

`ui.ColorPicker(value)` edits a `ui.Color`. The trigger shows a swatch of the color and
its hex text, such as "#3B82F6". A click, Enter or Space opens a panel below it, which
Escape or a click outside closes, with focus back on the trigger. The panel holds a
saturation and brightness area, a hue slider, an opacity slider, a hex text field and
the preset swatches.

```go
ui.ColorPicker(fill).Label("Fill color").Alpha().
	Swatches(ui.Hex("#ef4444"), ui.Hex("#22c55e"), ui.Hex("#3b82f6")).
	OnChange(func(c ui.Color) { ... })
```

- The area takes focus when the panel opens. Its arrow keys move the saturation (Left and
  Right) and the brightness (Up and Down) by 1 percent, and by 10 percent with Shift.
- `Alpha()` adds the opacity slider and the eight-digit hex form, `#RRGGBBAA`. The hex
  field accepts `#RGB` and `#RRGGBB` otherwise. It is applied on Enter or when it loses
  focus; text that is not a color is refused, and the color does not change.
- The hue is kept while the saturation or the brightness is 0, so a grey keeps the hue
  the user chose.
- Assistive technology sees the trigger as a button named by `Label` (default "Color"),
  and the area as a slider named "Saturation and brightness" with its percentages.
- `Disabled(true)` shows the trigger dimmed, with its color; it cannot be focused or
  opened, and assistive technology reads it as disabled. `FocusHandle`, `LabelledBy`,
  `Key`, `Small`, `Large` and the sizing methods are as for the other controls.
  `Tooltip` shows below the trigger, and assistive technology reads it after the hex
  text.
- Under high contrast the area shows its hue alone, since gradients are drawn solid
  there; the thumb still marks the saturation and the brightness.
- The look is `ui.ColorPickerStyle`, set in `Theme.ColorPicker` or with `.Styles(...)`.

## Forms

A form is a set of fields. `ui.Field` gives a control a label, an optional help
text, an optional required marker and an error, and wires them to the control for
assistive technology. `ui.NewForm` checks the fields on submit and runs the submit.
Build the form once and keep it across renders (in a `Local`, or in the app's
state), since it holds the submit state:

```go
f := ui.NewForm().OnSubmit(func() error { return save(name.Peek()) })
ui.Div().Column().Gap(12).
	Child(f.Summary()).
	Child(f.Field("name", "Name", ui.TextInput(name).Placeholder("Project name").
		Validate(ui.Required[string](), ui.MaxLength(80))).
		Description("Shown in the project list.").Required()).
	Child(f.Field("email", "Email", ui.TextInput(email).Validate(ui.Pattern(emailRE, "Enter an email address."))).Inline()).
	Child(ui.Button("Save").Primary().OnClick(f.Submit))
```

![A form after a submit with an empty name: the summary, the required name with its error](screenshots/forms-summary.png)

![The New project dialog after a submit with an empty name](screenshots/modal-new-project-errors.png)

**Field.** `ui.Field(label, control)` puts the label above the control, or beside
it with `Inline()`, which suits settings rows. `Description(s)` is the help text,
`Required()` adds an asterisk after the label, `Error(msg)` shows an error the app
found (such as one from a server), and `Check(fn)` checks a whole field, such as a
group of check boxes, with `fn() error` (it reads its signals with `Get`). `Focus(h)`
gives the element that takes focus for a field whose control is a group; a
`Checkbox` (like every other control) takes a handle with its `FocusHandle` method for that. A field
outside a form (`ui.Field(...)` on its own) is checked the same way, but only as the
user works with it.

- The control is named by the label (`Label` and `LabelledBy`), and it is described
  by the help text and the error (`DescribedBy` and `HelpText`). A required control
  reports `IsRequiredForForm`; a control with a shown error reports `StateInvalid`,
  which the Windows adapter answers as `IsDataValidForForm` false.
- The asterisk is drawn and hidden from assistive technology. The state is the
  announcement, so the asterisk carries no meaning that a screen reader misses.
- The error is drawn above the control, under the help text, with the alert icon and
  the same look as a number input's message. The control's frame is the invalid
  frame, and it stays invalid while the control has focus.

**Validators.** A control takes validators with `Validate`: `TextInput`, `TextArea`,
`NumberInput`, `Select`, `Combobox`, `RadioGroup`, `Checkbox` (a `Checkbox` with
`Required[bool]()` is "must accept"), `DatePicker`, `TimePicker` and `DateRangePicker`
(over a `ui.DateRange`). They are generic over the control's value type:

- `ui.Required[T]()`: rejects the zero value (blank text after trimming, zero, `false`,
  or the placeholder option of a `Select` or `RadioGroup`). A number input does not
  need it, since it always holds a number.
- `ui.MinLength(n)`, `ui.MaxLength(n)`, `ui.Pattern(re, msg)` for text, counted in runes.
  The empty text passes `MinLength` and `Pattern`, so `Required` decides whether a field
  must be filled in, and an optional field can use them.
- `ui.Range(lo, hi)` for numbers, inclusive.
- `ui.Custom(func(v T) error)` for anything else.

A validator returns an error whose text is the message the field shows. The first
failing validator of a control is its message.

**When errors show.** These follow the guidance cited below: no error before the user
has acted, errors that are found on leaving a field, and every error on submit.

- A field shows nothing until the user changes its value and then leaves it, or until
  the form is submitted. A field that the user only visits shows nothing.
- Once an error is shown, it updates with each change, so it clears as soon as the
  value is valid.
- `Error(msg)` is shown at once, since the app found it.
- On submit, every field is checked and every field's message is shown.

**Submit.** `f.Submit` checks every field. When one needs attention, focus moves to the
first such field, the form announces the number assertively ("2 fields need attention.")
and the summary lists each error as a link that moves focus to its field. When every
field is valid, the submit function runs. `OnSubmit(fn)` returns an error for the
form's own message, such as a refusal from a server; the message is shown in the
summary and announced. `OnSubmitAsync(fn)` runs work in the background: `fn` starts the
work and returns a channel that receives the result (nil, or the error for the message).
While it runs, `f.Submitting()` is true, `Submit` does nothing, and the button can be
disabled, with a `ProgressRing` beside it.

```go
f := ui.NewForm().OnSubmitAsync(func() <-chan error {
	done := make(chan error, 1)
	go func() { done <- saveToServer(name) }()
	return done
})
ui.Button("Save").Disabled(f.Submitting()).OnClick(f.Submit)
```

- Enter in a single-line field of a form submits the form. A check box, switch or
  radio button keeps Enter for the modal's default button, as it does outside a form.
- `f.Reset()` clears the submitted state and the form's message, and forgets its
  fields, which are added again when they are built. Call it when the form's modal
  closes, so the next opening starts clean.
- `f.Summary()` is empty until a submit has an error. Place it above the fields.
- A field's key is the key given to `f.Field` (unique in the form). Keys are the
  window's, so two forms can use the same field keys.

**Styling.** A field's parts are `ui.FormFieldStyle` (`Root`, `Label`, `InlineLabel`,
`Required`, `Description`, `Error`), set with `Theme.FormField` or `Field.Styles`. A
form's summary parts are `ui.FormStyle` (`Summary`, `SummaryTitle`, `SummaryItem`, with
its hover and focus states), set with `Theme.Form` or `Summary().Styles`. The invalid
looks of the controls are the parts `FrameInvalid`, `TriggerInvalid`, `BoxInvalid` and
`RingInvalid`. See docs/styling.md.

**Sources.** The timing and the summary follow the WCAG 2.2 success criteria for
error identification and error suggestion
(https://www.w3.org/WAI/WCAG22/Understanding/error-identification.html and
https://www.w3.org/WAI/WCAG22/Understanding/error-suggestion.html), and the W3C WAI
forms guidance on labels, instructions and errors (https://www.w3.org/WAI/tutorials/forms/).
The error summary with links to the fields, the error message above the input, and
the help text under the label follow the GOV.UK Design System
(https://design-system.service.gov.uk/components/error-summary/ and
https://design-system.service.gov.uk/components/error-message/). The UI Automation
properties are Microsoft's (https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-automation-element-propids):
`IsRequiredForForm` (30025), `IsDataValidForForm` (30103), `HelpText` (30013),
`DescribedBy` (30105) and `FullDescription` (30159, not yet answered).
