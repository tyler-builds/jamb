package ui

import "maps"

// Theme is a set of semantic colors and metrics used by built-in widgets and
// available to applications through Context.Theme.
type Theme struct {
	// Name is the name of the theme.
	Name string
	// Dark reports whether the theme is a dark theme.
	Dark bool
	// HighContrast is true for the high contrast theme, which HighContrastTheme
	// builds from the system colors. Apps can read it to choose their own colors.
	HighContrast bool

	Background Color // window background
	Panel      Color // secondary panels
	Surface    Color // cards, inputs
	// SurfaceHover is the surface color under the pointer.
	SurfaceHover Color
	// SurfaceActive is the surface color while it is pressed.
	SurfaceActive Color
	// Border is the color of the borders of the controls.
	Border Color
	// BorderStrong is the stronger border color, for a control that needs more contrast.
	BorderStrong Color

	// Text is the main text color.
	Text Color
	// TextMuted is the color of secondary text.
	TextMuted Color
	// TextSubtle is the color of the least important text.
	TextSubtle Color

	// Accent is the accent color, for primary buttons and selections.
	Accent Color
	Link   Color // hyperlink text; a separate token so that Accent can fill buttons
	// Disabled is the color of disabled text, icons and borders. Under high
	// contrast it is GrayText; in the normal themes disabled controls are dimmed
	// with Opacity instead, so it equals TextSubtle there.
	Disabled Color
	// AccentHover is the accent color under the pointer.
	AccentHover Color
	// AccentActive is the accent color while it is pressed.
	AccentActive Color
	AccentText   Color // text on Accent
	AccentSoft   Color // tinted backgrounds (selection, badges)

	// Success is the color of success states.
	Success Color
	// Warning is the color of warning states.
	Warning Color
	// Danger is the color of error and dangerous states.
	Danger Color

	// Scrollbar is the color of scroll bars.
	Scrollbar Color
	// FocusRing is the color of the ring that shows keyboard focus.
	FocusRing Color
	Scrim     Color // the dimmed backdrop behind a Modal

	// Radius is the default corner radius of the controls, in logical pixels.
	Radius float32
	// FontSize is the default text size, in logical pixels.
	FontSize float32

	// Syntax is the look of each syntax capture of a code editor, keyed by the
	// tree-sitter capture name (see SyntaxStyle). Under high contrast it keeps the
	// same names, with system colors.
	Syntax map[string]SyntaxStyle
	// CodeEditor is the colors of a code editor.
	CodeEditor CodeEditorColors

	// The parts of the built-in widgets, for every widget of that kind in the
	// app. They merge over the built-in defaults, which are made from the colors
	// above, and an instance's Styles merge over them. Under high contrast only
	// their sizes, spacing, fonts, radius and border widths apply (see
	// docs/styling.md).
	Tabs TabsStyle
	// Select is the look of every select box, including its list.
	Select SelectStyle
	// Combobox is the look of every combo box: its text frame (Field), its list and
	// its rows.
	Combobox ComboboxStyle
	// DatePicker and TimePicker are the look of every date picker (with its calendar) and
	// every time picker.
	DatePicker DatePickerStyle
	// TimePicker is the look of every time picker.
	TimePicker TimePickerStyle
	// Slider is the look of every slider.
	Slider SliderStyle
	// Split is the look of every split, including its dividers and grips.
	Split SplitStyle
	// Dock is the look of every dock: its tab groups, tabs, panels and drop preview.
	Dock DockStyle
	// Field is the text frame of every TextInput and TextArea, and of every
	// NumberInput's frame, before the NumberInput parts below.
	Field FieldStyle
	// NumberInput is the look of the parts of every number input, beyond its text frame.
	NumberInput NumberInputStyle
	// Button is the look of every button, whatever its variant. Buttons is the
	// look of each variant, and merges over Button.
	Button ButtonStyle
	// Buttons is the look of each variant of button, and merges over Button.
	Buttons ButtonVariants
	// Checkbox is the look of every check box, including CheckboxTri.
	Checkbox CheckboxStyle
	// RadioGroup is the look of every radio group.
	RadioGroup RadioGroupStyle
	// Switch is the look of every switch.
	Switch SwitchStyle
	// Collapsible is the look of every collapsible section, including the
	// sections of an Accordion.
	Collapsible CollapsibleStyle
	// Tree is the look of every tree.
	Tree TreeStyle
	// DataGrid is the look of every data grid.
	DataGrid DataGridStyle
	// Menu is the look of every menu: dropdown, context and the menus of a MenuBar.
	Menu MenuStyle
	// MenuBar is the look of every menu bar.
	MenuBar MenuBarStyle
	// Modal is the look of every modal dialog, Drawer of every drawer, and
	// Popover of every popover's panel.
	Modal ModalStyle
	// Drawer is the look of every drawer.
	Drawer DrawerStyle
	// Popover is the look of the panel of every popover.
	Popover PopoverStyle
	// FormField is the look of every field (its label, help text and error), and Form
	// is the look of every form's error summary.
	FormField FormFieldStyle
	// Form is the look of every form's error summary.
	Form FormStyle
	// ProgressBar is the look of every progress bar, determinate or not.
	ProgressBar ProgressBarStyle
	// ProgressRing is the look of every progress ring.
	ProgressRing ProgressRingStyle
	// Badge is the look of every badge, whatever its variant. Badges is the look
	// of each variant, and merges over Badge.
	Badge BadgeStyle
	// Badges is the look of each badge variant, and merges over Badge.
	Badges BadgeVariants
	// Toast is the look of every toast, whatever its kind. Toasts is the look of
	// each kind, and merges over Toast.
	Toast ToastStyle
	// Toasts is the look of each toast kind, and merges over Toast.
	Toasts ToastVariants
	// Tooltip is the look of every tooltip.
	Tooltip TooltipStyle
	// Avatar is the look of every avatar.
	Avatar AvatarStyle
	// Tag is the look of every tag.
	Tag TagStyle
	// Skeleton is the look of every skeleton placeholder.
	Skeleton SkeletonStyle
	// Sidebar is the look of every sidebar. Panel is the background color of the
	// theme's secondary panels, which the sidebar and other parts also use.
	Sidebar SidebarStyle
	// Breadcrumbs is the look of every breadcrumb trail.
	Breadcrumbs BreadcrumbsStyle
	// Pagination is the look of every pagination control.
	Pagination PaginationStyle
	// DateRangePicker is the look of every date range picker and its calendar. It is not
	// DatePicker: a range has its own parts, such as DayInRange.
	DateRangePicker DateRangePickerStyle
	// ColorPicker is the look of every color picker: its trigger, its panel and the
	// sliders of the panel.
	ColorPicker ColorPickerStyle

	// styles is the values that SetThemeStyle stores, keyed by their type (see
	// themeStyleKey). The map is never changed in place: SetThemeStyle stores a new
	// one, so copies of the theme can share it.
	styles map[any]any
}

