//go:build linux

package linux

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/input"
)

func TestRequestPath(t *testing.T) {
	if got := requestPath(":1.234", "jamb7"); got != "/org/freedesktop/portal/desktop/request/1_234/jamb7" {
		t.Errorf("requestPath = %s", got)
	}
}

func TestFileResult(t *testing.T) {
	ok := map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{"file:///home/someone/My%20Notes/a.md"})}
	if r := fileResult(dialog.Open, dialog.FileOptions{}, uint32(0), ok); len(r.Paths) != 1 || r.Paths[0] != "/home/someone/My Notes/a.md" {
		t.Errorf("open: %+v", r)
	}
	if r := fileResult(dialog.Open, dialog.FileOptions{}, uint32(1), nil); !r.Canceled {
		t.Errorf("cancel: %+v", r)
	}
	if r := fileResult(dialog.Open, dialog.FileOptions{}, uint32(2), nil); r.Err == nil {
		t.Errorf("failure: %+v", r)
	}
	save := map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{"file:///tmp/Untitled"})}
	if r := fileResult(dialog.Save, dialog.FileOptions{Extension: "md"}, uint32(0), save); r.Paths[0] != "/tmp/Untitled.md" {
		t.Errorf("save adds the extension: %+v", r)
	}
	remote := map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{"sftp://host/a"})}
	if r := fileResult(dialog.Open, dialog.FileOptions{}, uint32(0), remote); r.Err == nil {
		t.Errorf("a remote URI was accepted: %+v", r)
	}
}

func TestFileChooserOptions(t *testing.T) {
	o := fileChooserOptions(dialog.Open, dialog.FileOptions{Multiple: true, Folder: "/tmp",
		Filters: []dialog.Filter{{Name: "Notes", Patterns: []string{"*.md", "*.txt"}}}}, "tok")
	if o["multiple"].Value() != true || o["handle_token"].Value() != "tok" {
		t.Errorf("options %v", o)
	}
	fs, _ := o["filters"].Value().([]portalFilter)
	if len(fs) != 1 || fs[0].Name != "Notes" || len(fs[0].Patterns) != 2 || fs[0].Patterns[1].Pattern != "*.txt" {
		t.Errorf("filters %+v", fs)
	}
	if b, _ := o["current_folder"].Value().([]byte); string(b) != "/tmp\x00" {
		t.Errorf("current_folder %q", b)
	}
	if f := fileChooserOptions(dialog.Folder, dialog.FileOptions{}, "t"); f["directory"].Value() != true {
		t.Errorf("folder options %v", f)
	}
}

func TestKeyMapping(t *testing.T) {
	for code, want := range map[uint32]input.Key{2: input.Key1, 11: input.Key0, 12: input.KeyMinus, 30: input.KeyA,
		44: input.KeyZ, 41: input.KeyBacktick, 53: input.KeySlash, 57: input.KeySpace} {
		if got := keyOfCode(code); got != want {
			t.Errorf("keyOfCode(%d) = %v, want %v", code, got, want)
		}
	}
	for sym, want := range map[uint32]input.Key{0xff0d: input.KeyEnter, 0xfe20: input.KeyTab, 0xffbe: input.KeyF1,
		0xffc9: input.KeyF12, 0xff9c: input.KeyEnd, 0xffb5: input.Key5, 0xff67: input.KeyMenu} {
		if got := keyOfSym(sym); got != want {
			t.Errorf("keyOfSym(%#x) = %v, want %v", sym, got, want)
		}
	}
}

func TestPrintable(t *testing.T) {
	if printable("\x01") != "" || printable("\r") != "" || printable("\x7f") != "" {
		t.Error("a control character is text")
	}
	if printable("é") != "é" || printable("€") != "€" {
		t.Error("text was dropped")
	}
}

// The app ID is the one SetAppID set, else the executable's name.
func TestAppID(t *testing.T) {
	defer SetAppID("")
	exe := appID()
	if exe == "" {
		t.Fatal("no app ID without SetAppID")
	}
	SetAppID("JambExamples.Notes")
	if got := appID(); got != "JambExamples.Notes" {
		t.Errorf("app ID %q; want the one set", got)
	}
	SetAppID("")
	if got := appID(); got != exe {
		t.Errorf("app ID %q after clearing; want the executable's %q", got, exe)
	}
}
