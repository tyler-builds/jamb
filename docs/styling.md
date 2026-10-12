# Styling the built-in widgets

jamb owns the behavior of its complex widgets: keyboard handling, focus, selection,
roles and names for assistive technology. What a widget looks like is yours to
change. This document is the design for that: the named parts of a widget, the
typed styles that set them, how the theme and an instance combine, the states and
variants, render slots, and what customization cannot change.

The goal is that nobody rebuilds a built-in widget out of `Div`s because the API
did not expose the look they need. If you find yourself doing that, it is a gap in
this design, and the report of it is welcome.

## The parts of a widget

A widget is built from named parts. Each part has a typed style, and the widget
draws the part with that style merged over its built-in look. A part's name is part
of the typed style struct, so autocomplete and the documentation list every part.

Every built-in widget with a look of its own has parts. The table of widgets and
their parts is at the end of this document, and each widget's README section names
its style struct.

The parts are plain `ui.Style` values, so every field of `ui.Style` works in a part:
size, padding, margin, background, border, radius, shadow, opacity, text color, font
size and weight, transition. See the Style section of the README for the list.

```go
tabs := ui.Tabs(view, tabs...).Styles(ui.TabsStyle{
	Indicator:   ui.Style{Height: ui.Px(3), Background: t.Success},
	TabSelected: ui.Style{TextColor: t.Success},
})
```

## Two levels: the theme and the instance

Every part's look is resolved in three layers. Each layer merges over the one below
it, field by field, with the same rule as `ui.Style.Merge`: a field that is set in a
layer wins, and a zero field is unset and leaves the layer below in place.

1. **Built-in defaults.** Made from the theme's color tokens (`Accent`, `Surface`,
   `Border`, `Disabled` and the others), so the defaults follow the theme's colors.
   Some defaults depend on the state of the widget, such as a disabled select.
2. **The theme.** Set on the `ui.Theme` for every widget of that kind in the app:
   `Theme.Tabs`, `Theme.Select`, `Theme.Slider`, `Theme.Field` (the frame of every
   `TextInput`, `TextArea` and `NumberField`), `Theme.NumberField`, `Theme.Button`
   and `Theme.Buttons` (per variant), `Theme.Checkbox`, `Theme.RadioGroup`,
   `Theme.Switch`, `Theme.Collapsible`, `Theme.TreeView`, `Theme.DataGrid`, `Theme.Menu`,
   `Theme.MenuBar`, `Theme.Modal`, `Theme.ProgressBar`, `Theme.ProgressRing`,
   `Theme.Badge` and `Theme.Badges` (per variant), `Theme.Toast` and `Theme.Toasts`
   (per kind), `Theme.Tooltip`, and `Theme.FormField` (the label, help text and error
   of every field) and `Theme.Form` (the error summary of every form). Each is a field of `ui.Theme`, and the
   high-contrast theme takes it from the base theme.
3. **The instance.** Set on one widget with `.Styles(...)`. Calling `.Styles` again
   merges into the earlier values.

So an instance wins over the theme, and the theme wins over the defaults.

```go
// One tab list: its line is green, whatever the theme says.
ui.Tabs(view, tabs...).Styles(ui.TabsStyle{Indicator: ui.Style{Background: ui.Hex("#22c55e")}})
```

A zero field means unset, so a layer cannot turn a default off by setting it to
zero. To remove a default, such as a fill, a border or a radius, use the `Clear`
field (see "Turning a default off" below). The `Width` methods, such as
`Select.Width`, set the built-in size of the widget: the theme and `Styles` can
change it.

### The app's theme

To style the parts for a whole app, make the app's theme from a copy of a built-in
theme, and pass that copy to the app and to every theme switch. The copy is the
app's; the built-in `ui.DarkTheme` and `ui.LightTheme` stay as they are.

```go
// appTheme returns the dark theme of the app: a copy of the built-in one, with
// a 3 px line under every underline tab.
func appTheme() *ui.Theme {
	t := ui.DarkTheme.Clone()
	t.Tabs = ui.TabsStyle{Indicator: ui.Style{Height: ui.Px(3)}}
	return t
}

// The app starts with it, and the theme switch uses it.
application.Run(app.Window{Theme: appTheme(), Root: root})
ctx.SetTheme(appLightTheme()) // the same, from ui.LightTheme.Clone()
```

