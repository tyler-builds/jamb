# Theming

Themes, the predefined themes, the style parts of the built-in widgets, and the system settings that an app follows, including high contrast.

## Styling

Every built-in widget with a look of its own has named parts. Each part is a typed
`ui.Style`, grouped in a style struct for its widget: `ui.TabsStyle`,
`ui.SelectStyle`, `ui.SliderStyle`, `ui.FieldStyle` (shared by `TextInput`,
`TextArea` and a number input), `ui.NumberInputStyle`, `ui.ButtonStyle`,
`ui.CheckboxStyle`, `ui.RadioGroupStyle`, `ui.SwitchStyle`, `ui.CollapsibleStyle`, `ui.SplitStyle`,
`ui.TreeStyle`, `ui.MenuStyle`, `ui.MenuBarStyle`, `ui.ModalStyle`,
`ui.ProgressBarStyle`, `ui.ProgressRingStyle`, `ui.BadgeStyle`, `ui.ToastStyle` and
`ui.TooltipStyle`. Set a part for every widget of its kind in the theme (for
example `Theme.Tabs`, `Theme.Button`, `Theme.Buttons.Danger` for one variant,
`Theme.Toast`, `Theme.Tooltip`), or for one widget with `.Styles(...)`. The instance
wins over the theme, and the theme over the built-in defaults, which follow the
theme's colors. States are parts too (`TabSelected`, `OptionHighlighted`,
`RootHover`; each struct's doc comment lists its parts). Some widgets have
variants (`Tabs.Variant(ui.TabsSegmented)`, `NumberInput.SpinButtons(ui.SpinStacked)`,
`Button.Variant(ui.ButtonDanger)`, `Badge.Variant(ui.BadgeSuccess)`, `Toaster.Placement`),
and render slots (`Tabs.RenderTab`, `Select.RenderValue`, `Select.RenderOption`,
`Collapsible.RenderHeader`, `Tree.RenderRow`, `Tree.RenderLabel`,
`TextInput.Leading`, `TextInput.Trailing`) draw one part with your own content while
the widget keeps its keyboard, focus, selection and accessible names. Under high
contrast the parts use the system colors, and text scale and reduced motion still
apply.

```go
ui.Tabs(view, tabs...).Variant(ui.TabsSegmented).Styles(ui.TabsStyle{
	TabSelected: ui.Style{Background: t.Success},
})
```

To turn a default off (no fill, no border, square corners, no padding, no
shadow), set the part's `Clear` field, for example `ui.Style{Clear: ui.ClearBorder}`.
The parts, the precedence, the states, the guardrails, and how to add styling to a
new widget are in [docs/styling.md](styling.md).

## Reusable styles

Reusable styles are ordinary values. Zero-valued fields mean "unset":

```go
var Card = ui.Style{Padding: ui.All(16), Radius: 12, Background: ui.Hex("#18181b")}
ui.Div().Style(Card)
```

## Themes

Theming: `ctx.Theme()` returns semantic colors, and `ctx.SetTheme(ui.LightTheme())`
re-renders everything. `ui.DarkTheme()` and `ui.LightTheme()` return a new copy of
a built-in theme on each call.

A window follows Windows' app mode (Settings > Personalization > Colors > Choose
your app mode) unless it pins a theme: it uses `app.Window.DarkTheme` in dark mode
and `app.Window.LightTheme` in light mode (the built-in themes by default), switches
live when the setting changes, and its native title bar follows. `app.Window.Theme`
or `ctx.SetTheme` pins one theme instead, and `ctx.SystemDark()` reports the mode:

```go
app.Window{Root: root}                                   // follows Windows
app.Window{LightTheme: myLight, DarkTheme: myDark, ...}  // your pair, follows Windows
app.Window{Theme: ui.DarkTheme(), ...}                   // always dark
```

