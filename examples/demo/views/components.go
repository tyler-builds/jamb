package views

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ease"
	"github.com/tyler-builds/jamb/ui"
)

// Components is a reference page for the framework's controls and styles. Each
// section shows the options of one part of the API; it is not a design.
func Components(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		return ui.Div().Grow(1).ScrollY().
			Child(ui.Div().PaddingX(32).PaddingY(28).Gap(20).MaxWidth(ui.Px(1100)).
				Child(ui.Div().Gap(4).
					Child(ui.Div().Heading(1).Child(ui.Text("Components").FontSize(26).Semibold())).
					Child(ui.Text("One section per part of the framework. Hover, click and Tab to try them.").TextColor(t.TextMuted))).
				Child(section(t, "Text",
					sample(t, "Italic", ui.Text("Italic text").Italic()),
					sample(t, "Mono", ui.Text("Mono text").Mono()),
					sample(t, "Highlight: Underline", wordHighlight("Underline", ui.Highlight{Underline: true})),
					sample(t, "Highlight: Strikethrough", wordHighlight("Strikethrough", ui.Highlight{Strikethrough: true})),
					sample(t, "Highlight: Italic", wordHighlight("Italic", ui.Highlight{Italic: true})),
					sample(t, "Highlight: Mono", wordHighlight("Mono", ui.Highlight{Mono: true})),
					sample(t, "Highlight: Background", wordHighlight("Background", ui.Highlight{Background: t.AccentSoft, Color: t.Text})),
					clickSample(t))).
				Child(section(t, "Buttons",
					sample(t, "Primary", ui.Button("Primary").Primary().OnClick(pressed("Primary"))),
					sample(t, "Secondary", ui.Button("Secondary").Secondary().OnClick(pressed("Secondary"))),
					sample(t, "Ghost", ui.Button("Ghost").Ghost().OnClick(pressed("Ghost"))),
					sample(t, "Danger", ui.Button("Danger").Danger().OnClick(pressed("Danger"))),
					sample(t, "Small", ui.Button("Small").Small().OnClick(pressed("Small"))),
					sample(t, "Disabled", ui.Button("Disabled").Disabled(true)))).
				Child(section(t, "Overlays", placements(t))).
				Child(section(t, "Menus", dropdownSample(t), contextSample(t))).
				Child(navigationSection()).
				Child(splitSection()).
				Child(formControlsSection()).
				Child(formsSection(t)).
				Child(comboboxSection(ctx, t)).
				Child(dateTimeSection(ctx, t)).
				Child(dataGridSection(ctx, t)).
				Child(chartsSection(ctx, t)).
				Child(customizingSection()).
				Child(dialogsSection(ctx, t, s)).
				Child(section(t, "Images",
					fitSample(t, "Cover", ui.FitCover),
					fitSample(t, "Contain", ui.FitContain),
					fitSample(t, "Fill", ui.FitFill))).
				Child(section(t, "Gradients",
					sample(t, "sRGB", gradientBox(ui.LinearGradient(90, gradFrom, gradTo))),
					sample(t, "InOklab", gradientBox(ui.LinearGradient(90, gradFrom, gradTo).InOklab())),
					sample(t, "Stops 0.3 to 0.7", gradientBox(ui.LinearGradient(90, gradFrom, gradTo).Stops(0.3, 0.7))))).
				Child(drawingSection(t)).
				Child(widgetsSection(t)).
				Child(clipboardSection(t)).
				Child(clipSection(t)).
				Child(motionSection(t)).
				Child(focusSection(t)).
				Child(feedbackSection(ctx, t, s)))
	})
}

// section is a titled card holding the samples of one part of the framework.
func section(t *ui.Theme, title string, samples ...ui.Element) ui.Element {
	return Card(t).Gap(14).
		Child(SectionTitle(t, title)).
		Child(ui.Div().Row().Wrap().Align(ui.Start).Gap(22).Children(samples...))
}

// sample is one labeled example in a section.
func sample(t *ui.Theme, label string, el ui.Element) ui.Element {
	return ui.Div().Gap(6).Child(el).Child(ui.Text(label).FontSize(11).TextColor(t.TextSubtle))
}

// pressed is the action of a sample button. It announces the press, so that a
// screen reader user hears that the button ran.
func pressed(label string) func() {
	return func() { ui.Announce(label+" pressed", access.Polite) }
}

