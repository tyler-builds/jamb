package soft

import (
	"bytes"
	"testing"

	"github.com/tyler-builds/jamb/internal/render/rendertest"
)

// Rasterizing in bands on many goroutines gives the same pixels as one pass.
func TestBandsMatchOnePass(t *testing.T) {
	const w, h = 700, 500
	par := New(nil)
	if err := par.BeginFrame(w, h, black); err != nil {
		t.Fatal(err)
	}
	rendertest.BusyScene(par, w, h)
	if err := par.EndFrame(); err != nil {
		t.Fatal(err)
	}

	one := New(nil)
	if err := one.BeginFrame(w, h, black); err != nil {
		t.Fatal(err)
	}
	rendertest.BusyScene(one, w, h)
	one.draw(&band{pix: one.pix, stride: one.stride, width: w, y0: 0, y1: h})

	if !bytes.Equal(par.pix, one.pix) {
		for i := range par.pix {
			if par.pix[i] != one.pix[i] {
				t.Fatalf("bands differ from one pass first at pixel (%d,%d)", i/4%w, i/4/w)
			}
		}
	}
}

func BenchmarkBusyFrame1080p(b *testing.B) {
	r := New(nil)
	for b.Loop() {
		r.BeginFrame(1920, 1080, black)
		rendertest.BusyScene(r, 1920, 1080)
		r.EndFrame()
	}
}
