// Command jamb-monitor shows what the machine is doing: the load of each processor,
// the memory, the network traffic and the running processes, sampled every second.
// It can end a process, after asking, and it raises alerts when the load or the
// memory stays high, as OS notifications, in its notification area icon and on
// its taskbar button.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tyler-builds/jamb/app"
	// The visual inspector: Ctrl+Shift+I opens it (see App.DevTools).
	_ "github.com/tyler-builds/jamb/devtools"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
	"github.com/tyler-builds/jamb/examples/monitor/store"
	"github.com/tyler-builds/jamb/examples/monitor/views"
)

func main() {
	synthetic := flag.Bool("synthetic", false, "show a made-up machine instead of this one")
	flag.Parse()

	path := store.SettingsPath()
	settings := store.LoadSettings(path)
	every := time.Duration(settings.IntervalMS) * time.Millisecond
	if every <= 0 {
		every = time.Second
	}
	src := sample.Machine()
	if *synthetic {
		src = sample.NewSynthetic(time.Now(), time.Second)
	}
	s := store.New(src, every)
	if settings.Rules != nil {
		s.Rules.Set(*settings.Rules)
	}

	layout := views.Layout{
		Dock:        ui.NewSignal(views.DefaultDock.Clone()),
		Order:       ui.NewSignal(settings.Columns.Order),
		Widths:      ui.NewSignal(settings.Columns.Widths),
		Sort:        ui.NewSignal(views.DefaultSort),
		Notify:      ui.NewSignal(settings.Notify == nil || *settings.Notify),
		CloseToTray: ui.NewSignal(settings.CloseToTray),
		Theme:       ui.NewSignal(settings.Theme),
		Grouped:     ui.NewSignal(settings.Grouped == nil || *settings.Grouped),
		Floating:    app.OpenDockWindow,
		FileIcon:    app.FileIcon,
	}
	if settings.Layout != nil {
		layout.Dock.Set(*settings.Layout)
	}
	if settings.Sort != nil {
		layout.Sort.Set(*settings.Sort)
	}

	// save runs on the UI thread and from the save timer's goroutine.
	var mu sync.Mutex
	save := func() {
		mu.Lock()
		defer mu.Unlock()
		settings.IntervalMS = int(s.Interval.Peek() / time.Millisecond)
		settings.Theme = layout.Theme.Peek()
		dock := layout.Dock.Peek()
		settings.Layout = &dock
		settings.Columns = ui.ColumnState{Order: layout.Order.Peek(), Widths: layout.Widths.Peek()}
		sort := layout.Sort.Peek()
		settings.Sort = &sort
		rules := s.Rules.Peek()
		settings.Rules = &rules
		settings.CloseToTray = layout.CloseToTray.Peek()
		notify := layout.Notify.Peek()
		settings.Notify = &notify
		grouped := layout.Grouped.Peek()
		settings.Grouped = &grouped
		if err := store.SaveSettings(path, settings); err != nil {
			fmt.Fprintln(os.Stderr, "jamb-monitor: saving the settings failed:", err)
		}
	}
	// Each change restarts the timer, so a drag of a divider is saved once it pauses.
	var timer *time.Timer
	var timerMu sync.Mutex
	layout.OnChange = func() {
		timerMu.Lock()
		defer timerMu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(400*time.Millisecond, save)
	}
	a := views.New(s, layout)

	// The main window, once it has rendered; the tray and the notifications use it.
	var win atomic.Value // ui.Window
	window := func() ui.Window {
		w, _ := win.Load().(ui.Window)
		return w
	}
	show := func() {
		if w := window(); w != nil {
			w.Show()
		}
	}

	tray := app.NewTray(app.TrayOptions{
		Icon:          trayIcon(0),
		Tooltip:       "Monitor",
		OnClick:       show,
		OnDoubleClick: show,
		Menu: func() []app.TrayItem {
			return []app.TrayItem{
				{Label: "Show Monitor", OnClick: show},
				{Label: "Pause sampling", Checked: s.Paused.Peek(), OnClick: func() {
					s.Paused.Set(!s.Paused.Peek())
					s.Wake()
				}},
				app.TraySeparator(),
				{Label: "Quit", OnClick: func() {
					if w := window(); w != nil {
						w.Close()
					}
				}},
			}
		},
	})

	// Each sample updates the tray icon and its tooltip, and the taskbar badge while
	// an alert's condition holds. The icon is drawn again only when its bar moves.
	lastBar, lastFiring := -1, -1
	s.OnSample = func(snap sample.Snapshot) {
		if bar := int(snap.CPU / 5); bar != lastBar {
			lastBar = bar
			tray.SetIcon(trayIcon(snap.CPU))
		}
		tray.SetTooltip(fmt.Sprintf("Monitor: processor %.0f%%, memory %.0f%%", snap.CPU, snap.Mem.Percent()))
		firing := len(s.Firing.Peek())
		if w := window(); w != nil && firing != lastFiring {
			lastFiring = firing
			switch firing {
			case 0:
				w.SetTaskbarBadge(nil, "")
			case 1:
				w.SetTaskbarBadge(badgeIcon(), "An alert is active")
			default:
				w.SetTaskbarBadge(badgeIcon(), fmt.Sprintf("%d alerts are active", firing))
			}
		}
	}
	s.OnAlert = func(al store.Alert) {
		if !layout.Notify.Peek() {
			return
		}
		title := "The processor is busy"
		if al.Kind == store.AlertMemory {
			title = "Memory is running low"
		}
		body := al.Message
		if al.Top != "" {
			body += " The busiest process is " + al.Top + "."
		}
		if err := app.Notify(app.Notification{Title: title, Body: body, Tag: string(al.Kind),
			Args: string(al.Kind)}); err != nil {
			fmt.Fprintln(os.Stderr, "jamb-monitor: the notification was not shown:", err)
		}
	}

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	root := a.Root()
	w := app.Window{
		Title:          "Jamb Monitor",
		Width:          1360,
		Height:         860,
		MinWidth:       900,
		MinHeight:      560,
		Chrome:         app.ChromeCustom,
		QuitWhenClosed: true,
		CloseToTray:    layout.CloseToTray,
		Root: func(ctx *ui.Context) ui.Element {
			if win.Load() == nil {
				win.Store(ctx.Window())
			}
			return root(ctx)
		},
		Bindings: views.Bindings(),
		Actions:  views.Actions(),
		Zoom:     settings.Zoom,
		OnZoom: func(z float32) {
			mu.Lock()
			settings.Zoom = z
			mu.Unlock()
			layout.OnChange()
		},
		OnClose: func() bool {
			save()
			return true
		},
	}
	switch settings.Theme {
	case "light":
		w.Theme = ui.LightTheme()
	case "dark":
		w.Theme = ui.DarkTheme()
	}
	go s.Run(ctx)
	tray.Show()
	app.New().AppID("JambExamples.Monitor").DevTools(true).
		// A second launch shows this window instead of opening another.
		SingleInstance(func([]string, string) { show() }).
		// A click on an alert's notification shows the alerts.
		OnNotification(func(app.NotificationActivation) {
			show()
			a.ShowAlerts()
		}).
		Run(w)
}
