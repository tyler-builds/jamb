//go:build linux

package wl

import (
	"os"
	"testing"
)

// TestGlobals connects to the running compositor and lists its globals. It
// needs a Wayland session, so it runs only with JAMB_LIVE_TESTS=1.
func TestGlobals(t *testing.T) {
	if os.Getenv("JAMB_LIVE_TESTS") == "" {
		t.Skip("set JAMB_LIVE_TESTS=1 to connect to the compositor")
	}
	c, err := Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	globals := map[string]uint32{}
	reg := c.NewProxy("wl_registry", 1, func(op uint16, d *Decoder) {
		if op == 0 { // global(name, interface, version)
			d.Uint()
			iface, v := d.String(), d.Uint()
			globals[iface] = v
		}
	})
	c.Display.Send(1, NewID{reg}) // get_registry
	if err := c.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	for _, need := range []string{"wl_compositor", "wl_shm", "xdg_wm_base", "wl_seat"} {
		if globals[need] == 0 {
			t.Errorf("no %s", need)
		}
	}
	t.Log(globals)
}
