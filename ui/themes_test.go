package ui

import (
	"sort"
	"strings"
	"testing"
)

// Every predefined theme is checked against the WCAG 2 criteria that the
// built-in themes meet (contrast_test.go), for the pairs its widgets draw, and
// for the syntax captures and the editor. The converted themes, which are the
// ones from GPUI Kit, also meet the non-text criteria of their borders and
// focus ring, which the generator enforces (see internal/themegen). Border is
// decorative and is not checked.

func TestThemesListsBuiltInsThenConverted(t *testing.T) {
	infos := Themes()
	// Dark and Light, then 37 variants in 22 upstream files.
	if len(infos) != 39 {
		t.Fatalf("Themes lists %d themes, want 39", len(infos))
	}
	if infos[0].Name != "Dark" || infos[1].Name != "Light" {
		t.Errorf("Themes starts with %q and %q, want Dark and Light", infos[0].Name, infos[1].Name)
	}
	seen := map[string]bool{}
	for _, info := range infos {
		if seen[info.Name] {
			t.Errorf("Themes lists %q twice", info.Name)
		}
		seen[info.Name] = true
		if info.Family == "" {
			t.Errorf("%s has no family", info.Name)
		}
		th, ok := NewTheme(info.Name)
		if !ok {
			t.Errorf("NewTheme(%q) reports an unknown name", info.Name)
			continue
		}
		if th.Dark != info.Dark {
			t.Errorf("%s: Dark is %t, want %t", info.Name, th.Dark, info.Dark)
		}
		if info.Family != "jamb" && th.Name != info.Name {
			t.Errorf("%s: Theme.Name is %q", info.Name, th.Name)
		}
		if len(th.Syntax) == 0 {
			t.Errorf("%s has no syntax captures", info.Name)
		}
	}
}

func TestNewThemeUnknownName(t *testing.T) {
	for _, name := range []string{"", "Nope", "dark", "catppuccin mocha"} {
		if th, ok := NewTheme(name); ok || th != nil {
			t.Errorf("NewTheme(%q) returned a theme=%t and ok=%t, want nil and false", name, th != nil, ok)
		}
	}
}

// An app saves the Name of the theme it uses and restores it with NewTheme, so
// that name must be one NewTheme takes, for every predefined theme.
func TestNewThemeTakesTheNameOfEveryPredefinedTheme(t *testing.T) {
	for _, info := range Themes() {
		th, ok := NewTheme(info.Name)
		if !ok {
			t.Fatalf("NewTheme(%q) found no theme, though Themes lists it", info.Name)
		}
		rt := newTestRuntime(func(ctx *Context) Element { return Div() }, 200, 100, 1)
		rt.SetTheme(th)
		saved := rt.Theme().Name
		if saved != info.Name {
			t.Errorf("the Name of %q is %q, want the name in Themes", info.Name, saved)
		}
		if back, ok := NewTheme(saved); !ok || back.Name != saved {
			t.Errorf("NewTheme(%q) of the saved name returned ok=%t, want the theme back", saved, ok)
		}
	}
}

func TestNewThemeReturnsCopies(t *testing.T) {
	a, _ := NewTheme("Catppuccin Mocha")
	b, _ := NewTheme("Catppuccin Mocha")
	a.Background = Hex("#000000")
	a.Syntax["keyword"] = SyntaxStyle{}
	if b.Background == a.Background {
		t.Errorf("a change to one copy changed another copy's Background")
	}
	if b.Syntax["keyword"] == (SyntaxStyle{}) {
		t.Errorf("a change to one copy's Syntax changed another copy")
	}
}

func TestCloneCopiesSyntaxAndEditor(t *testing.T) {
	base := DarkTheme()
	c := base.Clone()
	c.Syntax["keyword"] = SyntaxStyle{Bold: true}
	c.Syntax["added"] = SyntaxStyle{}
	c.CodeEditor.Selection = Hex("#ffffff")
	if base.Syntax["keyword"] == c.Syntax["keyword"] {
		t.Errorf("Clone shares the Syntax map: a change to the copy changed the original")
	}
	if _, ok := base.Syntax["added"]; ok {
		t.Errorf("Clone shares the Syntax map: a new capture of the copy appears in the original")
	}
	if base.CodeEditor.Selection == c.CodeEditor.Selection {
		t.Errorf("a change to the copy's Editor changed the original")
	}
	if _, ok := darkTheme.Syntax["added"]; ok {
		t.Errorf("a change to a copy changed the built-in theme")
	}
}