The built-in themes are variables, so setting a field on `ui.DarkTheme` changes it
for the whole process, including other windows and any code that reads it. Do not
do that: make a copy with `Clone`. `Clone` copies the theme's fields; the parts'
shadows are shared with the built-in theme, so change a part by assigning a new
style, not by changing a slice in place.

## Turning a default off

A zero field means unset. So a style cannot remove a default by setting it to zero:
a transparent background, no border, square corners, no padding, no shadow. For
those, a style has a `Clear` field, which names the fields to remove before its own
values are applied:

| Clear | Removes |
| --- | --- |
| `ui.ClearBackground` | `Background` and `Gradient`: no fill |
| `ui.ClearTextColor` | `TextColor`: the text inherits its color |
| `ui.ClearBorder` | `BorderWidth`, `BorderEdges` and `BorderColor`: no border |
| `ui.ClearRadius` | `Radius` and `Corners`: square corners |
| `ui.ClearShadows` | `Shadows`: no shadow, and no focus ring from this style |
| `ui.ClearPadding` | `Padding` |
| `ui.ClearMargin` | `Margin` |
| `ui.ClearGap` | `Gap` |

Clears combine with `|`. A clear and a value for the same field give the value, so
a clear followed by a new look is one step. Other fields follow the zero rule: a
length of zero is set (`ui.Px(0)` makes an element with no height), but opacity 0
is unset, so use `Opacity: 1` to restore full opacity.

```go
// Tabs with no fill, square corners and no padding.
ui.TabsStyle{Tab: ui.Style{Clear: ui.ClearBackground | ui.ClearRadius | ui.ClearPadding}}

// A select with no border, and a popup with no shadow.
ui.SelectStyle{Trigger: ui.Style{Clear: ui.ClearBorder}, Popup: ui.Style{Clear: ui.ClearShadows}}

// A slider rail with square ends.
ui.SliderStyle{Rail: ui.Style{Clear: ui.ClearRadius}}

// No indicator line under the selected tab: no fill and no height.
ui.TabsStyle{Indicator: ui.Style{Clear: ui.ClearBackground, Height: ui.Px(0)}}
```

Why a set of clears and not a sentinel value: a sentinel color or length would have
to be recognized by paint and layout everywhere. A clear is an instruction applied
in `Style.Merge`, so the rest of the framework does not change, and a style without
clears means what it always did. It follows `NoShrink`, which already turns a
default off with a flag. A clear is kept when styles are merged, so an instance's
`Styles` still clears the defaults it names. It is not drawn itself.

Under high contrast the clears of colors, borders and shadows are dropped, as the
colors and the borders are: the system colors and the border boundaries stay, and a
focus ring stays. The clears of radius, padding, margin and gap still apply.

## States

A state is a part of its own, named after the part and the state. `TabSelected`
applies to the selected tab, `OptionHighlighted` to the option that has keyboard
focus in an open list, and `SpinButtonDisabled` to a spin button that cannot step
the value further.

| Suffix | Applies while | Drawn by |
| --- | --- | --- |
| `Hover` | the pointer is over an enabled part | the part's hover style (paint only) |
| `Pressed` | the part is being pressed | the part's pressed style (paint only) |
| `FocusVisible` | the part has keyboard focus (not after a click) | the part's focus-visible style |
| `FocusWithin` | keyboard focus is on the part or on something inside it | the part's focus-within style (paint only) |
| `Selected` | the part is the selected one | the part's selected style |
| `Disabled` | the part cannot be used | the part's disabled style |
| `Highlighted` | the option is highlighted in an open list | the option's highlight style |
| `Invalid` | the field's value is not valid: a NumberField's text, or a control that a Field shows an error for (TextInput, TextArea, NumberField, Select, RadioGroup, Checkbox) | the part's invalid style |
| `Checked` | the check box is on, or the switch is on, or the ring is the selected one | the part's checked style |
| `Mixed` | the check box is in the mixed (indeterminate) state | the part's mixed style |
| `Open` | the collapsible section is open | the part's open style |

