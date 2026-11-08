# Architecture and internals

The repository layout, the architecture and frame pipeline, and GPU device loss.

## Repository layout

```
go.work             workspace: the framework (.), cmd/jamb and the four examples
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
  render/          Renderer interface; render/d3d11 = Direct3D 11 backend, render/vulkan = Vulkan
                   backend (Linux), render/soft = CPU renderer
  dmabuf/          the types a GPU renderer and the Wayland adapter share to hand frames over as dma-bufs
  platform/        adapter contract; platform/windows (Win32), platform/linux (Wayland, experimental), darwin placeholder
  text/            fonts and fallback, shaping and line breaking (go-text/typesetting),
                   caret/selection geometry, glyph atlas
internal/win32/    minimal cgo-free Win32 bindings
internal/watch/    directory change notifications for live keymap reload (Win32 change
                   notifications; polling elsewhere)
cmd/jamb/          `jamb dev` runner and `jamb package`; module "github.com/tyler-builds/jamb/cmd/jamb"
                   (its own module, so packaging dependencies stay out of the framework's go.mod)
examples/budget/   module "github.com/tyler-builds/jamb/examples/budget", a household budget (see docs/examples.md)
  ledger/          accounts, categories, transactions and monthly budgets; undo; JSON; CSV import
  views/           sidebar pages, title bar with the month, transaction table and form, budget, accounts, import
examples/demo/     module "github.com/tyler-builds/jamb/examples/demo", consumes the framework like any app
  views/           sidebar, header, Dashboard, Projects, Activity, Settings
  assets/          embedded images
  jamb.json, icon.png   packaging settings and icon (see Packaging)
examples/hello/    module "github.com/tyler-builds/jamb/examples/hello", the smallest complete app: one window, a Signal and a button
examples/notes/    module "github.com/tyler-builds/jamb/examples/notes", a note-taking app (see below)
  store/           notes as signals, ordering, JSON persistence on a background goroutine
  views/           sidebar, note list, editor pane, actions and the command palette
examples/git/      module "github.com/tyler-builds/jamb/examples/git", a Git history browser (see docs/examples.md)
  repo/            reads a repository with the git command: commits, refs, changes, diffs
  graph/           lays the commits out in lanes, one row at a time
  store/           repositories as signals, history loaded a page at a time off the UI thread
  views/           title bar, sidebar, history grid with the graph, details, diff, tags
examples/monitor/  module "github.com/tyler-builds/jamb/examples/monitor", a system monitor (see docs/examples.md)
  sample/          reads the processors, memory, network and processes; a made-up machine for tests
  store/           samples on a timer into signals: history, processes, alert rules and alerts
  views/           title bar, overview tiles, dock of chart panels, process table, details, alerts
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
                             internal/render/vulkan (Linux; only package that knows Vulkan)
                             internal/render/soft (CPU, pure Go: Linux without a GPU, and screenshots in tests)
platform.Window/Handler ◄── internal/platform/windows (only package that knows Win32)
                             internal/platform/linux (Wayland, experimental)
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
   chain is flip-model with no scaling. Its buffers have a quarter more room
   than the window during a drag, so a drag mostly draws a frame of the new size
   in their top-left corner, which the unscaled chain shows 1:1, instead of
   reallocating them; the room is given back when the user finishes the move or
   resize. The shown part is not set with `IDXGISwapChain2::SetSourceSize`,
   since Windows scales a source size that differs from the window's, which
   magnified the content. While a window
   edge is dragged, the previous frame stays pinned 1:1 instead of being
   stretched, and frames for a new size are presented without waiting for
   vsync.
6. **Idle.** If nothing changed, the loop blocks in `GetMessage` and uses
   0% CPU. Animations keep it rendering, paced by vsync, but only those drawn
   on screen. The pacing follows the desktop compositor, which runs at the
   primary monitor's rate, so a window on a monitor with a slower refresh
   renders some frames that monitor never shows. A covered window stops
   rendering until DXGI reports it visible again
   (`RegisterOcclusionStatusWindow`). A shown toast wakes the window when
   its time runs out, and a caret wakes it at each blink.

## Vulkan on Linux

The Linux adapter speaks the Wayland protocol in pure Go, without libwayland, so it
has no `wl_display` pointer to give Vulkan's window-system integration
(`VK_KHR_wayland_surface`). `internal/render/vulkan` therefore draws each frame into
an image of its own, exported as a dma-buf, and the adapter hands that dma-buf to
the compositor over its own connection with `zwp_linux_dmabuf_v1`. The two packages
share only the types of `internal/dmabuf`.

- **Loading.** `libvulkan.so.1` is loaded at run time with purego, as libxkbcommon
  is. The device needs Vulkan 1.1 and `VK_KHR_external_memory_fd`,
  `VK_EXT_external_memory_dma_buf` and `VK_EXT_image_drm_format_modifier`. Software
  rasterizers (lavapipe) are skipped, since the CPU renderer is as fast.
- **Choosing the GPU.** A discrete GPU is preferred to an integrated one. When the
  compositor names the GPU it composites with (`zwp_linux_dmabuf_v1` version 4, with
  `VK_EXT_physical_device_drm`), that GPU is used when it qualifies, so frames are
  not copied between GPUs.
- **Images.** A window's images are B8G8R8A8 (`DRM_FORMAT_ARGB8888`), premultiplied.
  On the compositor's GPU they are in the GPU's memory, with the format modifier the
  driver prefers among those the compositor imports. For a compositor on another GPU
  (a laptop that composites on the integrated GPU while jamb draws on the discrete
  one), frames are drawn in the GPU's memory and copied into linear images in system
  memory, which both GPUs reach, as PRIME render offload does. A window keeps two or
  three images, and a fourth when the compositor holds them all. A new size gets new
  images.
- **Probing.** A compositor that cannot import a buffer sent with `create_immed`
  ends the connection. So before a window's first frame, a small image of each kind
  (same GPU, then shared, or the other way round when the compositor is on another
  GPU) is sent with `create`, which the compositor answers with `created` or
  `failed`, and the first kind it imports is used. When it imports none, the window
  is drawn on the CPU.
- **Frames.** One command buffer records texture uploads, the render pass and the
  hand-off of the image to the compositor (`VK_QUEUE_FAMILY_FOREIGN_EXT`). The frame
  waits for the GPU before the image is attached, so the compositor never reads an
  unfinished frame; the wait is the GPU's time for the frame, well under a
  millisecond for a UI. Before drawing into an image the compositor released, the
  frame waits (up to 100 ms) until the dma-buf polls writable, which is when the
  compositor's own reads of it are done, so a frame on screen is never changed.
  Frames are paced by the compositor's frame callbacks, as on the CPU path.
- **Shaders.** The GLSL in `internal/render/vulkan/shaders` is a port of the
  Direct3D 11 renderer's HLSL, and the two must change together. The SPIR-V it
  compiles to is committed and embedded, so a build needs no shader compiler. The
  `Shaders` workflow compiles it on every pull request that changes a shader and
  commits the result to the pull request's branch; `go generate
  ./internal/render/vulkan` does the same where `glslc` is installed.
- **Fallback.** When Vulkan, a qualifying GPU or the compositor's dma-buf support is
  missing, the window is drawn on the CPU, and the reason is printed once.
  `JAMB_RENDERER=cpu` asks for the CPU renderer.

Not done yet: images with spare room during a live resize (each step of a drag
makes new images), and explicit synchronization (`linux-drm-syncobj-v1`) in place of
waiting for the GPU on the CPU. A driver that does not track the compositor's reads
on the dma-buf (implicit synchronization) leaves the wait above without effect;
explicit synchronization would cover it. The shared images for a compositor on
another GPU were checked on one GPU only: GNOME on NVIDIA rejects them, and the
window then draws in the GPU's memory.

## GPU device loss

When the GPU device is removed or reset (a driver update or crash, a remote
desktop switch, a hybrid-GPU switch, sleep and resume), jamb rebuilds it. The app
needs no code for this, and the ui state of every window is kept.

- A frame detects the loss from `Present`, `ResizeBuffers`, `Map`, resource
  creation, or the device's removal reason, and reports it as
  `render.ErrDeviceLost`. On Linux, Vulkan reports `VK_ERROR_DEVICE_LOST`, or a
  frame does not finish within 5 seconds.
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