func TestPredefinedThemeContrast(t *testing.T) {
	for _, info := range Themes() {
		th, _ := NewTheme(info.Name)
		t.Run(info.Name, func(t *testing.T) {
			checkPredefinedContrast(t, th, info.Family != "jamb")
		})
	}
}

// checkPredefinedContrast fails for each pair below the ratio its criterion asks
// for. converted adds the non-text pairs of the converted themes.
func checkPredefinedContrast(t *testing.T, th *Theme, converted bool) {
	type themePair struct {
		name string
		fg   Color
		bg   Color
		min  float64
	}
	surfaces := []struct {
		name string
		c    Color
	}{
		{"Background", th.Background},
		{"Panel", th.Panel},
		{"Surface", th.Surface},
		{"SurfaceHover", th.SurfaceHover},
		{"SurfaceActive", th.SurfaceActive},
	}
	var ps []themePair
	for _, s := range surfaces {
		ps = append(ps,
			themePair{"Text on " + s.name, th.Text, s.c, minText},
			themePair{"TextMuted on " + s.name, th.TextMuted, s.c, minText},
			themePair{"TextSubtle on " + s.name, th.TextSubtle, s.c, minText},
			themePair{"Link on " + s.name, th.Link, s.c, minText},
		)
	}
	ps = append(ps,
		themePair{"AccentText on Accent", th.AccentText, th.Accent, minText},
		themePair{"AccentText on AccentHover", th.AccentText, th.AccentHover, minText},
		themePair{"AccentText on AccentActive", th.AccentText, th.AccentActive, minText},
		// widgets.go draws the danger label in #ffffff, on the fill and its states.
		themePair{"white on Danger", Hex("#ffffff"), th.Danger, minText},
		themePair{"white on Danger hover", Hex("#ffffff"), th.Danger.Lighten(0.08), minText},
		themePair{"white on Danger pressed", Hex("#ffffff"), th.Danger.Darken(0.1), minText},
		themePair{"Editor.Foreground on Editor.Background", th.CodeEditor.Foreground, th.CodeEditor.Background, minText},
	)
	names := make([]string, 0, len(th.Syntax))
	for name := range th.Syntax {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := th.Syntax[name]
		ps = append(ps,
			themePair{"syntax " + name + " on Background", s.Color, th.Background, minText},
			themePair{"syntax " + name + " on Editor.Background", s.Color, th.CodeEditor.Background, minText},
		)
	}
	if converted {
		ps = append(ps,
			themePair{"BorderStrong on Background", th.BorderStrong, th.Background, minNonText},
			themePair{"BorderStrong on Panel", th.BorderStrong, th.Panel, minNonText},
			themePair{"BorderStrong on Surface", th.BorderStrong, th.Surface, minNonText},
			themePair{"FocusRing on Background", th.FocusRing, th.Background, minNonText},
			themePair{"FocusRing on Panel", th.FocusRing, th.Panel, minNonText},
			themePair{"FocusRing on Surface", th.FocusRing, th.Surface, minNonText},
		)
	}
	for _, p := range ps {
		if r := contrastRatio(p.fg, p.bg); r < p.min {
			t.Errorf("%s: contrast %.2f, want at least %.1f", p.name, r, p.min)
		}
	}
}

func TestHighContrastKeepsSyntaxCaptures(t *testing.T) {
	for _, info := range Themes() {
		th, _ := NewTheme(info.Name)
		for _, sys := range []SystemColors{HighContrastBlack(), HighContrastWhite()} {
			hc := HighContrastTheme(sys, th)
			if len(hc.Syntax) != len(th.Syntax) {
				t.Errorf("%s: high contrast has %d syntax captures, want %d", info.Name, len(hc.Syntax), len(th.Syntax))
			}
			for name, s := range th.Syntax {
				got, ok := hc.Syntax[name]
				if !ok {
					t.Errorf("%s: high contrast drops syntax capture %q", info.Name, name)
					continue
				}
				want := sys.WindowText
				if strings.HasPrefix(name, "keyword") {
					want = sys.Highlight
				}
				if got.Color != want || got.Italic != s.Italic || got.Bold != s.Bold {
					t.Errorf("%s: high contrast capture %q is %+v, want color %v with the font styles of %+v", info.Name, name, got, want, s)
				}
			}
			if hc.CodeEditor.Foreground != sys.WindowText || hc.CodeEditor.Background != sys.Window {
				t.Errorf("%s: high contrast editor is not in system colors", info.Name)
			}
		}
	}
}
