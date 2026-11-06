package app

import (
	"encoding/json"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// TestHeadlessRunChild is the app that TestHeadlessRun runs in a child
// process, since a process runs one app. It does nothing in a normal run.
func TestHeadlessRunChild(t *testing.T) {
	if os.Getenv("JAMB_HEADLESS_TEST_CHILD") != "1" {
		t.Skip("run by TestHeadlessRun")
	}
	clicks := ui.NewSignal(0)
	New().Run(Window{Title: "Headless", Width: 400, Height: 300,
		Root: func(ctx *ui.Context) ui.Element {
			return ui.Div().Column().Padding(20).Gap(10).
				Child(ui.Text("Headless")).
				Child(ui.Button("Clicks: " + string(rune('0'+clicks.Get()))).Key("count").
					OnClick(func() { clicks.Update(func(n int) int { return n + 1 }) }))
		}})
}

// An app runs with no window on the screen under JAMB_HEADLESS=1: its script
// clicks a button by key, and the screenshot and the tree show the result.
func TestHeadlessRun(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process")
	}
	dir := t.TempDir()
	shot, tree := filepath.Join(dir, "after.png"), filepath.Join(dir, "tree.json")
	cmd := exec.Command(os.Args[0], "-test.run=^TestHeadlessRunChild$", "-test.v")
	cmd.Env = append(os.Environ(),
		"JAMB_HEADLESS_TEST_CHILD=1", "JAMB_HEADLESS=1", "JAMB_HEADLESS_SCALE=2",
		// No display: the run must not need one.
		"WAYLAND_DISPLAY=", "DISPLAY=",
		"JAMB_SCRIPT=idle; click #count; click #count; idle; shot "+shot+"; tree "+tree+"; exit")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("headless run: %v\n%s", err, out)
	}

	f, err := os.Open(shot)
	if err != nil {
		t.Fatalf("no screenshot: %v\n%s", err, out)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 600 {
		t.Errorf("screenshot is %v, want 800×600 at scale 2", b.Size())
	}

	data, err := os.ReadFile(tree)
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Nodes []ctlNode }
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range got.Nodes {
		names = append(names, n.Name)
		if n.Key == "count" && n.Name == "Clicks: 2" {
			if n.Bounds.W <= 0 || n.Bounds.W > 400 {
				t.Errorf("button bounds %+v are not in logical pixels", n.Bounds)
			}
			return
		}
	}
	t.Errorf("no button named \"Clicks: 2\" in the tree: %s", strings.Join(names, ", "))
}
