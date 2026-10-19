package ui

import (
	"testing"
	"time"
)

// precedenceCase is one widget whose part's color the three layers of styling set:
// the built-in default, the theme's part, and the instance's Styles. make renders
// the widget with theme th and, when inst is set, the instance's color; it returns
// the rig after the frames the widget needs. style returns the part's style for a
// color c (nil for none), and the read of the part is the color of its background,
// or of its border when border is set. overlay marks a widget with no instance.
type precedenceCase struct {
	name    string
	part    string
	key     string
	border  bool
	overlay bool
	theme   func(th *Theme, c Color)
	make    func(t *testing.T, th *Theme, inst *Color) *accessRig
}

// bgStyle returns a Style with the background c, or nothing when c is nil.
func bgStyle(c *Color) Style {
	if c == nil {
		return Style{}
	}
	return Style{Background: *c}
}

// borderStyle returns a Style with the border color c, or nothing when c is nil.
func borderStyle(c *Color) Style {
	if c == nil {
		return Style{}
	}
	return Style{BorderColor: *c}
}

// partColor returns the background, or the border color, of the first node drawn
// for part (with key, when given) in r.
func partColor(t *testing.T, r *accessRig, part, key string, border bool) Color {
	t.Helper()
	n := partNode(t, r.rt, part, key)
	if border {
		return n.style.BorderColor
	}
	return n.style.Background
}

