package ui

// HighContrastTheme returns the theme that the runtime uses while Windows high
// contrast is on. It is built from the system colors only: solid colors, no
// gradients, no shadows and no tints, so that text, focus and control borders
// stay readable (Microsoft's guidance is at
// https://learn.microsoft.com/en-us/windows/apps/design/accessibility/high-contrast-themes).
// Text is WindowText, and selection and the primary action are Highlight with
// HighlightText. Status colors are WindowText too, so a status is shown by its
// text and icon, not by its color. Radius, FontSize and the widget parts (the
// Theme fields for each widget kind, such as Tabs, Button and Toast) are taken
// from base; the parts keep only their sizes and spacing under high contrast (see
// plainStyle).
func HighContrastTheme(c SystemColors, base *Theme) *Theme {
	if c.isZero() {
		c = HighContrastBlack()
	}
	t := &Theme{
		Name:          "high contrast",
		HighContrast:  true,
		Dark:          (c.Window.R+c.Window.G+c.Window.B)/3 < 0.5,
		Background:    c.Window,
		Sidebar:       c.Window,
		Surface:       c.Window,
		SurfaceHover:  c.Window,
		SurfaceActive: c.Highlight,
		Border:        c.WindowText,
		BorderStrong:  c.WindowText,
		Text:          c.WindowText,
		TextMuted:     c.WindowText,
		TextSubtle:    c.WindowText,
		Accent:        c.Highlight,
		AccentHover:   c.Highlight,
		AccentActive:  c.Highlight,
		AccentText:    c.HighlightText,
		Link:          c.HotLight,
		Disabled:      c.GrayText,
		AccentSoft:    c.Window,
		Success:       c.WindowText,
		Warning:       c.WindowText,
		Danger:        c.WindowText,
		Scrollbar:     c.WindowText,
		FocusRing:     c.Highlight,
		Scrim:         c.Window,
		Radius:        base.Radius,
		FontSize:      base.FontSize,
		Tabs:          base.Tabs,
		Select:        base.Select,
		Slider:        base.Slider,
		Split:         base.Split,
		Field:         base.Field,
		NumberInput:   base.NumberInput,
		Button:        base.Button,
		Buttons:       base.Buttons,
		Checkbox:      base.Checkbox,
		RadioGroup:    base.RadioGroup,
		Switch:        base.Switch,
		Collapsible:   base.Collapsible,
		Tree:          base.Tree,
		DataGrid:      base.DataGrid,
		Menu:          base.Menu,
		MenuBar:       base.MenuBar,
		Modal:         base.Modal,
		FormField:     base.FormField,
		Form:          base.Form,
		ProgressBar:   base.ProgressBar,
		ProgressRing:  base.ProgressRing,
		Badge:         base.Badge,
		Badges:        base.Badges,
		Toast:         base.Toast,
		Toasts:        base.Toasts,
		Tooltip:       base.Tooltip,
		Avatar:        base.Avatar,
		Tag:           base.Tag,
		Skeleton:      base.Skeleton,
		SidebarNav:    base.SidebarNav,
		Breadcrumbs:   base.Breadcrumbs,
		Pagination:    base.Pagination,

		Combobox:        base.Combobox,
		DatePicker:      base.DatePicker,
		DateRangePicker: base.DateRangePicker,
		TimePicker:      base.TimePicker,
		ColorPicker:     base.ColorPicker,
		Drawer:          base.Drawer,
		Popover:         base.Popover,
	}
	return t
}

// HighContrastBlack is a representative high contrast scheme: white text on
// black. It is used when the system colors are not given, and by the dev
// override JAMB_HIGH_CONTRAST (see the app package). It is not read from a
// system.
func HighContrastBlack() SystemColors {
	black, white := Hex("#000000"), Hex("#ffffff")
	return SystemColors{
		Window: black, WindowText: white,
		Highlight: Hex("#1aebff"), HighlightText: black,
		HotLight: Hex("#ffff00"), GrayText: Hex("#a0a0a0"),
		ButtonFace: black, ButtonText: white,
		ButtonShadow: Hex("#808080"), WindowFrame: white,
		CaptionText: white, ActiveCaption: black,
		Menu: black, MenuText: white, InfoBack: black,
	}
}

// HighContrastWhite is a representative high contrast scheme: black text on
// white. It is used by the dev override JAMB_HIGH_CONTRAST=white. It is not
// read from a system.
func HighContrastWhite() SystemColors {
	black, white := Hex("#000000"), Hex("#ffffff")
	return SystemColors{
		Window: white, WindowText: black,
		Highlight: Hex("#0000ff"), HighlightText: white,
		HotLight: Hex("#0000ff"), GrayText: Hex("#6d6d6d"),
		ButtonFace: white, ButtonText: black,
		ButtonShadow: Hex("#a0a0a0"), WindowFrame: black,
		CaptionText: black, ActiveCaption: white,
		Menu: white, MenuText: black, InfoBack: white,
	}
}

// applyTheme makes the effective theme the app's theme, or the high contrast
// theme built from the app's theme's size and the system colors.
func (rt *Runtime) applyTheme() {
	if rt.themes != nil {
		rt.appTheme = rt.themes[0]
		if rt.sys.Dark {
			rt.appTheme = rt.themes[1]
		}
	}
	rt.theme = rt.appTheme
	if rt.sys.HighContrast {
		rt.theme = HighContrastTheme(rt.sys.Colors, rt.appTheme)
	}
}
