package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// sdkVersionPattern matches the version directories under Windows Kits\10\bin.
var sdkVersionPattern = regexp.MustCompile(`^\d+(\.\d+)+$`)

// findSDKTool returns the path of a Windows SDK tool such as makeappx.exe. When
// dir is set, the tool must be in it. Otherwise the newest
// installed SDK is searched, for this machine's architecture first.
func findSDKTool(name, dir string) (string, error) {
	if dir != "" {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s not found in --sdk-bin %s", name, dir)
		}
		return p, nil
	}
	root := filepath.Join(programFilesX86(), "Windows Kits", "10", "bin")
	entries, _ := os.ReadDir(root)
	var versions []string
	for _, e := range entries {
		if e.IsDir() && sdkVersionPattern.MatchString(e.Name()) {
			versions = append(versions, e.Name())
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versionLess(versions[j], versions[i]) })

	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	for _, v := range versions {
		for _, a := range []string{arch, "x64"} {
			p := filepath.Join(root, v, a, name)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found; install the Windows SDK (https://developer.microsoft.com/windows/downloads/windows-sdk/) with the \"Windows SDK Signing Tools for Desktop Apps\" feature, or pass --sdk-bin", name)
}

// programFilesX86 returns the 32-bit Program Files directory, where the
// Windows SDK installs.
func programFilesX86() string {
	if p := os.Getenv("ProgramFiles(x86)"); p != "" {
		return p
	}
	return `C:\Program Files (x86)`
}

// versionLess compares two dotted version numbers part by part as integers.
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			return na < nb
		}
	}
	return len(pa) < len(pb)
}

// runTool runs an SDK tool and waits for it. Its output is returned in the
// error only when it fails. Secrets must never be passed as arguments, because
// the command line is visible to other processes.
func runTool(path string, args ...string) error {
	var out bytes.Buffer
	cmd := exec.Command(path, args...)
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %v\n%s", filepath.Base(path), err, strings.TrimSpace(out.String()))
	}
	return nil
}