// TestWidgetPartPrecedence checks, for each phase 2 widget, that a part's color is
// the built-in default with no overrides, the theme's when the theme sets it, and the
// instance's over the theme. A tooltip has no instance, so only the theme is checked.
func TestWidgetPartPrecedence(t *testing.T) {
	themeColor, instColor := Hex("#111213"), Hex("#141516")
	ic := &instColor
	cases := []precedenceCase{
		{name: "Checkbox", part: "Checkbox.Box",
			theme: func(th *Theme, c Color) { th.Checkbox.Box.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Checkbox("Include", NewSignal(false)).Styles(CheckboxStyle{Box: bgStyle(inst)}))
				})
			}},
		{name: "RadioGroup", part: "RadioGroup.Ring",
			theme: func(th *Theme, c Color) { th.RadioGroup.Ring.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(RadioGroup(NewSignal("a"), Option[string]{Value: "a", Label: "A"}).
						Styles(RadioGroupStyle{Ring: bgStyle(inst)}))
				})
			}},
		{name: "Switch", part: "Switch.Track",
			theme: func(th *Theme, c Color) { th.Switch.Track.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Switch("Sync", NewSignal(false)).Styles(SwitchStyle{Track: bgStyle(inst)}))
				})
			}},
		{name: "RangeSlider", part: "RangeSlider.Fill",
			theme: func(th *Theme, c Color) { th.RangeSlider.Fill.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(RangeSlider(NewSignal(2.0), NewSignal(8.0), 0, 10, 1).Label("Levels").
						Styles(RangeSliderStyle{Fill: bgStyle(inst)}))
				})
			}},
		{name: "SplitButton", part: "SplitButton.Main",
			theme: func(th *Theme, c Color) { th.SplitButton.Main.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(SplitButton("Save", nil, nil).Styles(SplitButtonStyle{Main: bgStyle(inst)}))
				})
			}},
		{name: "Toggle", part: "Toggle.Root",
			theme: func(th *Theme, c Color) { th.Toggle.Root.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Toggle("Bold", NewSignal(false)).Styles(ToggleStyle{Root: bgStyle(inst)}))
				})
			}},
		{name: "ToggleGroup", part: "ToggleGroup.Group",
			theme: func(th *Theme, c Color) { th.ToggleGroup.Group.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(ToggleGroup(NewSignal("a"), Option[string]{Value: "a", Label: "A"}).
						Label("Letters").Styles(ToggleGroupStyle{Group: bgStyle(inst)}))
				})
			}},
		{name: "Rating", part: "Rating.Root",
			theme: func(th *Theme, c Color) { th.Rating.Root.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Rating("Score", NewSignal(2)).Styles(RatingStyle{Root: bgStyle(inst)}))
				})
			}},
		{name: "Calendar", part: "Calendar.Root",
			theme: func(th *Theme, c Color) { th.Calendar.Root.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Calendar(NewSignal(time.Time{})).Styles(CalendarStyle{Root: bgStyle(inst)}))
				})
			}},
		{name: "Carousel", part: "Carousel.Root",
			theme: func(th *Theme, c Color) { th.Carousel.Root.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Carousel(NewSignal(0), Text("One"), Text("Two")).
						Styles(CarouselStyle{Root: bgStyle(inst)}))
				})
			}},
		{name: "Collapsible", part: "Collapsible.Section", border: true,
			theme: func(th *Theme, c Color) { th.Collapsible.Section.BorderColor = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Collapsible("Advanced", NewSignal(false), Text("body")).
						Styles(CollapsibleStyle{Section: borderStyle(inst)}))
				})
			}},
		{name: "Tree", part: "Tree.Row",
			theme: func(th *Theme, c Color) { th.Tree.Row.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				nodes := []TreeNode{{Key: "src", Label: "Source"}}
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Tree(nodes, NewSignal("")).Label("Files").Styles(TreeStyle{Row: bgStyle(inst)}))
				})
			}},
		{name: "Menu", part: "Menu.Panel",
			theme: func(th *Theme, c Color) { th.Menu.Panel.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				r := newThemedRig(th, func(ctx *Context) Element {
					return Div().Padding(20).Child(DropdownMenu(Div().Key("trig").Size(Px(200), Px(30)).Focusable(),
						func() []MenuItem { return []MenuItem{{Label: "Open"}} }).Styles(MenuStyle{Panel: bgStyle(inst)}))
				})
				r.clickKey(t, "trig")
				return r
			}},
		{name: "MenuBar", part: "MenuBar.BarItem",
			theme: func(th *Theme, c Color) { th.MenuBar.BarItem.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(MenuBar(TopMenu{Label: "File", Items: func() []MenuItem {
						return []MenuItem{{Label: "Open"}}
					}}).Styles(MenuBarStyle{BarItem: bgStyle(inst)}))
				})
			}},
		{name: "Modal", part: "Modal.Panel",
			theme: func(th *Theme, c Color) { th.Modal.Panel.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Modal("Settings", Text("Body")).Styles(ModalStyle{Panel: bgStyle(inst)}))
				})
			}},
		{name: "CommandPalette", part: "CommandPalette.Panel",
			theme: func(th *Theme, c Color) { th.CommandPalette.Panel.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Size(Full, Full).
						Child(CommandPalette(NewSignal(true)).Styles(CommandPaletteStyle{Panel: bgStyle(inst)}))
				})
			}},
		{name: "HoverCard", part: "HoverCard.Panel",
			theme: func(th *Theme, c Color) { th.HoverCard.Panel.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				r := newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(HoverCard(Button("Dana Lee").Key("t"), func() Element { return Text("Designer") }).
						Styles(HoverCardStyle{Panel: bgStyle(inst)}))
				})
				rc := r.rt.nodeRect(keyNode(t, r.rt, "t"))
				r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
				r.now = r.now.Add(600 * time.Millisecond)
				r.frame()
				r.frame()
				return r
			}},
		{name: "ProgressBar", part: "ProgressBar.Track",
			theme: func(th *Theme, c Color) { th.ProgressBar.Track.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(ProgressBar(0.4).Styles(ProgressBarStyle{Track: bgStyle(inst)}))
				})
			}},
		{name: "ProgressRing", part: "ProgressRing.Ring",
			theme: func(th *Theme, c Color) { th.ProgressRing.Ring.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(ProgressRing().Styles(ProgressRingStyle{Ring: bgStyle(inst)}))
				})
			}},
		{name: "Badge", part: "Badge.Badge",
			theme: func(th *Theme, c Color) { th.Badge.Badge.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Badge(5).Styles(BadgeStyle{Badge: bgStyle(inst)}))
				})
			}},
		{name: "Toast", part: "Toast.Container",
			theme: func(th *Theme, c Color) { th.Toast.Container.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				r := newThemedRig(th, func(ctx *Context) Element { return Div().Column().Child(Toaster()) })
				r.rt.toast(ToastInfo, "Saved").Styles(ToastStyle{Container: bgStyle(inst)})
				r.frame()
				return r
			}},
		{name: "Tooltip", part: "Tooltip.Panel", overlay: true,
			theme: func(th *Theme, c Color) { th.Tooltip.Panel.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				r := newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(Div().Key("t").Size(Px(100), Px(30)).Tooltip("Hello"))
				})
				rc := r.rt.nodeRect(keyNode(t, r.rt, "t"))
				r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
				r.now = r.now.Add(600 * time.Millisecond)
				r.frame()
				return r
			}},
		{name: "DataGrid", part: "DataGrid.Grid",
			theme: func(th *Theme, c Color) { th.DataGrid.Grid.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, partDataGrid(3, DataGridStyle{Grid: bgStyle(inst)},
					NewSignal([]string(nil)), NewSignal(SortState{})))
			}},
		{name: "TextArea", part: "TextArea.Frame",
			theme: func(th *Theme, c Color) { th.Field.Frame.Background = c },
			make: func(t *testing.T, th *Theme, inst *Color) *accessRig {
				return newThemedRig(th, func(ctx *Context) Element {
					return Div().Column().Child(TextArea(NewSignal("")).Styles(FieldStyle{Frame: bgStyle(inst)}))
				})
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plain := *DarkTheme()
			withTheme := plain
			c.theme(&withTheme, themeColor)
			read := func(r *accessRig) Color { return partColor(t, r, c.part, c.key, c.border) }
			defColor := read(c.make(t, &plain, nil))
			if got := read(c.make(t, &withTheme, nil)); got != themeColor {
				t.Errorf("theme override: %v, want the theme's %v (default %v)", got, themeColor, defColor)
			}
			if c.overlay {
				return // a tooltip has no instance of its own
			}
			if got := read(c.make(t, &plain, ic)); got != instColor {
				t.Errorf("instance override: %v, want the instance's %v", got, instColor)
			}
			if got := read(c.make(t, &withTheme, ic)); got != instColor {
				t.Errorf("instance over theme: %v, want the instance's %v", got, instColor)
			}
		})
	}
}
