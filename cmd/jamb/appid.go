package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// printAppIDFlag is the internal flag with which a jamb app writes its AppID to stdout
// and exits, before it opens anything (see printAppIDFlag in app/identity.go, which this
// copies, since cmd/jamb is a module of its own).
const printAppIDFlag = "--jamb-print-appid"

// appIDMarker starts the line of the app's answer to printAppIDFlag, followed by the
// AppID; an app without an AppID writes the marker alone. It copies appIDMarker in
// app/identity.go, and TestParseAppID checks the lines the app's test expects.
const appIDMarker = "jamb-appid:"

// appIDTimeout is how long the built app has to answer printAppIDFlag. The app answers
// before it starts anything, so it needs a fraction of a second; the rest allows for a
// first run of a new executable, which an antivirus may scan.
const appIDTimeout = 10 * time.Second

// checkAppID runs the built app with printAppIDFlag and compares its AppID with
// identityName in jamb.json. A mismatch is an error when strict is set, which is when
// an installer or an MSIX is built, since both are keyed by identityName; for a zip
// alone it is a warning. When the check cannot be made, it is skipped with a warning or
// a note, and packaging goes on.
func checkAppID(dir, exe, arch string, cfg *config, strict bool) error {
	if !canRunHere(runtime.GOOS, runtime.GOARCH, arch) {
		pkgf("warning: the AppID check is skipped, since a windows/%s executable cannot run on this %s/%s machine; make sure the app calls App.AppID(%q), the identityName of jamb.json", arch, runtime.GOOS, runtime.GOARCH, cfg.IdentityName)
		return nil
	}
	if !takesFlag(exe, printAppIDFlag) {
		pkgf("note: the AppID check is skipped, since the app is built with a jamb version that cannot report its AppID; update the app's jamb dependency to have App.AppID checked against identityName")
		return nil
	}
	appID, problem := runForAppID(exe, appIDTimeout)
	if problem != "" {
		pkgf("warning: the AppID check is skipped: %s; make sure the app calls App.AppID(%q), the identityName of jamb.json", problem, cfg.IdentityName)
		return nil
	}
	if appID == cfg.IdentityName {
		return nil
	}
	data, _ := os.ReadFile(filepath.Join(dir, configFile))
	msg := appIDMismatch(cfg.IdentityName, appID, configFile+lineSuffix(jsonKeyLine(data, "identityName")), appIDCallSites(dir, appID))
	if appID == "" || !strict {
		pkgf("warning: %s", msg)
		return nil
	}
	return errors.New(msg)
}

// canRunHere reports whether a windows/arch executable runs on a hostOS/hostArch
// machine: Windows on the same architecture, or an amd64 executable on Windows on Arm,
// which emulates it.
func canRunHere(hostOS, hostArch, arch string) bool {
	return hostOS == "windows" && (arch == hostArch || (hostArch == "arm64" && arch == "amd64"))
}

// runForAppID runs the executable with printAppIDFlag, with no console and no window,
// and returns the AppID it reports. When it does not report one, problem says why. An
// app that does not answer within timeout is stopped.
func runForAppID(exe string, timeout time.Duration) (appID, problem string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, printAppIDFlag)
	cmd.Dir = filepath.Dir(exe)
	cmd.WaitDelay = time.Second
	hideProcess(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Sprintf("the app did not answer %s within %v and was stopped", printAppIDFlag, timeout)
	}
	if id, ok := parseAppID(stdout.Bytes()); ok {
		return id, ""
	}
	return "", noAnswer(err, stderr.Bytes())
}

// noAnswer describes a run that did not report an AppID: the error of the run, if any,
// and the first line the app wrote to stderr. An app that parses its own command line
// before App.Run can refuse the flag this way.
func noAnswer(err error, stderr []byte) string {
	s := "the app did not report its AppID"
	if err != nil {
		s += " (" + err.Error() + ")"
	}
	if line, _, _ := strings.Cut(strings.TrimSpace(string(stderr)), "\n"); line != "" {
		s += ": " + strings.TrimSpace(line)
	}
	return s + "; an app that reads its own command-line flags before App.Run may refuse " + printAppIDFlag
}

