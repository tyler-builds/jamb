package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// appIDHelperEnv makes the test binary act as a built app that is asked for its AppID:
// run with printAppIDFlag alone, it answers, refuses the flag or hangs, as the variable
// says. The process tests below start the test binary that way. It is a console
// program, so no window opens.
const appIDHelperEnv = "JAMB_TEST_APPID_HELPER"

func TestMain(m *testing.M) {
	if mode := os.Getenv(appIDHelperEnv); mode != "" && len(os.Args) == 2 && os.Args[1] == printAppIDFlag {
		switch mode {
		case "answer":
			fmt.Println("some other output")
			fmt.Println(appIDMarker + "JambExamples.Notes")
		case "refuse":
			fmt.Fprintln(os.Stderr, "flag provided but not defined: -jamb-print-appid")
			os.Exit(2)
		case "hang":
			time.Sleep(time.Minute)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestCanRunHere(t *testing.T) {
	for _, tc := range []struct {
		os, hostArch, arch string
		want               bool
	}{
		{"windows", "amd64", "amd64", true},
		{"windows", "amd64", "arm64", false},
		{"windows", "arm64", "arm64", true},
		{"windows", "arm64", "amd64", true},
		{"linux", "amd64", "amd64", false},
		{"darwin", "arm64", "arm64", false},
	} {
		if got := canRunHere(tc.os, tc.hostArch, tc.arch); got != tc.want {
			t.Errorf("canRunHere(%q, %q, %q) = %v, want %v", tc.os, tc.hostArch, tc.arch, got, tc.want)
		}
	}
}

// The answers are the lines that app's TestPrintAppID expects printAppID to write, with
// what an app or the console may add around them.
func TestParseAppID(t *testing.T) {
	for in, want := range map[string]struct {
		id string
		ok bool
	}{
		"jamb-appid:JambExamples.Notes\n":              {"JambExamples.Notes", true},
		"jamb-appid:\n":                                {"", true},
		"jamb-appid:JambExamples.Notes\r\n":            {"JambExamples.Notes", true},
		"starting\njamb-appid:My_App-1\n":              {"My_App-1", true},
		"jamb-appid:First\njamb-appid:Second\n":        {"First", true},
		"":                                             {"", false},
		"JambExamples.Notes\n":                         {"", false},
		"  jamb-appid:JambExamples.Notes\n":            {"", false},
		"flag provided but not defined: -jamb-print\n": {"", false},
	} {
		if id, ok := parseAppID([]byte(in)); id != want.id || ok != want.ok {
			t.Errorf("parseAppID(%q) = %q, %v, want %q, %v", in, id, ok, want.id, want.ok)
		}
	}
}

func TestAppIDMismatch(t *testing.T) {
	for i, tc := range []struct {
		identityName, appID string
		callSites           []string
		want, notWant       []string
	}{
		{
			"Example.Notes", "Example.Other", []string{"main.go:12"},
			[]string{`App.AppID:    "Example.Other" (main.go:12)`, `identityName: "Example.Notes" (jamb.json:8)`,
				`Change App.AppID("Example.Other") to App.AppID("Example.Notes") at main.go:12`,
				`or change "identityName" in jamb.json:8 to "Example.Other"`},
			nil,
		},
		{
			"Example.Notes", "Example.Other", nil,
			[]string{`App.AppID:    "Example.Other"` + "\n", `to App.AppID("Example.Notes") in the app's Go code`},
			[]string{"main.go"},
		},
		{
			// An AppID with '_' cannot be an identityName, so only the Go side is offered.
			"Example.Notes", "Example_Notes", []string{"main.go:3", "cmd/app/main.go:7"},
			[]string{`at main.go:3, cmd/app/main.go:7`},
			[]string{`change "identityName"`},
		},
		{
			// Case matters: the two must be equal.
			"Example.Notes", "example.notes", nil,
			[]string{`App.AppID("example.notes") to App.AppID("Example.Notes")`},
			nil,
		},
		{
			"Example.Notes", "", nil,
			[]string{"the app sets no AppID", `call App.AppID("Example.Notes")`},
			[]string{"does not match"},
		},
	} {
		got := appIDMismatch(tc.identityName, tc.appID, "jamb.json:8", tc.callSites)
		for _, w := range tc.want {
			if !strings.Contains(got, w) {
				t.Errorf("case %d: the message does not contain %q:\n%s", i, w, got)
			}
		}
		for _, w := range tc.notWant {
			if strings.Contains(got, w) {
				t.Errorf("case %d: the message contains %q:\n%s", i, w, got)
			}
		}
	}
}

func TestLines(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tapp.New().\n\t\tAppID(\"Example.Notes\").\n\t\tRun(w)\n}\n\nconst id = `Example.Notes`\nconst other = \"Example.Notes.Old\"\n")
	if got := fmt.Sprint(literalLines(src, "Example.Notes")); got != "[5 9]" {
		t.Errorf("literalLines = %s, want [5 9]", got)
	}
	cfg := []byte("{\n  \"name\": \"notes\",\n  \"identityName\": \"Example.Notes\"\n}\n")
	if got := jsonKeyLine(cfg, "identityName"); got != 3 {
		t.Errorf("jsonKeyLine = %d, want 3", got)
	}
	if got := jsonKeyLine(cfg, "icon"); got != 0 {
		t.Errorf("jsonKeyLine of a missing key = %d, want 0", got)
	}
	if lineSuffix(3) != ":3" || lineSuffix(0) != "" {
		t.Errorf("lineSuffix(3), lineSuffix(0) = %q, %q", lineSuffix(3), lineSuffix(0))
	}
}

func TestAppIDCallSites(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	call := "package main\n\nvar a = app.New().AppID(\"Example.Notes\")\n"
	write("main.go", call)
	write("internal/ids/ids.go", "package ids\n\nconst ID = `Example.Notes`\n")
	write("main_test.go", call)
	write("dist/old/main.go", call)
	write(".git/main.go", call)
	write("vendor/x/main.go", call)
	write("testdata/main.go", call)
	write("notes.txt", call)
	got := strings.Join(appIDCallSites(dir, "Example.Notes"), " ")
	if got != "internal/ids/ids.go:3 main.go:3" {
		t.Errorf("appIDCallSites = %q, want %q", got, "internal/ids/ids.go:3 main.go:3")
	}
	if sites := appIDCallSites(dir, ""); sites != nil {
		t.Errorf("appIDCallSites for no AppID = %q, want none", sites)
	}
}

func TestNoAnswer(t *testing.T) {
	got := noAnswer(errors.New("exit status 2"), []byte("flag provided but not defined: -jamb-print-appid\nUsage of app:\n"))
	if !strings.Contains(got, "(exit status 2): flag provided but not defined: -jamb-print-appid;") || strings.Contains(got, "Usage") {
		t.Errorf("noAnswer = %q", got)
	}
	if got := noAnswer(nil, nil); !strings.HasPrefix(got, "the app did not report its AppID; ") {
		t.Errorf("noAnswer with no error and no output = %q", got)
	}
}

// The process tests run the test binary, which TestMain turns into a console program
// that answers, refuses or hangs. No built app is run.
func TestRunForAppID(t *testing.T) {
	if !canRunHere(runtime.GOOS, runtime.GOARCH, runtime.GOARCH) {
		t.Skip("the AppID check runs an app only on Windows")
	}
	t.Setenv(appIDHelperEnv, "answer")
	if id, problem := runForAppID(os.Args[0], appIDTimeout); id != "JambExamples.Notes" || problem != "" {
		t.Errorf("an app that answers: runForAppID = %q, %q", id, problem)
	}
	t.Setenv(appIDHelperEnv, "refuse")
	if id, problem := runForAppID(os.Args[0], appIDTimeout); id != "" || !strings.Contains(problem, "did not report its AppID (exit status 2): flag provided") {
		t.Errorf("an app that refuses the flag: runForAppID = %q, %q", id, problem)
	}
	t.Setenv(appIDHelperEnv, "hang")
	start := time.Now()
	if id, problem := runForAppID(os.Args[0], 300*time.Millisecond); id != "" || !strings.Contains(problem, "did not answer --jamb-print-appid within 300ms and was stopped") {
		t.Errorf("an app that hangs: runForAppID = %q, %q", id, problem)
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Errorf("runForAppID waited %v for an app that hangs", d)
	}
}

func TestTakesFlag(t *testing.T) {
	dir := t.TempDir()
	with, without := filepath.Join(dir, "with.exe"), filepath.Join(dir, "without.exe")
	if err := os.WriteFile(with, []byte("MZ\x00"+printAppIDFlag+"\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(without, []byte("MZ\x00"+stampFlag+"\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !takesFlag(with, printAppIDFlag) || takesFlag(without, printAppIDFlag) || takesFlag(filepath.Join(dir, "missing.exe"), printAppIDFlag) {
		t.Error("takesFlag does not tell an app that takes the flag from one that does not")
	}
}