Hover and focus use the same mechanism as the `Hover` and `FocusVisible` styles of
`ui.Div`, so they are paint-only: they do not relayout the widget. `Selected`,
`Disabled`, `Highlighted` and `Invalid` are chosen when the widget is built.

Where a state has no part of its own, the widget applies the state to the part it
belongs to. For example, the hover of a tab is `TabHover`, and the selected tab is
`TabSelected` merged over `Tab`.

## Variants

A variant changes the built-in look, and the parts that are drawn. The behavior is
the same in every variant.

- `Tabs.Variant(ui.TabsUnderline)` (the default) draws a line under each tab, and the
  selected tab's line is drawn in `Indicator`.
- `Tabs.Variant(ui.TabsSegmented)` draws one row of segments. The selected segment is
  filled with `Accent` (`TabSelected`), and there is no `Indicator`.
- `NumberField.SpinButtons(ui.SpinInline)` (the default when buttons are shown) draws
  a minus button at the field's left edge and a plus button at its right edge, inside
  the frame and its border, with the text between them. Each button is the full height
  of the frame's inside and 24 px wide at the Text size setting of 100 percent, and both
  the buttons and the field's width scale with the setting, so the text keeps its room.
  The buttons are `SpinButton` (there is no `SpinButtons` group for them), and the Tab
  order is the text, then the minus and the plus button.
- `NumberField.SpinButtons(ui.SpinStacked)` draws an up arrow above a down arrow inside
  the field's right edge, as the up-down control of Windows does. The two buttons are
  the `SpinButtons` group, each half the field's height and at least 16 px wide, the
  arrows scale with the Text size, and the text takes the rest of the field, so it
  never runs under the buttons. The focus of a spin button is an inner outline, since a
  ring would fall outside the field's clipped edge.
- A spin button's corners follow the frame: on the stacked column the up button takes
  the frame's inner top-right corner and the down button its inner bottom-right corner,
  and inline the minus button takes the left corners and the plus button the right
  ones. The inner corners are the frame's radius less its border, so a hover or press
  fill follows the rounded border and never covers it, including the invalid frame's
  wider border. A `SpinButton` radius is replaced by these corners.

- `Button.Variant(ui.Primary | ui.Secondary | ui.Ghost | ui.Danger)` picks the look of
  a button. Its style is the theme's `Button` (every variant), then the variant's own
  in `Theme.Buttons` (`Primary`, `Secondary`, `Ghost`, `Danger`), then the instance's
  `Styles`. `Small()` is a size, not a variant, so it is not styled as one.
- `Badge.Variant(ui.BadgeNeutral | BadgeAccent | BadgeSuccess | BadgeWarning | BadgeDanger)`
  works the same way, with `Theme.Badge` and `Theme.Badges`.
- A toast's kind (`ToastInfo`, `ToastSuccess`, `ToastWarning`, `ToastError`) sets its
  icon and color; its style is `Theme.Toast`, then `Theme.Toasts` for that kind, then
  the toast's own `Styles`. `Toaster.Placement` picks the stack's position:
  `ToastBottomRight` (the default), `ToastTopRight` or `ToastBottomCenter`.

- `DataGrid.Density(ui.DataGridCompact)` sets the rows and the header to 26 logical pixels high, and
  `DataGridComfortable` (the default) to 36, both at the Text size setting. `DataGrid.Striped()` draws every
  other row in `RowAlternate`.

A part's style applies in every variant that draws the part. A variant's own defaults
come first, so `TabSelected` in the segmented variant is the filled segment, and an
override of `TabSelected` changes that segment as well.

## The text frame

`TextInput`, `TextArea` and `NumberField` share one text frame, and its parts are a
`ui.FieldStyle`: the box around the text (`Frame`) with its states (`FrameHover`,
`FrameFocusWithin`, `FrameDisabled` and `FrameInvalid`), the text inside (`Text`),
the placeholder (`Placeholder`), the selected text (`Selection`), and the containers
of the content before and after the text (`Leading` and `Trailing`). A `NumberField`
takes the same frame, and its `NumberFieldStyle` embeds the `FieldStyle`. Its
`Placeholder`, `Leading` and `Trailing` parts have no effect, because a number field
has none of that content.

