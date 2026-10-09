package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// recoveryTest is a gpuRecovery whose messages are collected instead of printed.
func recoveryTest() (*gpuRecovery, *[]string) {
	var msgs []string
	return &gpuRecovery{logf: func(format string, args ...any) {
		msgs = append(msgs, fmt.Sprintf(format, args...))
	}}, &msgs
}

// A lost device draws nothing until it is rebuilt, and then every window may
// draw once the rebuild has happened.
func TestRecoveryLostToRedraw(t *testing.T) {
	g, msgs := recoveryTest()
	now := time.Unix(1000, 0)
	if !g.lose(now, errSimulatedLoss) {
		t.Fatal("lose on a healthy device reported no change")
	}
	rebuilds := 0
	rebuild := func() error { rebuilds++; return nil }

	ready, recovered, _ := g.poll(now, rebuild)
	if !ready || !recovered || rebuilds != 1 {
		t.Fatalf("poll after loss: ready=%v recovered=%v rebuilds=%d; want true, true, 1", ready, recovered, rebuilds)
	}
	if g.lost {
		t.Fatal("device still marked lost after a successful rebuild")
	}
	if len(*msgs) != 2 {
		t.Fatalf("messages = %q; want the loss and the recovery", *msgs)
	}
}

// Several windows poll after one loss: the device is rebuilt once, by the
// first, and the others draw without rebuilding again.
func TestRecoveryRebuildsOnceForAllWindows(t *testing.T) {
	g, _ := recoveryTest()
	now := time.Unix(1000, 0)
	g.lose(now, errSimulatedLoss)
	rebuilds := 0
	rebuild := func() error { rebuilds++; return nil }

	recoveredCount := 0
	for window := 0; window < 4; window++ {
		ready, recovered, _ := g.poll(now, rebuild)
		if !ready {
			t.Fatalf("window %d not ready after the rebuild", window)
		}
		if recovered {
			recoveredCount++
		}
	}
	if rebuilds != 1 || recoveredCount != 1 {
		t.Fatalf("rebuilds=%d recovered=%d; want exactly one of each for four windows", rebuilds, recoveredCount)
	}
}

// A second loss while the device is already lost does not start over.
func TestRecoveryLoseIsIdempotent(t *testing.T) {
	g, _ := recoveryTest()
	now := time.Unix(1000, 0)
	if !g.lose(now, errSimulatedLoss) {
		t.Fatal("first loss not reported")
	}
	if g.lose(now, errSimulatedLoss) {
		t.Fatal("second loss reported as new; the caller would wake the windows twice")
	}
}

// While a rebuild is not due, nothing is rebuilt and the window is told when
// to try again.
func TestRecoveryWaitsUntilDue(t *testing.T) {
	g, _ := recoveryTest()
	now := time.Unix(1000, 0)
	g.lose(now, errSimulatedLoss)
	fail := errors.New("D3D11CreateDevice failed")
	g.poll(now, func() error { return fail }) // first attempt fails, due again in rebuildDelay

	calls := 0
	ready, recovered, retry := g.poll(now.Add(rebuildDelay/2), func() error { calls++; return nil })
	if ready || recovered || calls != 0 {
		t.Fatalf("early poll: ready=%v recovered=%v calls=%d; want false, false, 0", ready, recovered, calls)
	}
	if want := now.Add(rebuildDelay); !retry.Equal(want) {
		t.Fatalf("retry = %v; want %v", retry, want)
	}
}

// Failed rebuilds back off by doubling, capped at rebuildDelayMax, so a GPU
// that stays unavailable is polled a few times a second at most.
func TestRecoveryBackoff(t *testing.T) {
	want := []time.Duration{
		250 * time.Millisecond, 500 * time.Millisecond, time.Second,
		2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second,
	}
	for i, w := range want {
		if got := rebuildDelayAfter(i + 1); got != w {
			t.Errorf("rebuildDelayAfter(%d) = %v; want %v", i+1, got, w)
		}
	}

	g, _ := recoveryTest()
	now := time.Unix(1000, 0)
	g.lose(now, errSimulatedLoss)
	fail := errors.New("GPU still removed")
	attempts := 0
	failing := func() error { attempts++; return fail }
	for i, w := range want {
		_, _, retry := g.poll(now, failing)
		if got := retry.Sub(now); got != w {
			t.Fatalf("failure %d: next retry after %v; want %v", i+1, got, w)
		}
		now = retry
	}
	if attempts != len(want) {
		t.Fatalf("rebuild attempts = %d; want one per failure, none in between", attempts)
	}
}

// Each distinct failure is printed once for a loss, however often it repeats.
func TestRecoveryLogsDistinctFailuresOnce(t *testing.T) {
	g, msgs := recoveryTest()
	now := time.Unix(1000, 0)
	g.lose(now, errSimulatedLoss)
	a := errors.New("D3D11CreateDevice failed with HRESULT 0x887A0005")
	b := errors.New("CreateSwapChainForHwnd failed with HRESULT 0x80070057")
	for _, err := range []error{a, b, a, b, a} {
		_, _, retry := g.poll(now, func() error { return err })
		now = retry
	}
	failures := 0
	for _, m := range *msgs {
		if strings.HasPrefix(m, "GPU device not available yet") {
			failures++
		}
	}
	if failures != 2 {
		t.Fatalf("failure messages = %d; want 2 (one per distinct failure): %q", failures, *msgs)
	}
}

// The devicelost step is part of the JAMB_SCRIPT syntax.
func TestScriptAcceptsDeviceLost(t *testing.T) {
	if _, err := parseScript("idle; devicelost; wait 500; idle; exit"); err != nil {
		t.Fatalf("devicelost: %v", err)
	}
}

// The recovery message names the adapter the rebuilt device runs on, so a
// fallback to WARP shows in the log. Without an adapter name it is left out.
func TestRecoveryMessageNamesAdapter(t *testing.T) {
	now := time.Unix(1000, 0)
	cases := []struct {
		adapter func() string
		want    string
	}{
		{func() string { return "WARP software rasterizer" }, "GPU device recovered on WARP software rasterizer; redrawing every window"},
		{func() string { return "hardware" }, "GPU device recovered on hardware; redrawing every window"},
		{nil, "GPU device recovered; redrawing every window"},
	}
	for _, c := range cases {
		g, msgs := recoveryTest()
		g.adapter = c.adapter
		g.lose(now, errSimulatedLoss)
		if _, recovered, _ := g.poll(now, func() error { return nil }); !recovered {
			t.Fatal("poll after loss did not recover")
		}
		if got := (*msgs)[len(*msgs)-1]; got != c.want {
			t.Errorf("recovery message %q, want %q", got, c.want)
		}
	}
}
