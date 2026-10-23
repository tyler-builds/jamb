package icons

import (
	"bytes"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// The SVG of an icon is parsed on the first Rasterize, not when the icon is
// declared, so that the set costs an app nothing for the icons it never draws.
func TestIconParsesOnFirstRasterize(t *testing.T) {
	ic := &Icon{Name: "fresh", src: bellSrc}
	if ic.doc != nil {
		t.Fatal("the icon was parsed before its first Rasterize")
	}
	ic.Rasterize(image.NewAlpha(image.Rect(0, 0, 16, 16)))
	if ic.doc == nil {
		t.Error("the icon was not parsed by its first Rasterize")
	}
}

// The first Rasterize may run on several goroutines at once; the icon is still
// parsed once, and every call draws the same mask.
func TestIconRasterizeConcurrently(t *testing.T) {
	ic := &Icon{Name: "concurrent", src: bellSrc}
	want := image.NewAlpha(image.Rect(0, 0, 16, 16))
	Bell.Rasterize(want)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := image.NewAlpha(image.Rect(0, 0, 16, 16))
			ic.Rasterize(got)
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Error("a concurrent Rasterize drew a different mask")
			}
		}()
	}
	wg.Wait()
}

// A zero Icon has no source and draws nothing, without a panic.
func TestZeroIconDrawsNothing(t *testing.T) {
	dst := image.NewAlpha(image.Rect(0, 0, 8, 8))
	var ic Icon
	ic.Rasterize(dst)
	for _, a := range dst.Pix {
		if a != 0 {
			t.Fatal("a zero Icon drew coverage")
		}
	}
}

// Every icon parses and draws, and the names are unique, since the text cache
// keys its masks by name.
func TestEveryIconDraws(t *testing.T) {
	names := map[string]bool{}
	for _, ic := range All() {
		if names[ic.Name] {
			t.Errorf("two icons are named %q", ic.Name)
		}
		names[ic.Name] = true
		if ic.document() == nil {
			t.Errorf("%s has no parsed SVG", ic.Name)
			continue
		}
		dst := image.NewAlpha(image.Rect(0, 0, 24, 24))
		ic.Rasterize(dst)
		covered := 0
		for _, a := range dst.Pix {
			if a > 0 {
				covered++
			}
		}
		if covered == 0 {
			t.Errorf("%s drew no coverage", ic.Name)
		}
	}
}

// The strokes are 2 units wide on the 24 grid: Minus is a line at y = 12, so
// rows 11 and 12 are covered and rows 10 and 13 are not.
func TestStrokeIsTwoUnits(t *testing.T) {
	dst := image.NewAlpha(image.Rect(0, 0, 24, 24))
	Minus.Rasterize(dst)
	at := func(x, y int) uint8 { return dst.Pix[y*dst.Stride+x] }
	if at(12, 11) < 250 || at(12, 12) < 250 {
		t.Errorf("rows 11 and 12 at x = 12 are %d and %d, want full coverage", at(12, 11), at(12, 12))
	}
	if at(12, 10) != 0 || at(12, 13) != 0 {
		t.Errorf("rows 10 and 13 at x = 12 are %d and %d, want none", at(12, 10), at(12, 13))
	}
}

// A zero-length segment, as in Lucide's dots, draws a round dot.
func TestZeroLengthSegmentDraws(t *testing.T) {
	dst := image.NewAlpha(image.Rect(0, 0, 24, 24))
	CircleAlert.Rasterize(dst)
	if dst.Pix[16*dst.Stride+12] == 0 {
		t.Error("the dot of circle-alert at (12, 16) is not drawn")
	}
}

// The names from before Lucide point at the same icons.
func TestAliasesPointAtLucideIcons(t *testing.T) {
	for _, c := range []struct {
		old, lucide *Icon
		name        string
	}{
		{Alert, TriangleAlert, "triangle-alert"},
		{Dashboard, LayoutDashboard, "layout-dashboard"},
		{Filter, Funnel, "funnel"},
		{Home, House, "house"},
		{MoreHorizontal, Ellipsis, "ellipsis"},
		{Refresh, RotateCw, "rotate-cw"},
	} {
		if c.old != c.lucide || c.lucide.Name != c.name {
			t.Errorf("the alias for %s is not the Lucide icon %s", c.lucide.Name, c.name)
		}
	}
}

// An icon that a program does not name is not linked into its binary. The
// program in testdata names Bell only; the binary holds Bell's SVG and not the
// SVG of other icons, including an alias's target.
func TestUnnamedIconsStayOutOfABinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a program")
	}
	out := filepath.Join(t.TempDir(), "onlybell")
	cmd := exec.Command("go", "build", "-ldflags", "-s -w", "-o", out, "./testdata/onlybell")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, msg)
	}
	bin, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(bin, []byte(Bell.src)) {
		t.Error("the binary does not hold the SVG of bell, which it names")
	}
	for _, ic := range []*Icon{Moon, Funnel, Sun, StarFilled} {
		if bytes.Contains(bin, []byte(ic.src)) {
			t.Errorf("the binary holds the SVG of %s, which it does not name", ic.Name)
		}
	}
}
