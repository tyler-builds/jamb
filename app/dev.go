package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/ui"
)

// Development hooks, controlled by environment variables so that application
// code never changes:
//
//	JAMB_TRACE=1        print startup and per-frame timings to stderr
//	JAMB_SCRIPT="..."   drive the app with synthetic input (see below)
//	JAMB_STRESS=MODE    render continuously for JAMB_STRESS_SECONDS (default 5),
//	                    print frame/allocation/GC statistics and exit. MODE is
//	                    "rebuild" (re-render every component every frame) or
//	                    "paint" (repaint the retained tree every frame).
//	JAMB_FRAMESTATS=PATH  record every frame of the first window, from its first
//	                    frame until it closes (an `exit` step, a close from outside
//	                    or the end of JAMB_STRESS), with the performance counter.
//	                    At close it writes PATH, one row per frame: frame, t_ms
//	                    (from the first frame), cpu_ms (from the frame's start to
//	                    Present), present_interval_ms (from the previous Present;
//	                    empty for the first frame) and kind (rebuild, update or
//	                    repaint). It writes PATH-input.csv beside it (PATH with
//	                    "-input" before the extension), one row per input event a
//	                    frame rendered: kind (key, move, click or scroll), t_ms,
//	                    to_cpu_end_ms and to_present_ms, measured from the oldest
//	                    unrendered event of its kind. It prints a summary to stderr.
//	                    Input steps of JAMB_SCRIPT are timed when they inject
//	                    their event, and OS events when the window receives them.
//	                    Time to the screen is not measured (see framestats.go)
//	JAMB_HEAPLOG=PATH   write a CSV row of memory counters now and every 10 seconds
//	                    (JAMB_HEAPLOG_SECONDS changes the interval), for soak runs:
//	                    elapsed_s, the Go heap in use and its objects, total
//	                    allocated bytes and allocations, GC cycles, goroutines, and
//	                    the process's private bytes and working set
//	JAMB_VSYNC=0        present without waiting for vblank (throughput tests)
//	JAMB_GEOMETRY_FILE  remember the window position/size in this file
//	                    (set by the `jamb dev` runner across restarts). The file
//	                    holds "X Y W H" in physical pixels; X and Y may be
//	                    negative for a monitor left of or above the primary
//	JAMB_POSITION="X,Y" place the first window's top-left corner at logical
//	                    (X, Y), before it is first shown, instead of the saved
//	                    geometry. Later windows cascade from it as usual
//	JAMB_NOACTIVATE=1   show every window without activating it, and never ask
//	                    Windows to make one the foreground window, so a run does
//	                    not take the user's focus or move the pointer. Rendering,
//	                    scripts and stress mode are unchanged, but the window is
//	                    never active, so a text field draws no caret, as in any
//	                    background window
//	JAMB_A11Y_WARN=1    warn about accessibility problems, such as an unlabeled
//	                    button or an image without Alt. Also on under JAMB_TRACE=1
//	                    and `jamb dev`.
//	JAMB_INSPECT=1      open the visual inspector (see inspect.go) for the first
//	                    window at start; Ctrl+Shift+I opens it under `jamb dev`
//	                    and in an app that sets App.DevTools
//
// Settings overrides, for testing only. The real Windows settings are used in
// normal use, and each variable below replaces the matching setting for this
// run only, and changes no system setting. An unset variable keeps the real
// setting.
//
//	JAMB_HIGH_CONTRAST=1      high contrast on, with the representative black
//	                          scheme (ui.HighContrastBlack); "white" uses the
//	                          white scheme (ui.HighContrastWhite); 0 turns it off
//	JAMB_TEXT_SCALE=150       the Text size setting in percent, 100 to 225
//	JAMB_REDUCED_MOTION=1     reduced motion on; 0 turns it off
//	JAMB_DARK=1               the dark app mode; 0 the light one
//
// A script is a ';'-separated list of steps run after the first frame. The
// steps drive the target window, which is the first window until a
// `window N` step; a shot captures the target too:
//
//	window N        target window N from now on (1 is the first window opened)
//	move X Y        move the pointer to logical (X, Y)
//	click X Y       press and release the left button at (X, Y)
//	rclick X Y      press and release the right button at (X, Y), which opens a context menu
//	down X Y / up X Y
//	scroll X Y DY   scroll by DY logical pixels at (X, Y)
//	key NAME        press a key: enter, escape, tab, backspace, delete, insert,
//	                space, left, right, up, down, home, end, pageup, pagedown,
//	                menu (the Apps key), f1..f12, a..z, 0..9, = (plus), - (minus); optionally with
//	                modifiers ctrl, shift, alt, super: ctrl+z, ctrl+shift+left.
//	                An unknown key or modifier stops the script with an error
//	type TEXT       type TEXT exactly as written after the one space that follows
//	                "type"; other spaces are kept, and a ';' ends the step
//	drop X Y PATH... drop files at (X, Y), as if dragged in from the desktop;
//	                paths are separated by spaces
//	wait MS         wait MS milliseconds (rendering continues)
//	idle            wait until all animations have settled (repeating ones never
//	                settle, so they are not waited for)
//	devicelost      simulate a GPU device loss: the device is released, and the
//	                loss is handled as if the next Present had failed, so every
//	                window rebuilds its swap chain (see recovery.go)
//	shot PATH       save a PNG screenshot of the next frame
//	a11ydump PATH   write the accessibility tree as text (see ui.Runtime.AccessTree)
//	exit            close the window
//
// Example: JAMB_SCRIPT="idle; shot a.png; click 600 400; idle; shot b.png; exit"
type devOptions struct {
	trace      bool
	script     string
	stress     string
	stressSecs float64
	frameStats string  // JAMB_FRAMESTATS: the CSV path of the frame statistics, or ""
	heapLog    string  // JAMB_HEAPLOG: the CSV path of the memory counters, or ""
	heapSecs   float64 // JAMB_HEAPLOG_SECONDS: the interval of the rows, 10 by default
	noVSync    bool
	geometry   string
	position   *platform.Point // JAMB_POSITION: where the first window is placed, or nil
	noActivate bool            // JAMB_NOACTIVATE=1: no window is activated
	devStarted int64           // unix nanos when the dev runner noticed a change
	a11yWarn   bool            // development warnings about accessibility (see ui.Runtime.SetAccessWarnings)
	inspect    bool            // JAMB_INSPECT=1: the inspector opens for the first window
	// The settings overrides (see the package comment): highContrast is the
	// variable's value, textScale is a percent (0 when unset), reducedMotion
	// the variable's value.
	highContrast  string
	textScale     int
	reducedMotion string
	dark          string // JAMB_DARK: "1" for the dark app mode, "0" for the light one
}

