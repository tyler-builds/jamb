package app

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

func (f *ctlFixture) clockOp(req ctlRequest) ctlClock {
	f.t.Helper()
	req.Op = "clock"
	return f.must(req).(ctlClock)
}

func elapsedMS(c ctlClock) int64 {
	if c.ElapsedMS == nil {
		return -1
	}
	return *c.ElapsedMS
}

// Advancing the clock runs a timer when its time comes, and between the
// agent's requests the clock stands still, however long they take.
func TestControlClockAdvance(t *testing.T) {
	fired := false
	var after func(time.Duration, func()) func()
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		if after == nil {
			after = ctx.After
			after(time.Second, func() { fired = true })
		}
		return ui.Div()
	})
	start := f.now

	got := f.clockOp(ctlRequest{})
	if got.Mode != "live" || got.Now != start.Format("2006-01-02T15:04:05.000Z07:00") || got.ElapsedMS != nil ||
		got.NextTimerMS == nil || *got.NextTimerMS != 1000 {
		t.Fatalf("reading the clock: %+v", got)
	}

	f.now = f.now.Add(300 * time.Millisecond) // the wall clock goes on while live
	offscreens := f.offscreens
	got = f.clockOp(ctlRequest{AdvanceMS: 500})
	if got.Mode != "seek" || elapsedMS(got) != 500 || fired || *got.NextTimerMS != 200 {
		t.Fatalf("advancing 500 ms after 300 live: %+v, fired %v; want seek, the timer 200 ms away", got, fired)
	}
	if n := f.offscreens - offscreens; n != 32 { // 500 ms in frames of 16
		t.Errorf("advancing 500 ms ran %d frames; want 32", n)
	}
	if f.redraws == 0 {
		t.Error("the visible window was not asked to draw the moment the clock reached")
	}

	// Ten seconds of the wall clock and a frame of the window's own move nothing.
	f.now = f.now.Add(10 * time.Second)
	f.frame()
	if after := f.clockOp(ctlRequest{}); fired || after.Now != got.Now || *after.NextTimerMS != 200 {
		t.Fatalf("the clock moved between requests: %+v, fired %v", after, fired)
	}

	got = f.clockOp(ctlRequest{AdvanceMS: 200})
	if !fired || got.NextTimerMS != nil {
		t.Fatalf("advancing to the timer: %+v, fired %v", got, fired)
	}

	// Live again: the wall clock is ahead of where the agent left the clock, so
	// the clock follows it from now on. A timer scheduled while seeking, which
	// asked for no wake-up then, gets one.
	after(5*time.Second, func() {})
	f.wakes = nil
	got = f.clockOp(ctlRequest{Mode: "live"})
	if got.Mode != "live" || got.Now != f.now.Format("2006-01-02T15:04:05.000Z07:00") {
		t.Fatalf("live: %+v; want the wall clock's time", got)
	}
	if len(f.wakes) != 1 || *got.NextTimerMS != 0 {
		t.Errorf("going live asked for wake-ups at %v, with the next timer %d ms away; want one, due at once", f.wakes, *got.NextTimerMS)
	}
	if f.clock.offset != 0 {
		t.Errorf("offset %v; want none", f.clock.offset)
	}
}

// Going live after the agent moved the clock past the wall clock goes on from
// where the agent left it: the clock never steps back.
func TestControlClockLiveAhead(t *testing.T) {
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element { return ui.Div() })
	f.clockOp(ctlRequest{Mode: "seek"})
	f.clockOp(ctlRequest{AdvanceMS: 60000})
	seeking := f.clock.now()
	f.now = f.now.Add(time.Second)
	f.clockOp(ctlRequest{Mode: "live"})
	if got := f.clock.now(); !got.Equal(seeking) {
		t.Fatalf("live at %v; want %v, where the agent left the clock", got, seeking)
	}
	f.now = f.now.Add(250 * time.Millisecond)
	if got := f.clock.now(); !got.Equal(seeking.Add(250 * time.Millisecond)) {
		t.Errorf("live clock at %v; want it to follow the wall clock from %v", got, seeking)
	}
	// A runtime's wake-up is asked of the wall clock, and none while seeking.
	if wt, ok := f.clock.wakeTime(seeking.Add(time.Second)); !ok || !wt.Equal(f.now.Add(750*time.Millisecond)) {
		t.Errorf("wake time %v %v; want %v", wt, ok, f.now.Add(750*time.Millisecond))
	}
	f.clockOp(ctlRequest{Mode: "seek"})
	if _, ok := f.clock.wakeTime(f.now); ok {
		t.Error("a seeking clock asked the window to wake")
	}
}

