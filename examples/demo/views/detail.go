package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// OpenProject opens project id in a window of its own, or brings its window
// forward when it is open already. The window reads the store's signals, so
// starring or renaming the project in either window updates both.
func OpenProject(s *Store, id int) {
	if h := s.details[id]; h != nil && !h.Closed() {
		h.Focus()
		return
	}
	p, ok := s.Project(id)
	if !ok {
		return
	}
	s.details[id] = app.Open(app.Window{
		Title:     p.Name,
		Width:     680,
		Height:    640,
		MinWidth:  440,
		MinHeight: 380,
		// The window draws its own title bar, like the main window (see titleBar).
		Chrome: app.ChromeCustom,
		Root:   ProjectDetail(s, id),
		// The main window marks the project as open until this window closes.
		OnClosed: func() { s.windowClosed(id) },
	})
	s.setWindowOpen(id, true)
}

// windowClosed forgets the window of project id, so the project is no longer
// marked open and opening it again makes a new window.
func (s *Store) windowClosed(id int) {
	delete(s.details, id)
	s.setWindowOpen(id, false)
}

// ProjectDetail renders one project for its own window: its details, its name
// (which can be edited) and its comments.
func ProjectDetail(s *Store, id int) func(ctx *ui.Context) ui.Element {
	// The comments' list handle lives as long as the window.
	thread := ui.NewListHandle()
	return func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		focused := ctx.Window().Focused()
		tab := ui.Local(ctx, "tab", "overview")
		p, ok := s.Project(id)
		if !ok {
			return ui.Div().Child(titleBar(t, focused, "")).
				Child(ui.Div().Padding(24).Child(ui.Text("This project no longer exists.")))
		}
		label := "Star"
		if p.Starred {
			label = "Starred"
		}
		// The star is tinted like the project list's, so a starred project reads the same in both windows.
		star := ui.Div().Key("star").Label("Star").Checked(p.Starred).Row().Align(ui.Center).Gap(8).Height(ui.Px(32)).PaddingX(12).Radius(8).
			Background(t.Surface).Border(1, t.Border).Transition(150 * time.Millisecond).
			Hover(ui.Style{Background: t.SurfaceHover}).
			OnClick(func() { s.ToggleStar(id) }).
			Child(ui.Icon(icons.Star).Size(16).Color(starColor(t, p.Starred))).
			Child(ui.Text(label).Size(13).Medium())

		// The overview is the header and the details; the comments have a tab of their
		// own, so the thread gets the height under the tabs.
		content := ui.Div().Grow(1).MinHeight(ui.Px(0)).Padding(28).Gap(18).
			Child(ui.Div().Row().Align(ui.Center).Gap(16).
				Child(IconTile(t, p.Icon, p.Color, 52)).
				Child(ui.Div().Grow(1).Gap(6).MinWidth(ui.Px(0)).
					Child(rename(s, id, p.Name)).
					Child(ui.Div().Row().Align(ui.Center).Gap(8).
						Child(statusBadge(p.Status)).
						Child(ui.Text("Updated " + ago(p.Updated)).Size(12).Color(t.TextSubtle))))).
			Child(ui.Tabs(tab,
				ui.Tab[string]{Value: "overview", Label: "Overview", Panel: func() ui.Element {
					return ui.Div().Column().Gap(18).
						Child(ui.Text(p.Description).Color(t.TextMuted)).
						Child(ui.Div().Row().Align(ui.Center).Gap(12).
							Child(ui.ProgressBar(float64(p.Progress)).Color(progressColor(t, p.Color)).Label(p.Name + " progress").MaxWidth(ui.Px(376)))).
						Child(ui.Div().Row().Align(ui.Center).Gap(16).
							Child(ui.Text(fmt.Sprintf("%d open tasks", p.Tasks)).Size(13).Color(t.TextMuted)).
							Child(star))
				}},
				ui.Tab[string]{Value: "comments", Label: "Comments", Panel: func() ui.Element {
					return Comments(s, id, thread)
				}},
			).Key("detail-tabs").Label("Project sections").KeepPanels())
		return ui.Div().Size(ui.Full, ui.Full).Background(t.Background).TextColor(t.Text).
			Child(titleBar(t, focused, p.Name)).
			Child(content)
	}
}

// rename is the project's name as an editable field. Each change is saved to
// the store, and the window title follows it. An empty name is not saved.
func rename(s *Store, id int, name string) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		draft := ui.Local(ctx, "name", name)
		return ui.TextInput(draft).Key("name").Label("Project name").Borderless().Placeholder("Project name").
			Style(ui.Style{FontSize: 20, FontWeight: ui.FontSemibold}).
			OnChange(func(v string) {
				if v = strings.TrimSpace(v); v != "" {
					s.RenameProject(id, v)
				}
			})
	}).Key("rename")
}
