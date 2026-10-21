package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/ui"
)

// This file holds the frame statistics of JAMB_FRAMESTATS and the timing that
// stress mode and the statistics share. Every time is in nanoseconds on the clock
// of clockNow, which is the performance counter on Windows.
//
// Input latency stops at Present returning. The time to the screen is not
// measured: the windows use a flip-model swap chain, and the DXGI frame
// statistics (GetFrameStatistics, GetLastPresentCount) were not verified for
// windowed flip presentation. No live window was available to check that, so
// the screen time is left out rather than guessed.

// frameSample is one frame the window presented.
type frameSample struct {
	at         int64  // when the frame began, from the first frame
	cpu        int64  // from the frame's start to the start of Present, which blocks for vsync
	interval   int64  // from the previous Present's start to this one's; 0 for the first frame
	kind       string // "rebuild", "update" or "repaint", see frameKind
	continuous bool   // the previous frame was wanted when its Present returned, so there was no idle gap
}

// inputSample is one input event the frames rendered: from the oldest unrendered
// event of its kind, to the start of the frame's Present and to its return.
type inputSample struct {
	kind             inputKind
	at               int64 // when the event arrived, from the first frame
	toCPU, toPresent int64
}

// inputKind is a kind of input whose latency is reported.
type inputKind int

const (
	inputKey    inputKind = iota // a key press or release, or typed text
	inputMove                    // a pointer move
	inputClick                   // a button press or release
	inputScroll                  // a wheel or touchpad scroll
	inputKinds
)

// inputNames names the input kinds in the CSV files and the summary.
var inputNames = [inputKinds]string{"key", "move", "click", "scroll"}

// inputKindOf returns the input kind of a platform event, or false for the events
// that are not input.
func inputKindOf(k platform.EventKind) (inputKind, bool) {
	switch k {
	case platform.EventKeyDown, platform.EventKeyUp, platform.EventTextInput:
		return inputKey, true
	case platform.EventMouseMove:
		return inputMove, true
	case platform.EventMouseDown, platform.EventMouseUp:
		return inputClick, true
	case platform.EventScroll:
		return inputScroll, true
	}
	return 0, false
}

// scriptInputKind returns the input kind of a JAMB_SCRIPT step, or false for the
// steps that inject no input.
func scriptInputKind(op string) (inputKind, bool) {
	switch op {
	case "key", "type":
		return inputKey, true
	case "move":
		return inputMove, true
	case "click", "rclick", "down", "up":
		return inputClick, true
	case "scroll":
		return inputScroll, true
	}
	return 0, false
}

// frameKind names the work a frame did, from the runtime's stats: a rebuild of the
// tree, an update of some components or their layout, or a repaint of the tree.
func frameKind(st ui.FrameStats) string {
	switch {
	case st.Built:
		return "rebuild"
	case st.Rendered > 0 || st.Restyled > 0 || st.LaidOut:
		return "update"
	}
	return "repaint"
}

// presentTimes returns when the renderer's last Present call started and returned.
// A renderer that does not time its Present reports now for both.
func presentTimes(r render.Renderer, now int64) (start, end int64) {
	if p, ok := r.(interface{ LastPresent() (start, end int64) }); ok {
		return p.LastPresent()
	}
	return now, now
}

// refreshHz returns the refresh rate of the window's monitor, or 0 when unknown.
func refreshHz(w platform.Window) float64 {
	if r, ok := w.(interface{ RefreshRate() float64 }); ok {
		return r.RefreshRate()
	}
	return 0
}

