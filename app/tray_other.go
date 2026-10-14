//go:build !windows

package app

import "image"

// newTrayIcon returns the icon of t. Tray icons are supported on Windows only.
func newTrayIcon(*Tray) trayIcon { return unsupportedTray{} }

// unsupportedTray is the tray icon of a platform without one. It does not show.
type unsupportedTray struct{}

func (unsupportedTray) Show() error               { return errNotWindows }
func (unsupportedTray) SetTooltip(string)         {}
func (unsupportedTray) SetIcon(image.Image) error { return nil }
func (unsupportedTray) Remove()                   {}
