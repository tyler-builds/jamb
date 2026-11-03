//go:build linux

package linux

import (
	"bufio"
	"image"
	"image/color"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestTrayMenuNodes(t *testing.T) {
	next := int32(otherIDs)
	nodes := menuNodes([]MenuItem{
		{ID: 1, Label: "Show_window"},
		{Separator: true},
		{Label: "More", Submenu: []MenuItem{{ID: 2, Label: "Pause", Checked: true}, {ID: 3, Label: "Off", Disabled: true}}},
	}, &next)
	if len(nodes) != 3 || nodes[0].id != 1 || !nodes[0].chosen {
		t.Fatalf("first item: %+v", nodes[0])
	}
	if nodes[1].id <= otherIDs || nodes[2].id <= otherIDs || nodes[1].id == nodes[2].id {
		t.Errorf("separator and submenu IDs %d and %d; want distinct IDs above %d", nodes[1].id, nodes[2].id, otherIDs)
	}
	if nodes[2].chosen || !nodes[2].submenu || len(nodes[2].children) != 2 || nodes[2].children[1].id != 3 {
		t.Errorf("submenu: %+v", nodes[2])
	}

	p := nodes[0].props(nil)
	if got := p["label"].Value(); got != "Show__window" {
		t.Errorf("label %q; want the underscore doubled", got)
	}
	if _, ok := p["enabled"]; ok {
		t.Error("an enabled item sends enabled, which is the default")
	}
	if got := nodes[1].props(nil)["type"].Value(); got != "separator" {
		t.Errorf("separator type %v", got)
	}
	if got := nodes[2].props(nil)["children-display"].Value(); got != "submenu" {
		t.Errorf("submenu children-display %v", got)
	}
	pause := nodes[2].children[0].props(nil)
	if pause["toggle-type"].Value() != "checkmark" || pause["toggle-state"].Value() != int32(1) {
		t.Errorf("checked item: %v", pause)
	}
	if off := nodes[2].children[1].props([]string{"enabled"}); len(off) != 1 || off["enabled"].Value() != false {
		t.Errorf("disabled item filtered to enabled: %v", off)
	}

	root := &menuNode{submenu: true, children: nodes}
	if l := root.layout(0, nil); len(l.Children) != 0 {
		t.Errorf("depth 0 has %d children", len(l.Children))
	}
	if l := root.layout(1, nil); len(l.Children) != 3 || len(l.Children[2].Value().(menuLayout).Children) != 0 {
		t.Error("depth 1 should hold the items but not the submenu's")
	}
	if l := root.layout(-1, nil); len(l.Children[2].Value().(menuLayout).Children) != 2 {
		t.Error("depth -1 should hold every level")
	}
}

func TestTrayPixmaps(t *testing.T) {
	if p := pixmaps(nil); len(p) != 0 {
		t.Errorf("nil icon has %d pixmaps", len(p))
	}
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.SetNRGBA(x, y, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x80})
		}
	}
	ps := pixmaps(img)
	if len(ps) != len(iconSizes) {
		t.Fatalf("%d pixmaps; want %d", len(ps), len(iconSizes))
	}
	for i, p := range ps {
		if p.W != int32(iconSizes[i]) || len(p.Data) != iconSizes[i]*iconSizes[i]*4 {
			t.Errorf("pixmap %d is %dx%d with %d bytes", i, p.W, p.H, len(p.Data))
		}
	}
	if got := ps[0].Data[:4]; got[0] != 0x80 || got[1] != 0x11 || got[2] != 0x22 || got[3] != 0x33 {
		t.Errorf("first pixel % x; want ARGB 80 11 22 33", got)
	}
}

// privateBus starts a dbus-daemon of the test's own and points the session bus at
// it, or skips the test when there is none.
func privateBus(t *testing.T) {
	if _, err := exec.LookPath("dbus-daemon"); err != nil {
		t.Skip("no dbus-daemon")
	}
	cmd := exec.Command("dbus-daemon", "--session", "--nofork", "--print-address")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Skip("dbus-daemon:", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", strings.TrimSpace(addr))
}

// fakeWatcher records the items registered with it.
type fakeWatcher struct {
	mu    sync.Mutex
	items []string
}

func (w *fakeWatcher) RegisterStatusNotifierItem(sender dbus.Sender, service string) *dbus.Error {
	w.mu.Lock()
	w.items = append(w.items, service)
	w.mu.Unlock()
	return nil
}

func (w *fakeWatcher) registered() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.items)
}

// pump runs the posted functions, as the loop does, until the test ends.
func pump(t *testing.T) {
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(5 * time.Millisecond):
				runPosted()
			}
		}
	}()
}

