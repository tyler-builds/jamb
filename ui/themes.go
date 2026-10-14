package ui

//go:generate go run ../internal/themegen -src ../third_party/gpui-component-themes -out themes_gen.go

// ThemeInfo describes one predefined theme, as Themes lists it. The variants of
// one scheme share a Family, such as "Catppuccin" for Catppuccin Latte and
// Catppuccin Mocha, so an app can pick a light and a dark variant for the
// LightTheme and DarkTheme of an app.Window.
type ThemeInfo struct {
	Name   string // the name of the theme, such as "Catppuccin Mocha"; NewTheme takes it
	Family string // the scheme the theme is a variant of, or "jamb" for Dark and Light
	Author string // the author of the scheme as credited upstream; empty for Dark and Light
	URL    string // the upstream address of the scheme; empty for Dark and Light
	Dark   bool   // true for a dark variant
}

// New returns a new copy of the theme that the ThemeInfo names, as NewTheme
// returns it. It returns nil for a ThemeInfo that does not name a predefined
// theme.
func (i ThemeInfo) New() *Theme {
	t, _ := NewTheme(i.Name)
	return t
}

// predefinedTheme is one entry of the predefined themes: its description, and
// the function that returns a new copy of the theme.
type predefinedTheme struct {
	info  ThemeInfo
	build func() *Theme
}

// predefined lists the predefined themes in the order of Themes: Dark and Light,
// then the themes converted from gpui-component (see themes_gen.go).
var predefined = append([]predefinedTheme{
	{info: ThemeInfo{Name: "Dark", Family: "jamb", Dark: true}, build: DarkTheme},
	{info: ThemeInfo{Name: "Light", Family: "jamb"}, build: LightTheme},
}, generatedThemes...)

// Themes returns the predefined themes in a stable order: Dark and Light first,
// then the themes converted from gpui-component, in the order of their upstream
// files and of the variants in each file. The themes are described, not built;
// NewTheme or ThemeInfo.New returns a copy of one.
func Themes() []ThemeInfo {
	out := make([]ThemeInfo, len(predefined))
	for i, p := range predefined {
		out[i] = p.info
	}
	return out
}

// NewTheme returns a new copy of the predefined theme with the given name from
// Themes, and true. It returns nil and false for a name that is not in Themes.
// The name must match exactly. Each call returns a new copy, which the caller
// may change. The Name field of the copies of Dark and Light stays "dark" and
// "light", as DarkTheme and LightTheme return them.
func NewTheme(name string) (*Theme, bool) {
	for _, p := range predefined {
		if p.info.Name == name {
			return p.build(), true
		}
	}
	return nil, false
}
