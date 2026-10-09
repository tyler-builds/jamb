package ui

// Theme is a set of semantic colors and metrics used by built-in widgets and
// available to applications through Context.Theme.
type Theme struct {
	Name string
	Dark bool

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

	Accent       Color
	AccentHover  Color
	AccentActive Color
	AccentText   Color // text on Accent
	AccentSoft   Color // tinted backgrounds (selection, badges)

	Success Color
	Warning Color
	Danger  Color

	Scrollbar Color
	FocusRing Color

	Radius   float32
	FontSize float32
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
	BorderStrong:  Hex("#3a3a41"),
	Text:          Hex("#f4f4f5"),
	TextMuted:     Hex("#a1a1aa"),
	TextSubtle:    Hex("#71717a"),
	Accent:        Hex("#6366f1"),
	AccentHover:   Hex("#7376f4"),
	AccentActive:  Hex("#5457e0"),
	AccentText:    Hex("#ffffff"),
	AccentSoft:    Hex("#6366f126"),
	Success:       Hex("#22c55e"),
	Warning:       Hex("#f59e0b"),
	Danger:        Hex("#ef4444"),
	Scrollbar:     Hex("#ffffff33"),
	FocusRing:     Hex("#818cf8"),
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
	BorderStrong:  Hex("#d4d4d8"),
	Text:          Hex("#18181b"),
	TextMuted:     Hex("#52525b"),
	TextSubtle:    Hex("#a1a1aa"),
	Accent:        Hex("#4f46e5"),
	AccentHover:   Hex("#5b53f0"),
	AccentActive:  Hex("#4338ca"),
	AccentText:    Hex("#ffffff"),
	AccentSoft:    Hex("#4f46e51f"),
	Success:       Hex("#16a34a"),
	Warning:       Hex("#d97706"),
	Danger:        Hex("#dc2626"),
	Scrollbar:     Hex("#0000003d"),
	FocusRing:     Hex("#6366f1"),
	Radius:        8,
	FontSize:      14,
}
