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
their parts is at the end of this document, and each widget's section in the topic docs names
its style struct.

The parts are plain `ui.Style` values, so every field of `ui.Style` works in a part:
size, padding, margin, background, border, radius, shadow, opacity, text color, font
size and weight, transition. See [Styling](theming.md#styling) for the list.

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
   `Theme.Tabs`, `Theme.Select`, `Theme.Combobox`, `Theme.DatePicker`, `Theme.DateRangePicker`, `Theme.Calendar`, `Theme.TimePicker`, `Theme.Slider`, `Theme.Split`, `Theme.Dock`, `Theme.Field` (the frame of every
   `TextInput`, `TextArea` and `NumberInput`), `Theme.NumberInput`, `Theme.OTPInput`, `Theme.Button`
   and `Theme.Buttons` (per variant), `Theme.Checkbox`, `Theme.RadioGroup`,
   `Theme.Switch`, `Theme.Toggle`, `Theme.ToggleGroup`, `Theme.Rating`, `Theme.Collapsible`, `Theme.Tree`, `Theme.DataGrid`, `Theme.Menu`,
   `Theme.MenuBar`, `Theme.Modal`, `Theme.Drawer`, `Theme.Popover`, `Theme.CommandPalette`, `Theme.ProgressBar`, `Theme.ProgressRing`,
   `Theme.Badge` and `Theme.Badges` (per variant), `Theme.Toast` and `Theme.Toasts`
   (per kind), `Theme.Tooltip`, `Theme.Avatar`, `Theme.Tag`, `Theme.Skeleton`, `Theme.Sidebar`, `Theme.Breadcrumbs`, `Theme.Pagination`, `Theme.Stepper`, `Theme.Carousel`, `Theme.ColorPicker`, and `Theme.FormField` (the label, help text and error
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
field (see "Turning a default off" below). The sizing methods, such as `Width`,
set the outer size of a widget and win over the theme and `Styles`; the theme and
`Styles` set the size of the parts. See "Sizing" below.

### The app's theme

To style the parts for a whole app, start the app's theme from a built-in theme,
and pass it to the app and to every theme switch. `ui.DarkTheme()` and
`ui.LightTheme()` return a new copy on every call, so the app can change its copy
freely. `ui.NewTheme(name)` returns a copy of any predefined theme by its name, and
`ui.Themes()` lists them (see [Predefined themes](theming.md#predefined-themes)).

```go
// appTheme returns the dark theme of the app: the built-in one, with a 3 px
// line under every underline tab.
func appTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Tabs = ui.TabsStyle{Indicator: ui.Style{Height: ui.Px(3)}}
	return t
}

// The app starts with it, and the theme switch uses it.
application.Run(app.Window{Theme: appTheme(), Root: root})
ctx.SetTheme(appLightTheme()) // the same, from ui.LightTheme()
```

To derive several themes from one of your own, copy it with `Clone`. `Clone`
copies the theme's fields; the parts' shadows are shared with the original, so
change a part by assigning a new style, not by changing a slice in place.

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
| `Invalid` | the field's value is not valid: a NumberInput's text, or a control that a Field shows an error for (TextInput, TextArea, NumberInput, DatePicker, DateRangePicker, TimePicker, Select, RadioGroup, ToggleGroup, Checkbox) | the part's invalid style |
| `Checked` | the check box, the switch or the toggle is on, the ring is the selected one, or the star is at or below the rating | the part's checked style |
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
- `NumberInput.SpinButtons(ui.SpinInline)` (the default when buttons are shown) draws
  a minus button at the field's left edge and a plus button at its right edge, inside
  the frame and its border, with the text between them. Each button is 24 px wide at
  the Text size setting of 100 percent, and both the buttons and the field's width
  scale with the setting, so the text keeps its room. The buttons are `SpinButton`
  (there is no `SpinButtons` group for them), and the Tab order is the text, then the
  minus and the plus button.
- `NumberInput.SpinButtons(ui.SpinStacked)` draws an up arrow above a down arrow inside
  the field's right edge, as the up-down control of Windows does. The two buttons are
  the `SpinButtons` group: each is the field's height less the gap at the column's top
  and bottom and twice the gap between the two, and at least 16 px wide. The arrows
  scale with the Text size, and the text takes the rest of the field, so it never runs
  under the buttons.
- A spin button's fill is inset from the border by a gap, as the Windows 11 NumberBox
  draws its compact spin buttons. The gap is 2 px at the Text size setting of 100
  percent, and it scales with the Text size and the DPI. It is on every side that
  meets the border, so a hover or pressed fill (`SpinButtonHover`, `SpinButtonPressed`)
  never touches or covers the border. The fill changes only its background: the button
  keeps its size, with no growth, shadow or change of weight. The button is the hit
  target, so the gap is not clickable.
- A spin button's fill has the frame's inner radius less the gap on all four corners.
  That radius is concentric with the frame's inner curve, so the fill stays inside the
  curve, including the invalid frame's wider border. A `SpinButton` radius is replaced
  by these corners.
- The focus of a spin button (`SpinButtonFocusVisible`) is an inner outline 2 px wide,
  drawn on the button's fill, so it stays inside the gap and inside the border.

- `Button.Variant(ui.ButtonPrimary | ui.ButtonSecondary | ui.ButtonGhost | ui.ButtonDanger)` picks the look of
  a button. Its style is the theme's `Button` (every variant), then the variant's own
  in `Theme.Buttons` (`Primary`, `Secondary`, `Ghost`, `Danger`), then the instance's
  `Styles`. `Small()` and `Large()` are sizes, not variants, so they are not styled as one.
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

`TextInput`, `TextArea` and `NumberInput` share one text frame, and its parts are a
`ui.FieldStyle`: the box around the text (`Frame`) with its states (`FrameHover`,
`FrameFocusWithin`, `FrameDisabled`, `FrameReadOnly` and `FrameInvalid`), the text inside (`Text`),
the placeholder (`Placeholder`), the selected text (`Selection`), and the containers
of the content before and after the text (`Leading` and `Trailing`). A `NumberInput`
takes the same frame, and its `NumberInputStyle` embeds the `FieldStyle`. Its
`Placeholder`, `Leading` and `Trailing` parts have no effect, because a number input
has none of that content.

A read-only field (`ReadOnly(true)`) draws `FrameReadOnly` over its frame: by default
a slightly different fill (the theme's `SurfaceHover`, or `SurfaceActive` in a dark
theme), with the border and full-contrast text kept, since its text is meant to be
read and copied. Under high contrast it keeps the window color. It has no hover state. A disabled field
(`Disabled(true)`) draws `FrameDisabled`, dimmed by default (GrayText under high
contrast), and takes no pointer or keyboard input.

The frame's layers are the defaults, then `Theme.Field`, then the instance. A
`NumberInput` takes `Theme.NumberInput` between the theme's `Field` and its own
`Styles`. A text field's `Style` method is its instance `Frame`, and it is plain under
high contrast as the instance's `Styles` are.

A `NumberInput`'s value is centered in its text area by default. `Align` sets where
it sits: `Align(ui.Start)`, `Align(ui.Center)` (the default) or `Align(ui.End)`, such
as `End` for figures that line up in a column. The text area is the room between the
spin buttons: inline, it is the middle of the frame, and stacked, it is the room left
of the column. A value wider than the field scrolls, as before, and is not aligned.
The caret and the selection follow the alignment.

```go
ui.TextInput(query).Icon(icons.Search).Trailing(filterButton).Styles(ui.FieldStyle{
	FrameFocusWithin: ui.Style{BorderColor: t.Success},
	Placeholder:      ui.Style{TextColor: t.TextMuted},
})
```

- `Leading(el)` and `Trailing(el)` put an element inside the frame before and after
  the text, in the `Leading` and `Trailing` parts. The `Icon` of a `TextInput` is in
  `Leading` too, and it takes the `Leading` part's text color, as text does. A part
  is drawn only when the field has content for it.
- The buttons that a `TextInput` adds itself, the clear button of a `Clearable` field
  and the reveal button of a `Revealable` one, are in the `Trailing` part, in that
  order and before the `Trailing` element. Each is a `Button` part, with
  `ButtonHover` while the pointer is over it. A `TextArea` has none.
- A borderless field (`Borderless()`) draws no `Frame` part, so its frame styles and
  states do not apply. Its `Text`, `Placeholder`, `Selection`, `Leading` and `Trailing`
  parts still apply.
- An invalid number input draws its `FrameInvalid` part over the frame, and the
  `Message` row below it. The message's icon is drawn in the `Message` part's
  `TextColor` when it sets one, and in `Danger` otherwise (the text color under high
  contrast). The message's text takes the same color.
- A text field, text area or number input with an error from a `Field` (see the
  [Forms](widgets-forms.md#forms)) draws `FrameInvalid` over its frame, and it keeps that frame while
  it has focus, so the danger color is not hidden by the focus color. The error
  itself is a field part (`FormField.Error`), which has the same look as the number
  field's `Message`: both take `messageStyle` in the code.
- The caret is not a part. Its width and blink follow the system settings, and it is
  drawn in the text color.
- Under high contrast the frame's colors and its focus follow the system: the focus is
  the Highlight color, and an invalid frame is in the text color. A border width and
  the padding of the frame still apply.

## Sizing

Every built-in widget that has a box of its own takes the same sizing methods:
`Width`, `Height` (where a height means something), `MinWidth`, `MinHeight`,
`MaxWidth`, `MaxHeight`, `Grow` and `GridSpan`. The lengths are `ui.Px`, `ui.Percent`,
`ui.Full` (100 percent) and `ui.Auto`. `Grow` takes a share of the free space of a row
or a column, as a `Div` does, and `GridSpan(cols, rows)` covers more cells of a grid
(see [Grid layout](layout.md#grid-layout)). In a grid cell a widget fills the cell in place of
its default width; a width set with `Width` is kept. The methods size the widget's outer box: the frame of a text
field or a number input, the trigger of a select, the track of a slider and the bar
of a progress bar (its readout included), the control of a check box, the group of
radio buttons, the tab list of `Tabs`, the section of a collapsible, the panel of a
modal, and the track of a switch, and the split of `Split`. A chart (package `charts`) and
the code editor (package `codeedit`) take them too, on their box.

```go
ui.TextInput(query).Width(ui.Full)          // fills its container
ui.Select(channel, opts...).Width(ui.Px(240))
ui.ProgressBar(0.4).MaxWidth(ui.Px(336))    // at most 336 px, and a default minimum
ui.Button("Save").Width(ui.Full)            // stretches to the container
```

The defaults follow WinUI's, and CSS form controls where WinUI has no equivalent. At
the Text size of 100 percent:

| Widget | Default width |
| --- | --- |
| `Button`, `IconButton` | as wide as its label; `Width(ui.Full)` fills the container |
| `Checkbox`, `CheckboxTri`, `RadioGroup`, `Badge` | as wide as their content, also in a column |
| `Switch` | 38 px track (the knob stays 3 px from its end) |
| `Toggle`, `ToggleGroup` | as wide as their label or items, as a button is |
| `Rating` | as wide as its stars (20 px each, 2 px apart) |
| `TextInput`, `TextArea` | 240 px, a fixed size like a CSS input's; `Full` stretches it |
| `NumberInput` | 120 px, a fixed size for its column (the frame and the message row); `Full` stretches it, and `Grow` and the limits apply to the column |
| `OTPInput` | as wide as its boxes, each 40 px at the Text size of 100 percent, with the gaps between them and between their groups |
| `Field` | fills its container, as a column does; its control keeps its own width (see below) |
| `Select` | 220 px, a fixed size; `Full` stretches it, and the list is at least as wide as the field |
| `Combobox` | 240 px at the Text size of 100 percent, like a text field; `Full` stretches it, and the list is at least as wide as the field |
| `DatePicker` | 180 px at the Text size of 100 percent; `Full` stretches it, and the calendar is its own size |
| `DateRangePicker` | 280 px at the Text size of 100 percent, room for two dates; `Full` stretches it, and the calendar is its own size |
| `TimePicker` | 140 px at the Text size of 100 percent; `Full` stretches it, and the list is at least as wide as the field |
| `Calendar`, `CalendarRange` | as wide and as high as its month: seven days of 32 px at the Text size of 100 percent, and six weeks |
| `Slider`, `ProgressBar` (also when indeterminate) | fills the container, and at least 160 px, so it is visible where its container has no width |
| `Tabs` | the tab list: `TabsSegmented` as wide as its tabs; `TabsUnderline` fills the container, as WinUI's `TabView` does |
| `Breadcrumbs`, `Pagination` | as wide as their locations or pages; `Width`, `MinWidth` and `MaxWidth` set it |
| `Stepper` | fills the container when horizontal, with the lines sharing the room between the steps; as wide as its steps when `Vertical` |
| `StatusBar` | fills the container; the start items take the free room, and the end items keep their width |
| `Toolbar` | fills the container, as a row does; its controls keep their own widths |
| `Empty` | fills the container, with its parts centered in it |
| `Carousel` | fills the container's width, and is as high as its slide and its controls; with a `Height`, the viewport takes the room the controls leave |
| `Collapsible`, `Accordion` | fills the container |
| `DataGrid`, `Tree`, `ListOf`, `UniformList` | fills the container's width; give a height (`Height`, `MaxHeight`, or `Grow` in a column) so the rows scroll inside it |
| `Split` | fills the container's width; its height is the container's, as `Full` gives (set `Height` or `Grow` in a column with other children) |
| `Modal` | a 420 px panel; a percentage is of the window's width |
| `Drawer` | a 360 px panel at the left or right, the window's height; a 280 px panel at the top or bottom, the window's width |
| `Text` | automatic, as a `Div`: fills a column, as wide as its words in a row |

A form field (`Field`) takes the same methods on its outer box, so a labeled field is sized as a unit. Its control keeps its own width: a control whose width is `Auto` or `Full` fills the field, and a fixed one keeps its size. A content-sized control, such as a button, stays as wide as its content.

The fixed defaults of the text fields, the number input and the select scale with the
Text size, so their proportions hold at 225 percent. Content-sized widgets are not
stretched by their container, unlike a `Div`, which is why a button in a column is as
wide as its label.

Precedence, from the lowest:

1. **The built-in default** in the table above.
2. **The theme.** A size set in a part of the theme, such as `Theme.Field.Frame.Width`,
   replaces the default.
3. **The instance's `Styles` or `Style`.** A size set there replaces the theme's.
4. **The sizing method.** A method sets its field on the widget's outer box and wins
   over the default, the theme and `Styles`, whatever the order of the calls. A field
   the method does not set keeps the value from the layers below.

So `Width(ui.Px(120))` and `Styles(ButtonStyle{Root: Style{Width: ui.Px(250)}})` on
one button give 120 px, and `Styles` sets the fields that the method does not. Use the
method for the size of a widget, and the theme for the size of every widget of a kind.

Text fields may shrink in a row to room for six digits, and no further: a fixed width
is their ceiling, not their floor, so a field in a narrow row does not collapse to a
letter's width. A `MinWidth` replaces that minimum, as the "Shrinking and
minimum sizes" section of [layout.md](layout.md#shrinking-and-minimum-sizes) describes. A progress bar or slider in a container with no width of its
own is at least its default minimum, and `MinWidth` replaces that too.

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
| `Tree.RenderRow(fn)` | the icon and label of each row; the chevron stays | the `TreeNode` and a `PartState` |
| `Tree.RenderLabel(fn)` | the text of each row; the icon stays (not used with `RenderRow`) | the `TreeNode` and a `PartState` |
| `DataColumn.Cell` (in `DataGrid`) | the content of each cell of the column; the cell keeps its text, focus and selection | the row's data and nothing else (the content is hidden from assistive technology, unless the column is `Interactive`, when its controls are exposed) |
| `DataColumn.Editor` (in `DataGrid`) | the text field of an edit, replaced by the editor it returns (a Select, a NumberInput, a CheckBox); the cell keeps its frame and the message popover | the row's data, the pending value, and the `change`, `commit` and `cancel` functions |
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
  a selected row has a fill, and an invalid number input has a thicker frame and a
  message. These are defaults, so an override can change them. Keep one non-color cue
  in your styles.
- **No state changes the weight of the text.** Hover, press, focus, selection, the
  highlighted option and the sorted column keep the text's weight, so the built-in
  looks never make text bold on their own. Set a weight yourself in a part's style
  (`FontWeight`) if you want one. Under high contrast the cue is the system's: a
  selected tab keeps its indicator line, and an app cannot clear the line's color or
  change its height. A highlighted option and a selected row keep their Highlight fill,
  and a chosen option keeps its check mark. An invalid number input keeps its message.
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
`NumberInput`. In `ui`:

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
7. **Document it.** Add the widget's parts to the table below and to its section in the topic docs
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
  `<scope>/panel:<value>` (with `KeepMounted`). The `tab:` and `panel` prefixes keep a tab
  and the panel disjoint, so a value that formats as "panel" cannot collide with the
  default panel.

## Widgets and their parts

Every built-in widget with a look of its own is listed, with its parts, variants and
render slots. A row marked `planned` would be a widget that is not yet styled; none
is at this release. The drift test in `ui/parts_test.go` checks this table against the
parts that the code applies.

| Widget | Parts | Variants | Render slots | Status |
| --- | --- | --- | --- | --- |
| `Tabs` | `List`, `Tab`, `TabHover`, `TabSelected`, `TabDisabled`, `TabFocusVisible`, `Indicator`, `Panel`, `DropLine` | `TabsUnderline`, `TabsSegmented` | `RenderTab` | done |
| `FormField` | `Root`, `Label`, `InlineLabel`, `Required`, `Description`, `Error` | none | none | done |
| `Form` | `Summary`, `SummaryTitle`, `SummaryItem`, `SummaryItemHover`, `SummaryItemFocusVisible` | none | none | done |
| `Select` | `Trigger`, `TriggerHover`, `TriggerFocusVisible`, `TriggerDisabled`, `TriggerInvalid`, `Value`, `Placeholder`, `Chevron`, `Popup`, `Option`, `OptionHighlighted`, `OptionSelected`, `OptionDisabled` | none | `RenderValue`, `RenderOption` | done |
| `Combobox` | `Field`, `Popup`, `Option`, `OptionHighlighted`, `OptionDisabled`, `Match`, `Empty`, `Loading`, `Error` | none | `RenderOption` | done |
| `DatePicker` | `Frame`, `FrameHover`, `FrameFocusWithin`, `FrameDisabled`, `FrameInvalid`, `Text`, `Placeholder`, `Selection`, `CalendarButton`, `Message`, `Popup`, `Header`, `NavButton`, `WeekdayLabel`, `Day`, `DayHover`, `DaySelected`, `DayToday`, `DayDisabled`, `DayOutsideMonth`, `DayFocusVisible` | none | none | done |
| `TimePicker` | `Frame`, `FrameHover`, `FrameFocusWithin`, `FrameDisabled`, `FrameInvalid`, `Text`, `Placeholder`, `Selection`, `Button`, `Message`, `Popup`, `Option`, `OptionHighlighted`, `OptionSelected` | none | none | done |
| `DateRangePicker` | `Frame`, `FrameHover`, `FrameFocusWithin`, `FrameDisabled`, `FrameInvalid`, `Text`, `Placeholder`, `Selection`, `CalendarButton`, `Message`, `Popup`, `Header`, `NavButton`, `WeekdayLabel`, `Day`, `DayHover`, `DaySelected`, `DayInRange`, `DayToday`, `DayDisabled`, `DayOutsideMonth`, `DayFocusVisible` | none | none | done |
| `Calendar` | `Root`, `RootDisabled`, `Header`, `NavButton`, `WeekdayLabel`, `Day`, `DayHover`, `DaySelected`, `DayInRange`, `DayToday`, `DayDisabled`, `DayOutsideMonth`, `DayFocusVisible` | none | none | done |
| `Slider` | `Track`, `TrackFocusVisible`, `Rail`, `Fill`, `Thumb`, `ThumbHover`, `ThumbPressed`, `ValueText` | none | none (see above) | done |
| `ColorPicker` | `Trigger`, `TriggerHover`, `TriggerFocusVisible`, `TriggerDisabled`, `Swatch`, `Popup`, `Area`, `AreaFocusVisible`, `Thumb`, `Slider`, `Preset`, `PresetFocusVisible` | none | none | done |
| `Split` | `Divider`, `DividerHover`, `DividerActive`, `DividerFocusVisible`, `Grip`, `Pane` | none | none | done |
| `Dock` | `Group`, `Strip`, `Tab`, `TabHover`, `TabSelected`, `TabFocusVisible`, `Indicator`, `Panel`, `Preview` | none | none | done |
| `NumberInput` | `Frame`, `FrameHover`, `FrameFocusWithin`, `FrameDisabled`, `FrameInvalid`, `Text`, `Selection`, `Message`, `SpinButtons`, `SpinButton`, `SpinButtonHover`, `SpinButtonPressed`, `SpinButtonFocusVisible`, `SpinButtonDisabled` | `SpinInline`, `SpinStacked` | none | done |
| `OTPInput` | `Root`, `Group`, `Box`, `BoxFilled`, `BoxFocused`, `BoxDisabled`, `BoxInvalid`, `Text`, `Dot` | `Masked()`, `Alphanumeric()`, `Groups(...)` | none | done |
| `Button` | `Root`, `RootHover`, `RootPressed`, `RootDisabled`, `RootFocusVisible`, `Label`, `Icon` | `Primary`, `Secondary`, `Ghost`, `Danger` (style per variant in `Theme.Buttons`) | none | done |
| `TextInput` | `Frame`, `FrameHover`, `FrameFocusWithin`, `FrameDisabled`, `FrameReadOnly`, `FrameInvalid`, `Text`, `Placeholder`, `Selection`, `Leading`, `Trailing`, `Button`, `ButtonHover` (see "The text frame") | `Masked()`, `Revealable()`, `Clearable()` | `Leading`, `Trailing` (elements) | done |
| `TextArea` | `Frame`, `FrameHover`, `FrameFocusWithin`, `FrameDisabled`, `FrameReadOnly`, `FrameInvalid`, `Text`, `Placeholder`, `Selection`, `Leading`, `Trailing` (see "The text frame") | none | `Leading`, `Trailing` (elements) | done |
| `Checkbox`, `CheckboxTri` | `Control`, `ControlFocusVisible`, `ControlDisabled`, `Box`, `BoxHover`, `BoxChecked`, `BoxMixed`, `BoxDisabled`, `BoxInvalid`, `Mark`, `Dash`, `Label`, `LabelDisabled` | none | none | done |
| `Switch` | `Track`, `TrackChecked`, `TrackHover`, `TrackFocusVisible`, `TrackDisabled`, `Knob`, `KnobChecked`, `KnobDisabled` | none | none | done |
| `Toggle` | `Root`, `RootChecked`, `RootHover`, `RootPressed`, `RootDisabled`, `RootFocusVisible`, `Label`, `Icon` | `Ghost`, `Secondary` (the looks of those buttons) | none | done |
| `ToggleGroup` | `Group`, `GroupInvalid`, `Item`, `ItemSelected`, `ItemHover`, `ItemPressed`, `ItemFocusVisible`, `ItemDisabled`, `Label`, `Icon` | none | none | done |
| `Rating` | `Root`, `RootFocusVisible`, `RootDisabled`, `Star`, `StarChecked`, `StarHover`, `StarDisabled` | none | none | done |
| `RadioGroup` | `Group`, `Radio`, `RadioFocusVisible`, `RadioDisabled`, `Ring`, `RingChecked`, `RingDisabled`, `RingInvalid`, `Dot`, `DotDisabled`, `Label`, `LabelDisabled` | none | none | done |
| `Collapsible`, `Accordion` | `Section`, `Header`, `HeaderHover`, `HeaderOpen`, `HeaderFocusVisible`, `HeaderDisabled`, `Chevron`, `Content` | none | `RenderHeader` | done |
| `Tree` | `Row`, `RowHover`, `RowSelected`, `Chevron`, `Icon`, `Label`, `TreeFocusVisible` | none | `RenderRow`, `RenderLabel` | done |
| `DataGrid` | `Grid`, `GridFocusVisible`, `Header`, `HeaderCell`, `HeaderCellHover`, `HeaderCellSorted`, `SortIndicator`, `ResizeHandle`, `Row`, `RowHover`, `RowSelected`, `RowAlternate`, `Cell`, `CellFocused`, `Empty`, `CellEditing`, `Editor`, `EditorMessage` | `DataGridComfortable`, `DataGridCompact`, `Striped()` | `DataColumn.Cell`, `DataColumn.RenderHeader` | done |
| `DropdownMenu`, `ContextMenu` | `Panel`, `Item`, `ItemHighlighted`, `ItemDisabled`, `Separator`, `SectionLabel`, `Shortcut`, `Check`, `SubmenuChevron` | none | none | done |
| `MenuBar` | `BarItem`, `BarItemHover`, `BarItemFocusVisible`, `BarItemOpen` | none | none | done |
| `Modal` | `Scrim`, `Panel`, `Title`, `Body`, `Footer` | none | none | done |
| `Drawer` | `Scrim`, `Panel`, `Title`, `Body`, `Footer` | `Side` (`DrawerRight`, `DrawerLeft`, `DrawerTop`, `DrawerBottom`) | none | done |
| `Popover` | `Panel` | `Placement` | none | done |
| `HoverCard` | `Panel` | `Placement` | none | done |
| `CommandPalette` | `Panel`, `Input`, `List`, `Item`, `ItemHighlighted`, `Label`, `Match`, `Keys`, `Empty` | none | none | done |
| `ProgressBar` | `Track`, `Fill`, `Label` | `Indeterminate()` (the sliding segment is `Fill`) | none | done |
| `ProgressRing` | `Ring`, `Dot` | sizes (`RingSmall`, `RingMedium`, `RingLarge`) | none | done |
| `Badge` | `Badge`, `Label`, `Dot` | `Neutral`, `Accent`, `Success`, `Warning`, `Danger` (style per variant in `Theme.Badges`) | none | done |
| `Toast` | `Container`, `Icon`, `Text`, `Action`, `Close` | `Info`, `Success`, `Warning`, `Error` (style per kind in `Theme.Toasts`; placement with `Toaster.Placement`) | none | done |
| `Alert` | `Alert`, `Icon`, `Title`, `Description`, `Close` | `Info`, `Success`, `Warning`, `Danger` (style per variant in `Theme.Alerts`) | none | done |
| `Tooltip` | `Panel`, `Text` | none | none | done |
| `Avatar` | `Avatar`, `Initials` | none | none | done |
| `Tag` | `Tag`, `Label`, `Remove` | none | none | done |
| `Kbd` | `Kbd`, `Key` | none | none | done |
| `DescriptionList` | `DescriptionList`, `Item`, `Term`, `Value` | `Vertical()`, `Bordered()`, `Columns(n)` | none | done |
| `GroupBox` | `GroupBox`, `Header`, `Title`, `Description`, `Content` | `Fill()` | none | done |
| `Skeleton` | `Bone` | none | none | done |
| `Sidebar` | `Pane`, `Item`, `ItemHover`, `ItemSelected`, `ItemFocusVisible`, `ItemDisabled`, `Indicator`, `Heading`, `Toggle` | none | `Header`, `Footer` (elements) | done |
| `Breadcrumbs` | `Bar`, `Item`, `ItemHover`, `Current`, `Separator`, `Overflow` | none | none | done |
| `Pagination` | `Bar`, `Page`, `PageHover`, `PageCurrent`, `Nav`, `Ellipsis` | none | none | done |
| `Stepper` | `Stepper`, `Step`, `StepHover`, `StepFocusVisible`, `StepDisabled`, `Indicator`, `IndicatorComplete`, `IndicatorCurrent`, `Title`, `Description`, `Connector`, `ConnectorComplete` | `Vertical()` | none | done |
| `StatusBar` | `Bar`, `Start`, `End` | none | `Start`, `End` (elements) | done |
| `Toolbar` | `Bar`, `Separator` | none | none | done |
| `Empty` | `Empty`, `Icon`, `Title`, `Description`, `Action` | none | none | done |
| `Carousel` | `Root`, `RootDisabled`, `Viewport`, `Slide`, `Controls`, `NavButton`, `PlayPause`, `Dots`, `Dot`, `DotCurrent`, `DotHover`, `DotFocusVisible`, `DotDisabled` | none | none | done |

## Examples

A theme-level change, an instance override, a variant, and a render slot, from the
demo's Components page (Customizing):

```go
appTheme() // ui.DarkTheme(), with a 3 px line under underline tabs (see "The app's theme")

// One tab list, with a green line and a green selected label.
ui.Tabs(section, tabs...).Styles(ui.TabsStyle{
	Indicator:   ui.Style{Background: t.Success},
	TabSelected: ui.Style{TextColor: t.Success},
})

// A segmented tab list for a filter.
ui.Tabs(filter, tabs...).Variant(ui.TabsSegmented)

// A number input with stacked spin buttons.
ui.NumberInput(hour, 0, 23, 1).Label("Hour").SpinButtons(ui.SpinStacked)

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
- The spin buttons' fill is the button's own box, inset by the gap. A SpinButtons or
  SpinButton style that sets its padding, margin or size changes the fill and the hit
  target with it, so keep those as the variant sets them.
- The stacked column is beside the text, so a centered value is centered in the room
  left of the column, not in the whole field. The field's default width scales with
  the Text size, as the inline field's does. Set another width with `NumberInput.Width`
  when the value needs it.
- The caret of a text field is not a part (see "The text frame"). Its width and blink
  follow the system settings.
- The Slider has no render slot for the thumb (see above).
- The Split has no render slot: its panes are the app's own elements, and the divider has no content of its own. Its bar is 1 px wide by default, in a 9 px hit area that the style does not change, so a `Divider` style sets the bar's thickness and not the hit area.
- The size, the limits and the collapse of a pane are set on the `ui.Pane` (`Size`, `SizeSignal`, `Min`, `Max`, `Collapsible`, `Collapsed`), not in a style. The `Pane` part is the box that holds the pane's content, and it clips the content by default.
- The Split's grip is a pill by default, not dots. `Split(...).Grip(ui.SplitGripOnHover)` shows it only while the divider is hovered, dragged or focused, and `ui.SplitGripHidden` never shows it; a `Grip` style with `Clear: ui.ClearBackground` removes its color in every mode.
- The Dock has no render slot for its tabs or groups: its panels are the app's own content, from `DockPanel.Content`. Its dividers are the `Theme.Split`, and the `Preview` of an edge drop is the strip along that edge of the dock.
- The Tabs have no render slot for the panel: the panel is the app's own content.
- The list of a select is at least as wide as the field, whatever the field's width.
  Its `Popup` part can set a larger minimum width, and the `Trigger` part can set the
  field's width (the sizing method wins; see "Sizing").
- The Tree has no `RowFocused` part. The keyboard focus ring is drawn on the tree
  itself (`TreeFocusVisible`), because a row cannot see whether the tree has focus.
- The Tree's `Indent` (pixels per level) is a size on `TreeStyle`, not a part.
- `CalendarRange` draws the parts of `Calendar` (`Calendar.Root` and the others), and
  only a range draws `DayInRange`. The calendar of a `DatePicker` or a
  `DateRangePicker` is that picker's own (its `Popup`, `Day` and the others), which
  `Theme.Calendar` does not style. A calendar outlines the day the keys have reached
  (`DayFocusVisible`) only while it has keyboard focus, as the picker's open calendar
  always does.
- A Switch draws no text, so its label position is the app's. There is no label
  position option.
- A Carousel's previous, next and play buttons are `IconButton`s: `NavButton` and
  `PlayPause` are the boxes around them, and the buttons themselves take the theme's
  `Button`. The slide comes
  in with a fixed motion, which reduced motion turns off; it is not a part.
- A Rating's `StarHover` is not paint-only: it applies to every star up to the one
  under the pointer, so the rating is built again as the pointer moves. Its stars
  are a fixed 20 px icon, and a `Star` style sets their box and color, not the
  icon's size.
- A ToggleGroup sets the outer corners of its first and last `Item` from the
  `Group`'s radius and border, and the line between two items as a 1 px left border
  of the second, so an `Item` style's `Radius` applies to the inner corners only.
- A progress ring has sizes (`RingSmall`, `RingMedium`, `RingLarge`), but no arc
  variant: its dots are the only drawing.
- The menus have no render slot for their items. `DropdownMenu` takes `Styles`; a
  `ContextMenu` takes the theme's `Menu` only, as it has no instance to style.
- A Modal has no size variants: its panel is 420 px wide by default, and `Width`,
  `MinWidth` or `MaxWidth` on the modal sets another size (see "Sizing").
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
