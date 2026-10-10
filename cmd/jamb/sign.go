package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// signCommandEnv names the environment variable that holds the sign command when
// --sign-command is not given. The command is kept out of jamb.json, because the
// certificate it names differs between machines and CI runners.
const signCommandEnv = "JAMB_SIGN_COMMAND"

// filePlaceholder is replaced by the path of each file that the sign command signs.
const filePlaceholder = "{file}"

// signer runs the sign command once for each file. The command is split into
// arguments the way a Windows command line is split, and the program is started
// directly, with no shell, so a file name cannot inject a command.
type signer struct {
	program string
	args    []string
}

// newSigner parses a sign command such as
//
//	"C:\Program Files (x86)\Windows Kits\10\bin\10.0.26100.0\x64\signtool.exe" sign /fd SHA256 /sha1 THUMBPRINT {file}
//
// Arguments are separated by spaces or tabs. A double quote groups the characters
// inside it and is removed. Backslashes are kept, since they are Windows paths. The
// command must name a program, and {file} must appear in at least one argument.
func newSigner(command string) (*signer, error) {
	argv, err := splitCommand(command)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, errors.New("the sign command is empty; give the signing program and its arguments, with " + filePlaceholder + " for the file to sign")
	}
	if strings.Contains(argv[0], filePlaceholder) {
		return nil, errors.New(filePlaceholder + " must be in an argument, not in the program name")
	}
	switch strings.ToLower(filepath.Ext(argv[0])) {
	case ".bat", ".cmd":
		return nil, errors.New("a .bat or .cmd program runs through cmd.exe, which reads characters such as & and % in the file path; use an executable, or a .ps1 script run by powershell")
	}
	s := &signer{program: argv[0], args: argv[1:]}
	for _, a := range s.args {
		if strings.Contains(a, filePlaceholder) {
			return s, nil
		}
	}
	return nil, errors.New("the sign command has no " + filePlaceholder + "; it must pass the path of the file to sign, for example as the last argument")
}

// splitCommand splits command into arguments. It fails on an unmatched double
// quote.
func splitCommand(command string) ([]string, error) {
	var argv []string
	var cur strings.Builder
	inArg, quoted := false, false
	for _, r := range command {
		switch {
		case r == '"':
			quoted = !quoted
			inArg = true
		case (r == ' ' || r == '\t') && !quoted:
			if inArg {
				argv = append(argv, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quoted {
		return nil, errors.New(`the sign command has an unmatched double quote`)
	}
	if inArg {
		argv = append(argv, cur.String())
	}
	return argv, nil
}

// sign runs the sign command for file. The command's output goes to the console,
// so a signer that asks for a PIN can be answered. An error means the caller must
// stop packaging. The command itself is not printed, because it may name secrets.
func (s *signer) sign(file string) error {
	pkgf("signing %s", filepath.Base(file))
	args := make([]string, len(s.args))
	for i, a := range s.args {
		args[i] = strings.ReplaceAll(a, filePlaceholder, file)
	}
	cmd := exec.Command(s.program, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("signing %s failed: %v; nothing is written to dist", filepath.Base(file), err)
	}
	return nil
}
