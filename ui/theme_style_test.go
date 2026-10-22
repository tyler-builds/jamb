package ui

import "testing"

// widgetStyle and otherStyle stand for the style types of two widgets outside ui.
type widgetStyle struct{ Box Style }

type otherStyle struct{ Box Style }

// A value stored under a type is read back under that type only, and replacing it
// keeps one value.
func TestThemeStyleSetAndGet(t *testing.T) {
	th := DarkTheme()
	SetThemeStyle(th, widgetStyle{Box: Style{Gap: 3}})
	SetThemeStyle(th, widgetStyle{Box: Style{Gap: 4}})
	if s, ok := ThemeStyle[widgetStyle](th); !ok || s.Box.Gap != 4 {
		t.Fatalf("ThemeStyle[widgetStyle] = %+v, %v; want the last value set", s, ok)
	}
	if _, ok := ThemeStyle[otherStyle](th); ok {
		t.Fatal("ThemeStyle[otherStyle] found a value stored under another type")
	}
	if len(th.styles) != 1 {
		t.Fatalf("the theme holds %d styles, want 1", len(th.styles))
	}
}

// With nothing stored, or no theme, ThemeStyle returns the zero value and false. A
// nil stored under an interface type is found, and read back as nil.
func TestThemeStyleAbsent(t *testing.T) {
	if s, ok := ThemeStyle[widgetStyle](LightTheme()); ok || s.Box.Gap != 0 {
		t.Fatalf("ThemeStyle on a theme without styles = %+v, %v", s, ok)
	}
	if _, ok := ThemeStyle[widgetStyle](nil); ok {
		t.Fatal("ThemeStyle on a nil theme found a value")
	}
	th := DarkTheme()
	SetThemeStyle[any](th, nil)
	if s, ok := ThemeStyle[any](th); !ok || s != nil {
		t.Fatalf("ThemeStyle[any] after storing nil = %v, %v; want nil, true", s, ok)
	}
}

// A copy of a theme, by Clone, by a struct copy or by HighContrastTheme, keeps what
// was stored before the copy, and a later store on either side does not reach the
// other. The built-in themes are not changed by a store on their copies.
func TestThemeStyleCopiesAreIsolated(t *testing.T) {
	base := DarkTheme()
	SetThemeStyle(base, widgetStyle{Box: Style{Gap: 1}})
	clone := base.Clone()
	plain := *base
	hc := HighContrastTheme(SystemColors{}, base)
	for name, c := range map[string]*Theme{"Clone": clone, "struct copy": &plain, "HighContrastTheme": hc} {
		if s, ok := ThemeStyle[widgetStyle](c); !ok || s.Box.Gap != 1 {
			t.Errorf("%s: ThemeStyle = %+v, %v; want the base's value", name, s, ok)
		}
	}

	SetThemeStyle(clone, widgetStyle{Box: Style{Gap: 2}})
	SetThemeStyle(&plain, otherStyle{Box: Style{Gap: 5}})
	SetThemeStyle(base, widgetStyle{Box: Style{Gap: 3}})
	if s, _ := ThemeStyle[widgetStyle](clone); s.Box.Gap != 2 {
		t.Errorf("the clone's style = %+v, want its own", s)
	}
	if s, _ := ThemeStyle[widgetStyle](hc); s.Box.Gap != 1 {
		t.Errorf("the high contrast theme's style = %+v, want the one it was built with", s)
	}
	if s, _ := ThemeStyle[widgetStyle](&plain); s.Box.Gap != 1 {
		t.Errorf("the struct copy's style = %+v, want the one it was copied with", s)
	}
	if _, ok := ThemeStyle[otherStyle](base); ok {
		t.Error("a store on a struct copy reached the original")
	}
	if s, _ := ThemeStyle[widgetStyle](base); s.Box.Gap != 3 {
		t.Errorf("the base's style = %+v, want its own", s)
	}
	if _, ok := ThemeStyle[widgetStyle](DarkTheme()); ok {
		t.Error("a store on a copy changed a built-in theme")
	}
	if _, ok := ThemeStyle[widgetStyle](LightTheme()); ok {
		t.Error("a store on a copy changed a built-in theme")
	}
	mocha, _ := NewTheme("Catppuccin Mocha")
	if _, ok := ThemeStyle[widgetStyle](mocha); ok {
		t.Error("a predefined theme has a style stored on another theme")
	}
}
