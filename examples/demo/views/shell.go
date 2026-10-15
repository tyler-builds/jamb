package views

import (
	"image"
	"image/color"

	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/ui"
)

// The colors of the demo's notification area icon and taskbar badge.
var (
	trayColor  = color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0xff}
	badgeColor = color.NRGBA{R: 0xd9, G: 0x3a, B: 0x2f, A: 0xff}
)

// shellSection is the notification area and taskbar part of the Components page:
// the taskbar progress states and the taskbar badge of the window. The
// notification area icon is switched on the Settings page (see Settings).
func shellSection(ctx *ui.Context, s *Store) ui.Element {
	t := ctx.Theme()
	return section(t, "Notification area and taskbar",
		sample(t, "Taskbar progress", ui.Div().Row().Wrap().Gap(8).
			Child(progressButton(ctx, "None", ui.TaskbarProgress{})).
			Child(progressButton(ctx, "Normal, 40%", ui.TaskbarProgress{State: ui.TaskbarNormal, Value: 0.4})).
			Child(progressButton(ctx, "Indeterminate", ui.TaskbarProgress{State: ui.TaskbarIndeterminate})).
			Child(progressButton(ctx, "Paused, 60%", ui.TaskbarProgress{State: ui.TaskbarPaused, Value: 0.6})).
			Child(progressButton(ctx, "Error, 80%", ui.TaskbarProgress{State: ui.TaskbarError, Value: 0.8}))),
		sample(t, "Taskbar badge", ui.Switch("Taskbar badge", s.Badge).OnChange(func(on bool) {
			applyBadge(ctx.Window(), on)
		})),
	)
}

// progressButton sets the taskbar progress of the window to p when it is clicked.
func progressButton(ctx *ui.Context, label string, p ui.TaskbarProgress) ui.Element {
	return ui.Button(label).Secondary().OnClick(func() {
		ctx.Window().SetTaskbarProgress(p)
	})
}

// applyBadge shows the demo's taskbar badge on win, or removes it.
func applyBadge(win ui.Window, on bool) {
	if !on {
		win.SetTaskbarBadge(nil, "")
		return
	}
	win.SetTaskbarBadge(circleIcon(badgeColor, 32), "Attention needed")
}

// setTrayIcon shows or removes the demo's notification area icon. Its click and
// menu act on win. Removing the icon shows the window, so it is not left hidden
// with no icon to show it again.
func (s *Store) setTrayIcon(win ui.Window, on bool) {
	if !on {
		if s.tray != nil {
			s.tray.Hide()
		}
		win.Show()
		return
	}
	if s.tray == nil {
		s.tray = app.NewTray(app.TrayOptions{
			Icon:          circleIcon(trayColor, 64),
			Tooltip:       "Jamb Demo",
			OnClick:       win.Show,
			OnDoubleClick: win.Show,
			Menu:          func() []app.TrayItem { return s.trayMenu(win) },
		})
	}
	s.tray.Show()
}

// trayMenu is the menu of the notification area icon. Its checked item shows
// whether the taskbar badge is on.
func (s *Store) trayMenu(win ui.Window) []app.TrayItem {
	return []app.TrayItem{
		{Label: "Show window", OnClick: win.Show},
		{Label: "Hide window", OnClick: win.Hide},
		app.TraySeparator(),
		{Label: "Taskbar badge", Checked: s.Badge.Peek(), OnClick: func() {
			on := !s.Badge.Peek()
			s.Badge.Set(on)
			applyBadge(win, on)
		}},
		app.TraySeparator(),
		{Label: "Quit", OnClick: win.Close},
	}
}

// circleIcon returns a filled circle of color c, size pixels square.
func circleIcon(c color.NRGBA, size int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	r := float64(size) / 2
	for y := range size {
		for x := range size {
			dx, dy := float64(x)+0.5-r, float64(y)+0.5-r
			if dx*dx+dy*dy <= r*r {
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return img
}
