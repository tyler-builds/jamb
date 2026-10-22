# Benchmarks

This file reports jamb's measured performance. The current results, dated
2026-10-21, come first, followed by how to reproduce them and the coverage gaps.
The earlier measurements (2026-10-06 to 2026-10-14) follow under
[Earlier measurements](#earlier-measurements), with their original dates. Where a
2026-10-21 figure replaces an earlier one, the earlier figure is marked
superseded and kept, so the history stays readable.

## Current results (2026-10-21)

Every live figure below was measured on one machine, with the windows on a
secondary monitor. The machine was not dedicated to benchmarking: other
applications were running. The conditions matter more than usual here, so read
this table first.

| | |
|---|---|
| CPU | Intel Core i5-11600K, 6 cores, 12 threads, 3.90 GHz |
| GPU | NVIDIA GeForce RTX 3070 Ti, driver 32.0.16.1742 |
| RAM | 31.9 GB |
| OS | Windows 11 Enterprise, build 26200 |
| Toolchains | Go 1.27.1 windows/amd64; Rust 1.95.0 (GPUI comparison only) |
| Power plan | Balanced |
| Displays | Primary: 1920 x 1080 at 165 Hz, 100% scale. Secondary: 1920 x 1080 at 144 Hz, 100% scale, at X = -1920 |
| Live windows | Placed on the secondary monitor at logical (-1600, 150). Never activated (`JAMB_NOACTIVATE=1`), so they took no focus and no pointer |
| Background load | The machine was not dedicated to benchmarking: other applications were running |
| Commits | Live runs and the Go suite: `2c5c6b0` (the live run used its parent, `b6a6369`). The Go header reads `2c5c6b0-dirty` because a regenerated screenshot was present during the run and then discarded |
| Samples | Live: 3 runs, each figure the median across runs with the range in brackets. Soak: one 30-minute run. Go: `-count 10`, medians with benchstat's spread |

### Summary

| Area | Result, 2026-10-21 | Measured by |
|---|---|---|
| Binary size, stripped release (`-trimpath -ldflags "-s -w -H windowsgui"`) | hello **9.36 MB**, demo **19.61 MB**; default `go build`: hello 13.01 MB, demo 26.91 MB | `bench-live`, one build each |
| Startup, demo, warm launch (process start to first frame, `JAMB_TRACE`) | **214.8 ms** [212–247.1]; to first painted frame 251.8 ms [247.1–290] | `bench-live`, 3 runs |
| Startup, demo, first launch of a new copy | **214.9 ms** [209.8–226.1] trace; 640.4 ms [630.6–707] to first painted frame (the first launch includes the antivirus scan) | `bench-live`, 3 runs |
| Idle CPU, demo, unfocused, 60 s | **0.13%** of one core [0.1–0.13]; 65.6 context switches/s | `bench-live`, 3 runs |
| Idle memory, demo, unfocused, 60 s | private bytes **122.9 MB** [122.2–123.3]; working set 117.2 MB; 40 threads; 539 handles | `bench-live`, 3 runs |
| Scrolling the 10,000-event Activity list | **6.06 ms** median present interval on a 144 Hz monitor; CPU per frame p99 2.27 ms [1.97–2.88]; 7 frames over 1.5x the period [6–16] | `bench-live` scenario, 3 runs |
| Pointer movement over the Activity rows | CPU 15.3% of one core [15.3–17.94] | `bench-live` scenario, 3 runs |
| Click to Present, animation and code-editor scenarios | about 55 to 70 ms (68.09 ms [61.81–69.83] for the animation clicks, 60.92 ms p95 [53.65–65.71] for the code-editor clicks) | `bench-live` scenario input latency |
| Page switch, first visit, p95 | **26.73 ms** [25.64–32] | `bench-live` scenario input latency |
| Resize by `SetWindowPos`, 10 s | CPU 13.86% [10.42–14.5]; 2,473 context switches/s [2,383–2,518] | `bench-live` resize |
| Stress, full re-render, vsync on, 310-node page | **163.8 FPS** [163.3–164.1]; **936 KB** and 2,731 objects per frame; GC 10.5 cycles/s | `bench-live` stress |
| Stress, paint only, vsync off | 1,796 FPS [1,666.6–1,799.5]; 0.7 KB and 27 objects per frame | `bench-live` stress |
| Two windows (main and a project window), idle, 60 s | CPU 0.1% [0–0.21]; 65.4 context switches/s | `bench-live` multi-window |
| Soak, 30 min, demo loop | no growth after 5 min (see [Soak](#soak)) | `JAMB_HEAPLOG` soak run |
| Full re-render, `Runtime.Frame`, 10,000 nodes | 12.43 ms per op (Go suite, `Scale/nodes=10000/full-rebuild`) | `bench-go` |
| One signal change, 10,000 nodes | **310.4 µs** (Go suite, `Scale/nodes=10000/single-signal-update`) | `bench-go` |
| Key dispatch, 10,000 nodes | 272.9 µs (field caret) and 182.8 µs (global binding); both grow with the tree (see [Known issues](#known-issues)) | `bench-go` |
| Expanded tree, 10,100 rows, one frame | **80.5 ms** per frame | `bench-go` |
| Incremental build, jamb hello (`go build`, text change) | release **1.36 s** [1.34–1.37]; default **1.48 s** [1.48–1.49] | `compare-gpui`, 3 runs |

Not re-measured on 2026-10-21, so the figures are in
[Earlier measurements](#earlier-measurements): the build-footprint numbers from
`scripts/bench.sh` (cold build, no-op rebuild, save-to-window), the startup
phase breakdown, the repository size, and the highlighting comparison of
2026-10-19.

### How to reproduce

Each script is run from the repository root with
`powershell -ExecutionPolicy Bypass -File scripts\<name>.ps1`. Each writes its raw
results to a folder.

- **`scripts/bench-live.ps1`**: the live measurements of this section. Every window
  is placed off the primary monitor and shown without activation. It builds `examples/hello` and
  `examples/demo` and measures startup (a fresh copy, then warm launches), idle
  CPU and memory, the scenarios, resize, stress, the second window, and with
  `-SoakMinutes` the soak. Options: `-Runs` (default 3), `-Position` (default
  `-1600,150`), `-IdleSeconds` (60), `-ScenarioSeconds` (10), `-SettleSeconds` (8),
  `-WarmLaunches` (3), `-SoakMinutes` (0, which skips the soak), `-Skip` (any of
  `startup`, `idle`, `scenarios`, `resize`, `stress`, `multiwindow`, `soak`), and
  `-Note` to record the background load. The output folder holds `summary.md`,
  `summary.json` and `raw\run<N>\`.
- **`scripts/compare-gpui.ps1`**: the jamb and GPUI hello comparison, described in
  [docs/comparison-gpui.md](docs/comparison-gpui.md). `-Position` sets the window
  placement. `-IncludeFocused` adds the focused idle sample, which activates each
  window, so it is off by default.
- **`scripts/bench-go.ps1`**: every Go benchmark in the workspace, `-count 10` by
  default, with benchstat's summary. `-Filter`, `-Skip`, `-Package`, `-BenchTime`,
  `-Baseline` and `-CpuProfile` narrow or compare runs. The raw file starts with the
  machine header, so a result can be traced to its hardware and power plan.
- **Dev hooks** (documented with their exact semantics in
  [docs/tools.md](docs/tools.md) and `app/dev.go`):
  - `JAMB_FRAMESTATS=PATH` records every frame of the first window: its CPU time,
    the interval between Presents (idle gaps excluded), and the latency of each
    scripted input event, from its arrival to the return of `Present`. It writes
    `PATH-input.csv` beside the frame file.
  - `JAMB_HEAPLOG=PATH` writes the memory counters now and every
    `JAMB_HEAPLOG_SECONDS` (default 10) seconds, for soak runs.
  - `JAMB_NOACTIVATE=1` shows every window without activating it, so a run leaves the
    user's focus and pointer alone.
  - `JAMB_POSITION="X,Y"` places the first window's top-left corner at logical
    coordinates, including negative ones on a monitor left of or above the primary.
  - `JAMB_SCRIPT` steps `mark` and `repeat N` loop a scripted sequence. The scenario
    loops use them.
  - `JAMB_STRESS=paint|rebuild` with `JAMB_VSYNC=0` for uncapped runs.

Scripted input (`JAMB_SCRIPT`) is delivered inside the app, not by the operating
system. The real cursor, the keyboard focus and the window activation are never
touched by the live runs.

### Startup

Live, `bench-live`, 3 runs, process start to the first frame. The figures are
medians across the runs with the range in brackets. "Visible" is the window shown,
"painted" the first frame with content, and "trace" the app's own
`JAMB_TRACE` line ("first frame presented ... after process start").

| App | First launch: visible | First launch: painted | First launch: trace | Warm: visible | Warm: painted | Warm: trace |
|---|--:|--:|--:|--:|--:|--:|
| hello | 462 [459.8–514.1] | 487.4 [470.6–529] | 198.1 [197.5–210.1] | 213.8 [207.1–221.8] | 231.9 [221.2–237.1] | 195.8 [193.5–202.2] |
| demo | 608.7 [602.9–683.6] | 640.4 [630.6–707] | 214.9 [209.8–226.1] | 226 [213.7–260.8] | 251.8 [247.1–290] | 214.8 [212–247.1] |

All times are in milliseconds. A new copy of an executable is scanned by Windows
Defender before its first run, which accounts for the difference between the first
and warm launches. The same first-launch effect is in the GPUI comparison below.

### Idle

Live, `bench-live`, 60 s unfocused after an 8 s settle, 3 runs. The window was on
the secondary monitor.

| App | CPU, % of one core | Context switches/s | Private MB | Working set MB | Threads | Handles | GPU dedicated MB | GPU shared MB |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| hello | 0.08 [0.08–0.16] | 66 [65.7–66.3] | 78.1 [77.9–79.3] | 63.3 [63.2–63.4] | 39 | 509 | 13.7 | 1.5 |
| demo | 0.13 [0.1–0.13] | 65.6 [65.3–65.6] | 122.9 [122.2–123.3] | 117.2 [116.8–117.7] | 40 [40–41] | 539 [539–545] | 17.1 | 2.1 |

Two windows (the demo's main window and a project window), 60 s, 3 runs: CPU
0.1% [0–0.21], 65.4 context switches/s [64.9–65.9].

Idle with the window focused was not measured on 2026-10-21. The focused figures
are from 2026-10-12 in the GPUI comparison (jamb 0.29% of one core, 185 context
switches/s). That pass used the primary monitor, a focused window and an idle
machine.

The earlier demo memory figures (33 MB to 41 MB, from 2026-10-08 and 2026-10-12)
were Task Manager's private working set. That is a different counter from the
private bytes above, so the two are not compared. See the
[earlier memory measurements](#startup-memory-and-binary-size-2026-10-12).

### Scenarios

Live, `bench-live`, the demo at its default 1200 x 760 window, about 10 s per
scenario including the scripted setup, 3 runs. Each scenario's loop is scripted
inside the app: the Activity list is scrolled, the pointer moves over the Activity
rows, the Activity filter is typed in, a continuous animation runs (the easing rows
of the Components page), the app switches pages, and the code editor is typed in
and scrolled.

Frame statistics, from `JAMB_FRAMESTATS`:

- **CPU per frame** is the time for build, layout, paint and submit. It excludes the
  time the frame spends blocked in `Present`.
- **Present interval** is the time between one `Present` return and the next, for
  frames that follow directly on the previous one. A pause with no frame in it
  is not counted.
- **Over 1.5x** counts the frames whose present interval exceeded 1.5 times the
  period of the app's own refresh figure (144 Hz here).

| Scenario | CPU, % of one core | Frames | CPU per frame p50 / p95 / p99 (ms) | Present interval median / p95 (ms) | Over 1.5x | Input to Present, p50 / p95 (ms) |
|---|--:|--:|--:|--:|--:|--:|
| activity-scroll | 19.35 [19.21–24.81] | 2,118 [2,114–2,122] | 0.32 / 1.2 / 2.27 [1.97–2.88] | 6.06 / 7.06 | 7 [6–16] | scroll (409 events): 6.11 / 8.01; click (1): 7.02 |
| pointer-moves | 15.3 [15.3–17.94] | 2,088 [2,081–2,116] | 0.27 / 0.46 / 0.6 [0.57–0.62] | 6.06 / 6.21 | 7 [3–9] | move (506 events): 6.06 / 6.22; click (1): 6.81 |
| typing | 4.84 [4.37–7.17] | 346 [323–347] | 0.3 / 4.57 / 8.39 [6.13–8.55] | 6.05 / 6.21 | 0 | key (98 events): 7.86 / 17.39; click (2): 10.78 |
| animation | 18.58 [14.36–18.89] | 2,064 [2,055–2,075] | 0.32 / 0.59 / 0.72 [0.64–0.74] | 6.06 / 6.22 | 3 [3–6] | key (1): 5.29; click (2): 68.09 [61.81–69.83] |
| page-switch | 17.32 [16.23–17.8] | 1,824 [1,815–1,848] | 0.3 / 0.64 / 2.27 [2.22–2.28] | 6.06 / 6.25 | 9 [8–35] | click (42 events): 7.09 / 26.73 [25.64–32] |
| code-editor | 22.64 [22.03–24.04] | 1,236 [1,188–1,262] | 0.25 / 3.99 / 6.34 [3.45–7.16] | 5.95 / 11.9 | 40 [14–55] | key (165): 7.51 / 11.77; scroll (164): 8.1 / 12.39; click (3): 7.35 / 60.92 [53.65–65.71] |

Values are medians across the three runs, with the range in brackets where it
varies. Input latency runs from the scripted event's arrival to the return of
`Present`, and stops there. It is not the time to the photons on the screen. The
events are injected in-process, so the operating system's input path is not
included either. The animation and code-editor clicks, and the page-switch p95,
are first visits to a page, where the page is built for the first time in those
runs.

The median present interval is about 6 ms in every scenario (5.95 to 6.06 ms). That
is the 165 Hz primary monitor's period, not the 144 Hz secondary monitor's (6.94 ms).
See [Known issues](#known-issues).

### Resize

Live, `bench-live`, 10 s of `SetWindowPos` on the demo window, without activation,
3 runs.

| CPU, % of one core | Context switches/s | Frames | CPU per frame p50 / p95 (ms) | Over 1.5x |
|--:|--:|--:|--:|--:|
| 13.86 [10.42–14.5] | 2,473 [2,383–2,518] | 567 [555–569] | 1.42 [1.39–1.52] / 2.04 [1.95–3.06] | 0 |

The summary reports no present interval for resize (n/a).

### Stress

Live, `bench-live`, the demo for 10 s in each mode, 3 runs. `rebuild` re-renders
every component on every frame. `paint` repaints the retained tree. `vsync` is the
default, and uncapped runs set `JAMB_VSYNC=0`.

| Mode | Frames/s | CPU per frame avg / p50 / p99 (ms) | Allocated KB/frame | Objects/frame | GC cycles/s |
|---|--:|--:|--:|--:|--:|
| rebuild, vsync | 163.8 [163.3–164.1] | 0.97 [0.91–1.06] / 0.85 [0.82–0.97] / 2.15 [1.99–2.18] | 936.1 | 2,731 | 10.5 [10.4–10.6] |
| rebuild, uncapped | 729.2 [707–736.7] | 0.72 [0.69–0.75] / 0.64 [0.63–0.67] / 1.65 [1.58–1.78] | 934.1 | 2,713 | 46.2 [44.8–47.1] |
| paint, vsync | 164.6 | 0.3 [0.27–0.33] / 0.28 [0.26–0.3] / 0.5 [0.5–0.57] | 3.6 | 50 | 0.1 |
| paint, uncapped | 1,796.4 [1,666.6–1,799.5] | 0.15 [0.15–0.16] / 0.14 [0.14–0.15] / 0.25 [0.24–0.26] | 0.7 | 27 | 0.1 |

The page under stress has 310 nodes. The allocation figures for `rebuild` are a
regression from the earlier 110 KB and about 550 objects per frame on a 210-node
page (2026-10-06). See [Known issues](#known-issues).

### Two windows

The two-window figures are in [Idle](#idle). The counters are for the whole process.

### Soak

Live, `JAMB_HEAPLOG` over 30 minutes (one run, commit `2c5c6b0`). The loop scrolls,
switches pages and types in the demo. Memory counters, in MiB as the script reports
them:

| Counter | Start | End | Max |
|---|--:|--:|--:|
| Go heap in use | 5.66 | 66.55 | 129.28 |
| Private bytes | 56.79 | 231.89 | 257.59 |
| Working set | 22.46 | 227.29 | 252.96 |
| Go heap objects | 15,072 | 269,745 | 506,936 |
| Goroutines | 4 | 3 | 4 |

The script's own "slope per hour" column is fitted over the whole run, so it
includes the first five minutes, while the heap and the process are still growing
to their working size. That warm-up dominates the fit, and the column reads as
growth when there is none. The figures below use the minutes after the first five.
They were fitted by least squares to the heap log, from 300 s to 1,800 s:

- Go heap in use: **−6.2 MiB/hour**.
- Private bytes: **−5.0 MiB/hour**.
- Working set: **−4.8 MiB/hour**.

Once the warm-up is over, every 5-minute bucket stays in the same range: the Go
heap in use between **66 and 129 MiB**, and private bytes between **222 and 258
MiB**. Over the 30 minutes the process ran **831 GC cycles** and allocated **40.8
GiB** (about 23 MiB/s), with about 185 million allocations. The heap moves up and
down between collections, and none of the three memory counters fitted above rises
after the warm-up.

The fit has a short window and one run, so it shows the absence of growth in this
loop. It does not rule out a slow leak that a longer or different workload would
show.

### GPUI comparison

Summary only. The full runtime, build and binary tables, with the method and the
earlier passes, are in [docs/comparison-gpui.md](docs/comparison-gpui.md#third-pass-2026-10-21-current).
The third pass ran hello for jamb and for GPUI on the secondary monitor, without
activation, three runs each. jamb has the lower idle CPU (0.05% against 1.74%), the
lower pointer and resize CPU, and the faster warm launch (221.6 ms against 304.1 ms
to the first painted frame). GPUI has the lower private bytes (73.2 MB against 78.8
MB), working set (49.0 MB against 58.4 MB), threads and handles, the lower
first-launch window time, and the smaller release binary (9.22 MB against 9.36 MB).
Focused idle was not measured in that pass.

### Go benchmark suite

Go, `bench-go`, `-count 10`, 2026-10-21, commit `2c5c6b0`. Each value is the
benchstat median of the ten runs, with its spread, on one 12-thread machine. The
suite has 194 benchmark functions, and the run included the 100 MB rope benchmarks.
Rows with a wide spread are noted.

Time per operation (sec/op), grouped by package:

**`ui` (the framework runtime, widgets and lists)**

| Benchmark | Result |
|---|--:|
| `HitTest`, 100 / 1,000 / 10,000 nodes | 147.7 / 145.2 / 144.8 ns; 0 allocs |
| `KeyDispatch`, field caret, 100 / 1,000 / 10,000 nodes | 1.275 / 13.01 / 272.9 µs (±4%) |
| `KeyDispatch`, global binding, 100 / 1,000 / 10,000 nodes | 0.989 / 8.705 / 182.8 µs (±4%) |
| `TextInputTyping`, field / 10 KB area | 14.62 µs / 4.336 µs |
| `TreeExpanded`, 1,100 rows, frame / arrow down | 8.156 ms / 8.158 ms (±98%) |
| `TreeExpanded`, 10,100 rows, frame / arrow down | 80.53 ms / 82.04 ms |
| `Animations`, 10 / 100 / 1,000 running | 9.045 µs / 95.78 µs (±22%) / 1.986 ms |
| `AccessTree`, 100 / 1,000 / 10,000 nodes | 12.63 / 113.3 µs / 1.688 ms |
| `AccessUpdate`, 100 / 1,000 / 10,000 nodes | 85.95 µs / 226.3 µs / 2.752 ms |
| `Scale`, full rebuild, 100 / 1,000 / 10,000 nodes | 152.6 µs / 1.145 ms / 12.43 ms |
| `Scale`, single signal update, 100 / 1,000 / 10,000 nodes | 78.28 / 89.80 µs / 310.4 µs (±4%) |
| `Scale`, hover change, 100 / 1,000 / 10,000 nodes | 47.55 / 48.95 / 73.52 µs |
| `Scale`, repaint only, 100 / 1,000 / 10,000 nodes | 46.56 / 47.79 / 71.51 µs |
| `Scale`, window resize, 100 / 1,000 / 10,000 nodes | 55.10 µs / 142.1 µs / 1.880 ms |
| `InitialBuild`, 100 / 1,000 / 10,000 nodes | 221.8 µs / 1.196 ms / 11.86 ms |
| `FrameRebuild` | 259.2 µs |
| `ShrinkingTextRows` / `BuildTextContent` | 41.06 µs / 2.041 ms |
| `SplitsResized` / `SplitsResizedWalk` | 874.9 ns / 453.5 µs (±15%) |
| `WidgetFrameNoInspector` / `WidgetFrameInspector` | 2.166 ms / 28.10 ms |
| `UniformList`, build (list, 100k / div, 10k) | 287.3 µs / 61.72 ms |
| `UniformList`, scroll step (list, 100k / div, 10k) | 150.5 µs / 360.2 µs |
| `ListOf`, build / scroll / rebuild, 100k rows | 12.18 ms / 97.04 µs / 13.06 ms |
| `DataGridBuildFirst100k` / `DataGridKeyFrame100k` / `DataGridSort100k` | 11.47 ms / 377.2 µs / 7.640 ms |

**`examples/demo/views`**

| Benchmark | Result |
|---|--:|
| `ComponentsScroll` | 182.4 µs (±6%) |
| `ThemeSwitch` | 10.08 ms (±7%); 15.39 MiB, 76,600 allocs per op |
| `PageSwitch` | 2.979 ms (±3%); 4.618 MiB, 17,620 allocs per op |

The `PageSwitch` node count is 237 with a spread of ±1189%, so it is not a stable
measure; the time is.

**`internal/text`**

| Benchmark | Result |
|---|--:|
| `GlyphRaster`, latin, warm / cold | 10.83 µs / 228.0 µs (cold: 251 allocs, 32.5 KiB) |
| `GlyphRaster`, CJK, warm / cold | 3.373 µs / 44.98 µs |
| `LayoutScripts`, latin / bidi / arabic / devanagari / CJK | 351.3 / 648.9 / 518.7 µs / 1.314 ms / 525.0 µs |
| `LayoutParagraph` | 351.0 µs |
| `EditLongDocument` | 2.103 µs |

**`internal/layout`**

| Benchmark | Result |
|---|--:|
| `Layout`, 259 / 1,093 / 1,555 nodes | 2.975 / 3.003 / 2.941 ns; 0 allocs |
| `ShrinkingRows` | 17.25 µs |

These layout figures measure a layout that is already cached, not a full one. See
[Known issues](#known-issues).

**`internal/rope`**

| Benchmark | 2.6 MB / 100 MB |
|---|--:|
| `Build` | 1.994 ms / 74.47 ms |
| `Iterate` | 69.23 µs / 7.451 ms |
| `Line` | 206.3 ns / 642.2 ns |
| `Point` | 254.2 ns / 663.8 ns |
| `UTF16` | 345.6 ns / 740.5 ns |
| `Insert` | 973.8 ns / 1.132 µs |

**`codeedit`**

| Benchmark | Result |
|---|--:|
| `Open100kLines` | 5.771 ms |
| `Keystroke50kLines` / `KeystrokeWrapped50kLines` | 991.1 µs / 1.510 ms |
| `ScrollFrame50kLines` / `ScrollFrameWrapped50kLines` | 719.6 µs / 645.0 µs |
| `WrapWidthChange100kLines` | 4.108 ms |

**`markdown`**

| Benchmark | Result |
|---|--:|
| `Parse` | 1.700 ms (±6%) |
| `ParseAndBuild`, 300 / 2,000 lines | 1.761 ms / 12.25 ms |
| `Scroll` | 52.90 µs |

**`charts`**

| Benchmark | Result |
|---|--:|
| `Line100k` / `Area100k` | 346.6 µs / 348.5 µs |
| `LTTB` | 281.5 µs |

Allocation figures for each benchmark are in the raw output that `bench-go.ps1`
wrote for this run (`bench-2026-10-21-0429-2c5c6b0-dirty.txt`, with its
`.summary.txt` from benchstat).

### Known issues

These are the findings of this pass. Each is open. None has been fixed.

1. **Frame pacing on the 144 Hz monitor.** A live window on the 144 Hz secondary
   monitor presented at a median interval of 6.06 ms in every scenario, which is
   the 165 Hz primary monitor's period (6.06 ms), not the secondary's (6.94 ms). The
   frame-latency wait appears to follow the primary monitor's or DWM's clock, not
   the monitor the window is on. The vsync stress run reached 163.8 FPS, which is
   the same clock. This is an open question, not a result: none of the runs in this
   pass put a window on the primary monitor, so the cause is not confirmed.
2. **Allocation in a full re-render.** The `rebuild` stress allocates 934–936 KB and
   about 2,700 objects per frame on a 310-node page, against 110 KB and about 550
   objects on a 210-node page on 2026-10-06. GC runs 10.5 times a second with vsync
   and 46.2 times a second uncapped. Uncapped throughput is higher than it was
   (729 FPS against 285 on 2026-10-06), so the node count and the added work per
   node may account for part of the change, but the allocation increase has not been
   profiled. Treat it as a regression to investigate.
3. **Key dispatch scales with the tree.** `KeyDispatch` grows with node count: 1.3 µs
   at 100 nodes, 13 µs at 1,000 and 273 µs at 10,000 for a field caret. The cause is
   `Runtime.topModal()` in `ui/modal.go`, which walks the whole tree on every key down
   and key up to find an open modal. A CPU profile at 10,000 nodes attributed about
   97% of the time to that walk. The walk does not stop at the first modal, and its result is
   not cached. (The profile itself is not in the result folders of this pass.)
4. **Expanded trees.** A tree with 10,100 expanded rows takes about 80 ms per frame
   (`TreeExpanded`), and the rows scale linearly with their count. The 1,100-row case
   takes about 8 ms.
5. **First visit to a page.** A click that opens a page for the first time reaches
   Present in about 55 to 70 ms (the animation and code-editor clicks), and the
   page-switch p95 is about 27 ms. Both are first-visit costs. Later visits are not
   measured in the live runs.
6. **Activity list scroll.** Scrolling the 10,000-event Activity list costs a p99 of
   2.27 ms of CPU per frame [1.97–2.88], about seven times the p50 of 0.32 ms, and
   7 frames [6–16] exceed 1.5 times the 144 Hz period.
7. **Binary sizes grew since 2026-10-14.** The stripped hello binary is 9.36 MB,
   against 8.41 MB on 2026-10-14, and the stripped demo is 19.61 MB, against 15.99 MB.
   The default-flag builds are 13.01 MB (hello) and 26.91 MB (demo). The causes were
   not investigated in this pass.
8. **The layout benchmark measures the cache.** `Layout` re-runs `Compute` on an
   unchanged tree, which is answered from the layout cache. Its 3 ns figure is a
   cache hit, not a layout, so it is not comparable to the earlier layout figures of
   0.1 ms and 0.7 ms (2026-10-06). A benchmark that dirties the tree before each
   pass is needed for a full-layout figure.
9. **Noisy rows.** `PageSwitch` nodes (±1189%), `TreeExpanded` arrow down at 1,100
   rows (±98%), `Animations` at 100 running (±22%), and the `gc/op` of
   `Scale` at 10,000 nodes for single signal updates (±35%) have spreads too wide to
   read small changes from with ten runs.

### Gaps

What these numbers do not cover:

- **One machine.** Every figure is from one desktop with an i5-11600K and an RTX 3070 Ti.
  There is no integrated GPU, no laptop, no high-DPI display, and no multi-GPU setup.
- **No focused idle this pass.** The focused-window idle samples were not taken on
  2026-10-21, because the windows were never activated. Focused idle is in the
  2026-10-12 comparison only.
- **No peer frameworks except GPUI.** Gio and Fyne would take focus at launch, so the
  live method could not be used for them. The GPUI comparison is the only peer
  measured on this machine.
- **Latency stops at Present.** Input latency is measured from the injected event to
  the return of `Present`. It does not include the compositor, the monitor's scan-out
  or the photon, and the events are injected in-process.
- **Advisory CI benchmarks.** The pull-request benchmark job runs on shared
  GitHub-hosted runners, which are noisy. It reports a comparison with the base in
  the job summary and never fails a pull request for a slowdown.
- **Soak length and workload.** One 30-minute run of one loop.
- **Not measured on 2026-10-21:** build footprint (`scripts/bench.sh`), the
  cold-build and save-to-window times, the glyph-atlas memory, the code-editor
  highlighting comparison, and the multi-window memory delta. These keep their
  earlier dates.

## Earlier measurements

The sections below were measured between 2026-10-06 and 2026-10-14. They are kept
with their dates, because some of their figures are the only source for facts that
were not re-measured on 2026-10-21. A figure that a 2026-10-21 measurement replaces
is marked superseded where it appears.

The original machine and the summary of that period follow. The machine was a
desktop with a 6-core CPU and a 165 Hz display, with the windows focused and the
machine idle. The 2026-10-21 pass differs on each of those points.

| | |
|---|---|
| Machine | Desktop PC: 6-core CPU, discrete GPU, 32 GB RAM, 165 Hz display at 100% scale |
| OS / toolchain | Windows 11, Go 1.27.1 windows/amd64, no cgo |
| Demo | 4 pages; 210 nodes on Projects, 227 on Activity (10,000 events in a `UniformList`) |

### Summary of 2026-10-06 to 2026-10-14

| Metric | Result | Target |
|---|---|---|
| Incremental build (edit a demo or framework file) | **250–370 ms** for edits that don't change APIs, **~550–840 ms** for real edits (relink) | well under 1 s ✅ |
| No-op rebuild | 219 ms | |
| Cold build (empty GOCACHE, compiles the std lib too) | 7.7 s | |
| Save → new window on screen (`jamb dev`) | **1.05–1.36 s** | |
| Binary size | ~~With `-trimpath -ldflags "-s -w"`: demo **16.0 MB**, hello 8.4 MB (2026-10-14)~~ *superseded 2026-10-21: stripped demo 19.61 MB, hello 9.36 MB.* Earlier: demo 9.9 MB, hello 7.2 MB (2026-10-12; see [Startup memory and binary size](#startup-memory-and-binary-size-2026-10-12)) | 5–30 MB ✅ |
| Repository size | 487 KB (≈9k lines of Go, including tests) | |
| Dependencies | `golang.org/x/image` only (plus pieces of `x/text` and `x/sys`); 59 MB in the module cache | |
| Build cache produced by a cold build | 51 MB (in the shared GOCACHE, not the repo) | |
| Startup (process start → first frame on screen) | ~~**~195 ms**, of which ~180 ms is creating the Direct3D device (2026-10-12)~~ *superseded 2026-10-21 for the totals: 214.8 ms demo, 195.8 ms hello (trace, warm). The Direct3D share still applies.* | perceptually instant ✅ |
| Idle CPU | ~~**0 ms of CPU in 10 s** (1 frame rendered in total)~~ *superseded 2026-10-21: 0.13% of one core for the demo over 60 s unfocused, on the secondary monitor.* | ≈0% ✅ |
| Idle memory | ~~Demo: 33 MB private working set (Task Manager's Memory column; 71 MB working set, 88 MB private bytes). Hello: 23 MB private working set, 77 MB private bytes (2026-10-12)~~ *superseded 2026-10-21: a different measure. Demo private bytes 122.9 MB and working set 117.2 MB, 60 s unfocused.* | reasonably small ✅ |
| Interactive FPS | ~~holds 164 FPS (display refresh) in every mode tested~~ *superseded 2026-10-21: the vsync stress reaches 163.8 FPS, but the present interval on the 144 Hz monitor is 6.06 ms, not 6.94 ms. See [Known issues](#known-issues).* | 60 / 120 FPS ✅ |
| Uncapped throughput (vsync off) | ~~319 FPS repaint-only, 285 FPS with a full re-render every frame~~ *superseded 2026-10-21: 1,796 FPS paint-only, 729 FPS full re-render* | |
| CPU per frame, full re-render of every component | ~~**~0.3 ms** avg (build + layout + paint + GPU submit)~~ *superseded 2026-10-21: 0.97 ms average on a 310-node page* | budget 8.3 ms at 120 Hz |
| CPU per frame, repaint of retained tree | ~~**~0.13 ms** avg~~ *superseded 2026-10-21: 0.15 ms average uncapped, 0.30 ms at vsync* | |
| One signal change, 10,000-node tree | ~~**0.59 ms**: 1 component re-rendered, 5 nodes rebuilt, 7 re-laid out~~ *superseded: 0.31 ms (2026-10-21, Go suite); the 2026-10-08 figure was about 0.30 ms* | |
| Allocation | 3–6 KB/frame repainting; ~~110 KB/frame (≈550 objects) for a full re-render~~ *superseded 2026-10-21: 936 KB and 2,731 objects per frame for a full re-render, at vsync (a regression, see Known issues)* | |
| GC | 0.2 cycles/s repainting; ~~3.5–5.7 cycles/s during continuous full re-render, 3–4 ms total pause over 4 s~~ *superseded 2026-10-21: 0.1/s repainting; 10.5/s and 46.2/s for the full re-render at vsync and uncapped* | |
| Virtualized list, 100,000 rows: one 120 px scroll step and frame | ~~**169 µs**, 195 nodes built~~ *superseded: 150.5 µs (2026-10-21, Go suite `UniformList`)*. A plain container of 10,000 rows: 13.3 ms, 50,002 nodes (2026-10-06); 360 µs per step on 2026-10-21 | |

The rest of this section is the detail behind the table, in the order it was
measured.

## How each number was produced

These are the methods of the earlier measurements, 2026-10-06 to 2026-10-14. Each
section names its date. Figures in them are not re-measured on 2026-10-21 unless
the section says so.

All of these hooks are in the framework, so they work for any jamb app.

### Build and footprint

Measured 2026-10-06 and 2026-10-14 with `scripts/bench.sh`. Not re-measured on
2026-10-21. The build-time and footprint rows of the summary keep these dates.

```
bash scripts/bench.sh
```

The script builds into a throwaway `GOCACHE` for the cold build, so your
real cache is untouched. For incremental builds it appends a declaration to
a file, rebuilds, then restores the file. The first rebuild after each
restore is slower (~700 ms) because the restored content has to be
recompiled too; the table above reports steady-state numbers.

Edits that change a package's export data, such as a new string in a view,
force dependents to recompile and the binary to relink. That's the
550–840 ms band, and it's what `jamb dev` reports in practice. Skipping
DWARF (`-ldflags=-w`) saved only about 40 ms, so the runner uses default
flags.

### Startup

Measured 2026-10-06 and 2026-10-12. The totals in this section are superseded by
the 2026-10-21 trace figures in Current results (hello 195.8 ms and demo 214.8 ms
warm). The phase breakdown below was not re-measured.

```
JAMB_TRACE=1 JAMB_SCRIPT="exit" ./demo.exe
jamb: renderer ready in 180ms
jamb: first frame presented 202ms after process start
```

The window is created hidden and shown only after the first frame is
presented, so there's no white flash. Startup breaks down as follows:

| Phase | Time |
|---|---|
| Direct3D device and swap chain creation | 176–184 ms |
| Render target, pipelines and states (shader bytecode is cached on disk after the first run; compiling the HLSL costs ~50 ms once) | ~2 ms |
| First UI build, layout and paint, including font loading and glyph rasterization | ~6–10 ms |
| Everything else (window class, window, Go runtime) | ~10 ms |

The floor is GPU driver initialization. Creating the device on a goroutine
while the window is created would save at most ~20 ms.

### Idle CPU and memory

Measured 2026-10-06 and 2026-10-08. The memory figures here are Task Manager's
private working set, which is not the private-bytes counter of Current results, so
the two are not compared. The idle CPU figure (0 ms in 10 s) is superseded by the
60 s measurement of 2026-10-21.

Launch the demo, wait 3 s, then read `TotalProcessorTime` before and after
10 s (PowerShell `Get-Process`). With `JAMB_TRACE=1` the app also prints how
many frames it renders per second of activity. When idle it renders nothing:
the Win32 loop blocks in `GetMessage` and only wakes for input or a
signal change.

One run measured 10% CPU.

Working set and private bytes are dominated by the Direct3D driver. The Go
heap is 6–9 MB (reported by the stress mode below).

**Re-measured 2026-10-08** (memory of a default `go build` of the demo,
three runs each, after 6 s idle):

- Private working set (Task Manager's "Memory"): 41–42 MB. One earlier run
  read 73 MB for the same binary; Windows decides how much stays resident,
  so single readings vary. `go run` adds its own `go.exe` (~13 MB), which
  Task Manager groups with the app under the terminal.
- A window showing only "hello" uses the same as the demo, so the demo's
  content costs little. The Go heap is ~10 MB for "hello" (two 1024×1024
  glyph atlases, grayscale and ClearType, ~5 MB; Segoe UI and its parsed
  tables ~2 MB) and ~14 MB for the demo (its images and UI tree).
- The rest is outside Go: the GPU driver's user-mode libraries alone are
  mapped at ~190 MB, mostly shared with other processes. Windows Notepad
  measures about the same (40 MB private working set, 124 MB working set).
- Private bytes dropped from 133 MB to 102 MB after the shader cache key
  stopped using `crypto/sha256`: in current Go that import links the FIPS
  crypto module, whose DRBG reserves a 32 MB buffer in every jamb binary. The
  renderer now names cache files with FNV-128, and jamb imports no crypto
  package.
- 2026-10-14: the single-instance pipe name in `app` had brought the import back
  with `crypto/sha256`; it now uses FNV-1a, and hello and notes link no crypto
  package again (`crypto/internal/fips140/drbg.memory` is absent). An app that
  imports `codeedit` still links it, through wazero, which hashes modules with
  SHA-256, so such an app carries the 32 MB reservation. jamb cannot remove it
  with a `replace` directive, since that applies only to jamb's own module and
  not to the apps built on it.

### Idle work: animations, occlusion, caret and toasts (2026-10-12)

Measured 2026-10-12 with the windows focused on the 165 Hz primary monitor, on an
idle machine. Not re-measured on 2026-10-21, which measured unfocused windows on the
secondary monitor (see Current results).

The idle audit looked for work the app does when nothing changes. Method: each
binary runs alone for 60 s after a 5 s settle. CPU is the change in the
process's `TotalProcessorTime`. Wake-ups are the process's
`ContextSwitchesPersec`, summed over its threads (`Win32_PerfRawData_PerfProc_Thread`,
read at the start and end of the window). The frame count is the number of
`JAMB_TRACE` per-second lines. Each run has its own focus state, read back
with `GetForegroundWindow`. A process takes the foreground after an Alt tap, because
Windows refuses `SetForegroundWindow` from a background process. CPU readings have
15.6 ms granularity, so differences under about 0.2% over 60 s are noise.

| Case | Before: CPU / wake-ups/s / frames | After: CPU / wake-ups/s / frames |
|---|---|---|
| Plain window (hello), focused | 0.52% / 354 / 0 | 0.50% / 337 / 0 |
| Demo Dashboard, focused | 0.23% / 349 / 0 | 0.52% / 327 / 0 (noise) |
| Demo Dashboard, unfocused | 0.13% / 151 / 4 | 0.16% / 145 / 4 |
| Notes, caret blinking, focused (click every 10 s) | 0.29% / 153 / 26 | 0.08% / 156 / 26 |
| Notes, caret blinking, unfocused | 0.23% / 185 / 23 | 0.44% / 182 / 18 (CPU within noise) |
| Demo, Components page, motion bars off screen | 25% (1.27 s in 5 s) | 0% |
| Demo, Components page animating, window covered by another window (20 s) | 31% (6.25 s in 20 s) | 0%, no frames |

What the numbers show:

- Off-screen animations no longer run. The Components page's motion bars were the
  largest idle cost in the demo: a visible window with nothing visible animating
  now costs nothing.
- A covered window stops rendering. The covered demo draws nothing, and the
  current build presents nothing while covered. Rendering resumes when DXGI wakes the
  window (`WM_OCCLUSION`), which was observed in a live run. A live run that brought the
  window back with a visible animation on screen was not completed.
- The caret of an unfocused window is hidden and does not blink. The frame counts in
  the notes rows are dominated by the synthetic clicks that keep the caret active, so
  they do not isolate the blink. The unit test checks the behavior directly.

**Wake-ups that remain** (each one is reported, with its source):

- The GPU vendor's Direct3D user-mode driver runs one thread that wakes about 90 times
  a second once a Direct3D device exists. It is not jamb's code: Go's scheduler is idle
  (`GODEBUG=schedtrace` shows no activity), and the thread's start address maps into the
  driver's module. A plain Win32 window (Notepad) wakes 0.4 times a second. This cost
  is the same for any Direct3D 11 application on the same driver, so the comparison
  with another framework on Windows should be read with it in mind.
- Timers in jamb (only when pending): the caret blink (two wake-ups a second while
  the caret blinks, none after 15 s without input or in an unfocused window), tooltip
  delay, chord timeout, the toast expiry (one wake-up at the end of the toast's time,
  replacing one frame per vsync while a toast was shown), and debounce timers in the
  example apps. The keymap file watcher runs only in apps that set a keymap file.
- None of jamb's timers runs on the idle path of a plain window.

#### Memory, startup and binary size (measured, not changed)

These were changed later; see [Startup memory and binary size](#startup-memory-and-binary-size-2026-10-12).

- Memory: the private working set of the plain window and the demo is the same
  before and after (95 MB and 106 MB, 60 s idle). Most of it is outside Go's heap.
  The glyph atlases already start at 1024×1024 and double when full. The
  ClearType atlas keeps 4 bytes per texel (4 MB) and the grayscale one 1 MB, both
  kept as CPU copies so a lost device can be rebuilt without rasterizing again. That
  is a design choice, and the saving it would give up is small.
- Startup: one traced run of the current demo gives the renderer ready in 180 ms and the
  first frame 219 ms after process start, which matches the floor described above.
  No change was made: the device creation is the critical path.
- Binary size (default `go build`, Windows amd64): plain window 11.9 MB, demo 15.6 MB,
  unchanged by this pass. The largest contributors are go-text's script and language
  tables (about 1.8 MB, data the shaper uses), the runtime's function tables
  (`runtime.pclntab`, about 2 MB), and encoding/json (about 0.3 MB, linked by the keymap file loader).
  `regexp` is not linked.

### Startup memory and binary size (2026-10-12)

The figures in this section are from 2026-10-12. Binary size and memory were
measured again on 2026-10-21, and those figures are in Current results and in
[docs/comparison-gpui.md](docs/comparison-gpui.md). The reasoning below was not
re-checked.

A comparison with GPUI (`docs/comparison-gpui.md`) showed jamb behind on memory and
binary size, so this pass changed both. The hello figures come from the comparison
script (release build, `-trimpath -ldflags "-s -w -H windowsgui"`, memory read after
about two minutes idle), except the Go heap, which an in-process probe read after 3 s.

| | Before | After |
|---|---|---|
| Hello, private bytes | 100.7 MB | 78.2 MB |
| Hello, working set | 78.1 MB | 58.8 MB |
| Hello, GPU dedicated / shared memory | 18.6 / 6.2 MB | 13.8 / 0.6 MB |
| Hello, Go heap in use (after GC) | 17.9 MB | 6.8 MB |
| Hello, stripped binary | 8.45 MB | 7.18 MB |
| Demo, default `go build` (6 s idle) | 102 MB private bytes, 41 MB private working set | 88 MB private bytes, 33 MB private working set |
| Demo, stripped binary | (not measured) | 9.85 MB |

What changed:

- **Glyph atlases** start at 256×256 instead of 1024×1024 and double when full. The
  ClearType atlas keeps 4 bytes per texel on the CPU (for device-loss recovery) and on
  the GPU, so the old starting size cost about 5 MB on each side before any glyph was
  drawn.
- **System font files** are memory-mapped instead of read onto the Go heap. The pages
  are shared with other processes and do not count as private memory.
- **Startup font loading** parses the faces the built-in widgets use (Regular and
  Semibold; Medium resolves to Semibold in Segoe UI). Bold, which no built-in widget
  draws, loads on first use. Each parsed face costs a few megabytes in go-text.
- **Embedded fonts.** Windows builds embed only Go Regular. Segoe UI and Consolas ship
  with every supported Windows version, so the other eight Go faces were never used
  and added 1.3 MB to every binary.
- **Device creation** starts on a goroutine before the first window is created. It
  saves about 10 ms: the device takes about 180 ms inside the graphics driver, and the
  window and fonts were already faster than that.

What remains: the Direct3D driver accounts for about 50 MB of private bytes in every
process that creates a device. Of the Go heap, most is go-text's parsed font tables
(it parses each face's outlines when the font loads). The Go runtime's own
bookkeeping adds a few megabytes on top.

Startup was measured too. Launches of the same executable file paint the first frame
at about 266 ms from the launch call (the trace puts it about 195 ms after process
start). The first launch of a newly copied executable takes about 230 ms longer,
because Windows Defender scans the file first. The earlier comparison launched a new
copy every time, which is why it reported about 750 ms.

### Multiple windows

Measured 2026-10-09, not re-measured. The 2026-10-21 two-window figures (idle CPU
and wake-ups) are in Current results.

Measured 2026-10-09 with the same machine as above, the demo's main window
(custom title bar, 1200×760) and a project window (640×420, native title
bar), using `JAMB_TRACE=1` and PowerShell `Get-Process` (one run each, so
treat the numbers as rough):

| | Renderer ready | First frame presented |
|---|---|---|
| First window (creates the Direct3D device, fonts ready) | 186–196 ms | 213–231 ms after process start |
| Second window (device shared, swap chain only) | 1.1–1.6 ms | 4.2–4.9 ms after `app.Open` |

Memory, one window running for 1.2 s, then two windows open for about 5 s:

| | Private bytes | Working set |
|---|---|---|
| One window (before the project window opens) | 103.0 MB | 73.3 MB |
| Two windows (project window open) | 119.1 MB | 89.9 MB |
| Delta | +16.1 MB | +16.6 MB |

The delta covers the second window's runtime, its swap chain and its
resources, plus anything the process allocated for the first time while it
was open. It was not broken down further.

Idle CPU with the main window and a project window open and nothing
animating: 0 ms of `TotalProcessorTime` over 10 s.

Textures (glyph atlases, images) are uploaded once per device and shared by
every window. A second window that shows the same text does not upload the
atlas again.

#### Frame pacing with two animating windows

These figures are from the 165 Hz primary monitor, 2026-10-09. On 2026-10-21 a
window on the 144 Hz secondary monitor presented at the primary monitor's period
(6.06 ms); see [Known issues](#known-issues) in Current results.

Two windows that both animate used to run at half the display rate each: one
device-wide frame latency of one, and the windows rendering one after another,
so each `Present` waited for a vblank in turn. Each swap chain now has its own
frame latency waitable object (`IDXGISwapChain2::SetMaximumFrameLatency(1)`),
and the event loop renders a window when its waitable is signaled, sleeping in
`MsgWaitForMultipleObjectsEx` on the waitables and the message queue when none
is.

Measured on the same 165 Hz display, with `JAMB_TRACE=1` (frames in the last
1.0 s, one run each, a looping `ui.Animate` in a 500×300 window):

| | Before | After |
|---|---|---|
| One window | 166 fps | 165–166 fps |
| Two windows, each | 83 fps | 166 fps |

The demo's projects page scrolled with scripted wheel steps renders 166 fps
while scrolling. Idle CPU with a main window and a project window open is 0 ms
over 10 s once the windows have settled. The first 10 s after launch read
15.6 ms, one timer tick, which is startup work.

### Frame cost, FPS, allocation and GC

Measured 2026-10-06 to 2026-10-08 on the 165 Hz primary monitor. The sample output
below is superseded by the 2026-10-21 stress run in Current results (310 nodes, 936
KB and 2,731 objects per frame for a full re-render at vsync).

```
JAMB_STRESS=paint   JAMB_STRESS_SECONDS=4 ./demo.exe   # repaint every frame
JAMB_STRESS=rebuild JAMB_STRESS_SECONDS=4 ./demo.exe   # re-render every component every frame
JAMB_VSYNC=0 JAMB_STRESS=rebuild ./demo.exe            # uncapped
```

Sample output (vsync on, 165 Hz display):

```
jamb stress mode=rebuild nodes=210 frames=657 elapsed=4.00s fps=164.2
  cpu per frame (build+layout+paint+submit): avg=342µs p50=499µs p99=1.044ms max=7.501ms
  alloc: 112.7 KB/frame, 550 objects/frame, 18.96 MB/s; GC cycles=14 (3.5/s), pause total=3.7616ms; heap=8.6 MB
```

Per-frame CPU excludes time blocked in `Present`. On Windows, Go's
monotonic clock has ~0.5 ms granularity, so averages are meaningful but
p50/p99 values are quantized. The `max` values (6–7 ms) are the first
frames, where fonts are loaded and glyphs rasterized.

"Rebuild" is the worst case: it ignores signals and re-runs every
component every frame. Normal updates are incremental, as described in the
next section.

## Incremental updates and scaling

Measured 2026-10-06 and 2026-10-08. The same operations at the same sizes were
measured again on 2026-10-21 in the Go suite, and those are the current figures (see
the `Scale` and `InitialBuild` rows of Current results). The tables below are kept
for their history and their notes on culling.

```
go test ./ui -run xxx -bench 'Scale|InitialBuild' -benchtime 1s
```

The benchmark list is a scrolling column of keyed row components. Each row
has 5 nodes (row, icon box, text column, two text lines) and reads its own
signal. Times are total CPU per operation through `Runtime.Frame`: build,
layout and paint. GPU submission isn't included.

| Operation | 100 nodes | 1,000 nodes | 10,000 nodes |
|---|---|---|---|
| Initial build (startup, theme switch) | 162–213 µs | 1.01–1.06 ms | 10.5–11.2 ms |
| Full re-render of every component (`InvalidateAll`) | 124–165 µs | 930–976 µs | 11.6–12.6 ms |
| One signal change | **61–66 µs** | **77–81 µs** | **295–302 µs** |
| Hover moves to another row | 44–49 µs | 46–50 µs | 65–68 µs |
| Repaint only (animation frame) | 43–46 µs | 44–45 µs | 63–66 µs |
| Window resize (width-dependent re-layout) | 52–57 µs | 133–136 µs | 1.90–3.04 ms |
| Nodes painted by the frame (`painted`) | 73 | 73 | 73 |

Measured 2026-10-08 with culling, in three runs of `docs/benchmarks/bench-after.txt`'s
command. Each cell is the range across the runs. The machine is noisy: the
spread is up to a third at 100 nodes and at 10,000 nodes for window resize.

**Culling landed 2026-10-08.** Layout computes each node's *ink*, the
rectangle its drawing can reach (its box, shadows, offsets, scale, glyph
overshoot and content). Paint skips any subtree whose ink misses the visible
area, and hit testing visits only the nodes the last paint drew. The
benchmark's scroll column has 73 nodes on screen at every size, so painting
and hovering a 10,000-node tree now cost about what a 100-node tree did. The
same benchmark, run earlier the same day before culling:

| Operation | 100 nodes | 1,000 nodes | 10,000 nodes |
|---|---|---|---|
| Initial build | 191 µs | 1.02 ms | 10.2 ms |
| Full re-render | 114 µs | 849 µs | 10.6 ms |
| One signal change | 62 µs | 99 µs | 904 µs |
| Hover moves to another row | 44 µs | 92 µs | 1.30 ms |
| Repaint only | 43 µs | 70 µs | 632 µs |
| Window resize | 48 µs | 125 µs | 2.36 ms |
| Nodes painted | 102 | 1,002 | 10,002 |

The 2026-10-06 values, from before the overlay, list and animation work, were:
initial build 174 µs, 1.09 ms, 9.6 ms; full re-render 111 µs, 884 µs, 10.2 ms;
one signal change 48 µs, 74 µs, 588 µs; hover 42 µs, 67 µs, 682 µs; repaint
40 µs, 57 µs, 378 µs; resize 44 µs, 98 µs, 1.10 ms.

**Re-measured 2026-10-08, with culling.** At 10,000 nodes, repaint went from
632 µs to about 64 µs, a hover from 1.30 ms to about 66 µs, and one signal
change from 904 µs to about 300 µs (its frame includes a paint). The full
re-render is 10–20% slower at 1,000 and 10,000 nodes. The extra work is the
ink pass in layout, which measures leaves a second time to find their
overflow; I have not profiled it further. Window resize at 10,000 nodes
ranges 1.90–3.04 ms against 2.36 ms before, which is more noise than change.
The earlier figure of 0.93 ms for a 10,000-node repaint came from another
run; the same-day baseline above measured 0.63 ms.

Paint still pays per-node cost for what is on screen: a node is about 1 KB, so
a tree whose nodes are all visible is not made cheaper by culling. The compact
per-node paint struct is the remaining step. `ui.UniformList` avoids the cost
for long lists today.

The work the runtime reports for each operation, the same at every size:

| Operation | Components rendered | Nodes built | Nodes re-laid out | Allocations |
|---|---|---|---|---|
| One signal change | 1 | 5 | 7 | 35 (8.6 KB) |
| Hover | 0 | 0 | 0 | 1 (48 B) |
| Repaint | 0 | 0 | 0 | 0 |

A full re-render at 10,000 nodes allocates 16 MB (61,800 objects) and
triggers about 0.6 GCs per operation.

How an update stays local:

- **Signal change.** It marks the component that read the signal dirty.
  Only that component re-renders, and only its subtree is rebuilt and
  spliced into the retained tree in place; siblings and ancestors keep
  their nodes.
- **Layout.** Layout is invalidated from the changed node up to the root.
  Clean subtrees answer from their cached measurements, and a subtree is
  only re-arranged if it changed or its box changed size.
- **Hover, press and focus.** These restyle just the elements whose state
  flipped. They trigger layout only if the new style changes a layout
  property.

`ui/incremental_test.go` asserts this: exactly 1 render and 3 nodes built
for a one-row change, the other nodes kept by identity, and an unmoved
sibling at the same position.

The costs that remain at 10,000 nodes are not rebuilds:

- **Paint tests every child of every visible node.** Culling skips the
  subtrees that are off screen, but a visible container still runs the
  ink test on each of its children. The list here has 2,000 of them and
  about 14 are on screen. A container with 10,000 children runs 10,000
  tests per paint; each is a few comparisons, but the cost still grows
  with the child count.
- **A changed row's parent re-runs flex over all of its children.** The
  list has 2,000 children, and each is answered from cache. That's
  O(siblings), not O(tree).

## Virtualized lists

Measured 2026-10-08. The 2026-10-21 Go suite gives the `UniformList` scroll step at
150.5 µs (100,000 rows) and 360.2 µs for the plain container (10,000 rows); see
Current results.

`ui.UniformList` builds, lays out, paints and hit-tests only the rows in view
plus an overscan of four rows. Each operation below is a 120 px wheel step
followed by one frame:

```
go test ./ui -run xxx -bench UniformList -benchtime 2s
```

| Operation | UniformList, 100,000 rows | Plain ScrollY Div, 10,000 rows |
|---|---|---|
| Initial build (runtime and first frame) | 213–220 µs | 58.1–58.4 ms |
| Scroll step and frame | **157–164 µs** | 341–346 µs |
| Nodes in the tree | **195–201** | 50,002 |
| Allocations per step | 478 (99 KB) | 98–99 (6 KB) |

Measured 2026-10-08 with culling. Before culling the plain container's step
took 9.7 ms (13.3 ms in the earlier table): paint and the hover update after
each scroll both visited all 50,000 nodes. Now they visit only the rows on
screen, and the plain container's step is 341 µs.

Rows are the five-node rows of the scale benchmarks. The list rebuilds its
window roughly every other step, which accounts for its allocations. The plain
container's allocations are a different matter. This benchmark wraps through
all 10,000 rows, so most rows that enter view have had their text layouts
dropped from the text cache (entries unused for 240 frames are removed), and
those layouts are built again. Scrolling within rows that are still cached
allocates 2 per step. Turning culling off gives about the same wrap-around
count (97 per step measured, against 105 with culling), so culling does not
cause it. The 2 allocations per step recorded before culling came from the
same benchmark doing far fewer steps: at 13 ms a step, a 2 s run made about
150 steps, never passing frame 240, so no text layout had been evicted yet.
At today's speed the run makes thousands of steps and scrolls into evicted
rows. With a fixed step count the numbers agree: 10 allocations per step over
150 steps, 88 over 2,000 (`-benchtime 150x` / `2000x`).

### ListOf: variable-height rows

Measured 2026-10-08. The 2026-10-21 Go suite reports the `ListOf` scroll step at
97.04 µs for 100,000 rows, against 78 µs here.

`ui.ListOf` builds only the rows in view, as `UniformList` does, but each row
can have its own height. Its rows here alternate between 20 and 60 px and each
is a wrapper, a row and a text node. Both lists were measured in the same run:

```
go test ./ui -run xxx -bench 'ListOf|UniformList' -benchtime 2s
```

| Operation | ListOf, 100,000 rows (varied heights) | UniformList, 100,000 rows |
|---|---|---|
| Initial build (runtime and first frame) | 12.6 ms (includes indexing the 100,000 keys) | 207 µs |
| Scroll step and frame | **78 µs** | 153 µs |
| Nodes in the tree | **126** | 195 |
| Allocations per step | 527 (125 KB) | 480 (102 KB) |
| Rebuild after the items slice changes | 13.6 ms, 200,887 allocations | not applicable |

The rebuild is the O(n) pass that runs when the items are a new slice: it calls
the key function for every item and reads each key's measured height. In this
benchmark the key function is `fmt.Sprintf`, so most of the 12 ms is formatting
the keys. A re-render with the same slice does no rebuild, so ordinary frames
do not pay it.

## Text and editing

Measured before 2026-10-21 and not re-measured. The 2026-10-21 text benchmarks
(`GlyphRaster`, `LayoutScripts`, `LayoutParagraph`, `EditLongDocument`) measure
different operations and are listed in Current results.

The text stack was later rebuilt on `go-text/typesetting` (a pure-Go
HarfBuzz port with Unicode line breaking and bidi), and an editor was added
for the notes app. These numbers measure that version.

```
go test ./text -run xxx -bench . -benchtime 300x
```

| Measurement | Result |
|---|---|
| Shape and wrap a ~900-character paragraph, uncached | 0.37 ms |
| One keystroke in an 11,000-character, 50-paragraph note | **0.11 ms** (only the edited paragraph is re-shaped) |
| Notes app, first-frame layout | 3–11 ms (fonts and shaper warmed on a goroutine during GPU device creation; 23–36 ms before that) |
| CPU while the caret blinks | 31 ms per 10 s (two tiny frames per second, scheduled with a timer) |
| CPU after 15 s without typing (caret solid) | 16 ms per 10 s, the measurement floor |
| Release binary (`-trimpath -ldflags "-s -w"`) | demo 6.4 MB (was 3.9 MB before HarfBuzz), notes app 7.1 MB |
| Module cache added | `go-text/typesetting` 23 MB |

### Micro-benchmarks

The layout rows below are superseded. The 2026-10-21 `Layout` benchmark re-runs
`Compute` on an unchanged tree, which is answered from the layout cache, so its
3 ns figure is not comparable to the 0.1 ms and 0.7 ms rows (see Known issues in
Current results).

```
go test ./layout -bench . -run xxx
go test ./ui     -bench . -run xxx
```

| Benchmark | Result |
|---|---|
| Layout, 259 nodes | ~0.1 ms, 0 allocs |
| Layout, 1,093–1,555 nodes (deep or wide synthetic trees, every leaf wrapping text) | ~0.7 ms, 0 allocs |
| `ui` full rebuild + layout + paint, 251-node list | 146 µs, 559 allocs |
| Layout after one leaf changes (50-row list) | only the leaf's 4 ancestors re-arranged (`TestIncrementalLayoutSkipsCleanSubtrees`) |

### Code editor highlighting: tree-sitter and @pierre/highlights (2026-10-19)

Before the code editor's highlighting moved from tree-sitter (the grammars of
jamb-grammars v0.1.0) to @pierre/highlights 0.1.3, both ran side by side in the same
wazero runtime on an Intel i5-11600K (6 cores, 32 GB), Windows 11, Go 1.27.1, wazero
1.12.0. The tree-sitter side called the editor's tree-sitter instance directly (no
worker); the highlights side made the calls its own TypeScript host makes. Times are
medians of batched runs, each sample at least 25 ms, since the Windows clock moves in
steps of about 1 ms. The files are real: the Go standard library and jamb's own
`ui/*.go` (Go), Pierre's benchmark fixtures (TypeScript, CSS, HTML, JSONC), CPython
3.14's standard library (Python), regex-automata 0.4.14 (Rust), and zlib.h and
sqlite3.h (C). TS is tree-sitter and HL is highlights.

| Module | wasm size | cold compile | from disk cache |
|---|--:|--:|--:|
| tree-sitter go | 349 KB | 74 ms | 8.1 ms |
| tree-sitter typescript | 1516 KB | 74 ms | 8.6 ms |
| tree-sitter css | 282 KB | 116 ms | 8.0 ms |
| tree-sitter html | 163 KB | 68 ms | 8.3 ms |
| tree-sitter json | 145 KB | 66 ms | 8.4 ms |
| tree-sitter python | 583 KB | 73 ms | 8.0 ms |
| tree-sitter rust | 1224 KB | 75 ms | 8.0 ms |
| tree-sitter c | 762 KB | 99 ms | 8.7 ms |
| highlights (all 76 languages) | 171 KB | 87 ms | 7.6 ms |

| Document | Size | Lines | TS open | HL open (live init, whole doc) | TS full | HL full (one-shot) | TS MiB/s | HL MiB/s | TS edit | HL edit | TS opener | HL opener (whole file) | HL opener (visible first) | TS memory | HL memory (live) |
|---|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|--:|
| go small (parser.go) | 73 KB | 2957 | 4.0 ms | 2.6 ms | 47 ms | 741 µs | 1.5 | 96.3 | 1.1 ms | 12 µs | 4.2 ms | 1.8 ms | 63 µs | 6.2 MB | 0.8 MB |
| go large (runtime/proc.go) | 237 KB | 8177 | 4.3 ms | 5.9 ms | 91 ms | 1.6 ms | 2.5 | 149.1 | 5.1 ms | 12 µs | 7.9 ms | 3.6 ms | 89 µs | 11.1 MB | 1.4 MB |
| go huge (jamb ui/*.go) | 2758 KB | 78003 | 4.8 ms | 71 ms | 1620 ms | 25 ms | 1.7 | 105.7 | 21 ms | 11 µs | 85 ms | 28 ms | 59 µs | 173.9 MB | 14.0 MB |
| ts small | 30 KB | 826 | 43 ms | 972 µs | 19 ms | 327 µs | 1.6 | 91.4 | 1.5 ms | 12 µs | 2.3 ms | 282 µs | 123 µs | 5.4 MB | 0.4 MB |
| ts large | 516 KB | 10826 | 45 ms | 12 ms | 293 ms | 4.8 ms | 1.7 | 105.9 | 2.8 ms | 13 µs | 4.3 ms | 9.2 ms | 133 µs | 23.8 MB | 2.3 MB |
| css small | 23 KB | 1388 | 1.6 ms | 1.2 ms | 15 ms | 271 µs | 1.6 | 84.6 | 2.7 ms | 11 µs | 3.8 ms | 829 µs | 69 µs | 2.4 MB | 0.4 MB |
| css large | 379 KB | 17455 | 1.1 ms | 13 ms | 167 ms | 3.3 ms | 2.2 | 112.3 | 16 ms | 12 µs | 26 ms | 10 ms | 75 µs | 15.9 MB | 3.3 MB |
| html small | 27 KB | 809 | 319 µs | 628 µs | 10 ms | 120 µs | 2.6 | 227.3 | 632 µs | 11 µs | 590 µs | 125 µs | 78 µs | 1.9 MB | 0.4 MB |
| html large | 473 KB | 11329 | 590 µs | 7.2 ms | 105 ms | 1.3 ms | 4.4 | 350.3 | 1.5 ms | 11 µs | 32 ms | 2.4 ms | 103 µs | 10.1 MB | 2.2 MB |
| json(c) small | 33 KB | 1105 | 297 µs | 810 µs | 12 ms | 140 µs | 2.6 | 231.8 | 2.9 ms | 12 µs | 4.6 ms | 486 µs | 53 µs | 2.4 MB | 0.4 MB |
| json(c) large | 283 KB | 8561 | 515 µs | 6.2 ms | 61 ms | 795 µs | 4.6 | 348.7 | 3.2 ms | 11 µs | 4.6 ms | 4.6 ms | 96 µs | 7.9 MB | 2.1 MB |
| python (typing.py) | 132 KB | 3859 | 12 ms | 3.2 ms | 61 ms | 819 µs | 2.1 | 158.2 | 2.1 ms | 12 µs | 94 ms | 2.3 ms | 97 µs | 7.1 MB | 0.9 MB |
| python (_pydecimal.py) | 223 KB | 6407 | 12 ms | 4.7 ms | 100 ms | 1.3 ms | 2.2 | 164.3 | 13 ms | 12 µs | 122 ms | 3.4 ms | 95 µs | 10.4 MB | 1.2 MB |
| rust (onepass.rs) | 130 KB | 3209 | 42 ms | 2.3 ms | 42 ms | 578 µs | 3.1 | 221.2 | 1.5 ms | 11 µs | 13 ms | 1.4 ms | 97 µs | 6.6 MB | 0.9 MB |
| rust (dense.rs) | 214 KB | 5261 | 43 ms | 3.7 ms | 72 ms | 1.1 ms | 2.9 | 194.3 | 1.6 ms | 11 µs | 6.5 ms | 2.2 ms | 78 µs | 9.8 MB | 1.2 MB |
| c (zlib.h) | 101 KB | 2058 | 7.2 ms | 1.1 ms | 13 ms | 134 µs | 7.6 | 740.8 | 8.3 ms | 11 µs | 7.9 ms | 12 µs | 14 µs | 2.9 MB | 0.8 MB |
| c (sqlite3.h) | 674 KB | 14350 | 7.8 ms | 6.7 ms | 58 ms | 827 µs | 11.4 | 796.4 | 29 ms | 12 µs | 30 ms | 13 µs | 15 µs | 6.3 MB | 2.4 MB |

- TS open is a new instance, the text and the highlight query compiled, without a
  parse; TS full is the parse and the spans of the whole document. HL open includes
  lexing the whole document, so it compares with TS open plus TS full.
- Edit types one character in the middle of the file, highlights again and reads the
  120 visible lines. HL edit is about 11 µs everywhere, because the relex stops at the
  first line whose lexer state is unchanged.
- Opener types `/*` (or `"""`, `<!--`) at the start of a code line 10% into the file,
  which recolors the rest of it, and reads the 120 lines below. "Whole file" lexes to
  the end of the document before it returns; "visible first" stops after the visible
  lines and leaves the rest for later, as the session does. TS reparses the whole tree
  incrementally. In C, `/*` closes at the next `*/`, so the recolor stays short, and in
  zlib.h the opener landed inside a comment, so that row measures little.
- Memory is the instance's linear memory: for TS the text, tree and query after a
  full parse; for HL the text, tokens and line states of the live document.

Color agreement, on the characters that are not spaces, in coarse classes (comment,
string, keyword, function, type, constant, property and so on; variables and
punctuation count as plain). It measures how much the colors differ from what
tree-sitter painted, not which is right. About 9% of the Go differences are method
calls such as `p.init()`, which jamb's tree-sitter query painted as properties, because
its general `field_identifier` pattern came after the `function.method` one and the
later span won; highlights colors them as functions. "After name map" renames the
tokens that the two name differently for the same thing (JSON keys, CSS selectors).

| Document | Same class (raw names) | Same class (after name map) | Same class among colored chars (mapped) | Largest differences after map (TS → HL) |
|---|--:|--:|--:|---|
| go small (parser.go) | 80.6% | 80.6% | 72.0% | property → function 9.1% (e.g. `init`); type → property 4.1% (e.g. `File`); plain → type 2.6% (e.g. `Trace`); plain → function 2.0% (e.g. `len`) |
| go large (runtime/proc.go) | 89.5% | 89.5% | 87.1% | plain → function 7.2% (e.g. `main_main`); property → function 2.0% (e.g. `Itoa`); type → plain 0.6% (e.g. `m`); plain → type 0.3% (e.g. `Gosched`) |
| go huge (jamb ui/*.go) | 82.4% | 82.4% | 77.0% | property → function 6.4% (e.g. `access`); plain → function 6.3% (e.g. `panic`); plain → type 2.9% (e.g. `Text`); type → plain 0.8% (e.g. `accessInfo`) |
| ts small | 98.5% | 98.5% | 97.4% | comment → keyword 0.7% (e.g. `@`); function → property 0.2% (e.g. `getSources`); type → plain 0.2% (e.g. `string`); plain → function 0.1% (e.g. `captureMapping`) |
| ts large | 99.2% | 99.2% | 98.9% | property → type 0.2% (e.g. `None`); function → plain 0.2% (e.g. `parseTokenNode`); type → property 0.1% (e.g. `Identifier`); plain → function 0.1% (e.g. `parseOptionalToken`) |
| css small | 69.3% | 82.8% | 79.0% | plain → constant 10.0% (e.g. `-`); type → constant 5.0% (e.g. `16px`); plain → property 1.4% (e.g. `.`); plain → string 0.4% (e.g. `#`) |
| css large | 62.2% | 81.7% | 76.8% | property → plain 9.5% (e.g. `-`); plain → constant 4.7% (e.g. `system`); type → constant 2.0% (e.g. `180deg`); plain → property 1.6% (e.g. `.`) |
| html small | 94.4% | 94.4% | 92.0% | plain → string 5.4% (e.g. `"`); constant → tag 0.1% (e.g. `<`); plain → property 0.0% (e.g. `jQuery`); plain → function 0.0% (e.g. `write`) |
| html large | 93.6% | 93.6% | 89.7% | plain → string 5.1% (e.g. `"`); plain → property 0.9% (e.g. `text`); plain → constant 0.3% (e.g. `none`); plain → comment 0.1% (e.g. `/`) |
| json(c) small | 64.6% | 100.0% | 100.0% |  |
| json(c) large | 20.0% | 100.0% | 100.0% |  |
| python (typing.py) | 90.9% | 90.9% | 88.0% | function → type 3.3% (e.g. `ABCMeta`); property → function 1.5% (e.g. `join`); function → attribute 1.1% (e.g. `@`); plain → keyword 1.0% (e.g. `is`) |
| python (_pydecimal.py) | 93.0% | 93.0% | 91.1% | property → function 3.3% (e.g. `_fix_nan`); function → type 2.5% (e.g. `DecimalTuple`); plain → keyword 0.6% (e.g. `in`); function → constant 0.3% (e.g. `ROUND_05UP`) |
| rust (onepass.rs) | 96.7% | 96.7% | 96.3% | function → constant 0.8% (e.g. `DEAD`); function → type 0.7% (e.g. `Vec`); property → plain 0.6% (e.g. `match_kind`); plain → namespace 0.5% (e.g. `alloc`) |
| rust (dense.rs) | 96.7% | 96.7% | 96.2% | property → plain 0.8% (e.g. `accelerate`); function → type 0.6% (e.g. `BTreeMap`); attribute → plain 0.5% (e.g. `feature`); plain → namespace 0.5% (e.g. `core`) |
| c (zlib.h) | 93.8% | 93.8% | 93.5% | type → constant 1.8% (e.g. `FAR`); type → plain 1.4% (e.g. `alloc_func`); keyword → plain 0.9% (e.g. `#`); constant → plain 0.7% (e.g. `ZLIB_H`) |
| c (sqlite3.h) | 93.6% | 93.6% | 93.5% | constant → plain 2.2% (e.g. `SQLITE3_H`); type → plain 1.5% (e.g. `sqlite3`); comment → plain 0.8% (e.g. `/`); keyword → plain 0.8% (e.g. `#`) |

Binary size (`go build -trimpath`; stripped with `-ldflags "-s -w"`) of a program that
imports `codeedit`:

| Program | Default | Stripped |
|---|--:|--:|
| `codeedit` without grammars | 6.34 MB | 4.51 MB |
| with all 18 jamb-grammars modules | 26.49 MB | 24.64 MB |
| with highlights.wasm (76 languages) | 6.52 MB | 4.69 MB |

The compiled code in the cache directory took 0.67 to 1.79 MB for each grammar (18.2
MB for all 18), and 0.68 MB for highlights.

## Conclusion for the experiment

Written in October 2026 from the measurements of 2026-10-06 to 2026-10-14. Its
figures for startup, idle CPU, binary size, a signal change, frame costs and the
GC are superseded by Current results where that section says so.

The question was whether Go can give a GPUI-like experience with simpler
tooling and faster iteration without giving up runtime quality. On this
evidence, yes:

- Iteration is sub-second to build and about 1 s from save to a running window.
- There's no native toolchain: no cgo, no C compiler, no Rust, no Node.
- The repository is under 0.5 MB and the release binary was about 7 MB (4 MB before HarfBuzz shaping); after tier 3 the hello app's binary is 8.41 MB (measured 2026-10-14).
- The idle demo used no CPU over 10 s (the hello app in the GPUI comparison used 0.16–0.29% of a core, mostly the graphics driver's thread), and frame costs are 25–60× under a 120 Hz budget.
- Updates are incremental: a signal change in a 10,000-node tree costs about 0.6 ms.
- The GC is not a problem at this scale: sub-millisecond pauses, a few cycles per second only under a pathological full-rebuild loop.

The open risks are elsewhere, as expected:

- Text: there's no complex shaping, bidi, IME or color emoji yet.
- Platforms: macOS and Linux adapters still have to be written.
- Rich text editing.