// parseAppID returns the AppID in the app's answer to printAppIDFlag: the rest of the
// first line that starts with appIDMarker. ok is false when no line does, as for an app
// that did not understand the flag.
func parseAppID(out []byte) (appID string, ok bool) {
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if id, found := strings.CutPrefix(strings.TrimRight(sc.Text(), "\r"), appIDMarker); found {
			return strings.TrimSpace(id), true
		}
	}
	return "", false
}

// appIDMismatch describes an AppID that differs from identityName, and how to make the
// two equal. configAt is where identityName is, such as "jamb.json:8", and callSites are
// the places in the app's source that hold appID as a string literal. When the app has
// no AppID, it says to add one. When appID could not be an identityName, only the Go
// side is offered as the fix.
func appIDMismatch(identityName, appID, configAt string, callSites []string) string {
	if appID == "" {
		return fmt.Sprintf("the app sets no AppID, and identityName in %s is %q. The installer gives its Start menu shortcut that ID, and the app needs one for notifications, notification clicks, jump lists and a single instance: call App.AppID(%q) on the app before Run", configAt, identityName, identityName)
	}
	goAt, fixAt := "", " in the app's Go code"
	if len(callSites) > 0 {
		goAt = " (" + strings.Join(callSites, ", ") + ")"
		fixAt = " at " + strings.Join(callSites, ", ")
	}
	s := fmt.Sprintf("the app's AppID does not match identityName in jamb.json:\n  App.AppID:    %q%s\n  identityName: %q (%s)\n"+
		"The installer and the MSIX use identityName for the Start menu shortcut and the notification protocol, so while the two differ, the installed app's notifications, notification clicks and jump lists do not work. Change App.AppID(%q) to App.AppID(%q)%s",
		appID, goAt, identityName, configAt, appID, identityName, fixAt)
	if identityNamePattern.MatchString(appID) {
		s += fmt.Sprintf(`, or change "identityName" in %s to %q`, configAt, appID)
	}
	return s
}

// lineSuffix returns ":N" for a line number, and "" when the line is not known (0).
func lineSuffix(line int) string {
	if line <= 0 {
		return ""
	}
	return ":" + strconv.Itoa(line)
}

// jsonKeyLine returns the 1-based line of data on which the JSON key first appears, or
// 0 when it does not.
func jsonKeyLine(data []byte, key string) int {
	return firstLine(data, strconv.Quote(key))
}

// firstLine returns the 1-based line of data on which s first appears, or 0.
func firstLine(data []byte, s string) int {
	i := bytes.Index(data, []byte(s))
	if i < 0 {
		return 0
	}
	return bytes.Count(data[:i], []byte("\n")) + 1
}

// literalLines returns the 1-based lines of Go source src that hold s as a string
// literal, interpreted ("s") or raw (`s`).
func literalLines(src []byte, s string) []int {
	var lines []int
	for n, line := range bytes.Split(src, []byte("\n")) {
		if bytes.Contains(line, []byte(strconv.Quote(s))) || bytes.Contains(line, []byte("`"+s+"`")) {
			lines = append(lines, n+1)
		}
	}
	return lines
}

// maxCallSites is how many places appIDCallSites reports.
const maxCallSites = 3

// appIDCallSites returns the places, as path:line relative to dir, where the Go files of
// the app's directory and its subdirectories hold appID as a string literal, such as in
// App.AppID("JambExamples.Notes"). Tests, dist and hidden, vendor and testdata folders
// are skipped. It is a hint for the message, so a file that cannot be read is skipped.
func appIDCallSites(dir, appID string) []string {
	if appID == "" {
		return nil
	}
	var sites []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(sites) >= maxCallSites {
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (strings.HasPrefix(name, ".") || name == "dist" || name == "vendor" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			rel = path
		}
		for _, n := range literalLines(src, appID) {
			if len(sites) < maxCallSites {
				sites = append(sites, filepath.ToSlash(rel)+lineSuffix(n))
			}
		}
		return nil
	})
	return sites
}
