//go:build !windows

package app

import "errors"

// errNotificationsWindows is returned by the notification calls on platforms without
// a window adapter.
var errNotificationsWindows = errors.New("OS notifications are supported on Windows only")

// Notify returns an error: OS notifications are Windows toasts.
func Notify(Notification) error { return errNotificationsWindows }

// RemoveNotification returns an error; see Notify.
func RemoveNotification(string) error { return errNotificationsWindows }

// ClearNotifications returns an error; see Notify.
func ClearNotifications() error { return errNotificationsWindows }

// setNotifications does nothing on this platform.
func (a *App) setNotifications() {}