The frame's layers are the defaults, then `Theme.Field`, then the instance. A
`NumberField` takes `Theme.NumberField` between the theme's `Field` and its own
`Styles`. A text field's `Style` method is its instance `Frame`, and it is plain under
high contrast as the instance's `Styles` are.

```go
ui.TextInput(query).Icon(icons.Search).Trailing(clearButton).Styles(ui.FieldStyle{
	FrameFocusWithin: ui.Style{BorderColor: t.Success},
	Placeholder:      ui.Style{TextColor: t.TextMuted},
})
```

- `Leading(el)` and `Trailing(el)` put an element inside the frame before and after
  the text, in the `Leading` and `Trailing` parts. The `Icon` of a `TextInput` is in
  `Leading` too, and it takes the `Leading` part's text color, as text does. A part
  is drawn only when the field has content for it.
- A borderless field (`Borderless()`) draws no `Frame` part, so its frame styles and
  states do not apply. Its `Text`, `Placeholder`, `Selection`, `Leading` and `Trailing`
  parts still apply.
- An invalid number field draws its `FrameInvalid` part over the frame, and the
  `Message` row below it. The message's icon is drawn in the `Message` part's
  `TextColor` when it sets one, and in `Danger` otherwise (the text color under high
  contrast). The message's text takes the same color.
- A text field, text area or number field with an error from a `Field` (see the
  README, Forms) draws `FrameInvalid` over its frame, and it keeps that frame while
  it has focus, so the danger color is not hidden by the focus color. The error
  itself is a field part (`FormField.Error`), which has the same look as the number
  field's `Message`: both take `messageStyle` in the code.
- The caret is not a part. Its width and blink follow the system settings, and it is
  drawn in the text color.
- Under high contrast the frame's colors and its focus follow the system: the focus is
  the Highlight color, and an invalid frame is in the text color. A border width and
  the padding of the frame still apply.

## Render slots

A render slot replaces the content of one part with an element of your own, and the
widget keeps everything else: the keyboard, focus, selection, the states, and the
accessible role and name.

| Slot | Replaces | Called with |
| --- | --- | --- |
| `Tabs.RenderTab(fn)` | the icon and label of each tab | the `Tab` and a `PartState` |
| `Select.RenderValue(fn)` | the label of the chosen option in the field | the option and a `PartState` |
| `Select.RenderOption(fn)` | the label of each option in the open list | the option and a `PartState` |
| `TextInput.Leading(el)`, `TextArea.Leading(el)` | adds an element before the text, after the icon | the element you pass, drawn as it is |
| `TextInput.Trailing(el)`, `TextArea.Trailing(el)` | adds an element after the text | the element you pass, drawn as it is |
| `Collapsible.RenderHeader(fn)` | the title in the header; the chevron stays | the title and a `PartState` (`Selected` is open) |
| `TreeView.RenderRow(fn)` | the icon and label of each row; the chevron stays | the `TreeNode` and a `PartState` |
| `TreeView.RenderLabel(fn)` | the text of each row; the icon stays (not used with `RenderRow`) | the `TreeNode` and a `PartState` |
| `DataColumn.Cell` (in `DataGrid`) | the content of each cell of the column; the cell keeps its text, focus and selection | the row's data and nothing else (the content is hidden from assistive technology, unless the column is `Interactive`, when its controls are exposed) |
| `DataColumn.Editor` (in `DataGrid`) | the text field of an edit, replaced by the editor it returns (a Select, a NumberField, a CheckBox); the cell keeps its frame and the message popover | the row's data, the pending value, and the `change`, `commit` and `cancel` functions |
| `DataColumn.RenderHeader` (in `DataGrid`) | the text of the column's header; the sort arrow stays | the header label and a `PartState` (`Selected` is the sorted column) |

```go
ui.Tabs(inbox,
	ui.Tab[string]{Value: "open", Label: "Open, 4 items"},
	ui.Tab[string]{Value: "closed", Label: "Closed, 12 items"},
).RenderTab(func(tab ui.Tab[string], s ui.PartState) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(6).
		Child(ui.Text(names[tab.Value])).
		Child(ui.Badge(counts[tab.Value]))
})
```

