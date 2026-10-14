//go:build windows

package app

import "github.com/tyler-builds/jamb/internal/platform/windows"

// newTrayIcon returns the Windows icon of t. Its callbacks run the app's actions
// on the UI thread.
func newTrayIcon(t *Tray) trayIcon {
	o := windows.TrayOptions{
		ID:          t.id,
		Tooltip:     t.tooltip,
		Icon:        t.icon,
		Click:       t.opts.OnClick,
		DoubleClick: t.opts.OnDoubleClick,
	}
	if appID := application.appID; appID != "" {
		g := trayGUID(appID, t.id)
		o.GUID = &g
	}
	if t.opts.Menu != nil {
		// The menu is built each time it opens, and the actions of that menu are
		// the ones Chosen runs.
		o.Menu = func() []windows.MenuItem {
			t.actions = map[uint32]func(){}
			var last uint32
			return menuItems(t.opts.Menu(), &last, t.actions)
		}
	}
	o.Chosen = func(id uint32) {
		if run := t.actions[id]; run != nil {
			run()
		}
	}
	return windows.NewTrayIcon(o)
}

// menuItems returns the native items of items, in order. An item that runs an
// action takes the next ID after *last, and its action goes into actions under
// that ID. Separators and items with a submenu take no ID.
func menuItems(items []TrayItem, last *uint32, actions map[uint32]func()) []windows.MenuItem {
	out := make([]windows.MenuItem, 0, len(items))
	for _, it := range items {
		if it.separator {
			out = append(out, windows.MenuItem{Separator: true})
			continue
		}
		m := windows.MenuItem{Label: it.Label, Disabled: it.Disabled, Checked: it.Checked}
		if len(it.Submenu) > 0 {
			m.Submenu = menuItems(it.Submenu, last, actions)
		} else {
			*last++
			m.ID = *last
			actions[m.ID] = it.OnClick
		}
		out = append(out, m)
	}
	return out
}
