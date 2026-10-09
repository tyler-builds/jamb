package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindSDKToolInGivenDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "makeappx.exe"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := findSDKTool("makeappx.exe", dir)
	if err != nil || p != filepath.Join(dir, "makeappx.exe") {
		t.Errorf("findSDKTool = %q, %v", p, err)
	}
	_, err = findSDKTool("signtool.exe", dir)
	if err == nil || !strings.Contains(err.Error(), "signtool.exe") {
		t.Errorf("missing tool: err = %v; want it named", err)
	}
}

func TestFindSDKToolWithoutSDKSaysHowToGetIt(t *testing.T) {
	t.Setenv("ProgramFiles(x86)", t.TempDir())
	_, err := findSDKTool("makeappx.exe", "")
	if err == nil || !strings.Contains(err.Error(), "Windows SDK") || !strings.Contains(err.Error(), "--sdk-bin") {
		t.Errorf("err = %v; want the SDK install hint", err)
	}
}

func TestVersionLessIsNumeric(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"10.0.9.0", "10.0.26100.0", true},
		{"10.0.26100.0", "10.0.9.0", false},
		{"10.0.19041.0", "10.0.19041.0", false},
		{"10.0.19041", "10.0.19041.0", true},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%q, %q) = %v; want %v", c.a, c.b, got, c.want)
		}
	}
}
