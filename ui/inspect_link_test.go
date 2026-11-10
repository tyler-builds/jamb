package ui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A program that attaches no inspector links none of the functions that record
// the layers of a style or paint the inspector's marks: the rest of the package
// calls them through inspecting, which only Attach sets (see inspectFuncs). The
// program in testdata draws a button, whose style is merged from layers.
func TestInspectorFunctionsStayOutOfABinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a program")
	}
	out := filepath.Join(t.TempDir(), "plain")
	cmd := exec.Command("go", "build", "-ldflags", "-s -w", "-o", out, "./testdata/plain")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, msg)
	}
	bin, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	const pkg = "github.com/tyler-builds/jamb/ui."
	if !bytes.Contains(bin, []byte(pkg+"(*Runtime).Frame")) {
		t.Fatal("the binary does not hold the names of its functions, so the test checks nothing")
	}
	for _, fn := range []string{
		"tagLayer", "tagStyle", "nameStateFields", "renameLayers", "mergeProv", "formatStyleValue",
		"(*Runtime).paintInspect", "(*runtimeHost).Attach",
	} {
		if bytes.Contains(bin, []byte(pkg+fn)) {
			t.Errorf("the binary holds %s, though it attaches no inspector", fn)
		}
	}
}
