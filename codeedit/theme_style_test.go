package codeedit

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// resolvedIn returns the parts that the Styles s resolve to in one render with the theme
// th, under high contrast when hc is true, and the theme the render used.
func resolvedIn(s Style, th *ui.Theme, hc bool) (Style, *ui.Theme) {
	var st Style
	var used *ui.Theme
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		st, used = s.resolve(ctx), ctx.Theme()
		return ui.Div()
	}, th, nil)
	if hc {
		rt.SetSystemSettings(ui.SystemSettings{HighContrast: true})
	}
	rt.SetViewport(400, 300, 1)
	rt.Frame(time.Now())
	return st, used
}

// editorTheme returns a theme whose codeedit.Style sets the box's radius and padding, the
// gutter's color and the selection's fill.
func editorTheme() *ui.Theme {
	th := ui.DarkTheme()
	ui.SetThemeStyle(th, Style{
		Box:       ui.Style{Radius: 4, Padding: ui.All(3)},
		Gutter:    ui.Style{TextColor: ui.Hex("#ff0000")},
		Selection: ui.Style{Background: ui.Hex("#00ff00")},
	})
	return th
}

// The theme's codeedit.Style merges over the built-in defaults, field by field, and the
// editor's Styles over it.
func TestThemeStyleMergesBetweenDefaultsAndStyles(t *testing.T) {
	th := editorTheme()
	st, _ := resolvedIn(Style{Box: ui.Style{Radius: 2}}, th, false)
	if st.Box.Radius != 2 {
		t.Errorf("box radius = %v, want 2 from the Styles over the theme's 4", st.Box.Radius)
	}
	if st.Box.Padding != ui.All(3) || st.Gutter.TextColor != ui.Hex("#ff0000") || st.Selection.Background != ui.Hex("#00ff00") {
		t.Errorf("box padding %v, gutter color %v, selection %v; want the theme's", st.Box.Padding, st.Gutter.TextColor, st.Selection.Background)
	}
	if st.Box.Background != th.CodeEditor.Background || st.Box.BorderWidth != 1 || st.Gutter.FontSize != th.FontSize {
		t.Errorf("box background %v, border %v, gutter size %v; want the defaults the theme does not set",
			st.Box.Background, st.Box.BorderWidth, st.Gutter.FontSize)
	}

	plainTheme := ui.DarkTheme()
	if st, _ := resolvedIn(Style{}, plainTheme, false); st.Box.Radius != plainTheme.Radius || st.Gutter.TextColor != plainTheme.CodeEditor.LineNumber {
		t.Errorf("without a theme style: box radius %v, gutter color %v; want the defaults", st.Box.Radius, st.Gutter.TextColor)
	}
}

// Under high contrast the theme's codeedit.Style keeps its sizes and spacing, and its colors
// give way to the system colors, as the colors of the Styles do.
func TestThemeStyleUnderHighContrast(t *testing.T) {
	st, used := resolvedIn(Style{}, editorTheme(), true)
	if !used.HighContrast {
		t.Fatal("the render did not use the high contrast theme")
	}
	if st.Box.Radius != 4 || st.Box.Padding != ui.All(3) {
		t.Errorf("box radius %v, padding %v; want the theme's", st.Box.Radius, st.Box.Padding)
	}
	if st.Gutter.TextColor != used.CodeEditor.LineNumber || st.Selection.Background != used.CodeEditor.Selection {
		t.Errorf("gutter color %v, selection %v; want the system colors, not the theme's", st.Gutter.TextColor, st.Selection.Background)
	}
}

// The inspector shows the theme's layer between the built-in default and the Styles.
func TestEditorThemeStyleLayerIsNamed(t *testing.T) {
	ed := Editor(NewDocument("func main() {}")).Key("code").Styles(Style{Box: ui.Style{Radius: 2}})
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element { return ui.Div().Child(ed) }, editorTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Now())
	tg, ok := uihost.Of(rt).(inspect.Target)
	if !ok {
		t.Fatal("the runtime's host side does not implement inspect.Target")
	}
	tg.Attach(func() {})
	rt.Frame(time.Now())

	box, ok := findRadius(tg, tg.Root(), "2")
	if !ok {
		t.Fatal("no element with a radius of 2 from the instance Styles")
	}
	var radius, padding bool
	for _, f := range tg.Fields(box) {
		switch f.Name {
		case "Radius":
			radius = true
			if len(f.Layers) != 3 || f.Layers[0].Source != inspect.SourceDefault || !f.Layers[0].Overridden ||
				f.Layers[1].Source != inspect.SourceTheme || f.Layers[1].Value != "4" || !f.Layers[1].Overridden {
				t.Errorf("Radius of the editor box = %+v, want the default and the theme's 4 under the Styles' 2", f)
			}
		case "Padding":
			padding = true
			if f.Source != inspect.SourceTheme {
				t.Errorf("Padding of the editor box = %+v, want it from the theme", f)
			}
		}
	}
	if !radius || !padding {
		t.Fatalf("the editor box reports Radius %v and Padding %v", radius, padding)
	}
}
