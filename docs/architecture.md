# Architecture and internals

The repository layout, the architecture and frame pipeline, and GPU device loss.

## Repository layout

```
go.work             workspace: the framework (.), cmd/jamb, examples/demo, examples/hello and examples/notes
go.mod             module "github.com/tyler-builds/jamb", the framework
app/               runs an app: window + renderer + ui runtime, dev hooks
ui/                declarative API: Div, Text, Button, TextInput, TextArea, Icon, Image,
                   Style, Theme, Signal, Component, FocusHandle, Animate; runtime (build → layout → paint, events)
keymap/            key bindings from JSON files (Parse, Load, File); uses only ui's exported API
actions/           the text fields' actions (Undo, Copy, MoveWordLeft...), for key bindings and menus
ease/              easing functions for animation (Linear, OutCubic, Bounce...); imports nothing
dialog/            native dialog types (file open, save and folder; message box), shared by ui and platform
icons/             built-in line icons defined in Go (no SVG)
input/             platform-neutral keys, buttons, modifiers, cursors
internal/          packages apps don't import, so they can change without breaking apps:
  layout/          flexbox-subset layout engine (no drawing, no text)
  scene/           GPU-friendly primitives (Quad/shadow, Glyph, Image, Clip, Transform)
  render/          Renderer interface; render/d3d11 = Direct3D 11 backend
  platform/        adapter contract; platform/windows (Win32), darwin + linux placeholders
  text/            fonts and fallback, shaping and line breaking (go-text/typesetting),
                   caret/selection geometry, glyph atlas
internal/win32/    minimal cgo-free Win32 bindings
internal/watch/    directory change notifications for live keymap reload (Win32 change
                   notifications; polling elsewhere)
cmd/jamb/          `jamb dev` runner and `jamb package`; module "github.com/tyler-builds/jamb/cmd/jamb"
                   (its own module, so packaging dependencies stay out of the framework's go.mod)
examples/demo/     module "github.com/tyler-builds/jamb/examples/demo", consumes the framework like any app
  views/           sidebar, header, Dashboard, Projects, Activity, Settings
  assets/          embedded images
  jamb.json, icon.png   packaging settings and icon (see Packaging)
examples/hello/    module "github.com/tyler-builds/jamb/examples/hello", the smallest complete app: one window, a Signal and a button
examples/notes/    module "github.com/tyler-builds/jamb/examples/notes", a note-taking app (see below)
  store/           notes as signals, ordering, JSON persistence on a background goroutine
  views/           sidebar, note list, editor pane, actions and the command palette
docs/              screenshots/ (PNG captures), benchmarks/ (bench-before.txt, bench-after.txt)
scripts/bench.sh   reproduces the build and footprint numbers
scripts/a11y-check.ps1  live UI Automation checks of the examples (see Tools)
scripts/compare-gpui.ps1  reproduces the jamb and GPUI hello-world comparison (docs/comparison-gpui.md)
```

## Architecture

```
app code ──► ui (elements, signals, components)
               │ build: run dirty components, resolve styles/state
               ▼
             layout (flexbox subset, measure cache)
               │
               ▼
             scene (retained: quads, glyphs, images, clips)
               │
               ▼
             render.Renderer ◄── internal/render/d3d11 (only package that knows Direct3D)
platform.Window/Handler ◄── internal/platform/windows (only package that knows Win32)
```

Package rule: a concept gets its own package when it stands on its own; it
stays in `ui` when it needs the runtime's internals. Keymap files and easing
functions stand on their own, so they are `keymap` and `ease`. Animation
timing and paint transitions need the runtime, so they are in `ui`.

Each concern is replaceable independently:

- The ui, layout, scene and text packages don't import any platform or
  GPU code. The ui runtime therefore runs headless in tests. File watching
  for keymap reload is in `internal/watch`, which `app` uses, not `ui`.
- `ui` only talks to `scene`. A new GPU backend implements the seven-method
  `render.Renderer` interface.
- A new OS adapter implements `platform.Window` and drives a
  `platform.Handler` (see `internal/platform/darwin`). Application code never
  branches on the OS. An adapter runs one UI thread and one event loop for all
  of an app's windows.
- `app` is the only package that wires the concrete adapter and backend
  together, via build tags.

### Frame pipeline

