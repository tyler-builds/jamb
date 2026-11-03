//go:build linux

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tyler-builds/jamb/internal/platform/linux"
)

// On Linux, notifications go to the desktop's notification server over the
// session bus (org.freedesktop.Notifications). A click reaches the running app
// directly, so it needs neither AppID nor SingleInstance; a click after the app
// has closed does nothing, since the server keeps no way to start it.

// shown maps the notifications the app has shown to their tags and args, and
// tags to the server's ids, so a tag replaces and removes its notification.
var shown struct {
	mu    sync.Mutex
	byID  map[uint32]shownNotification
	byTag map[string]uint32
}

type shownNotification struct {
	tag, args string
	actions   map[string]bool
}

// Notify shows a notification. It must be called once the app runs: from a
// window, an action, or a goroutine started after Run. A notification with the
// Tag of one that is shown replaces it. It is safe from any goroutine.
func Notify(n Notification) error {
	s := notifyCurrent.Load()
	if s == nil {
		return errors.New("app.Notify: the app is not running; call it from a window or after Run has started")
	}
	if err := n.check(); err != nil {
		return err
	}
	ln := linux.Notification{AppName: notifyAppName(s), Title: n.Title, Body: n.Body, Image: n.Image, Silent: n.Silent}
	acts := map[string]bool{}
	for _, a := range n.Actions {
		ln.Actions = append(ln.Actions, [2]string{a.ID, a.Label})
		acts[a.ID] = true
	}
	shown.mu.Lock()
	if n.Tag != "" {
		ln.Replaces = shown.byTag[n.Tag]
	}
	shown.mu.Unlock()
	id, err := linux.ShowNotification(ln)
	if err != nil {
		return fmt.Errorf("app.Notify: %w", err)
	}
	shown.mu.Lock()
	defer shown.mu.Unlock()
	if shown.byID == nil {
		shown.byID, shown.byTag = map[uint32]shownNotification{}, map[string]uint32{}
	}
	if old, ok := shown.byTag[n.Tag]; ok && n.Tag != "" && old != id {
		delete(shown.byID, old)
	}
	shown.byID[id] = shownNotification{tag: n.Tag, args: n.Args, actions: acts}
	if n.Tag != "" {
		shown.byTag[n.Tag] = id
	}
	return nil
}

// notifyAppName names the app to the notification server: the AppID, or the
// executable's name.
func notifyAppName(s *notifySettings) string {
	if s.appID != "" {
		return s.appID
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Base(exe)
	}
	return "jamb"
}

// RemoveNotification removes the notification with the tag. A tag that is not
// shown is not an error.
func RemoveNotification(tag string) error {
	if notifyCurrent.Load() == nil {
		return errors.New("app.RemoveNotification: the app is not running; call it from a window or after Run has started")
	}
	if tag == "" {
		return errors.New("app.RemoveNotification: tag is empty")
	}
	shown.mu.Lock()
	id, ok := shown.byTag[tag]
	shown.mu.Unlock()
	if !ok {
		return nil
	}
	if err := linux.CloseNotification(id); err != nil {
		return fmt.Errorf("app.RemoveNotification: %w", err)
	}
	return nil
}

// ClearNotifications removes every notification the app has shown.
func ClearNotifications() error {
	if notifyCurrent.Load() == nil {
		return errors.New("app.ClearNotifications: the app is not running; call it from a window or after Run has started")
	}
	shown.mu.Lock()
	ids := make([]uint32, 0, len(shown.byID))
	for id := range shown.byID {
		ids = append(ids, id)
	}
	shown.mu.Unlock()
	var first error
	for _, id := range ids {
		if err := linux.CloseNotification(id); err != nil && first == nil {
			first = fmt.Errorf("app.ClearNotifications: %w", err)
		}
	}
	return first
}

// setNotifications records the app's settings and receives the clicks.
func (a *App) setNotifications() {
	s := &notifySettings{appID: a.appID, handler: a.notifyHandler}
	notifyCurrent.Store(s)
	linux.SetNotificationHandlers(notificationAction, notificationClosed)
}

// notificationAction reports a click on one of the app's notifications to the
// handler. It runs on the UI thread.
func notificationAction(id uint32, action string) {
	shown.mu.Lock()
	n, ok := shown.byID[id]
	shown.mu.Unlock()
	s := notifyCurrent.Load()
	if !ok || s == nil || s.handler == nil {
		return
	}
	if action == linux.DefaultAction {
		action = ""
	} else if !n.actions[action] {
		return // not a button of this notification
	}
	s.handler(NotificationActivation{Action: action, Tag: n.tag, Args: n.args})
}

// notificationClosed forgets a notification that is gone.
func notificationClosed(id uint32) {
	shown.mu.Lock()
	defer shown.mu.Unlock()
	if n, ok := shown.byID[id]; ok {
		delete(shown.byID, id)
		if shown.byTag[n.tag] == id {
			delete(shown.byTag, n.tag)
		}
	}
}