func loadDevOptions() devOptions {
	d := devOptions{
		trace:         os.Getenv("JAMB_TRACE") == "1",
		script:        os.Getenv("JAMB_SCRIPT"),
		stress:        os.Getenv("JAMB_STRESS"),
		stressSecs:    5,
		frameStats:    os.Getenv("JAMB_FRAMESTATS"),
		heapLog:       os.Getenv("JAMB_HEAPLOG"),
		heapSecs:      10,
		noVSync:       os.Getenv("JAMB_VSYNC") == "0",
		geometry:      os.Getenv("JAMB_GEOMETRY_FILE"),
		highContrast:  os.Getenv("JAMB_HIGH_CONTRAST"),
		reducedMotion: os.Getenv("JAMB_REDUCED_MOTION"),
		dark:          os.Getenv("JAMB_DARK"),
		inspect:       os.Getenv("JAMB_INSPECT") == "1",
	}
	if v, err := strconv.Atoi(os.Getenv("JAMB_TEXT_SCALE")); err == nil && v > 0 {
		d.textScale = min(max(v, 100), 225)
	}
	if s := os.Getenv("JAMB_POSITION"); s != "" {
		if p, ok := parsePosition(s); ok {
			d.position = &p
		} else {
			fmt.Fprintf(os.Stderr, "jamb: JAMB_POSITION=%q is not X,Y in whole logical pixels; ignored\n", s)
		}
	}
	d.noActivate = os.Getenv("JAMB_NOACTIVATE") == "1"
	d.devStarted, _ = strconv.ParseInt(os.Getenv("JAMB_DEV_CHANGED_AT"), 10, 64)
	// Accessibility warnings belong to development runs: a traced run, a run
	// under `jamb dev`, or one that asks for them.
	d.a11yWarn = d.trace || d.devStarted > 0 || os.Getenv("JAMB_A11Y_WARN") == "1"
	if v, err := strconv.ParseFloat(os.Getenv("JAMB_STRESS_SECONDS"), 64); err == nil && v > 0 {
		d.stressSecs = v
	}
	if v, err := strconv.ParseFloat(os.Getenv("JAMB_HEAPLOG_SECONDS"), 64); err == nil && v > 0 {
		d.heapSecs = v
	}
	return d
}

