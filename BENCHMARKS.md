# Benchmarks (Milestone 5)

Measured 2026-10-06 on the demo app (`examples/demo/`) unless noted.

After a code review the runtime was made incremental: components keep their
realized nodes between frames, and layout keeps its cache. See
[Incremental updates and scaling](#incremental-updates-and-scaling) for
those numbers.

The windowed FPS and stress rows below were measured before that change.
Re-running them afterwards wasn't possible because the display had gone to
sleep: DWM then throttles vsync-paced presentation to about 4 Hz. With
vsync off the app ran at 6,471 FPS, which shows `Present` wasn't really
compositing. The CPU-side cost per frame is covered by the headless
benchmarks instead.

| | |
|---|---|
| Machine | Desktop PC: 6-core CPU, discrete GPU, 32 GB RAM, 165 Hz display at 100% scale |
| OS / toolchain | Windows 11, Go 1.27.1 windows/amd64, no cgo |
| Demo | 4 pages; 210 nodes on Projects, 227 on Activity (10,000 events in a `UniformList`) |

## Summary

| Metric | Result | Target |
|---|---|---|
| Incremental build (edit a demo or framework file) | **250–370 ms** for edits that don't change APIs, **~550–840 ms** for real edits (relink) | well under 1 s ✅ |
| No-op rebuild | 219 ms | |
| Cold build (empty GOCACHE, compiles the std lib too) | 7.7 s | |
| Save → new window on screen (`jamb dev`) | **1.05–1.36 s** | |
| Binary size | With `-trimpath -ldflags "-s -w"`: demo **16.0 MB**, hello 8.4 MB (2026-10-14; see [docs/comparison-gpui.md](docs/comparison-gpui.md#binary-size-updated)). Earlier: demo 9.9 MB, hello 7.2 MB (2026-10-12; see [Startup memory and binary size](#startup-memory-and-binary-size-2026-10-12)) | 5–30 MB ✅ |
| Repository size | 487 KB (≈9k lines of Go, including tests) | |
| Dependencies | `golang.org/x/image` only (plus pieces of `x/text` and `x/sys`); 59 MB in the module cache | |
| Build cache produced by a cold build | 51 MB (in the shared GOCACHE, not the repo) | |
| Startup (process start → first frame on screen) | **~195 ms**, of which ~180 ms is creating the Direct3D device (2026-10-12) | perceptually instant ✅ |
| Idle CPU | **0 ms of CPU in 10 s** (1 frame rendered in total) | ≈0% ✅ |
| Idle memory | Demo: 33 MB private working set (Task Manager's Memory column; 71 MB working set, 88 MB private bytes). Hello: 23 MB private working set, 77 MB private bytes (2026-10-12; see below) | reasonably small ✅ |
| Interactive FPS | holds 164 FPS (display refresh) in every mode tested | 60 / 120 FPS ✅ |
| Uncapped throughput (vsync off) | 319 FPS repaint-only, 285 FPS with a full re-render every frame | |
| CPU per frame, full re-render of every component | **~0.3 ms** avg (build + layout + paint + GPU submit) | budget 8.3 ms at 120 Hz |
| CPU per frame, repaint of retained tree | **~0.13 ms** avg | |
| One signal change, 10,000-node tree | **0.59 ms**: 1 component re-rendered, 5 nodes rebuilt, 7 re-laid out | |
| Allocation | 3–6 KB/frame repainting; 110 KB/frame (≈550 objects) for a full re-render | |
| GC | 0.2 cycles/s repainting; 3.5–5.7 cycles/s during continuous full re-render, 3–4 ms total pause over 4 s | |
| Virtualized list, 100,000 rows: one 120 px scroll step and frame | **169 µs**, 195 nodes built (a plain container of 10,000 rows: 13.3 ms, 50,002 nodes) | |

## How each number was produced

All of these hooks are in the framework, so they work for any jamb app.

### Build and footprint

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

Launch the demo, wait 3 s, then read `TotalProcessorTime` before and after
10 s (PowerShell `Get-Process`). With `JAMB_TRACE=1` the app also prints how
many frames it renders per second of activity. When idle it renders nothing:
the Win32 loop blocks in `GetMessage` and only wakes for input or a
signal change.

One run measured 10% CPU. That run had the mouse moving over the window,
which is real hover work, not idle overhead.

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