High contrast is automatic. When Windows high contrast is on (Settings, Accessibility,
Contrast themes), the runtime uses `ui.HighContrastTheme`, a theme built from the
system colors (`GetSysColor`): window and text colors for surfaces and text,
Highlight and HighlightText for selection and the primary action, and no tints,
gradients or shadows. Icons take the text color, and a focus ring stays. The app
keeps its own theme: `ctx.AppTheme()` (or `rt.AppTheme()`) is what `SetTheme` set,
and `ctx.Theme().HighContrast` or `ctx.HighContrast()` tells the app that the
high contrast theme is in use, for content that picks its own colors. There is no
opt-out. Details are in [accessibility design](accessibility-design-phases-d-e.md#phase-d-what-changed-from-this-design).

Contrast of the built-in tokens is checked by `ui/contrast_test.go` against the
WCAG 2 ratios of each pair the built-in widgets use, in both themes.

## Predefined themes

Predefined themes: `ui.Themes()` lists the built-in Dark and Light themes, then 37
themes converted from gpui-component (Apache-2.0), in 22 families. `ui.NewTheme(name)`
returns a new copy of one of them by its exact name, which is the `Theme.Name` of the copy.
Each theme also has syntax colors for a code editor (`Theme.Syntax`, keyed by the tree-sitter
capture name) and its editor colors (`Theme.CodeEditor`).

```go
latte, _ := ui.NewTheme("Catppuccin Latte")
mocha, _ := ui.NewTheme("Catppuccin Mocha")
app.Window{LightTheme: latte, DarkTheme: mocha, Root: root} // one family, following Windows
ctx.SetTheme(mocha)                                          // pins one theme instead
```

The families, and their variants, are:

- Adventure: Adventure, Adventure Time
- Alduin: Alduin
- Asciinema: Asciinema
- Aurora: Aurora Light
- Ayu: Ayu Light, Ayu Dark
- Catppuccin: Catppuccin Latte, Catppuccin Frappe, Catppuccin Macchiato, Catppuccin Mocha
- Everforest: Everforest Light, Everforest Dark
- Fahrenheit: Fahrenheit
- Flexoki: Flexoki Light, Flexoki Dark
- Gruvbox: Gruvbox Light, Gruvbox Dark
- Harper: Harper
- Hybrid: Hybrid Light, Hybrid Dark
- Jellybeans: Jellybeans
- Kibble: Kibble
- macOS Classic: macOS Classic Light, macOS Classic Dark
- Matrix: Matrix
- Mellifluous: Mellifluous Light, Mellifluous Dark
- Molokai: Molokai Light, Molokai Dark
- Solarized: Solarized Light, Solarized Dark
- Spaceduck: Spaceduck
- Tokyo: Tokyo Night, Tokyo Storm, Tokyo Moon
- Twilight: Twilight

Each converted theme credits its scheme and author in `ThemeInfo`. Some colors are
moved in lightness so that text meets 4.5:1 and control borders and the focus ring meet 3:1;
the moves are listed in `third_party/gpui-component-themes/README.md`.

## System settings

jamb reads four Windows settings and applies them to every window (the app needs
no code):

- **Text size** (Settings, Accessibility, Text size, 100 to 225%, read from
  `HKCU\Software\Microsoft\Accessibility\TextScaleFactor`). It scales text only:
  font sizes and line heights. The zoom and the display scale work as before.
  Heights of controls are minimums that grow with their content. Apps with fixed
  sizes read `ctx.TextScale()` (percent) to scale them. See
  [Microsoft's guidance on text scaling](https://learn.microsoft.com/en-us/windows/apps/design/accessibility/text-size).
- **High contrast** (`SystemParametersInfo` with `SPI_GETHIGHCONTRAST`). See
  Themes above.
- **Reduced motion** (`SPI_GETCLIENTAREAANIMATION`, Animation effects). See
  Animation above.
- **Text cursor.** The caret of a text field blinks at the system's blink time
  (`GetCaretBlinkTime`; a blink time of infinity keeps it solid) and is as thick as
  the system's cursor thickness (`SPI_GETCARETWIDTH`) times 1.5 logical pixels.
  In a window that is not the active window the caret is hidden and does not
  blink, as a native text field does; it returns when the window is activated.

The settings are read at start and again on `WM_SETTINGCHANGE` and
`WM_SYSCOLORCHANGE`, so a change shows without a restart. jamb reads them and
never writes them.

For testing, four variables simulate the settings for one run, and change no system
setting. Real settings are used in normal use; a variable replaces its setting only
when it is set:

```
set JAMB_HIGH_CONTRAST=1        (or "white" for the white scheme, 0 for off)
set JAMB_TEXT_SCALE=225         (100 to 225)
set JAMB_REDUCED_MOTION=1       (0 for off)
set JAMB_DARK=1                 (the dark app mode; 0 for the light one)
go run ./examples/demo
```

The black and white schemes are representative values
(`ui.HighContrastBlack`, `ui.HighContrastWhite`), not read from a system.