// overrideSettings applies the settings overrides to s (see the package comment).
func (d devOptions) overrideSettings(s ui.SystemSettings) ui.SystemSettings {
	switch d.highContrast {
	case "":
	case "0":
		s.HighContrast = false
	case "white":
		s.HighContrast, s.Colors = true, ui.HighContrastWhite()
	default:
		s.HighContrast, s.Colors = true, ui.HighContrastBlack()
	}
	if d.textScale > 0 {
		s.TextScale = d.textScale
	}
	switch d.reducedMotion {
	case "":
	case "0":
		s.ReducedMotion = false
	default:
		s.ReducedMotion = true
	}
	switch d.dark {
	case "":
	case "0":
		s.Dark = false
	default:
		s.Dark = true
	}
	return s
}

func (d devOptions) loadGeometry() platform.Rect {
	if d.geometry == "" {
		return platform.Rect{}
	}
	b, err := os.ReadFile(d.geometry)
	if err != nil {
		return platform.Rect{}
	}
	var r platform.Rect
	if _, err := fmt.Sscanf(string(b), "%d %d %d %d", &r.X, &r.Y, &r.W, &r.H); err != nil || r.W < 100 || r.H < 100 {
		return platform.Rect{}
	}
	return r
}

// parsePosition reads a JAMB_POSITION value, "X,Y" in whole logical pixels, and
// reports false for anything else.
func parsePosition(s string) (platform.Point, bool) {
	xs, ys, found := strings.Cut(s, ",")
	if !found {
		return platform.Point{}, false
	}
	x, errX := strconv.Atoi(strings.TrimSpace(xs))
	y, errY := strconv.Atoi(strings.TrimSpace(ys))
	if errX != nil || errY != nil {
		return platform.Point{}, false
	}
	return platform.Point{X: x, Y: y}, true
}

func (d devOptions) saveGeometry(r platform.Rect) {
	if d.geometry != "" && r.W > 0 && r.H > 0 {
		_ = os.WriteFile(d.geometry, fmt.Appendf(nil, "%d %d %d %d", r.X, r.Y, r.W, r.H), 0o644)
	}
}

// inspectOn reports whether the visual inspector may be opened: under the dev
// runner, or when JAMB_INSPECT=1 asks for it.
func (d devOptions) inspectOn() bool { return d.inspect || d.devStarted > 0 }

// reportReady tells the dev runner how long a change took to reach the screen.
func (d devOptions) reportReady() {
	if d.devStarted > 0 {
		fmt.Fprintf(os.Stderr, "jamb dev: change -> new window on screen in %v\n",
			time.Since(time.Unix(0, d.devStarted)).Round(time.Millisecond))
	}
}

// stress measures sustained frame cost. Its times are on the clock of clockNow.
type stress struct {
	mode      string
	start     int64 // zero until the first frame
	until     int64
	cpu       []int64
	lastFrame int64
	intervals []int64
	mem0      runtime.MemStats
	nodes     int
}

func newStress(mode string) *stress { return &stress{mode: mode} }

