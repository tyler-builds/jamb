package text

import (
	"errors"
	"testing"

	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
)

// A family with the faces Segoe UI has: 300, 400, 600 and 700.
func segoeLike(t *testing.T) *Registry {
	t.Helper()
	r := &Registry{families: map[string]*family{}}
	for _, w := range []Weight{Light, Regular, Semibold, Bold} {
		if err := r.Register("T", w, goregular.TTF); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestWeightMatchingPrefersNearestThenHeavier(t *testing.T) {
	r := segoeLike(t)
	for _, c := range []struct{ want, got Weight }{
		{Medium, Semibold}, // a tie between 400 and 600: the heavier, as DirectWrite does
		{450, Regular},
		{Regular, Regular},
		{350, Regular}, // a tie between 300 and 400
		{100, Light},
		{550, Semibold},
		{800, Bold}, // nothing heavier: the nearest lighter
		{Semibold, Semibold},
	} {
		if got, want := r.Font("T", c.want, false), r.Font("T", c.got, false); got != want {
			t.Errorf("weight %d resolved to a different face than weight %d", c.want, c.got)
		}
	}
}

func TestEmbeddedFaceComesAfterSystemFaces(t *testing.T) {
	r := &Registry{families: map[string]*family{}}
	r.addSource("T", faceKey{Regular, false}, func() (*Font, error) { return ParseFont("system regular", goregular.TTF) })
	r.addEmbedded("T", faceKey{Medium, false}, "embedded medium", gomedium.TTF)
	if got := r.Font("T", Medium, false).Name; got != "system regular" {
		t.Errorf("Medium resolved to %q, want the family's own Regular face, not an embedded Medium in another typeface", got)
	}
	// Without any system face, the embedded face is used.
	r = &Registry{families: map[string]*family{}}
	r.addSource("T", faceKey{Regular, false}, func() (*Font, error) { return nil, errMissing })
	r.addEmbedded("T", faceKey{Medium, false}, "embedded medium", gomedium.TTF)
	if got := r.Font("T", Medium, false).Name; got != "embedded medium" {
		t.Errorf("Medium resolved to %q, want the embedded face when the system face is missing", got)
	}
}

var errMissing = errors.New("missing font file")
