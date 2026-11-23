package windows

import "testing"

// system returns a metric function that reports metric at every DPI.
func system(metric int32) func(uint32) int32 {
	return func(uint32) int32 { return metric }
}

// TestIconSize checks the small icon size at each DPI: the system's metric when
// it has one, else 16 pixels at 96 DPI, scaled and never below 16.
func TestIconSize(t *testing.T) {
	if got := iconSize(144, system(24)); got != 24 {
		t.Errorf("with the metric 24 at 144 DPI, size = %d; want 24", got)
	}
	for dpi, want := range map[uint32]int{96: 16, 120: 20, 144: 24, 168: 28, 192: 32, 0: 16} {
		if got := iconSize(dpi, system(0)); got != want {
			t.Errorf("without a metric at %d DPI, size = %d; want %d", dpi, got, want)
		}
	}
}

// TestBigIconSize checks the big icon size at each DPI: the system's metric when
// it has one, else 32 pixels at 96 DPI, scaled and never below 32. A window's two
// icons are made at these sizes, not one size stretched to the other.
func TestBigIconSize(t *testing.T) {
	if got := bigIconSize(144, system(48)); got != 48 {
		t.Errorf("with the metric 48 at 144 DPI, size = %d; want 48", got)
	}
	want := map[uint32]int{96: 32, 120: 40, 144: 48, 168: 56, 192: 64, 216: 72, 240: 80, 288: 96, 0: 32}
	for dpi, w := range want {
		if got := bigIconSize(dpi, system(0)); got != w {
			t.Errorf("without a metric at %d DPI, size = %d; want %d", dpi, got, w)
		}
		if small := iconSize(dpi, system(0)); 2*small != w {
			t.Errorf("at %d DPI the small icon is %d and the big one %d; want half", dpi, small, w)
		}
	}
}