The content a slot returns is hidden from assistive technology. The accessible name
of the element is still the widget's own: a tab's `Label`, an option's `Label`. So
the name is the text a user must hear, and the slot's content is for sighted users
only. If a count matters to a screen reader user, put it in the label, as the example
does ("Open, 4 items"). The slot's content is not announced, and it does not change
the role, selection or state the widget reports.

`PartState` carries the state the slot needs:

```go
type PartState struct {
	Hovered, Pressed, Focused, Selected, Disabled, HighContrast bool
}
```

- `Hovered` and `Pressed` are kept current. When the pointer enters or leaves the part,
  or a press starts or ends, the slot is called again with the new state.
- `Focused` is keyboard focus on the part. For a select option, which is not focused
  itself, it means the option is the highlighted one in the open list.
- `Selected`, `Disabled` and `HighContrast` are set when the widget is built.

A slot that ignores the state still works. A slot that uses `Selected` or `Focused`
must still leave a non-color cue for sighted users, such as the line, the fill or the
mark of the built-in look. The text's weight is not one of them: no built-in state
changes it.

`Select.RenderValue` is called only while an option is chosen. With no choice the
placeholder is drawn, as it is by default.

The `Leading` and `Trailing` slots of a text field take an element, not a function:
the element is drawn in its part, and the field keeps its focus, text, name and keys
(see "The text frame").

A Slider has no render slot. Its thumb is a fixed 16 px knob, and its size and
position are part of how the value maps to the track. Style its colors, border and
shadow with `Thumb`, `ThumbHover` and `ThumbPressed` instead.

## Guardrails

Customization cannot break these, and this is why.

- **High contrast uses the system colors.** Under high contrast the app's theme and
  instance overrides lose their colors, gradients, shadows and opacity, and their
  clears of colors, borders and shadows (`plainStyle`). Sizes, spacing, font size and
  weight, radius, border widths and transitions still apply, and so do the clears of
  radius, padding, margin and gap. Built-in high-contrast defaults use the system colors only, so a selected
  part is the system Highlight, a disabled part is GrayText and not dimmed, and the
  focus ring is a solid spread outline. An app cannot remove a built-in focus ring
  under high contrast, because the ring is part of the defaults and an override's
  shadow is dropped there.
- **State is not carried by color alone.** The built-in looks of the states also use a
  line, a fill or a mark: a selected tab has a line, a selected option has a check mark,
  a selected row has a fill, and an invalid number field has a thicker frame and a
  message. These are defaults, so an override can change them. Keep one non-color cue
  in your styles.
- **No state changes the weight of the text.** Hover, press, focus, selection, the
  highlighted option and the sorted column keep the text's weight, so the built-in
  looks never make text bold on their own. Set a weight yourself in a part's style
  (`FontWeight`) if you want one. Under high contrast the cue is the system's: a
  selected tab keeps its indicator line, and an app cannot clear the line's color or
  change its height. A highlighted option and a selected row keep their Highlight fill,
  and a chosen option keeps its check mark. An invalid number field keeps its message.
  Those are not parts, so no style removes them.
- **Text scale applies.** The runtime scales text with the Text size setting (100 to
  225 percent). That includes text that a part styles, so a part's font size grows
  with the setting, as all text does.
- **Reduced motion applies.** A part's `Transition` eases its color, opacity, offset
  and scale only when reduced motion is off. Under the setting a change is instant.
- **Keys, focus and accessibility are the widget's.** Keys, the Tab order, the roles,
  names and states the accessibility tree reports, and the keyboard behavior do not
  depend on any part's style. A render slot's content is hidden from assistive
  technology, so it cannot add or remove an accessible element.
- **Hit testing follows layout.** A slot's content and a part's padding change the
  box a click or drag hits, as they do for any element. The Thumb keeps its size so
  the value's position on the track stays right.

What customization can change is the look, and the size and spacing of the parts. It
cannot change which keys act, what is focusable, or what assistive technology is told.

## Adding styling to a widget

A new widget that has parts follows the same pattern as `Tabs`, `Select`, `Slider` and
`NumberField`. In `ui`:

1. **Name the parts.** Write `<Widget>Style` as a struct of `Style` fields, one per part,
   and one more per state (the naming rules are below). Add a `merge` method that merges
   each field and a `plain` method that applies `plainStyle` to each field.
