package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The sign command in these tests is the test binary itself. TestSignHelper
// appends the file it is given to the log named by signLogEnv. It exits 1 when
// the file name ends with the suffix in signFailEnv.
const (
	signLogEnv  = "JAMB_TEST_SIGN_LOG"
	signFailEnv = "JAMB_TEST_SIGN_FAIL"
)

// TestSignHelper is not a test of its own. It runs only as the sign command of the
// tests below, which start the test binary with -test.run=^TestSignHelper$.
func TestSignHelper(t *testing.T) {
	logPath := os.Getenv(signLogEnv)
	if logPath == "" {
		t.Skip("runs only as the sign command of the other tests")
	}
	file := os.Args[len(os.Args)-1]
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, file)
	f.Close()
	if fail := os.Getenv(signFailEnv); fail != "" && strings.HasSuffix(file, fail) {
		os.Exit(1)
	}
}

// helperSignCommand returns a sign command that runs the test binary as the signer,
// with {file} as its last argument. The log is kept in a temporary directory.
func helperSignCommand(t *testing.T) (command, logPath string) {
	t.Helper()
	logPath = filepath.Join(t.TempDir(), "sign.log")
	t.Setenv(signLogEnv, logPath)
	return fmt.Sprintf(`"%s" -test.run=^TestSignHelper$ %s`, os.Args[0], filePlaceholder), logPath
}

// signedFiles returns the base names of the files the helper signer was given, in
// order.
func signedFiles(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		names = append(names, filepath.Base(line))
	}
	return names
}

func TestNewSignerParsesQuotedProgramAndPlaceholder(t *testing.T) {
	s, err := newSigner(`"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe" sign /fd SHA256  /sha1 ABC /f:{file}`)
	if err != nil {
		t.Fatal(err)
	}
	if s.program != `C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe` {
		t.Errorf("program = %q", s.program)
	}
	want := []string{"sign", "/fd", "SHA256", "/sha1", "ABC", "/f:{file}"}
	if strings.Join(s.args, "|") != strings.Join(want, "|") {
		t.Errorf("args = %q; want %q", s.args, want)
	}
}

func TestNewSignerRefusesBadCommands(t *testing.T) {
	for _, tc := range []struct {
		name, command, want string
	}{
		{"empty", "   ", "is empty"},
		{"no placeholder", "signtool sign /fd SHA256", "has no {file}"},
		{"placeholder in program", "{file} sign", "not in the program name"},
		{"unmatched quote", `"C:\signtool.exe sign {file}`, "unmatched double quote"},
		{"batch file", `C:\sign.cmd {file}`, "cmd.exe"},
	} {
		if _, err := newSigner(tc.command); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v; want it to contain %q", tc.name, err, tc.want)
		}
	}
}

func TestSignerRunsOncePerFileWithTheFilePassedAsOneArgument(t *testing.T) {
	command, logPath := helperSignCommand(t)
	s, err := newSigner(command)
	if err != nil {
		t.Fatal(err)
	}
	// A path with spaces and an ampersand is passed as one argument, not split or
	// interpreted by a shell.
	dir := filepath.Join(t.TempDir(), "R&D files")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "one.exe")
	second := filepath.Join(dir, "two.msix")
	if err := s.sign(first); err != nil {
		t.Fatal(err)
	}
	if err := s.sign(second); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || lines[0] != first || lines[1] != second {
		t.Errorf("signer was called with %q; want %q then %q", lines, first, second)
	}
}

func TestSignerFailureIsReturned(t *testing.T) {
	command, _ := helperSignCommand(t)
	t.Setenv(signFailEnv, ".exe")
	s, err := newSigner(command)
	if err != nil {
		t.Fatal(err)
	}
	err = s.sign(filepath.Join(t.TempDir(), "app.exe"))
	if err == nil || !strings.Contains(err.Error(), "signing app.exe failed") {
		t.Errorf("err = %v; want a failure naming the file", err)
	}
}

func TestSignerErrorDoesNotShowCommand(t *testing.T) {
	const secret = "THUMBPRINT-SECRET-123"
	t.Setenv(signLogEnv, filepath.Join(t.TempDir(), "sign.log"))
	t.Setenv(signFailEnv, ".exe")
	s, err := newSigner(fmt.Sprintf(`"%s" -test.run=^TestSignHelper$ /sha1 %s {file}`, os.Args[0], secret))
	if err != nil {
		t.Fatal(err)
	}
	err = s.sign(filepath.Join(t.TempDir(), "app.exe"))
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("err = %v; want a failure that does not repeat the command", err)
	}
}
