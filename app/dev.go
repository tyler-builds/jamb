package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/platform"
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
//	JAMB_VSYNC=0        present without waiting for vblank (throughput tests)
//	JAMB_GEOMETRY_FILE  remember the window position/size in this file
//	                    (set by the `jamb dev` runner across restarts)
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
//	exit            close the window
//
// Example: JAMB_SCRIPT="idle; shot a.png; click 600 400; idle; shot b.png; exit"
type devOptions struct {
	trace      bool
	script     string
	stress     string
	stressSecs float64
	noVSync    bool
	geometry   string
	devStarted int64 // unix nanos when the dev runner noticed a change
}

func loadDevOptions() devOptions {
	d := devOptions{
		trace:      os.Getenv("JAMB_TRACE") == "1",
		script:     os.Getenv("JAMB_SCRIPT"),
		stress:     os.Getenv("JAMB_STRESS"),
		stressSecs: 5,
		noVSync:    os.Getenv("JAMB_VSYNC") == "0",
		geometry:   os.Getenv("JAMB_GEOMETRY_FILE"),
	}
	d.devStarted, _ = strconv.ParseInt(os.Getenv("JAMB_DEV_CHANGED_AT"), 10, 64)
	if v, err := strconv.ParseFloat(os.Getenv("JAMB_STRESS_SECONDS"), 64); err == nil && v > 0 {
		d.stressSecs = v
	}
	return d
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

func (d devOptions) saveGeometry(r platform.Rect) {
	if d.geometry != "" && r.W > 0 && r.H > 0 {
		_ = os.WriteFile(d.geometry, fmt.Appendf(nil, "%d %d %d %d", r.X, r.Y, r.W, r.H), 0o644)
	}
}

// reportReady tells the dev runner how long a change took to reach the screen.
func (d devOptions) reportReady() {
	if d.devStarted > 0 {
		fmt.Fprintf(os.Stderr, "jamb dev: change -> new window on screen in %v\n",
			time.Since(time.Unix(0, d.devStarted)).Round(time.Millisecond))
	}
}

// stress measures sustained frame cost.
type stress struct {
	mode      string
	start     time.Time
	until     time.Time
	cpu       []time.Duration
	lastFrame time.Time
	intervals []time.Duration
	mem0      runtime.MemStats
	nodes     int
}

func newStress(mode string) *stress { return &stress{mode: mode} }

// begin is called before each frame's runtime work.
func (s *stress) begin(h *host, secs float64) {
	now := time.Now()
	if s.start.IsZero() {
		s.start, s.until = now, now.Add(time.Duration(secs*float64(time.Second)))
		runtime.ReadMemStats(&s.mem0)
	} else {
		s.intervals = append(s.intervals, now.Sub(s.lastFrame))
	}
	s.lastFrame = now
	switch s.mode {
	case "rebuild":
		h.rt.InvalidateAll()
	default:
		h.rt.Invalidate()
	}
}

// end records the CPU time of a frame and reports when done.
func (s *stress) end(h *host, w platform.Window, cpu time.Duration) bool {
	s.cpu = append(s.cpu, cpu)
	s.nodes = h.rt.Stats().Nodes
	if time.Now().Before(s.until) {
		return true
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	n := len(s.cpu)
	sorted := append([]time.Duration(nil), s.cpu...)
	slices.Sort(sorted)
	var total time.Duration
	for _, d := range s.cpu {
		total += d
	}
	elapsed := time.Since(s.start)
	frames := float64(n)
	fmt.Fprintf(os.Stderr, "jamb stress mode=%s nodes=%d frames=%d elapsed=%.2fs fps=%.1f\n",
		s.mode, s.nodes, n, elapsed.Seconds(), frames/elapsed.Seconds())
	fmt.Fprintf(os.Stderr, "  cpu per frame (build+layout+paint+submit): avg=%v p50=%v p99=%v max=%v\n",
		(total / time.Duration(n)).Round(time.Microsecond), sorted[n/2].Round(time.Microsecond),
		sorted[n*99/100].Round(time.Microsecond), sorted[n-1].Round(time.Microsecond))
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
		case "key", "type", "shot":
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
			err = fmt.Errorf("JAMB_SCRIPT: unknown step %q; expected window, move, click, rclick, down, up, scroll, key, type, drop, wait, idle, devicelost, shot or exit", op)
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
			rt.Invalidate()
			return true // capture happens on the next frame
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
		s.i++
		if st.op != "wait" && st.op != "idle" {
			// Let the step's effects render before the next one.
			return true
		}
	}
	return false
}
