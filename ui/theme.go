package ui

// Theme is a set of semantic colors and metrics used by built-in widgets and
// available to applications through Context.Theme.
type Theme struct {
	Name string
	Dark bool
	// HighContrast is true for the high contrast theme, which HighContrastTheme
	// builds from the system colors. Apps can read it to choose their own colors.
	HighContrast bool

	Background    Color // window background
	Sidebar       Color // secondary panels
	Surface       Color // cards, inputs
	SurfaceHover  Color
	SurfaceActive Color
	Border        Color
	BorderStrong  Color

	Text       Color
	TextMuted  Color
	TextSubtle Color

	Accent Color
	Link   Color // hyperlink text; a separate token so that Accent can fill buttons
	// Disabled is the color of disabled text, icons and borders. Under high
	// contrast it is GrayText; in the normal themes disabled controls are dimmed
	// with Opacity instead, so it equals TextSubtle there.
	Disabled     Color
	AccentHover  Color
	AccentActive Color
	AccentText   Color // text on Accent
	AccentSoft   Color // tinted backgrounds (selection, badges)

	Success Color
	Warning Color
	Danger  Color

	Scrollbar Color
	FocusRing Color
	Scrim     Color // the dimmed backdrop behind a Modal

	Radius   float32
	FontSize float32

	// The parts of the built-in widgets, for every widget of that kind in the
	// app. They merge over the built-in defaults, which are made from the colors
	// above, and an instance's Styles merge over them. Under high contrast only
	// their sizes, spacing, fonts, radius and border widths apply (see
	// docs/styling.md).
	Tabs   TabsStyle
	Select SelectStyle
	// Combobox is the look of every combo box: its text frame (Field), its list and
	// its rows.
	Combobox ComboboxStyle
	// DatePicker and TimePicker are the look of every date picker (with its calendar) and
	// every time picker.
	DatePicker DatePickerStyle
	TimePicker TimePickerStyle
	Slider     SliderStyle
	// Field is the text frame of every TextInput and TextArea, and of every
	// NumberField's frame, before the NumberField parts below.
	Field       FieldStyle
	NumberField NumberFieldStyle
	// Button is the look of every button, whatever its variant. Buttons is the
	// look of each variant, and merges over Button.
	Button  ButtonStyle
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
	// TreeView is the look of every tree.
	TreeView TreeStyle
	// DataGrid is the look of every data grid.
	DataGrid DataGridStyle
	// Menu is the look of every menu: dropdown, context and the menus of a MenuBar.
	Menu MenuStyle
	// MenuBar is the look of every menu bar.
	MenuBar MenuBarStyle
	// Modal is the look of every modal dialog.
	Modal ModalStyle
	// FormField is the look of every field (its label, help text and error), and Form
	// is the look of every form's error summary.
	FormField FormFieldStyle
	Form      FormStyle
	// ProgressBar is the look of every progress bar, determinate or not.
	ProgressBar ProgressBarStyle
	// ProgressRing is the look of every progress ring.
	ProgressRing ProgressRingStyle
	// Badge is the look of every badge, whatever its variant. Badges is the look
	// of each variant, and merges over Badge.
	Badge  BadgeStyle
	Badges BadgeVariants
	// Toast is the look of every toast, whatever its kind. Toasts is the look of
	// each kind, and merges over Toast.
	Toast  ToastStyle
	Toasts ToastVariants
	// Tooltip is the look of every tooltip.
	Tooltip TooltipStyle
}

// DarkTheme is a neutral zinc palette with an indigo accent.
var DarkTheme = &Theme{
	Name:          "dark",
	Dark:          true,
	Background:    Hex("#0c0c0e"),
	Sidebar:       Hex("#111113"),
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
}

// LightTheme is the light counterpart of DarkTheme.
var LightTheme = &Theme{
	Name:          "light",
	Background:    Hex("#fafafa"),
	Sidebar:       Hex("#f4f4f5"),
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
}

// Clone returns a copy of the theme, to change for an app without changing the
// built-in theme it was copied from. The copy shares the Shadows slices of its
// part styles, so change a part by assigning a new style to its field, and do
// not append to or change those slices in place.
func (t *Theme) Clone() *Theme {
	c := *t
	return &c
}
