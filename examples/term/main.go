// Command term is a small terminal: tabs in the title bar, one shell per tab,
// each a PTY and an x/vt emulator shown with ui.CellGrid.
//
// Keys: Ctrl+Shift+T new tab, Ctrl+Shift+W close tab, Ctrl+PageUp/PageDown
// switch tabs, Ctrl+Shift+C copy, Ctrl+Shift+V paste, Shift+PageUp/PageDown
// scroll. Drag to select; the wheel scrolls back.
package main

import (
	"os"
	"slices"

	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

var (
	bgWindow = ui.Hex("#15151b")
	bgPane   = ui.Hex("#1c1c24")
	bgTab    = ui.Hex("#24242e")
	bgHover  = ui.Hex("#2a2a36")
	border   = ui.Hex("#2c2c38")
	fgText   = ui.Hex("#d8d8e0")
	fgMuted  = ui.Hex("#8a8a9a")
	accent   = ui.Hex("#7aa2f7")
	selectBG = ui.Hex("#364a82")
)

// tabs is the open sessions, in tab order, and the one shown.
type tabs struct {
	list   *ui.Signal[[]*session]
	active *ui.Signal[*session]
}

func (t *tabs) open() {
	s := newSession(t.close)
	t.list.Update(func(l []*session) []*session { return append(slices.Clone(l), s) })
	t.show(s)
}

// close ends s and removes its tab; closing the last tab quits. It is safe from
// any goroutine, since a shell that exits closes its tab from its reader.
func (t *tabs) close(s *session) {
	s.kill()
	var next *session
	t.list.Update(func(l []*session) []*session {
		i := slices.Index(l, s)
		if i < 0 {
			return l
		}
		l = slices.Delete(slices.Clone(l), i, i+1)
		if len(l) > 0 {
			next = l[min(i, len(l)-1)]
		}
		return l
	})
	if next == nil {
		if len(t.list.Peek()) == 0 {
			os.Exit(0)
		}
		return
	}
	if t.active.Peek() == s {
		t.show(next)
	}
}

func (t *tabs) show(s *session) {
	t.active.Set(s)
	s.focus.Focus()
}

// step shows the tab d places after the active one, wrapping around.
func (t *tabs) step(d int) {
	l := t.list.Peek()
	if i := slices.Index(l, t.active.Peek()); i >= 0 && len(l) > 0 {
		t.show(l[(i+d+len(l))%len(l)])
	}
}

func (t *tabs) tabBar() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		active := t.active.Get()
		bar := ui.Div().Row().Gap(6).PaddingX(10).Height(ui.Full).Align(ui.Center)
		for _, s := range t.list.Get() {
			bar.Child(t.tab(s, s == active))
		}
		bar.Child(ui.Div().Padding(6).Radius(8).Hover(ui.Style{Background: bgHover}).
			OnClick(t.open).Label("New tab").
			Child(ui.Icon(icons.Plus).Size(16).Color(fgMuted)))
		return ui.TitleBar().
			Styles(ui.TitleBarStyle{Bar: ui.Style{Background: bgWindow, Height: ui.Px(46)}}).
			Child(bar)
	})
}

func (t *tabs) tab(s *session, active bool) ui.Element {
	title := s.title.Get()
	bg, fg, icon := ui.Color{}, fgMuted, fgMuted
	if active {
		bg, fg, icon = bgTab, fgText, accent
	}
	closeBtn := ui.Div().Padding(3).Radius(6).Hover(ui.Style{Background: bgHover}).Label("Close tab").
		OnMouseDown(func(e *ui.MouseEvent) { e.StopPropagation() }).
		OnClick(func() { t.close(s) }).
		Child(ui.Icon(icons.X).Size(13).Color(fgMuted))
	return ui.Div().Key(s.key).Row().Gap(8).PaddingX(12).Height(ui.Px(32)).Width(ui.Px(210)).
		Radius(8).Background(bg).Hover(ui.Style{Background: bgHover}).
		Align(ui.Center).
		OnClick(func() { t.show(s) }).
		Child(ui.Icon(icons.SquareTerminal).Size(15).Color(icon)).
		Child(ui.Div().Grow(1).MinWidth(ui.Px(0)).
			Child(ui.Text(title).FontSize(13).TextColor(fg).Truncate())).
		Child(closeBtn)
}

// screen shows the active session's grid.
func (t *tabs) screen() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		s := t.active.Get()
		if s == nil {
			return ui.Div()
		}
		return ui.Div().Key(s.key).Grow(1).MarginX(10).MarginBottom(10).Radius(12).
			Background(bgPane).Border(1, border).Padding(12).
			Child(s.view(ctx, t))
	})
}

func main() {
	t := &tabs{list: ui.NewSignal[[]*session](nil), active: ui.NewSignal[*session](nil)}
	t.open()

	app.New().Run(app.Window{
		Title:  "term",
		Width:  1100,
		Height: 720,
		Chrome: app.ChromeCustom,
		Root: func(ctx *ui.Context) ui.Element {
			return ui.Div().Column().Size(ui.Full, ui.Full).Background(bgWindow).
				Child(t.tabBar()).
				Child(t.screen())
		},
	})
}
