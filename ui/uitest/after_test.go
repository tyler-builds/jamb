package uitest_test

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// Context.After runs on the UI's clock: not before its time, once at its time, and
// never when it is canceled first.
func TestAfterRunsOnTheClock(t *testing.T) {
	var ctx *ui.Context
	u := uitest.New(t, func(c *ui.Context) ui.Element { ctx = c; return ui.Div() })
	ran, canceled := 0, 0
	ctx.After(150*time.Millisecond, func() { ran++ })
	ctx.After(150*time.Millisecond, func() { canceled++ })()
	u.Advance(100 * time.Millisecond)
	if ran != 0 {
		t.Fatalf("ran %d times 100 ms into a 150 ms wait", ran)
	}
	u.Advance(100 * time.Millisecond)
	if ran != 1 || canceled != 0 {
		t.Errorf("ran %d times, and the canceled one %d times; want 1 and 0", ran, canceled)
	}
}