2. **Write the defaults.** A `<widget>Defaults(t *Theme, ...)` function returns the built-in
   look from the theme's tokens. It reads the state it needs (disabled, variant, width).
   Under high contrast it uses the system colors, and it keeps the focus ring.
3. **Resolve the layers.** A `resolve<Widget>` function starts from the defaults, then
   merges `t.<Widget>` and the instance's styles, in that order. Under high contrast, both
   overrides pass through `plain()` first.
4. **Add the fields.** Add the `<Widget>Style` field to `Theme`, and copy it in
   `HighContrastTheme` from the base theme. Add a `Styles` method to the widget that merges
   into an unexported field.
5. **Name the elements.** Call `asPart("<Widget>.<Part>")` on the element that draws each
   part. The name is how the drift test finds the part. A state with no element of its own
   is drawn by the element of the part it belongs to.
6. **Test it.** Add the part to the table in `TestEveryPartIsApplied`: set a distinctive style on
   the part, put its state in effect, and assert that the style reaches the node. Add the
   widget to the precedence and high-contrast tests.
7. **Document it.** Add the widget's parts to the table below and to the README section
   for the widget.

## Naming conventions

- Part names are nouns: `Tab`, `Indicator`, `Popup`, `Option`, `SpinButton`. A part that
  is a container of other parts is plural when it groups them (`SpinButtons`, `List`).
- A state is the part's name with a state suffix: `TabSelected`, `TriggerHover`,
  `OptionHighlighted`, `SpinButtonFocusVisible`. The state suffixes are the ones in the
  table above, and a new widget uses them with the same meaning.
- The accessibility name of an element is never a style name. Element names for the tests
  are `Widget.Part`, such as `Tabs.Tab` and `Select.Popup`. They are not exposed to
  assistive technology.
- Keys of the built-in parts are scoped by the widget's key. Tab keys are
  `<scope>/tab:<value>` and panel keys are `<scope>/panel` (for the one panel) or
  `<scope>/panel:<value>` (with `KeepPanels`). The `tab:` and `panel` prefixes keep a tab
  and the panel disjoint, so a value that formats as "panel" cannot collide with the
  default panel.

## Widgets and their parts

Every built-in widget with a look of its own is listed, with its parts, variants and
render slots. A row marked `planned` would be a widget that is not yet styled; none
is at this release. The drift test in `ui/parts_test.go` checks this table against the
parts that the code applies.

