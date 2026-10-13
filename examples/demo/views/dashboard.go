package views

import (
	"time"

	"github.com/tyler-builds/jamb/ease"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/demo/assets"
)

var banner = ui.MustImage(assets.BannerPNG)

var weekly = []float32{0.42, 0.65, 0.38, 0.81, 0.92, 0.55, 0.3, 0.6, 0.74, 0.48, 0.88, 0.67, 0.95, 0.71}

// Dashboard shows a banner, stat tiles, a bar chart and a live counter.
func Dashboard(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		projects := s.Projects.Get()
		active, tasks := 0, 0
		var progress float32
		for _, p := range projects {
			if p.Status == StatusActive {
				active++
			}
			tasks += p.Tasks
			progress += p.Progress
		}
		if len(projects) > 0 {
			progress /= float32(len(projects))
		}

		return ui.Div().Grow(1).ScrollY().
			Child(ui.Div().PaddingX(32).PaddingY(28).Gap(20).MaxWidth(ui.Px(1100)).
				Child(welcome(t, ctx.HighContrast())).
				Child(ui.Div().Row().Gap(14).
					Child(stat(t, icons.Folder, "Projects", ui.Textf("%d", len(projects)), "+2 this month", t.Accent.Lighten(0.2))).
					Child(stat(t, icons.Zap, "Active", ui.Textf("%d", active), "running now", t.Success)).
					Child(stat(t, icons.Check, "Open tasks", ui.Textf("%d", tasks), "across all projects", t.Warning)).
					Child(stat(t, icons.Activity, "Avg. progress", ui.Textf("%.0f%%", progress*100), "weighted by project", ui.Hex("#f472b6")))).
				Child(ui.Div().Row().Gap(14).Align(ui.Stretch).
					Child(chart(t).Grow(2).Basis(ui.Px(0))).
					Child(DeployCounter(s))))
	})
}

// welcome is the banner. Under high contrast it is a plain panel with the
// theme's text: the picture is decoration, and its gradient cannot hold the
// text's contrast.
func welcome(t *ui.Theme, highContrast bool) ui.Element {
	if highContrast {
		return ui.Div().Height(ui.Px(150)).Radius(14).Padding(28).Justify(ui.Center).Gap(6).
			Background(t.Surface).Border(1, t.Border).
			Child(ui.Text("Welcome back, Alex").FontSize(26).Bold().TextColor(t.Text)).
			Child(ui.Text("Three projects moved forward today. The renderer is idle at 0% CPU.").TextColor(t.Text))
	}
	return ui.Div().Height(ui.Px(150)).Radius(14).ClipContent().Shadow(ui.ShadowMd).
		Child(ui.Image(banner).Alt("Project banner").Size(ui.Full, ui.Full)).
		Child(ui.Div().Inset(0).Padding(28).Justify(ui.Center).Gap(6).
			Background(ui.Color{A: 0.12}).
			Child(ui.Text("Welcome back, Alex").FontSize(26).Bold().TextColor(ui.Hex("#ffffff"))).
			Child(ui.Text("Three projects moved forward today. The renderer is idle at 0% CPU.").
				TextColor(ui.Hex("#ffffffd9"))))
}

func stat(t *ui.Theme, ic *icons.Icon, label string, value *ui.TextElement, hint string, c ui.Color) ui.Element {
	return Card(t).Grow(1).Basis(ui.Px(0)).
		// The icon moves below the label when the tile is too narrow for both.
		Child(ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).Gap(8).Wrap().
			Child(ui.Text(label).FontSize(13).Medium().TextColor(t.TextMuted)).
			Child(IconTile(t, ic, c, 30))).
		Child(value.FontSize(28).Semibold()).
		Child(ui.Text(hint).FontSize(12).TextColor(t.TextSubtle))
}

// chart's bars grow from the baseline when the page is first shown. The
// animation restarts only when the page is shown again.
func chart(t *ui.Theme) *ui.DivElement {
	grow := ui.Animation{Duration: 600 * time.Millisecond, Easing: ease.OutCubic}
	return Card(t).Gap(16).
		Child(ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).
			Child(ui.Div().Gap(2).
				Child(SectionTitle(t, "Weekly activity")).
				Child(ui.Text("Commits per day, last two weeks").FontSize(12).TextColor(t.TextSubtle))).
			Child(ui.BadgeText("+18%").Variant(ui.BadgeSuccess).StatusDot())).
		Child(ui.Animate("weekly-bars", grow, func(p float32) ui.Element {
			bars := ui.Div().Row().Align(ui.End).Gap(8).Height(ui.Px(150))
			for i, v := range weekly {
				c := t.Accent
				if i == len(weekly)-1 {
					c = t.Accent.Lighten(0.25)
				}
				bars.Child(ui.Div().Grow(1).Height(ui.Percent(v * 100 * p)).
					RadiusCorners(ui.Corners{TopLeft: 4, TopRight: 4}).
					Background(barFill(t, c)).Transition(fast).
					Hover(ui.Style{Background: c.Lighten(0.2)}))
			}
			return bars
		}))
}

// DeployCounter is the classic counter: its own component, so clicking
// re-renders only this card.
func DeployCounter(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		n := s.Deploys.Get()
		// MinWidth keeps both buttons inside the card in narrow windows.
		return Card(t).Grow(1).Basis(ui.Px(0)).MinWidth(ui.Px(210)).Justify(ui.SpaceBetween).
			Child(ui.Div().Gap(2).
				Child(SectionTitle(t, "Deploys this week")).
				Child(ui.Text("A reactive counter backed by a signal").FontSize(12).TextColor(t.TextSubtle))).
			Child(ui.Textf("%d", n).FontSize(44).Bold()).
			Child(ui.Div().Row().Gap(8).
				Child(ui.Button("Deploy").Icon(icons.ArrowUpRight).OnClick(func() { s.Deploys.Set(s.Deploys.Peek() + 1) })).
				Child(ui.Button("Reset").Secondary().OnClick(func() { s.Deploys.Set(0) })))
	})
}
