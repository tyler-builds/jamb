package ui_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// docTabs is a tab list of two closable documents with an overflow menu and a "+"
// suffix, as an editor's document tabs are, styled with st. Each panel is an editor
// with its own key.
func docTabs(st ui.TabsStyle) func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		doc := ui.NewSignal("q1")
		editor := func() ui.Element {
			return ui.Div().Key("editor").Grow(1).Background(ctx.Theme().Surface).Padding(8).Child(ui.Text("SELECT 1;"))
		}
		return ui.Div().Column().Grow(1).Child(ui.Tabs(doc,
			ui.Tab[string]{Value: "q1", Label: "Query1.sql", Icon: icons.File, Closable: true, Panel: editor},
			ui.Tab[string]{Value: "q2", Label: "Query2.sql", Icon: icons.File, Closable: true, Panel: editor},
		).Key("q").Label("Documents").OnClose(func(string) {}).OverflowMenu().Styles(st).
			Suffix(ui.IconButton(icons.Plus).Small().Label("New query").Key("new")))
	}
}

// The space between the tab list and the panel is the Root part's Gap: 12 px by
// default, and any other space, none included, with Styles. It was a fixed 12 px.
func TestTabsRootGapSetsTheSpaceAboveThePanel(t *testing.T) {
	for _, c := range []struct {
		name string
		root ui.Style
		want float32
	}{
		{"default", ui.Style{}, 12},
		{"gap 4", ui.Style{Gap: 4}, 4},
		{"cleared", ui.Style{Clear: ui.ClearGap}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			u := uitest.New(t, docTabs(ui.TabsStyle{Root: c.root}), uitest.Size(480, 200))
			bar, ed := u.Get("q/bar").Bounds(), u.Get("editor").Bounds()
			if gap := ed.Y - (bar.Y + bar.H); math.Abs(float64(gap-c.want)) > 0.5 {
				t.Errorf("the panel is %.1f px under the bar, want %.0f", gap, c.want)
			}
		})
	}
}

// lineRows counts the rows of img, at column x and in the bottom 6 px of r, that are
// not the color of the row at r's top: the lines drawn there.
func lineRows(img *image.RGBA, r ui.Rect, x float32) (n int, last color.Color) {
	bg := img.At(int(x), int(r.Y)+2)
	for y := int(r.Y+r.H) - 6; y < int(r.Y+r.H); y++ {
		if c := img.At(int(x), y); c != bg {
			n++
			last = c
		}
	}
	return n, last
}

// While the bar is shown it draws the line under the tabs, so a border set on the
// Bar is the only line there and does not double up with the strip's, and the
// default line runs under the bar's buttons as well as under the strip.
func TestTabsBarOwnsTheLineUnderTheTabs(t *testing.T) {
	th := ui.DarkTheme()
	mark := ui.Hex("#ff0000")
	for _, c := range []struct {
		name string
		st   ui.TabsStyle
		want color.Color // the color of the line
	}{
		{"default", ui.TabsStyle{}, nil},
		{"bar border", ui.TabsStyle{Bar: ui.Style{BorderEdges: ui.Edges{Bottom: 1}, BorderColor: mark}},
			color.RGBA{255, 0, 0, 255}},
	} {
		t.Run(c.name, func(t *testing.T) {
			u := uitest.New(t, docTabs(c.st), uitest.Size(480, 120), uitest.Theme(th))
			u.Settle()
			img := u.Screenshot()
			bar := u.Get("q/bar").Bounds()
			list := u.ByRole(access.RoleTabList, "Documents").Bounds()
			tab2 := u.ByRole(access.RoleTab, "Query2.sql").Bounds()
			under, past := tab2.X+tab2.W/2, list.X+list.W+2
			if past >= u.ByRole(access.RoleButton, "All tabs").Bounds().X {
				t.Fatalf("no room between the strip (%+v) and the menu button", list)
			}
			nu, cu := lineRows(img, bar, under)
			np, cp := lineRows(img, bar, past)
			if nu != 1 || np != 1 {
				t.Fatalf("%d lines under the strip and %d past it, want one under both", nu, np)
			}
			if cu != cp {
				t.Errorf("the line is %v under the strip and %v past it, want one line", cu, cp)
			}
			if c.want != nil && cu != c.want {
				t.Errorf("the line is %v, want the Bar's border %v", cu, c.want)
			}
		})
	}
}

func centerY(r ui.Rect) float32 { return r.Y + r.H/2 }