| Widget | Parts | Variants | Render slots | Status |
| --- | --- | --- | --- | --- |
| `Tabs` | `List`, `Tab`, `TabHover`, `TabSelected`, `TabDisabled`, `TabFocusVisible`, `Indicator`, `Panel` | `TabsUnderline`, `TabsSegmented` | `RenderTab` | done |
| `FormField` | `Root`, `Label`, `InlineLabel`, `Required`, `Description`, `Error` | none | none | done |
| `Form` | `Summary`, `SummaryTitle`, `SummaryItem`, `SummaryItemHover`, `SummaryItemFocusVisible` | none | none | done |
| `Select` | `Trigger`, `TriggerHover`, `TriggerFocusVisible`, `TriggerDisabled`, `TriggerInvalid`, `Value`, `Placeholder`, `Chevron`, `Popup`, `Option`, `OptionHighlighted`, `OptionSelected`, `OptionDisabled` | none | `RenderValue`, `RenderOption` | done |
| `Slider` | `Track`, `TrackFocusVisible`, `Rail`, `Fill`, `Thumb`, `ThumbHover`, `ThumbPressed`, `ValueText` | none | none (see above) | done |
| `NumberField` | `Frame`, `FrameHover`, `FrameFocusWithin`, `FrameDisabled`, `FrameInvalid`, `Text`, `Selection`, `Message`, `SpinButtons`, `SpinButton`, `SpinButtonHover`, `SpinButtonPressed`, `SpinButtonFocusVisible`, `SpinButtonDisabled` | `SpinInline`, `SpinStacked` | none | done |
| `Button` | `Root`, `RootHover`, `RootPressed`, `RootDisabled`, `RootFocusVisible`, `Label`, `Icon` | `Primary`, `Secondary`, `Ghost`, `Danger` (style per variant in `Theme.Buttons`) | none | done |
| `TextInput, TextArea` | `Frame`, `FrameHover`, `FrameFocusWithin`, `FrameInvalid`, `Text`, `Placeholder`, `Selection`, `Leading`, `Trailing` (see "The text frame") | none | `Leading`, `Trailing` (elements) | done |
| `Checkbox`, `CheckboxTri` | `Control`, `ControlFocusVisible`, `ControlDisabled`, `Box`, `BoxHover`, `BoxChecked`, `BoxMixed`, `BoxDisabled`, `BoxInvalid`, `Mark`, `Dash`, `Label`, `LabelDisabled` | none | none | done |
| `Switch` | `Track`, `TrackChecked`, `TrackHover`, `TrackFocusVisible`, `TrackDisabled`, `Knob`, `KnobChecked`, `KnobDisabled` | none | none | done |
| `RadioGroup` | `Group`, `Radio`, `RadioFocusVisible`, `RadioDisabled`, `Ring`, `RingChecked`, `RingDisabled`, `RingInvalid`, `Dot`, `DotDisabled`, `Label`, `LabelDisabled` | none | none | done |
| `Collapsible`, `Accordion` | `Section`, `Header`, `HeaderHover`, `HeaderOpen`, `HeaderFocusVisible`, `HeaderDisabled`, `Chevron`, `Content` | none | `RenderHeader` | done |
| `TreeView` | `Row`, `RowHover`, `RowSelected`, `Chevron`, `Icon`, `Label`, `TreeFocusVisible` | none | `RenderRow`, `RenderLabel` | done |
| `DataGrid` | `Grid`, `GridFocusVisible`, `Header`, `HeaderCell`, `HeaderCellHover`, `HeaderCellSorted`, `SortIndicator`, `ResizeHandle`, `Row`, `RowHover`, `RowSelected`, `RowAlternate`, `Cell`, `CellFocused`, `Empty`, `CellEditing`, `Editor`, `EditorMessage` | `DataGridComfortable`, `DataGridCompact`, `Striped()` | `DataColumn.Cell`, `DataColumn.RenderHeader` | done |
| `DropdownMenu`, `ContextMenu` | `Panel`, `Item`, `ItemHighlighted`, `ItemDisabled`, `Separator`, `SectionLabel`, `Shortcut`, `Check`, `SubmenuChevron` | none | none | done |
| `MenuBar` | `BarItem`, `BarItemHover`, `BarItemFocusVisible`, `BarItemOpen` | none | none | done |
| `Modal` | `Scrim`, `Panel`, `Title`, `Body`, `Footer` | none | none | done |
| `ProgressBar` | `Track`, `Fill`, `Label` | none | none | done |
| `IndeterminateProgressBar` | `Track`, `Fill` | none | none | done |
| `ProgressRing` | `Ring`, `Dot` | sizes (`RingSmall`, `RingMedium`, `RingLarge`) | none | done |
| `Badge` | `Badge`, `Label`, `Dot` | `Neutral`, `Accent`, `Success`, `Warning`, `Danger` (style per variant in `Theme.Badges`) | none | done |
| `Toast` | `Container`, `Icon`, `Text`, `Action`, `Close` | `Info`, `Success`, `Warning`, `Error` (style per kind in `Theme.Toasts`; placement with `Toaster.Placement`) | none | done |
| `Tooltip` | `Panel`, `Text` | none | none | done |

## Examples

A theme-level change, an instance override, a variant, and a render slot, from the
demo's Components page (Customizing):

```go
appTheme() // a copy of ui.DarkTheme, with a 3 px line under underline tabs (see "The app's theme")

// One tab list, with a green line and a green selected label.
ui.Tabs(section, tabs...).Styles(ui.TabsStyle{
	Indicator:   ui.Style{Background: t.Success},
	TabSelected: ui.Style{TextColor: t.Success},
})

// A segmented tab list for a filter.
ui.Tabs(filter, tabs...).Variant(ui.TabsSegmented)

// A number field with stacked spin buttons.
ui.NumberField(hour, 0, 23, 1).Label("Hour").SpinButtons(ui.SpinStacked)

// A select whose options show a color swatch.
ui.Select(color, options...).Label("Color").
	RenderOption(func(o ui.Option[string], s ui.PartState) ui.Element {
		return swatchRow(o.Value, o.Label)
	})
```