// begin is called before each frame's runtime work.
func (s *stress) begin(h *host, secs float64) {
	now := clockNow()
	if s.start == 0 {
		s.start, s.until = now, now+int64(secs*float64(time.Second))
		runtime.ReadMemStats(&s.mem0)
	} else {
		s.intervals = append(s.intervals, now-s.lastFrame)
	}
	s.lastFrame = now
	switch s.mode {
	case "rebuild":
		h.rh.InvalidateAll()
	default:
		h.rh.Invalidate()
	}
}

// end records the CPU time of a frame and reports when done.
func (s *stress) end(h *host, w platform.Window, cpu int64) bool {
	s.cpu = append(s.cpu, cpu)
	s.nodes = h.rt.Stats().Nodes
	now := clockNow()
	if now < s.until {
		return true
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	n := len(s.cpu)
	c := summarize(s.cpu)
	elapsed := time.Duration(now - s.start)
	frames := float64(n)
	fmt.Fprintf(os.Stderr, "jamb stress mode=%s nodes=%d frames=%d elapsed=%.2fs fps=%.1f\n",
		s.mode, s.nodes, n, elapsed.Seconds(), frames/elapsed.Seconds())
	fmt.Fprintf(os.Stderr, "  cpu per frame (build+layout+paint+submit): avg=%v p50=%v p99=%v max=%v\n",
		time.Duration(c.avg).Round(time.Microsecond), time.Duration(c.p50).Round(time.Microsecond),
		time.Duration(c.p99).Round(time.Microsecond), time.Duration(c.max).Round(time.Microsecond))
	fmt.Fprintf(os.Stderr, "  alloc: %.1f KB/frame, %.0f objects/frame, %.2f MB/s; GC cycles=%d (%.1f/s), pause total=%v; heap=%.1f MB\n",
		float64(m.TotalAlloc-s.mem0.TotalAlloc)/frames/1024, float64(m.Mallocs-s.mem0.Mallocs)/frames,
		float64(m.TotalAlloc-s.mem0.TotalAlloc)/elapsed.Seconds()/1e6,
		m.NumGC-s.mem0.NumGC, float64(m.NumGC-s.mem0.NumGC)/elapsed.Seconds(),
		time.Duration(m.PauseTotalNs-s.mem0.PauseTotalNs), float64(m.HeapAlloc)/1e6)
	w.Close()
	return false
}

func (d devOptions) tracef(format string, args ...any) {
	if d.trace {
		fmt.Fprintf(os.Stderr, "jamb: "+format+"\n", args...)
	}
}

type step struct {
	op    string
	x, y  float32
	n     float32
	arg   string
	paths []string
}

type script struct {
	steps     []step
	i         int
	target    int // 1-based index of the window the steps drive
	waitUntil time.Time
	shotPath  string
	idleCount int
}

// typeText returns the text of a type step: everything after the single space
// that separates it from "type". Other spaces, leading or trailing, are kept.
func typeText(seg string) string {
	_, text, _ := strings.Cut(strings.TrimLeft(seg, " \t"), " ")
	return text
}

func parseScript(src string) (*script, error) {
	s := &script{target: 1}
	for _, seg := range strings.Split(src, ";") {
		raw := strings.TrimSpace(seg)
		if raw == "" {
			continue
		}
		op, rest, _ := strings.Cut(raw, " ")
		st := step{op: op, arg: strings.TrimSpace(rest)}
		if op == "type" {
			st.arg = typeText(seg)
		}
		nums := strings.Fields(rest)
		parse := func(i int) (float32, error) {
			if i >= len(nums) {
				return 0, fmt.Errorf("JAMB_SCRIPT step %q: missing number #%d", raw, i+1)
			}
			v, err := strconv.ParseFloat(nums[i], 32)
			if err != nil {
				return 0, fmt.Errorf("JAMB_SCRIPT step %q: %q is not a number", raw, nums[i])
			}
			return float32(v), nil
		}
		var err error
		switch op {
		case "move", "click", "rclick", "down", "up":
			if st.x, err = parse(0); err == nil {
				st.y, err = parse(1)
			}
		case "scroll":
			if st.x, err = parse(0); err == nil {
				if st.y, err = parse(1); err == nil {
					st.n, err = parse(2)
				}
			}
		case "drop":
			if st.x, err = parse(0); err == nil {
				st.y, err = parse(1)
			}
			if err == nil && len(nums) < 3 {
				err = fmt.Errorf("JAMB_SCRIPT step %q: drop needs at least one path after X and Y", raw)
			}
			st.paths = nums[min(2, len(nums)):]
		case "wait":
			st.n, err = parse(0)
		case "key", "type", "shot", "a11ydump":
			if st.arg == "" {
				err = fmt.Errorf("JAMB_SCRIPT step %q: missing argument", raw)
			} else if op == "key" {
				if _, _, ok := parseKey(st.arg); !ok {
					err = fmt.Errorf("JAMB_SCRIPT step %q: unknown key or modifier in %q", raw, st.arg)
				}
			}
		case "window":
			st.n, err = parse(0)
			if err == nil && st.n < 1 {
				err = fmt.Errorf("JAMB_SCRIPT step %q: windows are numbered from 1", raw)
			}
		case "idle", "exit", "devicelost":
		default:
			err = fmt.Errorf("JAMB_SCRIPT: unknown step %q; expected window, move, click, rclick, down, up, scroll, key, type, drop, wait, idle, devicelost, shot, a11ydump or exit", op)
		}
		if err != nil {
			return nil, err
		}
		s.steps = append(s.steps, st)
	}
	return s, nil
}

var keyNames = map[string]input.Key{
	"enter": input.KeyEnter, "escape": input.KeyEscape, "tab": input.KeyTab,
	"backspace": input.KeyBackspace, "delete": input.KeyDelete, "left": input.KeyLeft,
	"right": input.KeyRight, "up": input.KeyUp, "down": input.KeyDown,
	"pageup": input.KeyPageUp, "pagedown": input.KeyPageDown,
	"=": input.KeyEqual, "plus": input.KeyEqual, "-": input.KeyMinus, "minus": input.KeyMinus, "0": input.Key0,
	"home": input.KeyHome, "end": input.KeyEnd, "space": input.KeySpace, "insert": input.KeyInsert,
	"menu": input.KeyMenu,
	// The punctuation keys name the physical key, as the US layout prints it (see ui.Bind).
	"/": input.KeySlash, "\\": input.KeyBackslash, "[": input.KeyLeftBracket, "]": input.KeyRightBracket,
	";": input.KeySemicolon, "'": input.KeyApostrophe, ",": input.KeyComma, ".": input.KeyPeriod,
	"`": input.KeyBacktick,
}

// parseKey reads "enter", "a", "5", "f2", "ctrl+z" or "ctrl+shift+left". It
// reports false for an unknown key or modifier.
func parseKey(spec string) (input.Key, input.Modifiers, bool) {
	parts := strings.Split(strings.ToLower(spec), "+")
	var mods input.Modifiers
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "ctrl":
			mods |= input.ModCtrl
		case "shift":
			mods |= input.ModShift
		case "alt":
			mods |= input.ModAlt
		case "super":
			mods |= input.ModSuper
		default:
			return 0, 0, false
		}
	}
	name := parts[len(parts)-1]
	if k, ok := keyNames[name]; ok {
		return k, mods, true
	}
	if len(name) == 1 && name[0] >= 'a' && name[0] <= 'z' {
		return input.KeyA + input.Key(name[0]-'a'), mods, true
	}
	if len(name) == 1 && name[0] >= '0' && name[0] <= '9' {
		return input.Key0 + input.Key(name[0]-'0'), mods, true
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(name, "f")); err == nil && name[0] == 'f' && n >= 1 && n <= 12 {
		return input.KeyF1 + input.Key(n-1), mods, true
	}
	return 0, 0, false
}

