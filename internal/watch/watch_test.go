package watch

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchReportsWrite(t *testing.T) {
	dir := t.TempDir()
	calls := make(chan struct{}, 16)
	stop, err := Watch(dir, 100*time.Millisecond, func() { calls <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if err := os.WriteFile(filepath.Join(dir, "keymap.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-calls:
	case <-time.After(2 * time.Second):
		t.Fatal("no call within 2s of a write")
	}
}

func TestWatchDebouncesBursts(t *testing.T) {
	dir := t.TempDir()
	var n atomic.Int32
	const debounce = 400 * time.Millisecond
	stop, err := Watch(dir, debounce, func() { n.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	path := filepath.Join(dir, "keymap.json")
	start := time.Now()
	for i := range 5 {
		if err := os.WriteFile(path, []byte{byte('0' + i)}, 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// On a loaded machine the sleeps can stretch; a burst longer than the debounce is
	// rightly two bursts, so the test judges only a burst that fit in it.
	if d := time.Since(start); d > debounce/2 {
		t.Skipf("the writes took %v, too long to be one burst for a %v debounce", d, debounce)
	}
	time.Sleep(3 * debounce)
	if got := n.Load(); got != 1 {
		t.Errorf("%d calls for one burst of writes, want 1", got)
	}
}

func TestWatchStopsCalls(t *testing.T) {
	dir := t.TempDir()
	var n atomic.Int32
	stop, err := Watch(dir, 50*time.Millisecond, func() { n.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	stop()
	if d := time.Since(start); d > time.Second {
		t.Errorf("stop took %v", d)
	}
	stop() // a second stop is harmless
	if err := os.WriteFile(filepath.Join(dir, "x.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := n.Load(); got != 0 {
		t.Errorf("%d calls after stop, want 0", got)
	}
}

func TestWatchMissingDirectory(t *testing.T) {
	if _, err := Watch(filepath.Join(t.TempDir(), "missing"), time.Millisecond, func() {}); err == nil {
		t.Error("watching a missing directory returned no error")
	}
}