1. **State.** `ui.Signal[T]` values live on the Go heap. Reading one inside
   a component's render function subscribes that component. `Set` marks the
   component dirty and requests a frame. Any number of changes before the
   next frame produce a single wake-up. Signals are safe from any goroutine;
   rendering always happens on the one UI thread.
2. **Build.** The realized node tree is retained between frames. Only dirty
   components re-run, outermost first. Each rebuilds just its own subtree,
   which is spliced into the tree in place. Hover, press and focus changes
   restyle only the elements whose state flipped and re-run nothing.
3. **Layout.** Changed nodes invalidate themselves and their ancestors. The
   flex engine keeps its measurement cache across frames and skips any
   subtree that is unchanged and the same size, so a change re-lays out its
   path to the root rather than the whole tree. A full layout costs about
   0.5 µs per node and allocates nothing.
4. **Paint.** This stage rewrites a retained `scene.Scene` whose slices are
   reused, so repaint frames don't allocate. Color transitions and smooth
   scrolling animate here without re-running components or layout, and so do
   opacity, offset and scale transitions and enter and exit motion. Paint
   culls: layout gives every node its *ink*, the rectangle its drawing can
   reach (box, shadows, offsets, scale, glyph overshoot), and paint skips a
   subtree whose ink misses the clip. Hit testing visits only the nodes the
   last paint drew, and off-screen elements keep their layout rectangle for
   `Find`. Click-outside listeners are registered when built, so an
   off-screen one still hears clicks; an element removed while off screen
   has nothing visible to fade, so it gets no exit ghost.
5. **Render.** Every batch is one instanced draw. Rounded rects, borders and
   Gaussian shadows are analytic SDFs in the pixel shader. Glyphs and icons
   come from one alpha atlas, and images use a second pipeline. The swap
   chain is flip-model with no scaling. While a window edge is dragged, the
   previous frame stays pinned 1:1 instead of being stretched, and frames for
   a new size are presented without waiting for vsync.
6. **Idle.** If nothing changed, the loop blocks in `GetMessage` and uses
   0% CPU. Animations keep it rendering, paced by vsync, but only those drawn
   on screen. A covered window stops rendering until DXGI reports it visible
   again (`RegisterOcclusionStatusWindow`). A shown toast wakes the window when
   its time runs out, and a caret wakes it at each blink.

## GPU device loss

When the GPU device is removed or reset (a driver update or crash, a remote
desktop switch, a hybrid-GPU switch, sleep and resume), jamb rebuilds it. The app
needs no code for this, and the ui state of every window is kept.

- A frame detects the loss from `Present`, `ResizeBuffers`, `Map`, resource
  creation, or the device's removal reason, and reports it as
  `render.ErrDeviceLost`.
- The first window to draw afterwards recreates the shared device (hardware
  first, then WARP, as at startup), its pipelines and states, and then each
  window's swap chain. The windows draw again. One loss causes one rebuild, however
  many windows are open.
- The recovery log line names the adapter the device runs on, `hardware` or
  `WARP software rasterizer`, so a fallback to WARP shows in the log.
- Textures are uploaded again from memory, and nothing is decoded or rasterized
  twice. Each image and glyph atlas keeps its CPU pixels for its whole life, so
  this needs no extra memory: width × height × 4 bytes per image, and each glyph
  atlas at its current size (up to 16 MB for grayscale and 64 MB for ClearType).
  The atlases are not reset, so glyphs stay cached.
- While the device is unavailable, a failed rebuild is retried after 250 ms, and
  the delay doubles up to 5 s. Each distinct failure is printed once per loss.
  Windows do not draw in between, and they stay responsive, so closing one still
  works.

Known limits:

- A window opened while the device is lost does not open; it fails with a message
  and can be opened again once the device is back.
- If the hardware device cannot be created, the WARP software fallback may succeed.
  The app then keeps drawing on WARP until it restarts.
- A device lost while every window is minimized is noticed when a window is
  restored.
- The rebuild has been checked with the simulation below, which releases the
  device itself. A real removal from the driver has not been checked automatically.

`JAMB_SCRIPT` step `devicelost` simulates a loss. It releases the device and its
objects, then handles the loss as if the next `Present` had reported
`DXGI_ERROR_DEVICE_REMOVED`. The next frame rebuilds the device and redraws every
window. It is the only way to exercise this path without a real driver reset, and
it does not reset the driver.
