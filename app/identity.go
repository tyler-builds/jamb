package app

import (
	"errors"
	"fmt"
	"os"
)

// maxAppIDLen is the longest Application User Model ID Windows accepts.
const maxAppIDLen = 128

// AppID sets the app's Application User Model ID, the identifier that Windows uses
// to group the app's windows on the taskbar, and to find its jump list, its Recent
// documents and its single instance. It is set when the app starts, before the
// first window opens. Use a stable identifier in the form CompanyName.ProductName,
// such as "JambExamples.Notes": it is letters, digits, '.', '-' and '_', starts
// with a letter or digit, and is at most 128 characters. Keep it the same across
// versions. A packaged (MSIX) app has an identity from its package, and the
// package's ID is used instead; see the README's Jump lists and single instance
// section.
func (a *App) AppID(id string) *App {
	a.appID = id
	return a
}

// validAppID reports whether id is a usable Application User Model ID.
func validAppID(id string) error {
	if len(id) > maxAppIDLen {
		return fmt.Errorf("app.App.AppID is %d characters; the limit is %d", len(id), maxAppIDLen)
	}
	for i, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case i > 0 && (c == '.' || c == '-' || c == '_'):
		default:
			return fmt.Errorf("app.App.AppID %q: use letters, digits, '.', '-' and '_', starting with a letter or digit", id)
		}
	}
	return nil
}

// checkIdentity reports a problem with the identity settings, before anything
// starts. A jump list and a single instance both need an AppID, which Windows uses
// to find them.
func (a *App) checkIdentity() error {
	if a.appID != "" {
		if err := validAppID(a.appID); err != nil {
			return err
		}
	}
	if (a.jumpList != nil || a.singleInstance) && a.appID == "" {
		return errors.New(`app.App: AppID is required with JumpList and SingleInstance, such as App.AppID("CompanyName.ProductName")`)
	}
	if a.jumpList != nil {
		return a.jumpList.check()
	}
	return nil
}

// setIdentity gives the process its AppID and commits its jump list. It runs on the
// UI thread before the first window is created, because Windows reads the ID when
// the window appears. A failure is printed, and the app runs without the feature.
func (a *App) setIdentity() {
	application.appID = a.appID
	if a.appID != "" {
		if err := setAppID(a.appID); err != nil {
			fmt.Fprintln(os.Stderr, "jamb: AppID:", err)
		}
	}
	if a.jumpList != nil {
		if err := commitJumpList(a.jumpList); err != nil {
			fmt.Fprintln(os.Stderr, "jamb: jump list:", err)
		}
	}
}