// frameStats records the frames of the first window, from its first frame until it
// closes, for JAMB_FRAMESTATS. Its frames and input events are the only ones
// recorded; other windows are not.
type frameStats struct {
	path      string // the CSV of frames; the input CSV is beside it
	origin    int64  // when the first frame began
	frames    []frameSample
	inputs    []inputSample
	lastStart int64 // the start of the previous Present

	// pending and pendingAt hold, per kind, whether an input event waits for a
	// frame, and the arrival of the oldest one that does. A coalesced run of events
	// is timed from its oldest.
	pending   [inputKinds]bool
	pendingAt [inputKinds]int64

	refreshHz float64 // the refresh rate of the window's monitor, read at the first frame

	// wanted is whether a frame was wanted when the previous Present returned.
	wanted bool
}

func newFrameStats(path string) *frameStats { return &frameStats{path: path} }

// presented records a frame the window presented. The frame began at frameAt, and its
// Present call ran from start to end. wanted is whether another frame was wanted
// when Present returned (see ui.Runtime.FrameRequested): if so, the next frame came
// at once, and its present interval has no idle gap in it.
func (f *frameStats) presented(st ui.FrameStats, frameAt, start, end int64, wanted bool, w platform.Window) {
	if len(f.frames) == 0 {
		f.origin = frameAt
		f.refreshHz = refreshHz(w)
	}
	s := frameSample{at: frameAt - f.origin, cpu: start - frameAt, kind: frameKind(st)}
	if len(f.frames) > 0 {
		s.interval = start - f.lastStart
		s.continuous = f.wanted
	}
	f.wanted = wanted
	f.lastStart = start
	f.frames = append(f.frames, s)
	for k := range inputKinds {
		if !f.pending[k] {
			continue
		}
		f.pending[k] = false
		f.inputs = append(f.inputs, inputSample{kind: k, at: f.pendingAt[k] - f.origin,
			toCPU: start - f.pendingAt[k], toPresent: end - f.pendingAt[k]})
	}
}

// input notes an input event that arrived at arrived. needsFrame is whether the
// runtime has work for a frame after the event, so an event that changes nothing
// waits for no frame.
func (f *frameStats) input(k inputKind, arrived int64, needsFrame bool) {
	if needsFrame && !f.pending[k] {
		f.pending[k], f.pendingAt[k] = true, arrived
	}
}

// summary is the distribution of a set of durations, in nanoseconds.
type summary struct {
	count                   int
	avg, p50, p95, p99, max int64
}

// summarize returns the distribution of vals, which it does not reorder. A
// percentile is the value at index n*pct/100 of the sorted values, the way the stress
// report has always computed them.
func summarize(vals []int64) summary {
	n := len(vals)
	if n == 0 {
		return summary{}
	}
	sorted := slices.Clone(vals)
	slices.Sort(sorted)
	var total int64
	for _, v := range sorted {
		total += v
	}
	return summary{count: n, avg: total / int64(n), p50: percentile(sorted, 50),
		p95: percentile(sorted, 95), p99: percentile(sorted, 99), max: sorted[n-1]}
}

// percentile returns the value at pct percent of the sorted values.
func percentile(sorted []int64, pct int) int64 {
	return sorted[min(len(sorted)*pct/100, len(sorted)-1)]
}

// overBudget counts the values longer than limit.
func overBudget(vals []int64, limit int64) int {
	n := 0
	for _, v := range vals {
		if v > limit {
			n++
		}
	}
	return n
}

// String formats the distribution: the count, then the average, the 50th, 95th and
// 99th percentiles and the maximum, in microseconds or milliseconds.
func (s summary) String() string {
	if s.count == 0 {
		return "count=0"
	}
	d := func(ns int64) time.Duration { return time.Duration(ns).Round(time.Microsecond) }
	return fmt.Sprintf("count=%d avg=%v p50=%v p95=%v p99=%v max=%v",
		s.count, d(s.avg), d(s.p50), d(s.p95), d(s.p99), d(s.max))
}

// csvMillis formats a duration in nanoseconds as milliseconds, for the CSV files.
func csvMillis(ns int64) string { return fmt.Sprintf("%.3f", float64(ns)/1e6) }

