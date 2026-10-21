package app

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/ui"
)

func TestSummarizeUsesTheStressPercentiles(t *testing.T) {
	// 100 values 1..100: the percentile at index n*pct/100 of the sorted values.
	vals := make([]int64, 100)
	for i := range vals {
		vals[i] = int64(100 - i) // unsorted on purpose
	}
	got := summarize(vals)
	want := summary{count: 100, avg: 50, p50: 51, p95: 96, p99: 100, max: 100}
	if got != want {
		t.Errorf("summarize = %+v, want %+v", got, want)
	}
	if vals[0] != 100 {
		t.Error("summarize reordered its input")
	}
}

func TestSummarizeEmptyAndSingle(t *testing.T) {
	if got := summarize(nil); got != (summary{}) {
		t.Errorf("summarize(nil) = %+v, want zero", got)
	}
	if got := summarize([]int64{7}); got != (summary{count: 1, avg: 7, p50: 7, p95: 7, p99: 7, max: 7}) {
		t.Errorf("summarize one value = %+v", got)
	}
	if s := (summary{}).String(); s != "count=0" {
		t.Errorf("empty summary prints %q", s)
	}
}

func TestOverBudgetCountsLongerValues(t *testing.T) {
	// 1.5 times a 60 Hz period is 25 ms; exactly 25 ms is within budget.
	vals := []int64{16_666_000, 25_000_000, 25_000_001, 40_000_000}
	if got := overBudget(vals, 25_000_000); got != 2 {
		t.Errorf("overBudget = %d, want 2", got)
	}
}

func TestInputKindOf(t *testing.T) {
	cases := map[platform.EventKind]inputKind{
		platform.EventKeyDown:   inputKey,
		platform.EventKeyUp:     inputKey,
		platform.EventTextInput: inputKey,
		platform.EventMouseMove: inputMove,
		platform.EventMouseDown: inputClick,
		platform.EventMouseUp:   inputClick,
		platform.EventScroll:    inputScroll,
	}
	for kind, want := range cases {
		if got, ok := inputKindOf(kind); !ok || got != want {
			t.Errorf("inputKindOf(%v) = %v, %v; want %v, true", kind, got, ok, want)
		}
	}
	for _, kind := range []platform.EventKind{platform.EventResize, platform.EventFocus, platform.EventFileDrop} {
		if _, ok := inputKindOf(kind); ok {
			t.Errorf("inputKindOf(%v) reports input; want none", kind)
		}
	}
}

func TestScriptInputKind(t *testing.T) {
	cases := map[string]inputKind{
		"key": inputKey, "type": inputKey, "move": inputMove,
		"click": inputClick, "rclick": inputClick, "down": inputClick, "up": inputClick,
		"scroll": inputScroll,
	}
	for op, want := range cases {
		if got, ok := scriptInputKind(op); !ok || got != want {
			t.Errorf("scriptInputKind(%q) = %v, %v; want %v, true", op, got, ok, want)
		}
	}
	for _, op := range []string{"wait", "idle", "shot", "exit", "mark", "repeat"} {
		if _, ok := scriptInputKind(op); ok {
			t.Errorf("scriptInputKind(%q) reports input; want none", op)
		}
	}
}

