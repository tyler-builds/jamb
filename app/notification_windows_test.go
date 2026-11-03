//go:build windows

package app

import (
	"path/filepath"
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

func TestPlanNotifyRegistrationWritesNothing(t *testing.T) {
	programs := filepath.Join(`C:\Users\x\AppData\Roaming`, "Microsoft", "Windows", "Start Menu", "Programs")
	exe := `C:\Apps\Notes Pro.exe`

	plain := planNotifyRegistration("JambExamples.Notes", exe, programs, nil, false)
	if plain.Action != shortcutCreate || plain.Shortcut != filepath.Join(programs, "Notes Pro.lnk") || plain.AppID != "JambExamples.Notes" || plain.Target != exe {
		t.Errorf("plan = %+v", plain)
	}
	if len(plain.Values) != 0 {
		t.Errorf("without a protocol the plan writes %d values, want none", len(plain.Values))
	}
	existing := []shortcutCandidate{{Path: filepath.Join(programs, "Notes Pro.lnk"), Target: exe, Writable: true}}
	stamp := planNotifyRegistration("JambExamples.Notes", exe, programs, existing, false)
	if stamp.Action != shortcutStamp || stamp.Shortcut != existing[0].Path {
		t.Errorf("a writable shortcut to the program: plan = %+v; want it stamped", stamp)
	}

	proto := planNotifyRegistration("JambExamples.Notes", exe, programs, nil, true)
	if len(proto.Values) != 3 {
		t.Fatalf("protocol plan has %d values, want 3", len(proto.Values))
	}
	key := `Software\Classes\jamb-jambexamples.notes`
	want := []regValue{
		{Key: key, Name: "", Value: "URL:jamb-jambexamples.notes"},
		{Key: key, Name: "URL Protocol", Value: ""},
		{Key: key + `\shell\open\command`, Name: "", Value: `"C:\Apps\Notes Pro.exe" "%1"`},
	}
	for i := range want {
		if proto.Values[i] != want[i] {
			t.Errorf("value %d = %+v, want %+v", i, proto.Values[i], want[i])
		}
	}
	for _, v := range proto.Values {
		if !strings.HasPrefix(v.Key, `Software\Classes\`) {
			t.Errorf("value written outside HKCU\\Software\\Classes: %q", v.Key)
		}
	}
}

func TestShortcutName(t *testing.T) {
	tests := map[string]string{
		`C:\Apps\Notes Pro.exe`: "Notes Pro",
		`C:\Apps\a<b>c:d.exe`:   "abcd",
		`C:\Apps\..exe`:         "JambExamples.Notes", // nothing left: the AppID
		`C:\Apps\CON"name".exe`: `CONname`,
		`C:\Apps\trailing .exe`: "trailing",
	}
	for exe, want := range tests {
		if got := shortcutName(exe, "JambExamples.Notes"); got != want {
			t.Errorf("shortcutName(%q) = %q, want %q", exe, got, want)
		}
	}
}