// wordHighlight is a word styled by one Highlight.
func wordHighlight(word string, h ui.Highlight) *ui.TextElement {
	return ui.Text(word).FontSize(15).Highlight(0, len(word), h)
}

// clickSample is a rich text with a hover style and a clickable segment. Its
// count is kept in the component, so only this sample re-renders.
func clickSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		clicks := ui.Local(ctx, "clicks", 0)
		return sample(t, "Rich: hover and click", ui.Div().Gap(4).
			Child(ui.Rich(
				ui.Seg("Hover "),
				ui.Seg("this word").Hover(ui.Highlight{Color: t.Accent, Underline: true}),
				ui.Seg(", then click "),
				ui.Seg("this one").Color(t.Accent).Underline().OnClick(func() { clicks.Set(clicks.Peek() + 1) }),
			).FontSize(15)).
			Child(ui.Textf("Clicked %d times", clicks.Get()).FontSize(12).TextColor(t.TextMuted)))
	})
}

// placementList is every Placement, in the order the samples show them.
var placementList = []struct {
	name string
	at   ui.Placement
}{
	{"BelowStart", ui.BelowStart}, {"BelowCenter", ui.BelowCenter}, {"BelowEnd", ui.BelowEnd},
	{"AboveStart", ui.AboveStart}, {"AboveCenter", ui.AboveCenter}, {"AboveEnd", ui.AboveEnd},
	{"RightStart", ui.RightStart}, {"RightCenter", ui.RightCenter}, {"RightEnd", ui.RightEnd},
	{"LeftStart", ui.LeftStart}, {"LeftCenter", ui.LeftCenter}, {"LeftEnd", ui.LeftEnd},
}

// placements has one button per Placement. Each opens a popover beside its
// button, 10 px away. A click outside it, or Escape, closes it.
func placements(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		open := ui.Local(ctx, "open", -1) // index into placementList, -1 for none
		row := ui.Div().Row().Wrap().Gap(16).Align(ui.Start)
		for i, p := range placementList {
			anchor := ui.Div().Key("place-" + p.name).
				Child(ui.Button(p.name).Secondary().Small().OnClick(func() { open.Set(i) }))
			if open.Get() == i {
				anchor.Child(ui.Div().Key("popover").Overlay(p.at).OverlayGap(10).Padding(10).Radius(8).
					Background(t.Surface).Border(1, t.Border).Shadow(ui.ShadowMd).
					OnClickOutside(func() { open.Set(-1) }).
					Child(ui.Text("Opened " + p.name).FontSize(12).Medium()))
			}
			row.Child(anchor)
		}
		return ui.Div().Gap(12).Child(row).
			Child(ui.Text("Escape closes an open popover, as a click outside it does.").FontSize(12).TextColor(t.TextMuted)).
			Child(popoverSample(t))
	})
}

// dropdownSample is a button that opens a menu below it. The menu's shortcuts
// are the window's bindings, so they follow any change to the keymap.
func dropdownSample(t *ui.Theme) ui.Element {
	return sample(t, "Dropdown", ui.DropdownMenu(ui.Button("Project actions").Key("project-actions").Secondary().Small(), func() []ui.MenuItem {
		return []ui.MenuItem{
			{Action: GoProjects},
			{Action: FocusSearch},
			ui.MenuSeparator(),
			{Label: "Dark theme", Action: ToggleTheme, Check: ui.MenuCheckBox, Checked: t.Dark},
			ui.MenuSeparator(),
			{Label: "Delete project", Disabled: true},
		}
	}))
}

// contextSample is a box with a context menu, which opens at the pointer, or
// below the box for Shift+F10 when the box has focus. Its layout and completed
// flags live in the component, so the menu shows their state when it opens.
func contextSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		layout := ui.Local(ctx, "layout", "Grid")
		completed := ui.Local(ctx, "completed", false)
		box := ui.Div().Key("context-menu-sample").Width(ui.Px(240)).MinHeight(ui.Px(64)).Padding(8).Radius(8).Border(1, t.Border).Center().
			Focusable().FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}}).
			Child(ui.Text("Right-click, or focus it and press Shift+F10").FontSize(12).TextColor(t.TextMuted)).
			ContextMenu(func() []ui.MenuItem {
				return []ui.MenuItem{
					{Action: GoProjects},
					{Action: FocusSearch},
					ui.MenuSeparator(),
					{Label: "Layout", Submenu: []ui.MenuItem{
						layoutItem("Grid", layout), layoutItem("List", layout), layoutItem("Board", layout),
					}},
					{Label: "Show completed", Check: ui.MenuCheckBox, Checked: completed.Peek(),
						OnClick: func() { completed.Set(!completed.Peek()) }},
					ui.MenuSeparator(),
					{Label: "Archive", Disabled: true},
				}
			})
		return sample(t, "Context menu", box)
	})
}

