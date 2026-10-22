package ui

import (
	"testing"
	"unsafe"
)

// TestElementSizeClasses pins the largest sizes of the types that are allocated for
// every element, Style, DivElement and node, to the malloc size classes they fill.
// Go gives an allocation that holds pointers and is over 512 bytes an 8-byte header,
// so DivElement (1,024 class with the header) and node (1,280 class) must stay at
// most 1,016 and 1,272 bytes, and Style, which has no header, at most 448 bytes. One
// more byte in any of them takes the next class up for every element built, so a
// field added to one of them needs a repack or a new limit here.
func TestElementSizeClasses(t *testing.T) {
	for _, c := range []struct {
		name  string
		size  uintptr
		limit uintptr
	}{
		{"Style", unsafe.Sizeof(Style{}), 448},
		{"DivElement", unsafe.Sizeof(DivElement{}), 1016},
		{"node", unsafe.Sizeof(node{}), 1272},
	} {
		if c.size > c.limit {
			t.Errorf("%s is %d bytes, want at most %d (see the size classes of the test)", c.name, c.size, c.limit)
		}
	}
}