func TestFrameKind(t *testing.T) {
	cases := []struct {
		name string
		st   ui.FrameStats
		want string
	}{
		{"rebuild", ui.FrameStats{Built: true, Rendered: 40}, "rebuild"},
		{"update", ui.FrameStats{Rendered: 2, LaidOut: true}, "update"},
		{"restyle", ui.FrameStats{Restyled: 1}, "update"},
		{"repaint", ui.FrameStats{Painted: true}, "repaint"},
	}
	for _, c := range cases {
		if got := frameKind(c.st); got != c.want {
			t.Errorf("%s: frameKind = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestInputPathKeepsTheExtension(t *testing.T) {
	cases := map[string]string{
		"frames.csv":          "frames-input.csv",
		"out/run 2.csv":       "out/run 2-input.csv",
		"frames":              "frames-input",
		"results/frames.v2.c": "results/frames.v2-input.c",
	}
	for in, want := range cases {
		if got := inputPath(in); got != want {
			t.Errorf("inputPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// presentedAt records a frame whose CPU work starts at frameAt and whose Present
// call runs from start to end, all in nanoseconds.
func presentedAt(f *frameStats, st ui.FrameStats, frameAt, start, end int64) {
	f.presented(st, frameAt, start, end, nil)
}

func TestFrameStatsRecordsFramesAndCoalescedInput(t *testing.T) {
	f := newFrameStats(filepath.Join(t.TempDir(), "frames.csv"))
	rebuild := ui.FrameStats{Built: true}
	repaint := ui.FrameStats{Painted: true}

	// Two moves arrive before the first frame; the latency counts from the first.
	f.input(inputMove, 1_000_000, true)
	f.input(inputMove, 1_500_000, true)
	// A key that changes nothing is not waiting for a frame.
	f.input(inputKey, 1_600_000, false)
	presentedAt(f, rebuild, 1_000_000, 3_000_000, 3_500_000)
	// A frame with no input, 16 ms later.
	presentedAt(f, repaint, 17_000_000, 17_200_000, 17_300_000)

	if len(f.frames) != 2 {
		t.Fatalf("frames = %d, want 2", len(f.frames))
	}
	if f.frames[0].interval != 0 || f.frames[1].interval != 14_200_000 {
		t.Errorf("intervals = %d, %d; want 0, 14200000", f.frames[0].interval, f.frames[1].interval)
	}
	if f.frames[0].cpu != 2_000_000 || f.frames[0].kind != "rebuild" || f.frames[1].kind != "repaint" {
		t.Errorf("first frame = %+v, second = %+v", f.frames[0], f.frames[1])
	}
	if len(f.inputs) != 1 {
		t.Fatalf("input samples = %d, want 1 (the key waited for no frame)", len(f.inputs))
	}
	in := f.inputs[0]
	if in.kind != inputMove || in.toCPU != 2_000_000 || in.toPresent != 2_500_000 {
		t.Errorf("input = %+v; want move, toCPU 2000000, toPresent 2500000", in)
	}
	if f.pending[inputMove] || f.pending[inputKey] {
		t.Error("input stayed pending after its frame")
	}
}

func TestFrameStatsCSV(t *testing.T) {
	f := newFrameStats("unused.csv")
	f.input(inputClick, 0, true)
	presentedAt(f, ui.FrameStats{Built: true}, 0, 1_500_000, 2_000_000)
	presentedAt(f, ui.FrameStats{Painted: true}, 16_000_000, 16_250_000, 16_300_000)

	wantFrames := "frame,t_ms,cpu_ms,present_interval_ms,kind\n" +
		"1,0.000,1.500,,rebuild\n" +
		"2,16.000,0.250,14.750,repaint\n"
	if got := f.framesCSV(); got != wantFrames {
		t.Errorf("framesCSV =\n%s\nwant\n%s", got, wantFrames)
	}
	wantInputs := "kind,t_ms,to_cpu_end_ms,to_present_ms\n" +
		"click,0.000,1.500,2.000\n"
	if got := f.inputsCSV(); got != wantInputs {
		t.Errorf("inputsCSV =\n%s\nwant\n%s", got, wantInputs)
	}
}

func TestFrameStatsReportLines(t *testing.T) {
	f := newFrameStats("unused.csv")
	f.input(inputKey, 0, true)
	presentedAt(f, ui.FrameStats{Built: true}, 0, 1_000_000, 1_200_000)
	// The refresh rate is read at the first frame from the window, which a test lacks.
	f.refreshHz = 60
	presentedAt(f, ui.FrameStats{Painted: true}, 16_666_666, 16_700_000, 16_800_000)
	presentedAt(f, ui.FrameStats{Painted: true}, 50_000_000, 50_100_000, 50_200_000)

	var b bytes.Buffer
	f.report(&b)
	out := b.String()
	for _, want := range []string{
		"jamb framestats: frames=3 ",
		"refresh=60.00Hz (period 16.667ms)",
		"  cpu per frame: count=3 ",
		"  present interval: count=2 ",
		"over 1.5x period: 1",
		"  input key, to CPU end: count=1 ",
		"  input key, to Present returned: count=1 ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "input move") {
		t.Errorf("report prints a kind with no events:\n%s", out)
	}
}

func TestFrameStatsReportWithoutRefresh(t *testing.T) {
	f := newFrameStats("unused.csv")
	presentedAt(f, ui.FrameStats{Painted: true}, 0, 1_000_000, 1_100_000)
	var b bytes.Buffer
	f.report(&b)
	if out := b.String(); !strings.Contains(out, "refresh=unknown") || !strings.Contains(out, "over 1.5x period: n/a") {
		t.Errorf("report without a refresh rate:\n%s", out)
	}
}