// framesCSV returns the frame CSV: one row per frame, the present interval
// empty for the first frame, and continuous 1 when the interval has no idle gap.
func (f *frameStats) framesCSV() string {
	var b strings.Builder
	b.WriteString("frame,t_ms,cpu_ms,present_interval_ms,kind,continuous\n")
	for i, s := range f.frames {
		interval := ""
		if s.interval > 0 {
			interval = csvMillis(s.interval)
		}
		continuous := 0
		if s.continuous {
			continuous = 1
		}
		fmt.Fprintf(&b, "%d,%s,%s,%s,%s,%d\n", i+1, csvMillis(s.at), csvMillis(s.cpu), interval, s.kind, continuous)
	}
	return b.String()
}

// inputsCSV returns the input CSV: one row per input event a frame rendered.
func (f *frameStats) inputsCSV() string {
	var b strings.Builder
	b.WriteString("kind,t_ms,to_cpu_end_ms,to_present_ms\n")
	for _, s := range f.inputs {
		fmt.Fprintf(&b, "%s,%s,%s,%s\n", inputNames[s.kind], csvMillis(s.at), csvMillis(s.toCPU), csvMillis(s.toPresent))
	}
	return b.String()
}

// inputPath is the input CSV's path: the frame CSV's, with "-input" before its extension.
func inputPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + "-input" + ext
}

// report prints the summary, one line per metric, to w. The present interval line
// covers only the continuous frames, those with no idle gap before them, and counts
// those whose interval exceeded 1.5 times the refresh period. An idle gap of any
// length is not a present interval of the display.
func (f *frameStats) report(w io.Writer) {
	n := len(f.frames)
	var elapsed time.Duration
	if n > 0 {
		elapsed = time.Duration(f.frames[n-1].at)
	}
	refresh := "unknown"
	var limit int64
	if f.refreshHz > 0 {
		period := 1e9 / f.refreshHz
		limit = int64(1.5 * period)
		refresh = fmt.Sprintf("%.2fHz (period %v)", f.refreshHz, time.Duration(period).Round(time.Microsecond))
	}
	fmt.Fprintf(w, "jamb framestats: frames=%d elapsed=%.2fs refresh=%s\n", n, elapsed.Seconds(), refresh)

	cpu := make([]int64, n)
	var intervals []int64
	for i, s := range f.frames {
		cpu[i] = s.cpu
		if s.continuous {
			intervals = append(intervals, s.interval)
		}
	}
	fmt.Fprintf(w, "  cpu per frame: %v\n", summarize(cpu))
	over := "n/a"
	if limit > 0 {
		over = fmt.Sprint(overBudget(intervals, limit))
	}
	withInterval := max(n-1, 0)
	fmt.Fprintf(w, "  present interval, continuous frames only (%d of %d): %v; over 1.5x period: %s\n",
		len(intervals), withInterval, summarize(intervals), over)
	for k := range inputKinds {
		var toCPU, toPresent []int64
		for _, s := range f.inputs {
			if s.kind == k {
				toCPU = append(toCPU, s.toCPU)
				toPresent = append(toPresent, s.toPresent)
			}
		}
		if len(toCPU) == 0 {
			continue
		}
		fmt.Fprintf(w, "  input %s, to CPU end: %v\n", inputNames[k], summarize(toCPU))
		fmt.Fprintf(w, "  input %s, to Present returned: %v\n", inputNames[k], summarize(toPresent))
	}
}

// finish writes the CSV files and prints the summary, when the window has closed.
// A file that cannot be written is reported, and the others are still written.
func (f *frameStats) finish() {
	if err := os.WriteFile(f.path, []byte(f.framesCSV()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "jamb: JAMB_FRAMESTATS:", err)
	}
	if err := os.WriteFile(inputPath(f.path), []byte(f.inputsCSV()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "jamb: JAMB_FRAMESTATS:", err)
	}
	f.report(os.Stderr)
}