// layoutItem is one radio item of the Layout submenu: selected when it is the
// current layout.
func layoutItem(label string, current *ui.Signal[string]) ui.MenuItem {
	return ui.MenuItem{Label: label, Check: ui.MenuCheckRadio, Checked: current.Peek() == label,
		OnClick: func() { current.Set(label) }}
}

// fitSample is the banner drawn in a box of fixed size with one ImageFit.
func fitSample(t *ui.Theme, label string, fit ui.ImageFit) ui.Element {
	return sample(t, label, ui.Div().Radius(8).Background(t.SurfaceActive).
		Child(ui.Image(banner).Alt("Sample banner").Size(ui.Px(150), ui.Px(90)).Fit(fit).Radius(8)))
}

var (
	gradFrom = ui.Hex("#2563eb") // blue, to yellow: sRGB mixing turns gray here
	gradTo   = ui.Hex("#facc15")
)

func gradientBox(g ui.Gradient) ui.Element {
	return ui.Div().Size(ui.Px(150), ui.Px(56)).Radius(10).BackgroundGradient(g)
}

// clipSection shows content clipped to rounded corners, and borders with a
// width for each side.
func clipSection(t *ui.Theme) ui.Element {
	rows := ui.Div()
	for i := range 12 {
		rows.Child(ui.Div().Height(ui.Px(34)).Background(t.SurfaceActive).Center().
			Child(ui.Text(fmt.Sprintf("Row %d", i+1)).FontSize(12).TextColor(t.TextMuted)))
	}
	scroller := ui.Div().Size(ui.Px(220), ui.Px(120)).Radius(14).Background(t.Surface).
		Border(1, t.Border).ScrollY().Child(rows)
	tab := func(label string, active bool) ui.Element {
		line, text := t.Border, t.TextMuted
		if active {
			line, text = t.Accent, t.Text
		}
		return ui.Div().PaddingX(4).PaddingY(6).BorderEdges(ui.Edges{Bottom: 2}, line).
			Child(ui.Text(label).FontSize(13).TextColor(text))
	}
	bar := ui.Div().Width(ui.Px(220)).PaddingX(12).PaddingY(10).Radius(8).Background(t.Surface).
		BorderEdges(ui.Edges{Left: 3}, t.Accent).Child(ui.Text("Left accent bar").FontSize(12))
	mixed := ui.Div().Width(ui.Px(220)).PaddingX(12).PaddingY(10).Radius(12).Background(t.Surface).
		BorderEdges(ui.Edges{Top: 1, Right: 3, Bottom: 1, Left: 3}, t.Accent).Child(ui.Text("Mixed widths").FontSize(12))
	return Card(t).Gap(14).
		Child(SectionTitle(t, "Clipping and borders")).
		Child(ui.Div().Row().Wrap().Align(ui.Start).Gap(22).
			Child(sample(t, "Rounded scroll container", scroller)).
			Child(sample(t, "Underline tabs", ui.Div().Row().Gap(10).Child(tab("Overview", true)).Child(tab("Activity", false)))).
			Child(sample(t, "Left accent bar", bar)).
			Child(sample(t, "Per-side widths with radius", mixed)))
}

// motionSection shows easing curves running side by side, transforms, and
// the stacking order that ZIndex sets.
func motionSection(t *ui.Theme) ui.Element {
	curves := []struct {
		name string
		fn   func(float32) float32
	}{
		{"Linear", ease.Linear},
		{"OutCubic", ease.OutCubic},
		{"InOutCubic", ease.InOutCubic},
		{"Bounce(Out)", ease.Bounce(ease.Out)},
		{"Reverse(Out)", ease.Reverse(ease.Out)},
	}
	card := Card(t).Gap(14).Child(SectionTitle(t, "Motion"))
	for _, c := range curves {
		card.Child(easeRow(t, c.name, c.fn))
	}
	stack := ui.Div().Size(ui.Px(150), ui.Px(70)).
		Child(ui.Div().Absolute().Left(0).Top(0).Size(ui.Px(90), ui.Px(56)).Radius(10).Background(t.Accent).ZIndex(2)).
		Child(ui.Div().Absolute().Left(50).Top(14).Size(ui.Px(90), ui.Px(56)).Radius(10).Background(t.Warning).ZIndex(1))
	return card.Child(ui.Divider()).Child(ui.Div().Row().Wrap().Align(ui.Start).Gap(22).
		Child(sample(t, "Scale(0.8)", tile(t, "Card").Scale(0.8))).
		Child(sample(t, "Offset(10, 6)", tile(t, "Card").Offset(10, 6))).
		Child(sample(t, "Hover: scale and lift", tile(t, "Card").Transition(fast).Hover(ui.Style{Scale: 1.08, OffsetY: -4}))).
		Child(sample(t, "ZIndex: first child on top", stack)))
}

