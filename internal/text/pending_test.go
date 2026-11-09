package text

import (
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// A face still on its way (as a page fetches it, see fonts_fetch.go) is passed
// over for the nearest face there is, and used once it has arrived and the
// system forgot the faces it picked.
func TestPendingFaceUsedOnceArrived(t *testing.T) {
	sys := NewSystem()
	r := sys.Registry()
	arrived := false
	r.fam("T").sources[faceKey{Regular, false}] = &lazyFont{load: func() (*Font, error) { return ParseFont("regular", goregular.TTF) }}
	r.fam("T").sources[faceKey{Bold, false}] = &lazyFont{load: func() (*Font, error) {
		if !arrived {
			return nil, errFontPending
		}
		return ParseFont("bold", gobold.TTF)
	}}
	if got := r.Font("T", Bold, false).Name; got != "regular" {
		t.Fatalf("Bold, still on its way, resolved to %q, want the Regular face", got)
	}
	before := sys.Layout("Bold text", Style{Family: "T", Weight: Bold, Size: 20}, 1000).Width

	arrived = true
	if got := r.Font("T", Bold, false).Name; got != "regular" {
		t.Errorf("Bold resolved to %q before FontsChanged, want the face picked before", got)
	}
	sys.FontsChanged()
	if got := r.Font("T", Bold, false).Name; got != "bold" {
		t.Errorf("Bold resolved to %q after it arrived, want bold", got)
	}
	if after := sys.Layout("Bold text", Style{Family: "T", Weight: Bold, Size: 20}, 1000).Width; after == before {
		t.Errorf("the layout of bold text is %v wide before and after Bold arrived: it was kept", after)
	}
}