// current returns the window the steps drive, or nil if it has closed.
func (s *script) current() *host {
	if s.target < 1 || s.target > len(application.windows) {
		return nil
	}
	return application.windows[s.target-1]
}

func (s *script) beforeDraw(h *host, w platform.Window) {
	if s.shotPath == "" || s.current() != h {
		return
	}
	path := s.shotPath
	s.shotPath = ""
	h.r.CaptureNextFrame(func(img *image.RGBA, err error) {
		if err == nil {
			err = savePNG(path, img)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "jamb: screenshot failed:", err)
		} else {
			h.dev.tracef("screenshot saved to %s", path)
		}
	})
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// afterDraw advances the script; it returns true while more frames are needed.
func (s *script) afterDraw(h *host, w platform.Window) bool {
	if s.current() != h {
		return false // the steps run in the target window's frames
	}
	rt := h.rt
	for s.i < len(s.steps) {
		if time.Now().Before(s.waitUntil) {
			// Sleep until then instead of rendering continuously, so a
			// waiting script measures the app's real idle behavior.
			w.WakeAt(s.waitUntil)
			return false
		}
		st := s.steps[s.i]
		arrived := clockNow() // an input step's event arrives when the step injects it
		switch st.op {
		case "move":
			rt.PointerMove(st.x, st.y, 0)
		case "click":
			rt.PointerMove(st.x, st.y, 0)
			rt.PointerDown(st.x, st.y, input.ButtonLeft, 1, 0)
			rt.PointerUp(st.x, st.y, input.ButtonLeft, 0)
		case "rclick":
			rt.PointerMove(st.x, st.y, 0)
			rt.PointerDown(st.x, st.y, input.ButtonRight, 1, 0)
			rt.PointerUp(st.x, st.y, input.ButtonRight, 0)
		case "down":
			rt.PointerMove(st.x, st.y, 0)
			rt.PointerDown(st.x, st.y, input.ButtonLeft, 1, 0)
		case "up":
			rt.PointerUp(st.x, st.y, input.ButtonLeft, 0)
		case "scroll":
			rt.Scroll(st.x, st.y, 0, st.n, false, 0)
		case "key":
			k, mods, ok := parseKey(st.arg)
			if ok {
				rt.KeyDown(k, mods, false)
				rt.KeyUp(k, mods)
			}
		case "type":
			for _, r := range st.arg {
				rt.TextInput(string(r))
			}
		case "drop":
			// Through the same handler the platform uses. The step's coordinates
			// are device-logical, like move's, so they are scaled to physical
			// pixels for it.
			h.OnEvent(w, platform.Event{Kind: platform.EventFileDrop,
				X: st.x * w.Scale(), Y: st.y * w.Scale(), Paths: st.paths})
		case "wait":
			s.waitUntil = time.Now().Add(time.Duration(st.n) * time.Millisecond)
		case "idle":
			// Require a couple of quiet frames so hover/layout settle too.
			// Repeating animations never end, so they are not waited for.
			if !rt.Settled() {
				s.idleCount = 0
				return true
			}
			if s.idleCount++; s.idleCount < 3 {
				return true
			}
			s.idleCount = 0
		case "devicelost":
			simulateDeviceLoss()
		case "shot":
			s.shotPath = st.arg
			s.i++
			hostOf(rt).Invalidate()
			return true // capture happens on the next frame
		case "a11ydump":
			if err := os.WriteFile(st.arg, []byte(rt.AccessTree().Dump()), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, "jamb: a11ydump failed:", err)
			}
		case "window":
			s.target = int(st.n)
			t := s.current()
			if t == nil {
				fmt.Fprintf(os.Stderr, "jamb: JAMB_SCRIPT: there is no window %d\n", s.target)
				os.Exit(1)
			}
			// The rest of the script runs in that window's frames.
			s.i++
			t.win.Wake()
			return false
		case "exit":
			s.i = len(s.steps)
			w.Close()
			return false
		}
		if k, ok := scriptInputKind(st.op); ok && h.fs != nil {
			h.fs.input(k, arrived, rt.NeedsFrame())
		}
		s.i++
		if st.op != "wait" && st.op != "idle" {
			// Let the step's effects render before the next one.
			return true
		}
	}
	return false
}