// Settle runs a transition and a debounce to their ends and reports how long
// that took; in seek mode the transition stands still until then.
func TestControlClockSettle(t *testing.T) {
	on := ui.NewSignal(true)
	debounced := false
	var after func(time.Duration, func()) func()
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		after = ctx.After
		op := float32(1)
		if !on.Get() {
			op = 0
		}
		return ui.Div().Child(ui.Div().Key("panel").Width(ui.Px(40)).Height(ui.Px(20)).
			Transition(200 * time.Millisecond).Opacity(op))
	})
	if got := f.clockOp(ctlRequest{Settle: true}); elapsedMS(got) != 0 || got.Settled == nil || !*got.Settled || got.Mode != "seek" {
		t.Fatalf("settling a UI at rest: %+v", got)
	}

	on.Set(false)
	f.frame()
	for range 5 {
		f.now = f.now.Add(100 * time.Millisecond)
		f.frame()
	}
	if f.rt.Settled() {
		t.Fatal("the transition ran on the wall clock while the clock was seeking")
	}
	// A wait for idle does not wait for an animation that only the agent moves.
	_, ch, ok := f.call(ctlRequest{Op: "wait", For: "idle"})
	for i := 0; !ok && i < ctlQuietFrames; i++ {
		f.frame()
		f.c.frame(1, f.w)
		select {
		case g := <-ch:
			if g.err != nil {
				t.Fatalf("wait for idle: %v", g.err)
			}
			ok = true
		default:
		}
	}
	if !ok {
		t.Fatal("a wait for idle waited for an animation that stands still")
	}

	// A debounce, due after the transition has come to rest (in about 480 ms).
	after(700*time.Millisecond, func() { debounced = true })

	got := f.clockOp(ctlRequest{Settle: true})
	if got.Settled == nil || !*got.Settled || !f.rt.Settled() || !debounced {
		t.Fatalf("settle: %+v; settled %v, debounced %v", got, f.rt.Settled(), debounced)
	}
	if e := elapsedMS(got); e < 700 || e > 700+2*ctlClockStep.Milliseconds() {
		t.Errorf("settle took %d ms; want the 700 of the debounce", e)
	}
}

// A timer that keeps coming due keeps the UI from settling: settle gives up
// after its limit and says so.
func TestControlClockSettleGivesUp(t *testing.T) {
	ticks := 0
	var tick func()
	var after func(time.Duration, func()) func()
	tick = func() { ticks++; after(time.Second, tick) }
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element {
		if after == nil {
			after = ctx.After
			after(time.Second, tick)
		}
		return ui.Div()
	})
	got := f.clockOp(ctlRequest{Settle: true})
	if got.Settled == nil || *got.Settled || elapsedMS(got) != ctlClockSettleBy.Milliseconds() {
		t.Fatalf("settle with a repeating timer: settled %v after %d ms; want not settled after %v", *got.Settled, elapsedMS(got), ctlClockSettleBy)
	}
	if ticks < 9 {
		t.Errorf("the timer ran %d times in %v", ticks, ctlClockSettleBy)
	}
}

func TestControlClockBadRequests(t *testing.T) {
	f := newCtlFixture(t, func(ctx *ui.Context) ui.Element { return ui.Div() })
	for _, req := range []ctlRequest{
		{AdvanceMS: -1},
		{AdvanceMS: int(ctlClockMax.Milliseconds()) + 1},
		{AdvanceMS: 10, Settle: true},
		{Settle: true, Mode: "live"},
		{Mode: "fast"},
	} {
		req.Op = "clock"
		f.fails(req, "bad_request")
	}
	if f.clock.seek {
		t.Error("a bad request moved the clock to seek mode")
	}
}

