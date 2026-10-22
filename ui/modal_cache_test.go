package ui

import (
	"fmt"
	"testing"
)

// TestTopModalFollowsBuilds checks that topModal, which is kept for a build
// generation (see topModal), still answers for a modal that opens and closes
// between frames, and that the answer is the same on every call in between.
func TestTopModalFollowsBuilds(t *testing.T) {
	c := newNonModalTest(findPanel)
	c.r.focusKey(t, "opener")
	c.r.frame()
	if c.r.rt.topModal() != nil {
		t.Fatal("topModal is set before any modal is open")
	}
	c.modal.Set(true)
	c.r.frame()
	m := c.r.rt.topModal()
	if m == nil {
		t.Fatal("topModal is nil while a modal is open")
	}
	for range 3 {
		if got := c.r.rt.topModal(); got != m {
			t.Fatal("topModal changed between two calls with no build in between")
		}
	}
	c.modal.Set(false)
	c.r.frame()
	if c.r.rt.topModal() != nil {
		t.Fatal("topModal still returns the modal after it closed")
	}
}

// BenchmarkTopModal times the key path's check for an open modal, which the
// cache makes independent of the size of the tree: it should not grow with the
// window.
func BenchmarkTopModal(b *testing.B) {
	for _, size := range []int{100, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			hits := 0
			rt, _ := dispatchBenchApp(size, &hits)
			b.ReportAllocs()
			for b.Loop() {
				rt.topModal()
			}
		})
	}
}
