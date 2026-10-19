package text

import (
	"runtime"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

// countFaces returns the number of entries in the faces map.
func countFaces() int {
	n := 0
	faces.Range(func(any, any) bool { n++; return true })
	return n
}

// A font that nothing uses any more is freed, and its entry leaves the faces
// map. The map held every font strongly before, so each System's fonts (one
// System per window, and per runtime in the tests) stayed for the life of the
// process: the ui tests grew to about 8 GB.
func TestUnusedFontsAreFreed(t *testing.T) {
	f, err := ParseFont("Go Regular", goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	if fontOf(f.face) != f {
		t.Fatal("fontOf does not find the font of its face")
	}
	before := countFaces()
	f = nil
	for i := 0; i < 50 && countFaces() >= before; i++ {
		runtime.GC()
		runtime.Gosched()
	}
	if after := countFaces(); after >= before {
		t.Fatalf("the faces map still has %d entries after the font was dropped, want fewer than %d", after, before)
	}
}

// A System that is dropped after laying out text gives its fonts back.
func TestDroppedSystemFreesItsFonts(t *testing.T) {
	var ms runtime.MemStats
	heap := func() uint64 {
		runtime.GC()
		runtime.GC()
		runtime.ReadMemStats(&ms)
		return ms.HeapInuse
	}
	layout := func() {
		sys := NewSystem()
		sys.Layout("Hello, world", Style{Family: FamilySans, Size: 14}, 200)
	}
	layout() // the first System also fills the process-wide caches
	base := heap()
	for range 10 {
		layout()
	}
	if grown := int64(heap()) - int64(base); grown > 20<<20 {
		t.Fatalf("heap grew by %d MB over 10 dropped Systems, want it about flat", grown>>20)
	}
}
