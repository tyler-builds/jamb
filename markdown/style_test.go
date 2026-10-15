package markdown

import (
	"reflect"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// resolvedIn returns the parts of the view v as one render resolves them in the theme
// th, under high contrast when hc is true, and the theme the render used.
func resolvedIn(v *ViewElement, th *ui.Theme, hc bool) (Style, *ui.Theme) {
	var st Style
	var used *ui.Theme
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		st, used = v.resolveStyle(ctx), ctx.Theme()
		return ui.Div()
	}, th, nil)
	if hc {
		rt.SetSystemSettings(ui.SystemSettings{HighContrast: true})
	}
	rt.SetViewport(400, 300, 1)
	rt.Frame(time.Now())
	return st, used
}

// themedView returns a theme whose markdown.Style sets the first heading's size and
// color and the code block's background, and a view whose Styles set the first
// heading's size and the link's color.
func themedView() (*ui.Theme, *ViewElement) {
	th := ui.DarkTheme()
	ui.SetThemeStyle(th, Style{
		Heading1:  ui.Style{FontSize: 30, TextColor: ui.Hex("#ff0000")},
		CodeBlock: ui.Style{Background: ui.Hex("#00ff00"), Radius: 9},
	})
	return th, View("# Read me").Styles(Style{Heading1: ui.Style{FontSize: 34}, Link: ui.Style{TextColor: ui.Hex("#0000ff")}})
}

// The theme's markdown.Style merges over the built-in defaults, field by field, and
// the view's Styles over it.
func TestStylePrecedence(t *testing.T) {
	th, v := themedView()
	st, _ := resolvedIn(v, th, false)
	if st.Heading1.FontSize != 34 || st.Heading1.TextColor != ui.Hex("#ff0000") || st.Heading1.FontWeight != ui.FontSemibold {
		t.Errorf("Heading1 %+v; want the size of the Styles, the color of the theme and the default weight", st.Heading1)
	}
	if st.CodeBlock.Background != ui.Hex("#00ff00") || st.CodeBlock.Radius != 9 || st.CodeBlock.FontFamily != "mono" {
		t.Errorf("CodeBlock %+v; want the theme's background and radius over the default font", st.CodeBlock)
	}
	if st.Link.TextColor != ui.Hex("#0000ff") {
		t.Errorf("Link color %v, want the Styles'", st.Link.TextColor)
	}
	plain := ui.DarkTheme()
	if st, _ := resolvedIn(v, plain, false); st.CodeBlock.Background != plain.CodeEditor.Background || st.Link.TextColor != ui.Hex("#0000ff") {
		t.Errorf("without a theme style: code background %v, link %v", st.CodeBlock.Background, st.Link.TextColor)
	}
}

// Under high contrast the sizes of the theme's style and of the Styles stay, and the
// colors are the system colors of the high contrast theme.
func TestStyleUnderHighContrast(t *testing.T) {
	th, v := themedView()
	st, used := resolvedIn(v, th, true)
	if !used.HighContrast {
		t.Fatal("the render did not use the high contrast theme")
	}
	if st.Heading1.FontSize != 34 || st.CodeBlock.Radius != 9 {
		t.Errorf("heading size %v, code radius %v; want the sizes of the Styles and the theme", st.Heading1.FontSize, st.CodeBlock.Radius)
	}
	if !st.Heading1.TextColor.IsZero() || st.Link.TextColor != used.Link || st.CodeBlock.Background != used.CodeEditor.Background {
		t.Errorf("heading color %v, link %v, code background %v; want no app colors, and the system's",
			st.Heading1.TextColor, st.Link.TextColor, st.CodeBlock.Background)
	}
	if st.TaskChecked.Background != used.Accent || st.Rule.Background != used.Text || !st.TableHeader.Background.IsZero() {
		t.Errorf("task %v, rule %v, header %v; want the highlight and text colors, and no header fill",
			st.TaskChecked.Background, st.Rule.Background, st.TableHeader.Background)
	}
}

// No part changes the weight of its text for a state: the parts have no state styles,
// and the weights they set are those of their content (headings), not of hover.
func TestDefaultsSetNoStateWeight(t *testing.T) {
	st := defaults(ui.DarkTheme())
	if st.LinkHover.FontWeight != 0 || st.Link.FontWeight != 0 || st.TaskChecked.FontWeight != 0 {
		t.Errorf("link %v, hover %v, done task %v; want no weight", st.Link.FontWeight, st.LinkHover.FontWeight, st.TaskChecked.FontWeight)
	}
}

// Every part is merged: a Style with every part set comes through merge whole.
func TestMergeCoversEveryPart(t *testing.T) {
	var full Style
	v := reflect.ValueOf(&full).Elem()
	for i := range v.NumField() {
		v.Field(i).Set(reflect.ValueOf(ui.Style{Gap: float32(i + 1)}))
	}
	if got := (Style{}).merge(full); !reflect.DeepEqual(got, full) {
		t.Errorf("merge dropped parts: %+v", got)
	}
	if got := full.plain(); !reflect.DeepEqual(got, full) {
		t.Errorf("plain dropped parts: %+v", got)
	}
}

// The inspector names the layers of a part: the built-in default and the theme under
// the Styles.
func TestStyleLayersAreNamed(t *testing.T) {
	th, v := themedView()
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element { return ui.Div().Child(v) }, th, nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Now())
	tg, ok := uihost.Of(rt).(inspect.Target)
	if !ok {
		t.Fatal("the runtime's host side does not implement inspect.Target")
	}
	tg.Attach(func() {})
	rt.Frame(time.Now())
	heading, ok := findDiv(tg, tg.Root(), "Read me")
	if !ok {
		t.Fatal("no element holds the heading")
	}
	for _, f := range tg.Fields(heading) {
		if f.Name != "FontSize" {
			continue
		}
		if f.Source != inspect.SourceInstance || f.Value != "34" || len(f.Layers) != 3 ||
			f.Layers[0].Source != inspect.SourceDefault || f.Layers[1].Source != inspect.SourceTheme || f.Layers[1].Value != "30" {
			t.Fatalf("FontSize of the heading = %+v, want the default and the theme's 30 under the Styles' 34", f)
		}
		return
	}
	t.Fatal("the heading has no FontSize")
}

// findDiv returns the element whose only child is the text element with text s.
func findDiv(tg inspect.Target, id uint64, s string) (uint64, bool) {
	e, ok := tg.Element(id)
	if !ok {
		return 0, false
	}
	if len(e.Children) == 1 {
		if c, ok := tg.Element(e.Children[0]); ok && c.Kind == "text" && c.Text == s {
			return id, true
		}
	}
	for _, c := range e.Children {
		if f, ok := findDiv(tg, c, s); ok {
			return f, true
		}
	}
	return 0, false
}
