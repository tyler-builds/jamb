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

Phase 1 covers four widgets. The full table is at the end of this document.

| Widget | Parts |
| --- | --- |
| `Tabs` | `List`, `Tab`, `TabHover`, `TabSelected`, `TabDisabled`, `TabFocusVisible`, `Indicator`, `Panel` |
| `Select` | `Trigger`, `TriggerHover`, `TriggerFocusVisible`, `TriggerDisabled`, `Value`, `Placeholder`, `Chevron`, `Popup`, `Option`, `OptionHighlighted`, `OptionSelected`, `OptionDisabled` |
| `Slider` | `Track`, `TrackFocusVisible`, `Rail`, `Fill`, `Thumb`, `ThumbHover`, `ThumbPressed`, `ValueText` |
| `NumberField` | `Field`, `FieldInvalid`, `FieldDisabled`, `SpinButtons`, `SpinButton`, `SpinButtonHover`, `SpinButtonPressed`, `SpinButtonFocusVisible`, `SpinButtonDisabled` |

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
   `Theme.Tabs`, `Theme.Select`, `Theme.Slider` and `Theme.NumberField`.
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
| `Selected` | the part is the selected one | the part's selected style |
| `Disabled` | the part cannot be used | the part's disabled style |
| `Highlighted` | the option is highlighted in an open list | the option's highlight style |

Hover and focus use the same mechanism as the `Hover` and `FocusVisible` styles of
`ui.Div`, so they are paint-only: they do not relayout the widget. `Selected`,
`Disabled` and `Highlighted` are chosen when the widget is built.

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
  a decrease and an increase button after the field, with a minus and a plus sign.
- `NumberField.SpinButtons(ui.SpinStacked)` draws an up arrow above a down arrow inside
  the field's right edge, as the up-down control of Windows does. Each button is half
  the field's height and at least 16 px wide, the arrows scale with the Text size, and
  the text takes the rest of the field, so it never runs under the buttons. The focus
  of a stacked button is an inner outline, since a ring would fall outside the field's
  clipped edge.

A part's style applies in every variant that draws the part. A variant's own defaults
come first, so `TabSelected` in the segmented variant is the filled segment, and an
override of `TabSelected` changes that segment as well.

## Render slots

A render slot replaces the content of one part with an element of your own, and the
widget keeps everything else: the keyboard, focus, selection, the states, and the
accessible role and name.

| Slot | Replaces | Called with |
| --- | --- | --- |
| `Tabs.RenderTab(fn)` | the icon and label of each tab | the `Tab` and a `PartState` |
| `Select.RenderValue(fn)` | the label of the chosen option in the field | the option and a `PartState` |
| `Select.RenderOption(fn)` | the label of each option in the open list | the option and a `PartState` |

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
must still leave a non-color cue for sighted users, such as the weight or the line of
the built-in look.

`Select.RenderValue` is called only while an option is chosen. With no choice the
placeholder is drawn, as it is by default.

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
  weight, a line or a mark: a selected tab is semibold with a line, a selected option
  has a check mark, and an invalid number field has a thicker frame and a message.
  These are defaults, so an override can change them. Keep one non-color cue in your
  styles.
- **A selected tab and a highlighted option keep their weight.** The selected tab's
  text is at least semibold, and so is a highlighted option's, whatever the styles
  say. With the indicator, the fill or the highlight cleared, the weight still shows
  the selection or the keyboard position. A chosen option keeps its check mark, and
  an invalid number field keeps its message: those are not parts, so no style removes
  them.
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

Phase 1 widgets are filled in. The other built-in widgets are listed as planned for
phase 2: they keep their current look and their existing `Style` methods until then.

| Widget | Parts | Variants | Render slots | Status |
| --- | --- | --- | --- | --- |
| `Tabs` | `List`, `Tab`, `TabHover`, `TabSelected`, `TabDisabled`, `TabFocusVisible`, `Indicator`, `Panel` | `TabsUnderline`, `TabsSegmented` | `RenderTab` | phase 1 |
| `Select` | `Trigger`, `TriggerHover`, `TriggerFocusVisible`, `TriggerDisabled`, `Value`, `Placeholder`, `Chevron`, `Popup`, `Option`, `OptionHighlighted`, `OptionSelected`, `OptionDisabled` | none | `RenderValue`, `RenderOption` | phase 1 |
| `Slider` | `Track`, `TrackFocusVisible`, `Rail`, `Fill`, `Thumb`, `ThumbHover`, `ThumbPressed`, `ValueText` | none | none (see above) | phase 1 |
| `NumberField` | `Field`, `FieldInvalid`, `FieldDisabled`, `SpinButtons`, `SpinButton`, `SpinButtonHover`, `SpinButtonPressed`, `SpinButtonFocusVisible`, `SpinButtonDisabled` | `SpinInline`, `SpinStacked` | none | phase 1 |
| `Button` | variants only (`Primary`, `Secondary`, `Ghost`, `Danger`), `Style` | the variants | none | planned: parts |
| `TextInput`, `TextArea` | frame via `Style`, `Borderless` | none | none | planned: `Field`, `FieldFocusWithin` |
| `Checkbox`, `CheckboxTri` | `Box`, `Mark`, `Label` | none | none | planned |
| `Switch` | `Track`, `Knob`, states | none | none | planned |
| `RadioGroup` | `Radio`, `Mark`, `Label` | none | none | planned |
| `Collapsible`, `Accordion` | `Header`, `Chevron`, `Content` | none | none | planned |
| `TreeView` | `Row`, `RowSelected`, `Chevron`, `Label` | none | `RenderRow` (candidate) | planned |
| `DropdownMenu`, `ContextMenu`, `MenuBar` | `Popup`, `Item`, `ItemHighlighted`, `ItemDisabled`, `Separator` | none | `RenderItem` (candidate) | planned |
| `Modal` | `Backdrop`, `Panel`, `Header`, `Footer` | none | none | planned |
| `ProgressBar`, `BusyRing` | `Track`, `Fill`, `Label` | sizes | none | planned |
| `Badge` | `Badge` per variant | `Neutral`, `Accent`, `Success`, `Warning`, `Danger` | none | planned |
| `Toast` | `Toast`, `Action`, `Close` | variants | none | planned |

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

## Limits in phase 1

- The stacked spin buttons take the field's minimum height at the Text size setting.
  A Field style that sets a different minimum height does not change their halves,
  so keep the field's height at its default when you use SpinStacked.
- The Slider has no render slot for the thumb (see above).
- The Tabs have no render slot for the panel: the panel is the app's own content.
- `Select.Width` sets the field's width and the list's minimum width together. The
  `Trigger` and `Popup` parts can override either.
- Phase 2 covers the widgets marked "planned" in the table above.
