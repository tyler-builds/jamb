//go:build windows

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tyler-builds/jamb/internal/platform/windows"
)

// Notify shows a notification. It needs AppID, and it must be called once the app
// runs: from a window, an action, or a goroutine started after Run. The first call
// registers the app for the user, as described in the README's OS notifications
// section: a Start menu shortcut that carries the AppID, and, when OnNotification is
// set, the protocol that a click starts the app through. Nothing is written before
// the first call.
//
// Notify returns an error when the notification cannot be shown, such as when the
// registration fails or Windows refuses the toast. It is safe from any goroutine.
// A failure after Windows accepts the toast, such as the user's settings hiding
// it, is not reported; a failure that Windows raises for the toast is printed on
// stderr.
func Notify(n Notification) error {
	s, err := notifyReady("app.Notify")
	if err != nil {
		return err
	}
	if err := n.check(); err != nil {
		return err
	}
	return notifyNow(s, n)
}

// RemoveNotification removes the notification with the tag from the Notification
// Center, and from the screen when it is still shown. A tag that is not shown is
// not an error.
func RemoveNotification(tag string) error {
	if _, err := notifyReady("app.RemoveNotification"); err != nil {
		return err
	}
	if tag == "" {
		return errors.New("app.RemoveNotification: tag is empty")
	}
	if n := len([]rune(tag)); n > maxNotificationTag {
		return fmt.Errorf("app.RemoveNotification: tag is %d characters; the limit is %d", n, maxNotificationTag)
	}
	return onUI(func() error {
		if err := windows.RemoveToast(tag); err != nil {
			return fmt.Errorf("app.RemoveNotification: %w", err)
		}
		return nil
	})
}

// ClearNotifications removes all of the app's notifications from the Notification
// Center.
func ClearNotifications() error {
	if _, err := notifyReady("app.ClearNotifications"); err != nil {
		return err
	}
	return onUI(func() error {
		if err := windows.ClearToasts(); err != nil {
			return fmt.Errorf("app.ClearNotifications: %w", err)
		}
		return nil
	})
}

// notifyReady returns the settings of the running app, or the error that a
// notification call returns before the app is ready.
func notifyReady(call string) (*notifySettings, error) {
	s := notifyCurrent.Load()
	if s == nil {
		return nil, fmt.Errorf("%s: the app is not running; call it from a window or after Run has started", call)
	}
	if s.appID == "" {
		return nil, fmt.Errorf(`%s: AppID is required; set it with App.AppID, such as App.AppID("CompanyName.ProductName")`, call)
	}
	return s, nil
}

// notifyNow builds the toast of n, and shows it on the UI thread. The images and the
// activation data are prepared here, on the calling goroutine.
func notifyNow(s *notifySettings, n Notification) error {
	nonce, err := newNonce()
	if err != nil {
		return err
	}
	scheme := activationScheme(s.appID)
	protocol := s.handler != nil

	bodyURI, err := encodeActivation(scheme, activationData{Tag: n.Tag, Args: n.Args, Nonce: nonce})
	if err != nil {
		return err
	}
	actions := make([]toastAction, len(n.Actions))
	for i, act := range n.Actions {
		uri, err := encodeActivation(scheme, activationData{Action: act.ID, Tag: n.Tag, Args: n.Args, Nonce: nonce})
		if err != nil {
			return err
		}
		actions[i] = toastAction{Label: act.Label, Arguments: uri}
	}
	content := toastContent{
		Title: n.Title, Body: n.Body, Protocol: protocol, Launch: bodyURI,
		Actions: actions, Silent: n.Silent,
	}
	if n.Image != nil {
		path, err := writeNotificationImage(notificationDir(), n.Image)
		if err != nil {
			return fmt.Errorf("app.Notify: saving the image: %w", err)
		}
		content.ImageURI = fileURI(path)
	}
	xmlDoc := toastXML(content)

	var plan notifyPlan
	appID := s.appID
	if s.packaged {
		// A packaged app has the identity of its package, and needs no registration.
		appID = ""
	} else {
		plan, err = currentNotifyPlan(s.appID, protocol)
		if err != nil {
			return err
		}
	}
	return onUI(func() error {
		if !s.packaged {
			if err := register(plan); err != nil {
				return err
			}
		}
		if err := windows.ShowToast(appID, xmlDoc, n.Tag); err != nil {
			return fmt.Errorf("app.Notify: %w", err)
		}
		return nil
	})
}

// currentNotifyPlan returns the registration of this program: the executable's path
// and the user's Start menu Programs folder.
func currentNotifyPlan(appID string, protocol bool) (notifyPlan, error) {
	exe, err := os.Executable()
	if err != nil {
		return notifyPlan{}, fmt.Errorf("app.Notify: %w", err)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return notifyPlan{}, fmt.Errorf("app.Notify: %w", err)
	}
	programs := filepath.Join(config, "Microsoft", "Windows", "Start Menu", "Programs")
	return planNotifyRegistration(appID, exe, programs, protocol), nil
}

// registered is the last registration written by this process. It is only used on
// the UI thread.
var registered string

// register writes the registration of plan, unless this process has written it
// already. The shortcut is written first, because Windows matches a toast to the
// shortcut that carries its AppID.
func register(p notifyPlan) error {
	key := fmt.Sprintf("%v", p)
	if key == registered {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p.Shortcut), 0o755); err != nil {
		return fmt.Errorf("app.Notify: %w", err)
	}
	if err := windows.WriteShortcut(p.Shortcut, p.Target, p.AppID); err != nil {
		return fmt.Errorf("app.Notify: writing the Start menu shortcut %s: %w", p.Shortcut, err)
	}
	for _, v := range p.Values {
		if err := windows.SetCurrentUserString(v.Key, v.Name, v.Value); err != nil {
			return fmt.Errorf("app.Notify: registering %s: %w", v.Key, err)
		}
	}
	registered = key
	return nil
}

// onUI runs fn on the UI thread and returns its error. A caller on another goroutine
// waits for it, and gives up when the app has quit.
func onUI(fn func() error) error {
	if onUIThread() {
		return fn()
	}
	ch := make(chan error, 1)
	postUI(func() { ch <- fn() })
	select {
	case err := <-ch:
		return err
	case <-uiDone:
		return errors.New("the app has quit")
	}
}

// setNotifications records the notification settings of the app, on the UI thread
// when it starts, and routes the events of the toasts to the handler.
func (a *App) setNotifications() {
	s := &notifySettings{appID: a.appID, handler: a.notifyHandler, packaged: windows.PackagedProcess()}
	if a.appID != "" {
		s.scheme = activationScheme(a.appID)
	}
	notifyCurrent.Store(s)
	windows.SetToastCallbacks(
		// The events run on the thread Windows raises them on, so the click is posted.
		func(arguments string) { postUI(func() { deliverActivation(arguments) }) },
		func(hr uint32) { fmt.Fprintf(os.Stderr, "jamb: a notification failed with HRESULT 0x%08X\n", hr) },
	)
}
