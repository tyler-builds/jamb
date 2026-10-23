package ui

import (
	"maps"
	"time"
)

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
	// Motion is the timing of the built-in widgets' motion. Its zero value
	// keeps the built-in timings.
	Motion MotionTokens

	// Syntax is the look of each syntax token of a code editor, keyed by the
	// token name (see SyntaxStyle). Under high contrast it keeps the
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
	// RangeSlider is the look of every range slider.
	RangeSlider RangeSliderStyle
	// SplitButton is the look of every split button.
	SplitButton SplitButtonStyle
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
	// Toggle is the look of every toggle.
	Toggle ToggleStyle
	// ToggleGroup is the look of every toggle group.
	ToggleGroup ToggleGroupStyle
	// Rating is the look of every rating.
	Rating RatingStyle
	// Collapsible is the look of every collapsible section, including the
	// sections of an Accordion.
	Collapsible CollapsibleStyle
	// Tree is the look of every tree.
	Tree TreeStyle
	// ListBox is the look of every list box.
	ListBox ListBoxStyle
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
	// AlertDialog is the look of every alert dialog.
	AlertDialog AlertDialogStyle
	// Popover is the look of the panel of every popover.
	Popover PopoverStyle
	// HoverCard is the look of the panel of every hover card.
	HoverCard HoverCardStyle
	// CommandPalette is the look of every command palette.
	CommandPalette CommandPaletteStyle
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
	// Alert is the look of every alert, whatever its variant. Alerts is the look
	// of each variant, and merges over Alert.
	Alert AlertStyle
	// Alerts is the look of each alert variant, and merges over Alert.
	Alerts AlertVariants
	// Tooltip is the look of every tooltip.
	Tooltip TooltipStyle
	// Avatar is the look of every avatar.
	Avatar AvatarStyle
	// Tag is the look of every tag.
	Tag TagStyle
	// GroupBox is the look of every group box, outlined or filled.
	GroupBox GroupBoxStyle
	// DescriptionList is the look of every description list.
	DescriptionList DescriptionListStyle
	// Kbd is the look of every keyboard shortcut drawn as key caps.
	Kbd KbdStyle
	// Skeleton is the look of every skeleton placeholder.
	Skeleton SkeletonStyle
	// Sidebar is the look of every sidebar. Panel is the background color of the
	// theme's secondary panels, which the sidebar and other parts also use.
	Sidebar SidebarStyle
	// Breadcrumbs is the look of every breadcrumb trail.
	Breadcrumbs BreadcrumbsStyle
	// Pagination is the look of every pagination control.
	Pagination PaginationStyle
	// Stepper is the look of every stepper.
	Stepper StepperStyle
	// OTPInput is the look of every one-time code input.
	OTPInput OTPInputStyle
	// DateRangePicker is the look of every date range picker and its calendar. It is not
	// DatePicker: a range has its own parts, such as DayInRange.
	DateRangePicker DateRangePickerStyle
	// Calendar is the look of every calendar on the page, Calendar and CalendarRange. It is
	// not DatePicker: the calendar of a picker is its popup, and its parts are the picker's.
	Calendar CalendarStyle
	// Carousel is the look of every carousel. Its previous and next buttons are
	// IconButtons, which Button styles.
	Carousel CarouselStyle
	// ColorPicker is the look of every color picker: its trigger, its panel and the
	// sliders of the panel.
	ColorPicker ColorPickerStyle
	// StatusBar is the look of every status bar.
	StatusBar StatusBarStyle
	// Toolbar is the look of every toolbar and its separators.
	Toolbar ToolbarStyle
	// Empty is the look of every empty state.
	Empty EmptyStyle

	// styles is the values that SetThemeStyle stores, keyed by their type (see
	// themeStyleKey). The map is never changed in place: SetThemeStyle stores a new
	// one, so copies of the theme can share it.
	styles map[any]any
}

// MotionTokens are the timings of the built-in widgets' motion, for a theme
// to make them faster, slower or springier. A zero field keeps the built-in
// default. Reduced motion still turns the motion off.
type MotionTokens struct {
	// Hover is how long the colors of the built-in controls ease on hover and
	// press: 110 ms by default.
	Hover time.Duration
	// Enter is how long menus, lists, dialogs, popovers and tooltips take to
	// appear. By default menus, lists and dialogs take 140 ms, and popovers,
	// hover cards and tooltips 120 ms.
	Enter time.Duration
	// Exit is how long they take to go: by default 90 ms for menus, lists and
	// dialogs, and 80 ms for popovers, hover cards and tooltips.
	Exit time.Duration
	// Spring, when set, moves the opacity, offset and scale of the built-in
	// controls whose colors ease on hover (as a theme's hover style that lifts
	// or grows them would change them) as this spring. The zero Spring keeps
	// their transitions.
	Spring Spring
}

// hover returns the duration of the built-in controls' hover transitions.
func (m MotionTokens) hover() time.Duration {
	if m.Hover > 0 {
		return m.Hover
	}
	return 110 * time.Millisecond
}

// enter returns the duration of a built-in entrance whose default is def.
func (m MotionTokens) enter(def time.Duration) time.Duration {
	if m.Enter > 0 {
		return m.Enter
	}
	return def
}

// exit returns the duration of a built-in exit whose default is def.
func (m MotionTokens) exit(def time.Duration) time.Duration {
	if m.Exit > 0 {
		return m.Exit
	}
	return def
}

// newDarkTheme and newLightTheme make the built-in themes. They are functions,
// not package-level values, so that the literals cost code rather than data in
// the binary. Each call returns a new theme, so changing a theme never changes
// it for another window.
func newDarkTheme() *Theme {
	return &Theme{
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
		Syntax:        maps.Clone(darkSyntax),
		CodeEditor:    darkEditor,
	}
}

func newLightTheme() *Theme {
	return &Theme{
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
		Syntax:        maps.Clone(lightSyntax),
		CodeEditor:    lightEditor,
	}
}

// DarkTheme returns the built-in dark theme: a neutral zinc palette with an
// indigo accent. Each call returns a new copy, which the caller may change.
func DarkTheme() *Theme { return newDarkTheme() }

// LightTheme returns the built-in light theme, the counterpart of DarkTheme.
// Each call returns a new copy, which the caller may change.
func LightTheme() *Theme { return newLightTheme() }

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
