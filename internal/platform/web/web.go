//go:build js

// Package web is the platform adapter of a web page: a js build of an app, run
// in a browser. A window is a canvas that fills the page (or the <canvas
// id="jamb"> the page holds), the CPU renderer (render/soft) draws its frames,
// and the page's pointer, wheel, keyboard and focus events are its input.
//
// The UI runs on one goroutine, as on every platform. Browser callbacks run on
// goroutines of their own; they only queue events for it and wake it, since a
// callback must not block. Frames are paced by requestAnimationFrame, so a page
// in a background tab draws nothing.
//
// A page holds one window. Text input methods, the system clipboard, file
// dialogs and message boxes come later: text arrives from key presses, the
// clipboard is the page's own, and dialogs answer as canceled.
package web

import (
	"errors"
	"sync"
	"sync/atomic"
	"syscall/js"
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
)

var (
	postMu sync.Mutex
	posted []func()
	wakeCh = make(chan struct{}, 1)

	// uiActive is set while the UI goroutine runs the loop's work, and clear
	// while it waits for an event. WebAssembly in a browser has one thread, and
	// goroutines take turns only when one of them blocks, so another goroutine
	// runs only while the UI goroutine waits: then uiActive is clear. Code on the
	// UI goroutine that blocks itself (the highlighter's first compile does)
	// leaves it set, and a goroutine that runs meanwhile counts as the UI.
	uiActive atomic.Bool

	running bool      // Run is running; UI thread only
	quit    bool      // Quit was asked for; UI thread only
	windows []*window // open windows, UI thread only
	timers  []*timer  // UI thread only

	start = time.Now()

	settingsOnce sync.Once
	queries      struct{ dark, reducedMotion, forcedColors js.Value }
)

// Run runs the loop on the calling goroutine until Quit. start runs on the UI
// goroutine first; its error is returned at once.
func Run(startFn func() error) error {
	if js.Global().Get("document").IsUndefined() {
		return errors.New("platform/web: no document: a js build opens windows only in a web page")
	}
	running, quit = true, false
	uiActive.Store(true)
	defer func() {
		uiActive.Store(false)
		running = false
	}()
	watchSettings()
	if err := startFn(); err != nil {
		return err
	}
	for !quit {
		runPosted()
		runTimers()
		if quit {
			break
		}
		for _, w := range append([]*window(nil), windows...) {
			w.frame()
		}
		if anyPosted() || anyFrameReady() {
			continue
		}
		next, ok := nextTimer()
		uiActive.Store(false)
		if !ok {
			<-wakeCh
		} else {
			select {
			case <-wakeCh:
			case <-time.After(time.Until(next)):
			}
		}
		uiActive.Store(true)
	}
	return nil
}

// Quit ends the loop when it next runs. It is safe from any goroutine.
func Quit() { Post(func() { quit = true }) }

// OnUIThread reports whether the caller runs on the UI goroutine (see uiActive).
func OnUIThread() bool { return uiActive.Load() }

// Post runs fn on the UI goroutine, in order with the other posted functions.
// It is safe from any goroutine, browser callbacks included.
func Post(fn func()) {
	postMu.Lock()
	posted = append(posted, fn)
	postMu.Unlock()
	signal()
}

// Clock returns a monotonic clock in nanoseconds, the clock of Event.Arrived.
func Clock() int64 { return int64(time.Since(start)) }

func signal() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

func anyPosted() bool {
	postMu.Lock()
	defer postMu.Unlock()
	return len(posted) > 0
}

func runPosted() {
	postMu.Lock()
	fns := posted
	posted = nil
	postMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

type timer struct {
	at      time.Time
	fn      func()
	stopped bool
}

func after(d time.Duration, fn func()) *timer {
	t := &timer{at: time.Now().Add(d), fn: fn}
	timers = append(timers, t)
	return t
}

func (t *timer) stop() {
	if t != nil {
		t.stopped = true
	}
}

func nextTimer() (time.Time, bool) {
	var next time.Time
	for _, t := range timers {
		if !t.stopped && (next.IsZero() || t.at.Before(next)) {
			next = t.at
		}
	}
	return next, !next.IsZero()
}

func runTimers() {
	now := time.Now()
	var due []*timer
	keep := timers[:0]
	for _, t := range timers {
		switch {
		case t.stopped:
		case !t.at.After(now):
			due = append(due, t)
		default:
			keep = append(keep, t)
		}
	}
	clear(timers[len(keep):])
	timers = keep
	for _, t := range due {
		if !t.stopped {
			t.fn()
		}
	}
}

// ReadSettings reports the page's settings: the dark color scheme, reduced
// motion, and forced colors (Windows' high contrast, as the browser reports it).
// The system colors stay zero, so a high-contrast theme uses jamb's own. A title
// bar that the app draws shows no caption buttons, since a page does not
// minimize, maximize or close itself.
func ReadSettings() platform.Settings {
	s := platform.DefaultSettings()
	s.CaptionButtons = ":" // none at the start, none at the end
	if js.Global().Get("matchMedia").IsUndefined() {
		return s
	}
	settingsOnce.Do(initQueries)
	s.Dark = queries.dark.Get("matches").Bool()
	s.ReducedMotion = queries.reducedMotion.Get("matches").Bool()
	s.HighContrast = queries.forcedColors.Get("matches").Bool()
	return s
}

func initQueries() {
	mm := func(q string) js.Value { return js.Global().Call("matchMedia", q) }
	queries.dark = mm("(prefers-color-scheme: dark)")
	queries.reducedMotion = mm("(prefers-reduced-motion: reduce)")
	queries.forcedColors = mm("(forced-colors: active)")
}

// watchSettings sends EventSettingsChanged to every window when a setting that
// ReadSettings reports changes.
func watchSettings() {
	if js.Global().Get("matchMedia").IsUndefined() {
		return
	}
	settingsOnce.Do(initQueries)
	changed := js.FuncOf(func(js.Value, []js.Value) any {
		Post(func() {
			for _, w := range windows {
				w.emit(platform.Event{Kind: platform.EventSettingsChanged})
			}
		})
		return nil
	})
	for _, q := range []js.Value{queries.dark, queries.reducedMotion, queries.forcedColors} {
		q.Call("addEventListener", "change", changed)
	}
}
