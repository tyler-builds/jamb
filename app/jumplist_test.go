package app

import (
	"strings"
	"testing"
)

func TestValidAppID(t *testing.T) {
	valid := []string{"JambExamples.Notes", "a", "1x", "A-b_c.d", "com.example.app-2"}
	for _, id := range valid {
		if err := validAppID(id); err != nil {
			t.Errorf("validAppID(%q) = %v; want nil", id, err)
		}
	}
	invalid := []string{".a", "-a", "_a", "a b", `a\b`, "a/b", "a:b", "é", strings.Repeat("a", maxAppIDLen+1)}
	for _, id := range invalid {
		if err := validAppID(id); err == nil {
			t.Errorf("validAppID(%q) = nil; want an error", id)
		}
	}
	if err := validAppID(strings.Repeat("a", maxAppIDLen)); err != nil {
		t.Errorf("a %d-character ID: %v", maxAppIDLen, err)
	}
}

func TestParseIcon(t *testing.T) {
	for _, tc := range []struct {
		icon, path string
		index      int
	}{
		{"", "", 0},
		{`C:\app\app.exe`, `C:\app\app.exe`, 0},
		{`C:\app\icons.dll,2`, `C:\app\icons.dll`, 2},
		{`C:\app\icons.dll,-3`, `C:\app\icons.dll`, -3},
		{`C:\a,b.ico`, `C:\a,b.ico`, 0},
		{`C:\a,b.ico,4`, `C:\a,b.ico`, 4},
	} {
		path, index := parseIcon(tc.icon)
		if path != tc.path || index != tc.index {
			t.Errorf("parseIcon(%q) = %q, %d; want %q, %d", tc.icon, path, index, tc.path, tc.index)
		}
	}
}

func TestJumpListCheck(t *testing.T) {
	ok := JumpList{Tasks: []JumpTask{{Title: "New note"}}}
	if err := ok.check(); err != nil {
		t.Errorf("check = %v; want nil", err)
	}
	bad := JumpList{Tasks: []JumpTask{{Title: "New note"}, {Title: "  "}}}
	if err := bad.check(); err == nil || !strings.Contains(err.Error(), "task 2") {
		t.Errorf("check = %v; want an error naming task 2", err)
	}
}

func TestCheckIdentity(t *testing.T) {
	list := JumpList{Tasks: []JumpTask{{Title: "New note"}}}
	for name, tc := range map[string]struct {
		app     *App
		wantErr bool
	}{
		"no settings":            {New(), false},
		"jump list without ID":   {New().JumpList(list), true},
		"single instance no ID":  {New().SingleInstance(nil), true},
		"invalid ID":             {New().AppID("a b"), true},
		"ID alone":               {New().AppID("JambExamples.Notes"), false},
		"ID and jump list":       {New().AppID("JambExamples.Notes").JumpList(list), false},
		"ID and single instance": {New().AppID("JambExamples.Notes").SingleInstance(nil), false},
		"untitled task":          {New().AppID("JambExamples.Notes").JumpList(JumpList{Tasks: []JumpTask{{}}}), true},
	} {
		err := tc.app.checkIdentity()
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: checkIdentity = %v; want an error: %v", name, err, tc.wantErr)
		}
	}
}

// jamb package runs the built app with the flag alone and reads the answer; cmd/jamb's
// TestParseAppID parses the same lines, since it keeps its own copy of the marker.
func TestPrintAppID(t *testing.T) {
	for name, tc := range map[string]struct {
		args   []string
		appID  string
		want   string
		answer bool
	}{
		"flag":               {[]string{"app.exe", "--jamb-print-appid"}, "JambExamples.Notes", "jamb-appid:JambExamples.Notes\n", true},
		"flag, no AppID":     {[]string{"app.exe", "--jamb-print-appid"}, "", "jamb-appid:\n", true},
		"no arguments":       {[]string{"app.exe"}, "JambExamples.Notes", "", false},
		"other argument":     {[]string{"app.exe", "notes.txt"}, "JambExamples.Notes", "", false},
		"flag with argument": {[]string{"app.exe", "--jamb-print-appid", "x"}, "JambExamples.Notes", "", false},
		"flag not first":     {[]string{"app.exe", "x", "--jamb-print-appid"}, "JambExamples.Notes", "", false},
	} {
		var out strings.Builder
		if got := printAppID(tc.args, tc.appID, &out); got != tc.answer || out.String() != tc.want {
			t.Errorf("%s: printAppID = %v, %q; want %v, %q", name, got, out.String(), tc.answer, tc.want)
		}
	}
}