Customizing the other widgets follows the same pattern:

```go
// Every button has a 4 px radius; the primary button is also green.
t.Button = ui.ButtonStyle{Root: ui.Style{Radius: 4}}
t.Buttons.Primary = ui.ButtonStyle{Root: ui.Style{Background: t.Success}}

// One checkbox is green when it is on.
ui.Checkbox("Notify me", notify).Styles(ui.CheckboxStyle{
	BoxChecked: ui.Style{Background: t.Success, BorderColor: t.Success},
})

// A section header shows its count; the title keeps the count for assistive technology.
ui.Collapsible("Sync, 3 items", open, body).RenderHeader(func(title string, s ui.PartState) ui.Element {
	return ui.Div().Row().Gap(6).Child(ui.Text("Sync")).Child(ui.Badge(3))
})

// A toast of its own, with a green icon, at the top right of the window.
ui.Toaster().Placement(ui.ToastTopRight)
ctx.Toast(ui.ToastSuccess, "Saved").Styles(ui.ToastStyle{Icon: ui.Style{TextColor: t.Success}})
```

## Limits

- The stacked spin buttons take the field's minimum height at the Text size setting.
  A Frame style that sets a different minimum height does not change their halves,
  so keep the field's height at its default when you use SpinStacked.
- The inline spin buttons are laid over the frame's left and right padding. The
  variant sets that padding to the width of a button, so a Frame style that sets a
  smaller left or right padding puts the text under a button.
- The caret of a text field is not a part (see "The text frame"). Its width and blink
  follow the system settings.
- The Slider has no render slot for the thumb (see above).
- The Tabs have no render slot for the panel: the panel is the app's own content.
- `Select.Width` sets the field's width and the list's minimum width together. The
  `Trigger` and `Popup` parts can override either.
- The Tree has no `RowFocused` part. The keyboard focus ring is drawn on the tree
  itself (`TreeFocusVisible`), because a row cannot see whether the tree has focus.
- The Tree's `Indent` (pixels per level) is a size on `TreeStyle`, not a part.
- A Switch draws no text, so its label position is the app's. There is no label
  position option.
- A progress ring has sizes (`RingSmall`, `RingMedium`, `RingLarge`), but no arc
  variant: its dots are the only drawing.
- The menus have no render slot for their items. `DropdownMenu` takes `Styles`; a
  `ContextMenu` takes the theme's `Menu` only, as it has no instance to style.
- A Modal has no size variants: its panel is 420 px wide, and its `Panel` part
  (`Width`) sets another width.
- A Tooltip takes the theme's `Tooltip` only.
- A toast's `Styles` applies to that toast alone; `Toaster` has no `Styles` of its
  own.
- A DataGrid's header and rows are as tall as its density (see Variants). A `Header` or `Row` style that sets
  another `MinHeight` does not move the padding that keeps the header above the rows, so keep the height at the density's.
- The DataGrid's `Grid` padding is the header's height, which the grid sets. It has no frozen column and no
  multi-column sort. The double-click fit of a column edge measures the `Text` of the rows in view, not a `Cell` slot.
- An editing cell is perceivable under high contrast without its colors: the `Editor` frame of a failed save is
  the text color at the invalid width (`FrameInvalid`), and `EditorMessage` is drawn in the text color with a
  text-colored border. `CellEditing` and `Editor` take the plain look of the other parts. The `EditorMessage`
  popover is drawn above the window, so no ancestor clips it; the editor's own frame stays visible.
- A cell clips what it draws to its column (`OverflowHidden` in the `Cell` part). A `Cell` style that sets an
  overflow changes that, and content wider than its column is cut at the column's edge, not drawn over the next one.
- A custom editor (`DataColumn.Editor`) is the app's to style: the grid draws the cell around it and the message
  popover, and the `Editor` part does not apply to it.

Where a look changed under high contrast on purpose, the change is in the guardrails
above: the selected text of a text field and the highlighted row of a menu are the
Highlight color with HighlightText on it.