// darkTheme and lightTheme are the built-in themes. DarkTheme and LightTheme
// return copies, so that changing a theme never changes it for another window.
var darkTheme = &Theme{
	Name:          "Dark",
	Dark:          true,
	Background:    Hex("#0c0c0e"),
	Panel:         Hex("#111113"),
	Surface:       Hex("#18181b"),
	SurfaceHover:  Hex("#1f1f23"),
	SurfaceActive: Hex("#27272a"),
	Border:        Hex("#26262b"),
	BorderStrong:  Hex("#66666d"),
	Text:          Hex("#f4f4f5"),
	TextMuted:     Hex("#a1a1aa"),
	TextSubtle:    Hex("#8e8e96"),
	Accent:        Hex("#6265ed"),
	Link:          Hex("#a5b4fc"), // 4.5:1 or more on every dark surface
	Disabled:      Hex("#8e8e96"), // = TextSubtle
	AccentHover:   Hex("#5b5ee7"),
	AccentActive:  Hex("#5457e0"),
	AccentText:    Hex("#ffffff"),
	AccentSoft:    Hex("#6366f126"),
	Success:       Hex("#22c55e"),
	Warning:       Hex("#f59e0b"),
	Danger:        Hex("#d62828"),
	Scrollbar:     Hex("#ffffff"),
	FocusRing:     Hex("#818cf8"),
	Scrim:         Hex("#00000099"),
	Radius:        8,
	FontSize:      14,
	Syntax:        darkSyntax,
	CodeEditor:    darkEditor,
}

var lightTheme = &Theme{
	Name:          "Light",
	Background:    Hex("#fafafa"),
	Panel:         Hex("#f4f4f5"),
	Surface:       Hex("#ffffff"),
	SurfaceHover:  Hex("#f4f4f5"),
	SurfaceActive: Hex("#e4e4e7"),
	Border:        Hex("#e4e4e7"),
	BorderStrong:  Hex("#8a8a92"),
	Text:          Hex("#18181b"),
	TextMuted:     Hex("#52525b"),
	TextSubtle:    Hex("#62626a"),
	Accent:        Hex("#4f46e5"),
	Link:          Hex("#4338ca"), // 4.5:1 or more on every light surface
	Disabled:      Hex("#62626a"), // = TextSubtle
	AccentHover:   Hex("#5b53f0"),
	AccentActive:  Hex("#4338ca"),
	AccentText:    Hex("#ffffff"),
	AccentSoft:    Hex("#4f46e51f"),
	Success:       Hex("#16a34a"),
	Warning:       Hex("#d97706"),
	Danger:        Hex("#d12a2a"),
	Scrollbar:     Hex("#000000"),
	FocusRing:     Hex("#6366f1"),
	Scrim:         Hex("#18181b59"),
	Radius:        8,
	FontSize:      14,
	Syntax:        lightSyntax,
	CodeEditor:    lightEditor,
}

// DarkTheme returns the built-in dark theme: a neutral zinc palette with an
// indigo accent. Each call returns a new copy, which the caller may change.
func DarkTheme() *Theme { return darkTheme.Clone() }

// LightTheme returns the built-in light theme, the counterpart of DarkTheme.
// Each call returns a new copy, which the caller may change.
func LightTheme() *Theme { return lightTheme.Clone() }

// Clone returns a copy of the theme, to change for an app without changing the
// built-in theme it was copied from. The copy shares the Shadows slices of its
// part styles, so change a part by assigning a new style to its field, and do
// not append to or change those slices in place. The copy has its own Syntax
// map. It has the styles that SetThemeStyle stored on t; setting one on the copy
// does not change t, nor the other way round.
func (t *Theme) Clone() *Theme {
	c := *t
	c.Syntax = maps.Clone(t.Syntax)
	return &c
}
