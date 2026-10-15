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

// predefinedTheme is one entry of the predefined themes: its description, and
// either the function that returns a new copy of the theme (Dark and Light) or
// the spec it is built from (the themes converted from gpui-component).
type predefinedTheme struct {
	info  ThemeInfo
	build func() *Theme
	spec  *themeSpec
}

// predefined lists the predefined themes in the order of Themes: Dark and Light,
// then the themes converted from gpui-component (see themes_gen.go).
var predefined = func() []predefinedTheme {
	out := []predefinedTheme{
		{info: ThemeInfo{Name: "Dark", Family: "jamb", Dark: true}, build: DarkTheme},
		{info: ThemeInfo{Name: "Light", Family: "jamb"}, build: LightTheme},
	}
	for _, s := range generatedThemes {
		out = append(out, predefinedTheme{info: s.Info, spec: s})
	}
	return out
}()

// themeSpec is a predefined theme as data: the colors it sets over the built-in
// theme of its mode, written as hexadecimal strings that Hex parses (#rrggbb or
// #rrggbbaa). The generated specs in themes_gen.go are kept as data rather than
// code, so that the themes an app does not use cost little in its binary.
type themeSpec struct {
	Info ThemeInfo

	Background, Panel, Surface, SurfaceHover, SurfaceActive string
	Border, BorderStrong                                    string
	Text, TextMuted, TextSubtle                             string
	Accent, Link, Disabled, AccentHover, AccentActive       string
	AccentText, AccentSoft                                  string
	Success, Warning, Danger                                string
	Scrollbar, FocusRing, Scrim                             string

	Syntax     []syntaxSpec
	CodeEditor codeEditorSpec
}

// syntaxSpec is one capture of a themeSpec's Syntax: its name, its color, and
// whether it is italic or bold.
type syntaxSpec struct {
	name, color string
	style       fontStyle
}

// fontStyle marks a syntax capture as italic, bold or both.
type fontStyle uint8

const (
	plain      fontStyle = 0
	italic     fontStyle = 1
	bold       fontStyle = 2
	boldItalic           = italic | bold
)

// codeEditorSpec is the CodeEditorColors of a themeSpec.
type codeEditorSpec struct {
	Background, Foreground, ActiveLine, LineNumber, ActiveLineNumber, Selection string
}

// build returns a new theme from the spec: a copy of the built-in theme of its
// mode, with the spec's colors set.
func (s *themeSpec) build() *Theme {
	t := lightTheme.Clone()
	if s.Info.Dark {
		t = darkTheme.Clone()
	}
	t.Name = s.Info.Name
	for _, f := range []struct {
		dst *Color
		hex string
	}{
		{&t.Background, s.Background}, {&t.Panel, s.Panel}, {&t.Surface, s.Surface},
		{&t.SurfaceHover, s.SurfaceHover}, {&t.SurfaceActive, s.SurfaceActive},
		{&t.Border, s.Border}, {&t.BorderStrong, s.BorderStrong},
		{&t.Text, s.Text}, {&t.TextMuted, s.TextMuted}, {&t.TextSubtle, s.TextSubtle},
		{&t.Accent, s.Accent}, {&t.Link, s.Link}, {&t.Disabled, s.Disabled},
		{&t.AccentHover, s.AccentHover}, {&t.AccentActive, s.AccentActive},
		{&t.AccentText, s.AccentText}, {&t.AccentSoft, s.AccentSoft},
		{&t.Success, s.Success}, {&t.Warning, s.Warning}, {&t.Danger, s.Danger},
		{&t.Scrollbar, s.Scrollbar}, {&t.FocusRing, s.FocusRing}, {&t.Scrim, s.Scrim},
	} {
		*f.dst = Hex(f.hex)
	}
	t.Syntax = make(map[string]SyntaxStyle, len(s.Syntax))
	for _, c := range s.Syntax {
		t.Syntax[c.name] = SyntaxStyle{Color: Hex(c.color), Italic: c.style&italic != 0, Bold: c.style&bold != 0}
	}
	e := s.CodeEditor
	t.CodeEditor = CodeEditorColors{
		Background: Hex(e.Background), Foreground: Hex(e.Foreground), ActiveLine: Hex(e.ActiveLine),
		LineNumber: Hex(e.LineNumber), ActiveLineNumber: Hex(e.ActiveLineNumber), Selection: Hex(e.Selection),
	}
	return t
}

// Themes returns the predefined themes in a stable order: Dark and Light first,
// then the themes converted from gpui-component, in the order of their upstream
// files and of the variants in each file. The themes are described, not built;
// NewTheme returns a copy of one.
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
// may change. The Name of each copy is the name in Themes, so NewTheme(t.Name)
// returns a theme for every predefined theme.
func NewTheme(name string) (*Theme, bool) {
	for _, p := range predefined {
		if p.info.Name == name {
			if p.spec != nil {
				return p.spec.build(), true
			}
			return p.build(), true
		}
	}
	return nil, false
}