// TestTrayBus shows an icon to a watcher on a private bus, and drives it as a
// panel does: it reads the item's properties and menu, opens the menu, chooses
// an item and clicks the icon.
func TestTrayBus(t *testing.T) {
	privateBus(t)

	icon := NewTrayIcon(TrayOptions{ID: 7, Tooltip: "hi"})
	if err := icon.Show(); err != ErrNoTrayHost {
		t.Fatalf("Show without a watcher: %v; want ErrNoTrayHost", err)
	}

	host, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	watcher := &fakeWatcher{}
	host.Export(watcher, watcherPath, watcherName)
	if r, err := host.RequestName(watcherName, 0); err != nil || r != dbus.RequestNameReplyPrimaryOwner {
		t.Fatal("watcher name:", r, err)
	}
	pump(t)

	var (
		mu                       sync.Mutex
		paused                   bool
		clicks, doubles, choices int
	)
	icon = NewTrayIcon(TrayOptions{
		ID: 7, AppName: "jamb-test", Tooltip: "hi",
		Click:       func() { mu.Lock(); clicks++; mu.Unlock() },
		DoubleClick: func() { mu.Lock(); doubles++; mu.Unlock() },
		Menu: func() []MenuItem {
			mu.Lock()
			defer mu.Unlock()
			return []MenuItem{{ID: 1, Label: "Pause", Checked: paused}, {Separator: true}, {ID: 2, Label: "Quit"}}
		},
		Chosen: func(id uint32) {
			mu.Lock()
			defer mu.Unlock()
			choices++
			if id == 1 {
				paused = !paused
			}
		},
	})
	if err := icon.Show(); err != nil {
		t.Fatal(err)
	}
	defer icon.Remove()
	name := "org.kde.StatusNotifierItem-" + strconv.Itoa(os.Getpid()) + "-7"
	if got := watcher.registered(); !slices.Equal(got, []string{name}) {
		t.Fatalf("registered %v; want [%s]", got, name)
	}

	item := host.Object(name, sniPath)
	var props map[string]dbus.Variant
	if err := item.Call(propsIface+".GetAll", 0, sniIface).Store(&props); err != nil {
		t.Fatal(err)
	}
	if props["Id"].Value() != "jamb-test" || props["Menu"].Value() != menuPath || props["IconName"].Value() != fallbackIcon {
		t.Errorf("properties %v", props)
	}
	if props["ItemIsMenu"].Value() != false {
		t.Error("an icon with a click action is a menu")
	}

	menu := host.Object(name, menuPath)
	layout := func() (uint32, menuLayout) {
		var rev uint32
		var l menuLayout
		if err := menu.Call(menuIface+".GetLayout", 0, int32(0), int32(-1), []string{}).Store(&rev, &l); err != nil {
			t.Fatal(err)
		}
		return rev, l
	}
	rev, l := layout()
	if len(l.Children) != 3 {
		t.Fatalf("menu has %d items; want 3", len(l.Children))
	}
	var needUpdate bool
	if err := menu.Call(menuIface+".AboutToShow", 0, int32(0)).Store(&needUpdate); err != nil || needUpdate {
		t.Errorf("AboutToShow of an unchanged menu: %v, %v", needUpdate, err)
	}

	if err := menu.Call(menuIface+".Event", 0, int32(1), "clicked", dbus.MakeVariant(""), uint32(0)).Err; err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the choice", func() bool { mu.Lock(); defer mu.Unlock(); return choices == 1 })
	// The menu is built again after the choice, with Pause checked.
	waitFor(t, "the new layout", func() bool { r, _ := layout(); return r > rev })
	_, l = layout()
	if first := child(t, l, 0); first.Props["toggle-state"].Value() != int32(1) {
		t.Errorf("Pause after the choice: %v", first.Props)
	}
	// A separator is never chosen.
	sep := child(t, l, 1).ID
	menu.Call(menuIface+".Event", 0, sep, "clicked", dbus.MakeVariant(""), uint32(0))

	item.Call(sniIface+".Activate", 0, int32(0), int32(0))
	item.Call(sniIface+".Activate", 0, int32(0), int32(0))
	waitFor(t, "the click and double click", func() bool { mu.Lock(); defer mu.Unlock(); return clicks == 1 && doubles == 1 })
	mu.Lock()
	if choices != 1 {
		t.Errorf("%d choices; the separator was chosen", choices)
	}
	mu.Unlock()

	// Remove drops the icon's name, which is how the watcher learns it is gone.
	icon.Remove()
	waitFor(t, "the name to go", func() bool {
		var has bool
		host.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, name).Store(&has)
		return !has
	})
}

// child decodes item i of a layout read from the bus.
func child(t *testing.T, l menuLayout, i int) menuLayout {
	t.Helper()
	var c menuLayout
	if err := dbus.Store([]any{l.Children[i].Value()}, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestTrayLive shows an icon in the desktop's panel for a few seconds. It needs a
// desktop with a StatusNotifierWatcher, so it runs only with JAMB_LIVE_TESTS=1.
func TestTrayLive(t *testing.T) {
	if os.Getenv("JAMB_LIVE_TESTS") == "" {
		t.Skip("set JAMB_LIVE_TESTS=1 to show a tray icon")
	}
	pump(t)
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for i := range img.Pix {
		img.Pix[i] = []byte{0x20, 0x90, 0xe0, 0xff}[i%4]
	}
	icon := NewTrayIcon(TrayOptions{ID: 1, AppName: "jamb-test", Tooltip: "jamb test icon", Icon: img,
		Menu: func() []MenuItem { return []MenuItem{{ID: 1, Label: "Hello"}} }})
	if err := icon.Show(); err != nil {
		t.Fatal(err)
	}
	name := "org.kde.StatusNotifierItem-" + strconv.Itoa(os.Getpid()) + "-1"
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	registered := func() []string {
		v, err := conn.Object(watcherName, watcherPath).GetProperty(watcherName + ".RegisteredStatusNotifierItems")
		if err != nil {
			t.Fatal(err)
		}
		items, _ := v.Value().([]string)
		return items
	}
	waitFor(t, "the watcher to list the icon", func() bool {
		return slices.ContainsFunc(registered(), func(s string) bool { return strings.HasPrefix(s, name) })
	})
	time.Sleep(3 * time.Second)
	icon.Remove()
	waitFor(t, "the watcher to drop the icon", func() bool {
		return !slices.ContainsFunc(registered(), func(s string) bool { return strings.HasPrefix(s, name) })
	})
}
