//go:build windows

package app

import (
	"strings"
	"testing"
)

// TestNotifyRefusesBeforeTheAppIsReady checks the errors that come before any
// toast, registration or COM call: an app that is not running, and one without an
// AppID. Neither reaches the platform layer.
func TestNotifyRefusesBeforeTheAppIsReady(t *testing.T) {
	defer notifyCurrent.Store(nil)

	notifyCurrent.Store(nil)
	if err := Notify(Notification{Title: "x"}); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Errorf("Notify before Run = %v, want the not-running error", err)
	}

	notifyCurrent.Store(&notifySettings{})
	if err := Notify(Notification{Title: "x"}); err == nil || !strings.Contains(err.Error(), "AppID is required") {
		t.Errorf("Notify without an AppID = %v, want the AppID error", err)
	}
	if err := RemoveNotification("t"); err == nil || !strings.Contains(err.Error(), "AppID is required") {
		t.Errorf("RemoveNotification without an AppID = %v, want the AppID error", err)
	}
	if err := RemoveNotification(""); err == nil {
		t.Error("RemoveNotification with no tag was accepted")
	}
}
