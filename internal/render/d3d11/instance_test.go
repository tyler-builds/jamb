//go:build windows

package d3d11

import "testing"

// The first instance buffer is instanceBufferMin, not larger, and holds a need
// with half again the room it needs.
func TestInstanceCapacity(t *testing.T) {
	cases := []struct {
		size, want int
	}{
		{0, instanceBufferMin},
		{100, instanceBufferMin},
		{2 * 1024, instanceBufferMin},
		{5000, 10000},
		{64 * 1024, 128 * 1024},
	}
	for _, c := range cases {
		if got := instanceCapacity(c.size); got != c.want {
			t.Errorf("instanceCapacity(%d) = %d, want %d", c.size, got, c.want)
		}
	}
}

// A buffer that grows a little at a time is recreated a few times, not on every
// step: from nothing to 1 MiB in 512-byte steps takes about a dozen buffers.
func TestInstanceCapacityGrowsGeometrically(t *testing.T) {
	capacity, creates := 0, 0
	for size := 1; size <= 1<<20; size += 512 {
		if capacity < size {
			capacity = instanceCapacity(size)
			creates++
		}
	}
	if creates > 12 {
		t.Errorf("instance buffers created = %d, want at most 12", creates)
	}
}