// TestHeadlessClockChild is the app that TestHeadlessClock runs in a child
// process. It does nothing in a normal run.
func TestHeadlessClockChild(t *testing.T) {
	if os.Getenv("JAMB_CLOCK_TEST_CHILD") != "1" {
		t.Skip("run by TestHeadlessClock")
	}
	RegisterControl()
	done := ui.NewSignal(false)
	New().Run(Window{Title: "Clock", Width: 300, Height: 200,
		Root: func(ctx *ui.Context) ui.Element {
			d := ui.Div().Column().Padding(20).Child(ui.Button("Start").Key("start").
				OnClick(func() { ctx.After(200*time.Millisecond, func() { done.Set(true) }) }))
			if done.Get() {
				d.Child(ui.Text("Done"))
			}
			return d
		}})
}

// A running app's windows follow the agent's clock: in seek mode a timer does
// not run on the wall clock, and runs when the agent advances the clock to it.
func TestHeadlessClock(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a child process")
	}
	dir, err := os.MkdirTemp("", "jc") // short: socket paths are limited to about 100 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	cmd := exec.Command(os.Args[0], "-test.run=^TestHeadlessClockChild$")
	cmd.Env = append(os.Environ(), "JAMB_CLOCK_TEST_CHILD=1", "JAMB_HEADLESS=1", "JAMB_CONTROL=1",
		"JAMB_CONTROL_DIR="+dir, "WAYLAND_DISPLAY=", "DISPLAY=")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })

	socket := filepath.Join(dir, strconv.Itoa(cmd.Process.Pid)+".sock")
	var conn net.Conn
	for deadline := time.Now().Add(20 * time.Second); conn == nil; {
		if _, err := os.Stat(filepath.Join(dir, strconv.Itoa(cmd.Process.Pid)+".json")); err == nil {
			conn, _ = net.Dial("unix", socket)
		}
		if conn == nil {
			if time.Now().After(deadline) {
				t.Fatal("the app did not open its control channel")
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(20 * time.Second))
	in := bufio.NewScanner(conn)
	ask := func(req map[string]any) map[string]any {
		t.Helper()
		b, _ := json.Marshal(req)
		if _, err := conn.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
		if !in.Scan() {
			t.Fatalf("no answer to %s: %v", b, in.Err())
		}
		var m map[string]any
		json.Unmarshal(in.Bytes(), &m)
		if m["error"] != nil {
			t.Fatalf("%s: %v", b, m["error"])
		}
		return m["result"].(map[string]any)
	}
	found := func() float64 {
		return ask(map[string]any{"op": "find", "contains": "Done"})["total"].(float64)
	}

	ask(map[string]any{"op": "wait", "for": "idle"})
	if got := ask(map[string]any{"op": "clock", "mode": "seek"}); got["mode"] != "seek" {
		t.Fatalf("seek: %v", got)
	}
	ask(map[string]any{"op": "click", "key": "start"})
	time.Sleep(400 * time.Millisecond) // twice the timer's delay, on the wall clock
	if n := found(); n != 0 {
		t.Fatal("the timer ran on the wall clock while the agent's clock stood still")
	}
	if got := ask(map[string]any{"op": "clock"}); got["next_timer_ms"] != 200.0 {
		t.Fatalf("the clock after 400 ms of wall time: %v; want the timer still 200 ms away", got)
	}
	ask(map[string]any{"op": "clock", "advance_ms": 200})
	if n := found(); n != 1 {
		t.Fatalf("after advancing the clock to the timer, %v nodes say Done; want 1", n)
	}
	if got := ask(map[string]any{"op": "clock", "mode": "live"}); got["mode"] != "live" {
		t.Fatalf("live: %v", got)
	}
}
