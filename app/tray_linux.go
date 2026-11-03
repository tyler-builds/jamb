//go:build linux

package app

import (
	"os"
	"path/filepath"

	"github.com/tyler-builds/jamb/internal/platform/linux"
)

// newTrayIcon returns the StatusNotifierItem of t. Its callbacks run the app's
// actions on the UI thread.
func newTrayIcon(t *Tray) trayIcon {
	name := application.appID
	if name == "" {
		if exe, err := os.Executable(); err == nil {
			name = filepath.Base(exe)
		}
	}
	o := linux.TrayOptions{
		ID:          t.id,
		AppName:     name,
		Tooltip:     t.tooltip,
		Icon:        t.icon,
		Click:       t.opts.OnClick,
		DoubleClick: t.opts.OnDoubleClick,
	}
	if t.opts.Menu != nil {
		// The menu is built each time it opens, and the actions of that menu are
		// the ones Chosen runs.
		o.Menu = func() []linux.MenuItem {
			t.actions = map[uint32]func(){}
			var last uint32
			return linuxMenuItems(t.opts.Menu(), &last, t.actions)
		}
	}
	o.Chosen = func(id uint32) {
		if run := t.actions[id]; run != nil {
			run()
		}
	}
	return linux.NewTrayIcon(o)
}

// linuxMenuItems returns the native items of items, in order, as menuItems does
// on Windows.
func linuxMenuItems(items []TrayItem, last *uint32, actions map[uint32]func()) []linux.MenuItem {
	out := make([]linux.MenuItem, 0, len(items))
	for _, it := range items {
		if it.separator {
			out = append(out, linux.MenuItem{Separator: true})
			continue
		}
		m := linux.MenuItem{Label: it.Label, Disabled: it.Disabled, Checked: it.Checked}
		if len(it.Submenu) > 0 {
			m.Submenu = linuxMenuItems(it.Submenu, last, actions)
		} else {
			*last++
			m.ID = *last
			actions[m.ID] = it.OnClick
		}
		out = append(out, m)
	}
	return out
}