// easeRow is a ball that travels a track on the curve, repeating forever.
func easeRow(t *ui.Theme, name string, curve func(float32) float32) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(12).
		Child(ui.Text(name).FontSize(12).TextColor(t.TextMuted).Style(ui.Style{MinWidth: ui.Px(110)})).
		Child(ui.Animate("ease-"+name, ui.Animation{Duration: 1600 * time.Millisecond, Easing: curve, Repeat: true},
			func(p float32) ui.Element {
				return ui.Div().Height(ui.Px(14)).Width(ui.Px(260)).Radius(7).Background(t.SurfaceActive).
					Child(ui.Div().Size(ui.Px(14), ui.Px(14)).Radius(7).Background(t.Accent).Offset(p*246, 0))
			}))
}

// tile is a small labeled card for the transform and scroll samples.
func tile(t *ui.Theme, label string) *ui.DivElement {
	return ui.Div().Size(ui.Px(90), ui.Px(56)).Radius(10).Background(t.Surface).Border(1, t.Border).
		Center().Child(ui.Text(label).FontSize(12).TextColor(t.TextMuted))
}

// focusSection shows the focus styles, the Tab order, and a horizontal scroller.
func focusSection(t *ui.Theme) ui.Element {
	ring := ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}}
	box := func(label string) *ui.DivElement {
		// Minimums, not a size: the box grows with its label at a large Text size.
		return ui.Div().MinWidth(ui.Px(96)).MinHeight(ui.Px(34)).Radius(8).Border(1, t.Border).Center().
			FocusVisible(ring).Child(ui.Text(label).FontSize(12))
	}
	scroller := ui.Div().Row().ScrollX().Gap(10).Padding(4)
	for i := range 12 {
		scroller.Child(tile(t, fmt.Sprintf("Item %d", i+1)).Shrink(0))
	}
	return Card(t).Gap(14).
		Child(SectionTitle(t, "Focus and scrolling")).
		Child(ui.Div().Row().Wrap().Align(ui.Start).Gap(22).
			Child(sample(t, "FocusWithin", focusWithinSample(t))).
			Child(sample(t, "Focused", box("Focus me").Focusable().Focused(ui.Style{BorderColor: t.Accent}))).
			Child(sample(t, "TabIndex 2", box("Second").TabIndex(2))).
			Child(sample(t, "TabIndex 1", box("First").TabIndex(1))).
			Child(sample(t, "TabStop(false)", box("Click only").Focusable().TabStop(false)))).
		Child(ui.Divider()).
		Child(sample(t, "ScrollX", scroller))
}

// focusWithinSample is a field in a frame that lights up while the field has
// focus.
func focusWithinSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		draft := ui.Local(ctx, "draft", "")
		return ui.Div().Width(ui.Px(220)).Padding(8).Radius(8).Border(1, t.Border).
			FocusWithin(ui.Style{BorderColor: t.Accent}).
			Child(ui.TextInput(draft).Label("Draft").Placeholder("Click inside the frame").Width(ui.Full))
	})
}

// popoverSample is a Popover: a button that opens a small form next to it,
// which takes focus, and closes on Escape or a click outside.
func popoverSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		query := ui.Local(ctx, "query", "")
		mine := ui.Local(ctx, "mine", false)
		return sample(t, "Popover", ui.Popover(ui.Button("Filters").Key("filters").Secondary().Small(), func() ui.Element {
			return ui.Div().Column().Gap(10).Width(ui.Px(240)).
				Child(ui.Text("Filter projects").Semibold()).
				Child(ui.TextInput(query).Label("Name contains").Placeholder("Name contains").Width(ui.Full)).
				Child(ui.Checkbox("Only my projects", mine))
		}))
	})
}
