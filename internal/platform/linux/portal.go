//go:build linux

package linux

import (
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/tyler-builds/jamb/internal/platform"
)

// The desktop's settings come from the XDG desktop portal
// (org.freedesktop.portal.Settings), which GNOME, KDE and the other desktops
// implement over the session bus.

const (
	portalName     = "org.freedesktop.portal.Desktop"
	portalPath     = "/org/freedesktop/portal/desktop"
	portalSettings = "org.freedesktop.portal.Settings"
	appearance     = "org.freedesktop.appearance"
	gnomeInterface = "org.gnome.desktop.interface"
	gnomeWM        = "org.gnome.desktop.wm.preferences"
)

var session struct {
	once     sync.Once
	conn     *dbus.Conn
	err      error
	watching bool
}

// sessionBus returns the shared session bus connection.
func sessionBus() (*dbus.Conn, error) {
	session.once.Do(func() {
		session.conn, session.err = dbus.ConnectSessionBus()
	})
	return session.conn, session.err
}

// readSetting reads one portal setting, or nil.
func readSetting(namespace, key string) any {
	conn, err := sessionBus()
	if err != nil {
		return nil
	}
	obj := conn.Object(portalName, portalPath)
	var v dbus.Variant
	if err := obj.Call(portalSettings+".ReadOne", 0, namespace, key).Store(&v); err == nil {
		return v.Value()
	}
	// Portals before version 2 have only Read, which wraps the value twice.
	if err := obj.Call(portalSettings+".Read", 0, namespace, key).Store(&v); err != nil {
		return nil
	}
	if inner, ok := v.Value().(dbus.Variant); ok {
		return inner.Value()
	}
	return v.Value()
}

// ReadSettings reads the desktop's settings from the settings portal: the
// dark style, high contrast, reduced motion, and on GNOME the text scale, caret
// blink and the title bar's buttons. What the portal does not report keeps its
// default. The caption buttons the app draws are GNOME's round ones.
func ReadSettings() platform.Settings {
	s := platform.DefaultSettings()
	s.RoundCaptionButtons = true
	if v, ok := readSetting(gnomeWM, "button-layout").(string); ok {
		s.CaptionButtons = v
	}
	if v, ok := readSetting(appearance, "color-scheme").(uint32); ok {
		s.Dark = v == 1 // 1 prefers dark, 2 prefers light, 0 has no preference
	}
	if v, ok := readSetting(appearance, "contrast").(uint32); ok {
		s.HighContrast = v == 1
	}
	if v, ok := readSetting(appearance, "reduced-motion").(uint32); ok {
		s.ReducedMotion = v == 1
	}
	if v, ok := readSetting(gnomeInterface, "text-scaling-factor").(float64); ok && v > 0 {
		s.TextScale = min(max(int(v*100+0.5), 100), 225)
	}
	if v, ok := readSetting(gnomeInterface, "enable-animations").(bool); ok && !v {
		s.ReducedMotion = true
	}
	if v, ok := readSetting(gnomeInterface, "cursor-blink").(bool); ok && !v {
		s.CaretBlink = 0
	} else if v, ok := readSetting(gnomeInterface, "cursor-blink-time").(int32); ok && v > 0 {
		s.CaretBlink = msDuration(v / 2) // the setting is a whole on-off cycle
	}
	return s
}

// watchSettings tells every window when a desktop setting changes. It runs
// once per process.
func watchSettings() {
	conn, err := sessionBus()
	if err != nil || session.watching {
		return
	}
	session.watching = true
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(portalSettings), dbus.WithMatchMember("SettingChanged")); err != nil {
		return
	}
	ch := make(chan *dbus.Signal, 16)
	conn.Signal(ch)
	go func() {
		for sig := range ch {
			if sig.Name != portalSettings+".SettingChanged" || len(sig.Body) < 1 {
				continue
			}
			if ns, _ := sig.Body[0].(string); ns != appearance && ns != gnomeInterface && ns != gnomeWM {
				continue
			}
			Post(func() {
				if d := disp; d != nil {
					for _, w := range d.windows {
						w.emit(platform.Event{Kind: platform.EventSettingsChanged})
					}
				}
			})
		}
	}()
}

func msDuration(ms int32) time.Duration { return time.Duration(ms) * time.Millisecond }
