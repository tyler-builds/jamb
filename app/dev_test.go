package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
)

func TestParsePosition(t *testing.T) {
	cases := []struct {
		s    string
		want platform.Point
		ok   bool
	}{
		{"-1600,200", platform.Point{X: -1600, Y: 200}, true},
		{" 10 , -20 ", platform.Point{X: 10, Y: -20}, true},
		{"0,0", platform.Point{}, true},
		{"100", platform.Point{}, false},
		{"100,", platform.Point{}, false},
		{"1.5,2", platform.Point{}, false},
		{"", platform.Point{}, false},
	}
	for _, c := range cases {
		p, ok := parsePosition(c.s)
		if ok != c.ok || p != c.want {
			t.Errorf("parsePosition(%q) = %v, %v; want %v, %v", c.s, p, ok, c.want, c.ok)
		}
	}
}

// TestGeometryFileKeepsNegativePosition checks that a saved position left of or above
// the primary monitor is read back as saved, not clamped or rejected.
func TestGeometryFileKeepsNegativePosition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "geometry.txt")
	if err := os.WriteFile(path, []byte("-3200 -400 800 600"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := devOptions{geometry: path}.loadGeometry()
	if want := (platform.Rect{X: -3200, Y: -400, W: 800, H: 600}); got != want {
		t.Errorf("loadGeometry = %v, want %v", got, want)
	}
}

func TestParseKey(t *testing.T) {
	cases := []struct {
		spec string
		key  input.Key
		mods input.Modifiers
	}{
		{"enter", input.KeyEnter, 0},
		{"z", input.KeyZ, 0},
		{"ctrl+z", input.KeyZ, input.ModCtrl},
		{"Ctrl+Shift+Left", input.KeyLeft, input.ModCtrl | input.ModShift},
		{"0", input.Key0, 0},
		{"ctrl+5", input.Key5, input.ModCtrl},
		{"9", input.Key9, 0},
		{"f", input.KeyF, 0},
		{"f1", input.KeyF1, 0},
		{"alt+f12", input.KeyF12, input.ModAlt},
		{"insert", input.KeyInsert, 0},
		{"super+d", input.KeyD, input.ModSuper},
		{"ctrl+plus", input.KeyEqual, input.ModCtrl},
		{"menu", input.KeyMenu, 0},
		{"shift+f10", input.KeyF10, input.ModShift},
	}
	for _, c := range cases {
		k, mods, ok := parseKey(c.spec)
		if !ok || k != c.key || mods != c.mods {
			t.Errorf("parseKey(%q) = %v, %v, %v; want %v, %v, true", c.spec, k, mods, ok, c.key, c.mods)
		}
	}
}

func TestParseKeyRejectsUnknown(t *testing.T) {
	for _, spec := range []string{"cmd+k", "ctrl+", "f0", "f13", "10", "ctrl+enterr", "+"} {
		if _, _, ok := parseKey(spec); ok {
			t.Errorf("parseKey(%q) succeeded; want an error", spec)
		}
	}
}

// rclick is a right-button press at a point, like click but for the right
// button; it needs both coordinates.
func TestScriptReadsRightClick(t *testing.T) {
	s, err := parseScript("idle; rclick 120 240; exit")
	if err != nil {
		t.Fatalf("rclick 120 240: %v", err)
	}
	if st := s.steps[1]; st.op != "rclick" || st.x != 120 || st.y != 240 {
		t.Errorf("step = %+v; want rclick at 120, 240", st)
	}
	if _, err := parseScript("rclick 120; exit"); err == nil || !strings.Contains(err.Error(), "missing number") {
		t.Errorf("rclick 120: err = %v; want a missing-number error", err)
	}
}

// A key the script cannot press stops the script when it is read, rather
// than being skipped while the script runs.
func TestScriptRejectsUnknownKey(t *testing.T) {
	if _, err := parseScript("idle; key ctrl+4; exit"); err != nil {
		t.Fatalf("ctrl+4: %v", err)
	}
	_, err := parseScript("idle; key cmd+k; exit")
	if err == nil || !strings.Contains(err.Error(), "cmd+k") {
		t.Fatalf("cmd+k: err = %v; want an error naming the key", err)
	}
}

// type keeps its text exactly after the one space that follows the step name:
// the leading spaces of the text are typed, and so are trailing ones.
func TestScriptTypeKeepsText(t *testing.T) {
	cases := []struct{ src, want string }{
		{"type  and after", " and after"},
		{"type hi", "hi"},
		{"type hi ", "hi "},
		{"idle; type a b ; exit", "a b "},
		{"type   three", "  three"},
	}
	for _, c := range cases {
		s, err := parseScript(c.src)
		if err != nil {
			t.Fatalf("parseScript(%q): %v", c.src, err)
		}
		var got string
		for _, st := range s.steps {
			if st.op == "type" {
				got = st.arg
			}
		}
		if got != c.want {
			t.Errorf("parseScript(%q): type text %q, want %q", c.src, got, c.want)
		}
	}
	if _, err := parseScript("type"); err == nil {
		t.Error("a type step with no text parsed; want an error")
	}
}

func TestParseScriptReadsLoops(t *testing.T) {
	s, err := parseScript("click 1 2; mark; wait 5; repeat 3; mark; repeat 0")
	if err != nil {
		t.Fatalf("parseScript: %v", err)
	}
	var ops []string
	for _, st := range s.steps {
		ops = append(ops, st.op)
	}
	if got, want := strings.Join(ops, " "), "click mark wait repeat mark repeat"; got != want {
		t.Errorf("steps = %q, want %q", got, want)
	}
	if s.steps[3].n != 3 || s.steps[5].n != 0 {
		t.Errorf("repeat counts = %v, %v; want 3 and 0", s.steps[3].n, s.steps[5].n)
	}
}

func TestParseScriptRejectsBadLoops(t *testing.T) {
	for _, src := range []string{
		"repeat 2",          // no mark before it
		"repeat 2; mark",    // the mark comes after
		"mark; repeat",      // no count
		"mark; repeat -1",   // negative
		"mark; repeat 1.5",  // not whole
		"mark; repeat many", // not a number
	} {
		if _, err := parseScript(src); err == nil {
			t.Errorf("parseScript(%q) succeeded; want an error", src)
		}
	}
	if _, err := parseScript("mark; idle; repeat 1"); err != nil {
		t.Errorf("a repeat after a mark and an idle: %v", err)
	}
}

// TestRepeatAgainCountsTheLoops checks that repeat N jumps back N times, and that a
// repeat is forgotten once its count runs out, so an enclosing loop starts it again.
func TestRepeatAgainCountsTheLoops(t *testing.T) {
	s, err := parseScript("mark; repeat 2")
	if err != nil {
		t.Fatal(err)
	}
	var jumps []bool
	for range 3 {
		jumps = append(jumps, s.repeatAgain(1, 2))
	}
	if want := []bool{true, true, false}; !slices.Equal(jumps, want) {
		t.Errorf("repeat 2 jumped %v, want %v", jumps, want)
	}
	if len(s.repeats) != 0 {
		t.Errorf("an exhausted repeat is still remembered: %v", s.repeats)
	}
	for range 5 {
		if !s.repeatAgain(1, 0) {
			t.Fatal("repeat 0 stopped jumping")
		}
	}
	if len(s.repeats) != 0 {
		t.Errorf("repeat 0 is remembered: %v", s.repeats)
	}
}