// A DropdownMenu is aligned by its parent as its trigger would be: the overflow menu's
// button sits on the middle of the tab bar, as the suffix does, and a dropdown in a
// centered toolbar is centered beside a button. Its wrapper was aligned to the top.
// In a column it keeps the trigger's width.
func TestDropdownMenuIsAlignedByItsParent(t *testing.T) {
	u := uitest.New(t, docTabs(ui.TabsStyle{}), uitest.Size(480, 120))
	// The bar is centered on its content; its bottom border is the line under the tabs.
	tab := u.ByRole(access.RoleTab, "Query1.sql").Bounds()
	menu := u.ByRole(access.RoleButton, "All tabs").Bounds()
	plus := u.Get("new").Bounds()
	if d := centerY(menu) - centerY(tab); math.Abs(float64(d)) > 0.25 {
		t.Errorf("the All tabs button's middle is %.1f px off the tabs' (%+v, tab %+v)", d, menu, tab)
	}
	if d := centerY(menu) - centerY(plus); math.Abs(float64(d)) > 0.25 {
		t.Errorf("the All tabs button's middle is %.1f px off the suffix's", d)
	}

	items := func() []ui.MenuItem { return []ui.MenuItem{{Label: "Grid", OnClick: func() {}}} }
	u2 := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Gap(8).Children(
			ui.Div().Row().Align(ui.Center).Gap(6).Height(ui.Px(48)).Children(
				ui.Button("Execute").Small().Key("exec"),
				ui.DropdownMenu(ui.Button("Results to").Small().Secondary().Key("trigger"), items).Key("dd")),
			ui.DropdownMenu(ui.Button("Column").Small().Key("col-trigger"), items).Key("col-dd"),
		)
	}, uitest.Size(400, 200))
	exec, dd := u2.Get("exec").Bounds(), u2.Get("dd").Bounds()
	if d := centerY(dd) - centerY(exec); math.Abs(float64(d)) > 0.25 {
		t.Errorf("the dropdown's middle is %.1f px off the button beside it (%+v, button %+v)", d, dd, exec)
	}
	col, trig := u2.Get("col-dd").Bounds(), u2.Get("col-trigger").Bounds()
	if math.Abs(float64(col.W-trig.W)) > 0.5 || math.Abs(float64(col.H-trig.H)) > 0.5 {
		t.Errorf("in a column the dropdown is %+v, want its trigger's size %+v", col, trig)
	}
}

// The tabs have a button's sizes: a segmented track is as tall as a button of the
// same size, 34 px by default (it was 36), and Small and Large make the tabs small
// and large as they make a button.
func TestTabsSizesMatchButtons(t *testing.T) {
	view := ui.NewSignal("results")
	tabs := func(key string, v ui.TabsVariant, size string) ui.Element {
		tv := ui.Tabs(view,
			ui.Tab[string]{Value: "results", Label: "Results", Icon: icons.Table},
			ui.Tab[string]{Value: "messages", Label: "Messages"},
		).Variant(v).Label(key)
		switch size {
		case "small":
			tv.Small()
		case "large":
			tv.Large()
		}
		return ui.Div().Row().Child(tv)
	}
	button := func(key, size string) ui.Element {
		b := ui.Button("Execute").Key(key)
		switch size {
		case "small":
			b.Small()
		case "large":
			b.Large()
		}
		return b
	}
	for _, size := range []string{"small", "default", "large"} {
		t.Run(size, func(t *testing.T) {
			u := uitest.New(t, func(ctx *ui.Context) ui.Element {
				return ui.Div().Column().Gap(8).Children(
					ui.Div().Row().Align(ui.Start).Gap(8).Children(tabs("segmented", ui.TabsSegmented, size), button("button", size)),
					ui.Div().Row().Child(tabs("underline", ui.TabsUnderline, size)),
				)
			}, uitest.Size(600, 300))
			want := u.Get("button").Bounds().H
			if got := u.ByRole(access.RoleTabList, "segmented").Bounds().H; math.Abs(float64(got-want)) > 0.5 {
				t.Errorf("the segmented track is %.1f px high, want the button's %.1f", got, want)
			}
			// An underline tab is a button's height and 2 px for its line, as the default
			// tab's 36 px is.
			var tab ui.Rect
			u.Tree().Walk(func(n *access.Node) bool {
				if n.Role == access.RoleTab && n.Name == "Messages" {
					tab = ui.Rect{X: n.Bounds.X, Y: n.Bounds.Y, W: n.Bounds.W, H: n.Bounds.H}
				}
				return true
			})
			if math.Abs(float64(tab.H-(want+2))) > 0.5 {
				t.Errorf("an underline tab is %.1f px high, want the button's %.1f and 2 for the line", tab.H, want)
			}
		})
	}
}
