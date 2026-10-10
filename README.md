# jamb — a native GPU UI framework experiment in Go

jamb (formerly goui) is a small, declarative, GPU-rendered desktop UI framework written in
plain Go. There's no Electron, WebView, HTML/CSS, JavaScript, Node, Rust or
cgo. Windows is the first platform: pure-Go Win32 windowing plus a
Direct3D 11 renderer reached through COM vtables.

```go
func (v *ProjectsView) Render(ctx *ui.Context) ui.Element {
    t := ctx.Theme()
    return ui.Div().Row().Height(ui.Px(52)).Align(ui.Center).Justify(ui.SpaceBetween).PaddingX(16).
        Child(ui.Text("Projects").Size(22).Bold()).
        Child(ui.Button("New Project").Icon(icons.Plus).OnClick(v.createProject)).
        Background(t.Surface)
}
```

## Quick start

Windows 10+ and Go 1.27+. Nothing else.

```
go run ./examples/demo                         # run the demo
go run ./examples/notes                        # run the notes app
go run ./cmd/jamb dev ./examples/notes         # rebuild + restart on every save, window position preserved
go run ./cmd/jamb package --installer ./examples/demo   # Windows zip and setup.exe in examples/demo/dist (see Packaging)
go test ./... ./cmd/jamb/... ./examples/demo/... ./examples/notes/...   # headless tests (no window, no GPU)
go build -trimpath -ldflags "-s -w -H windowsgui" -o jamb-demo.exe ./examples/demo   # ~7 MB release build
```

Results against the performance targets are in [BENCHMARKS.md](BENCHMARKS.md):

- Builds take ~0.3–0.8 s after an edit.
- Startup to the first frame is ~200 ms.
- Idle CPU is zero.
- A full re-render costs ~0.3 ms of CPU per frame.

## Repository layout

```
go.work             workspace: the framework (.), cmd/jamb, examples/demo and examples/notes
go.mod             module "github.com/tyler-builds/jamb", the framework
app/               runs an app: window + renderer + ui runtime, dev hooks
ui/                declarative API: Div, Text, Button, TextInput, TextArea, Icon, Image,
                   Style, Theme, Signal, Component, FocusHandle, Animate; runtime (build → layout → paint, events)
keymap/            key bindings from JSON files (Parse, Load, File); uses only ui's exported API
ease/              easing functions for animation (Linear, OutCubic, Bounce...); imports nothing
dialog/            native dialog types (file open, save and folder; message box), shared by ui and platform
layout/            flexbox-subset layout engine (no drawing, no text)
scene/             GPU-friendly primitives (Quad/shadow, Glyph, Image, Clip, Transform)
render/            Renderer interface; render/d3d11 = Direct3D 11 backend
text/              fonts and fallback, shaping and line breaking (go-text/typesetting),
                   caret/selection geometry, glyph atlas
icons/             built-in line icons defined in Go (no SVG)
input/             platform-neutral keys, buttons, modifiers, cursors
platform/          adapter contract; platform/windows (Win32), darwin + linux placeholders
internal/win32/    minimal cgo-free Win32 bindings
internal/watch/    directory change notifications for live keymap reload (Win32 change
                   notifications; polling elsewhere)
cmd/jamb/          `jamb dev` runner and `jamb package`; module "github.com/tyler-builds/jamb/cmd/jamb"
                   (its own module, so packaging dependencies stay out of the framework's go.mod)
examples/demo/     module "github.com/tyler-builds/jamb/examples/demo", consumes the framework like any app
  views/           sidebar, header, Dashboard, Projects, Activity, Settings
  assets/          embedded images
  jamb.json, icon.png   packaging settings and icon (see Packaging)
examples/notes/    module "github.com/tyler-builds/jamb/examples/notes", a note-taking app (see below)
  store/           notes as signals, ordering, JSON persistence on a background goroutine
  views/           sidebar, note list, editor pane, actions and the command palette
docs/              screenshots/ (PNG captures), benchmarks/ (bench-before.txt, bench-after.txt)
scripts/bench.sh   reproduces the build and footprint numbers
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
             render.Renderer ◄── render/d3d11 (only package that knows Direct3D)
platform.Window/Handler ◄── platform/windows (only package that knows Win32)
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
  `platform.Handler` (see `platform/darwin`). Application code never
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
   0% CPU. Animations keep it rendering, paced by vsync.

## API guide

Everything is plain, typed Go. Chained methods configure elements.

```go
ui.Div()                       // flex container; children stack vertically by default
  .Row() / .Column()           // main axis
  .Wrap()                      // flow children onto more lines (chips, tags, galleries)
  .Gap(8).Padding(16).PaddingX(12).PaddingY(6).Margin(4).MarginTop(8)
  .Width(ui.Px(240)).Height(ui.Full).Size(w, h).MinWidth(...).MaxWidth(ui.Px(1100))
  .Grow(1).Shrink(0).Basis(ui.Px(0))          // Basis(0)+Grow(1) = equal columns
  .Align(ui.Center).Justify(ui.SpaceBetween).Center().AlignSelf(ui.End)
  .Absolute().Top(8).Right(8)  /  .Inset(0)   // positioned relative to the parent
  .ZIndex(1).ClipContent().ScrollY().ScrollX()  // overflowing containers get scrollbars:
                               // drag the thumb, click the track to page by 90%;
                               // horizontal bars too; presses never click content below
                               // with a Radius, content is clipped to the rounded corners
  .Background(c).Border(1, c).Radius(8).RadiusCorners(ui.Corners{...})
  .BorderEdges(ui.Edges{Bottom: 2}, c)     // a width per side (top, right, bottom, left); see Borders
  .BackgroundGradient(ui.LinearGradient(135, from, to))   // replaces Background; see Gradients
  .Shadow(ui.ShadowMd).Opacity(0.5).Cursor(ui.CursorPointer)
  // Shadow{Spread: 2, Color: c} (no blur, no offset) is a crisp ring around the box
  .TextColor(c).FontSize(13).FontWeight(ui.FontMedium)   // inherited by descendants
  .Hover(ui.Style{...}).Pressed(ui.Style{...}).Focused(ui.Style{...})
  .FocusVisible(ui.Style{...}).FocusWithin(ui.Style{...})  // focus rings; see Focus and the keyboard
  .TabIndex(2).TabStop(false)                            // place in the Tab order
  .Transition(120 * time.Millisecond)                    // ease colors, Opacity, Offset and Scale
  .Offset(0, -4).Scale(1.02)                             // paint-only: moved or scaled after layout
  .Enter(ui.Fade().Slide(0, -6), ui.Animation{Duration: 140 * time.Millisecond}) // on first appearance
  .Exit(ui.Fade(), ui.Animation{Duration: 90 * time.Millisecond})               // ghost while leaving
  .When(cond, func(d *ui.DivElement) { ... })            // conditional styling
  .OnClick(fn).OnDoubleClick(fn).OnMouseEnter(fn).OnMouseLeave(fn)
  .OnMouseDown(func(e *ui.MouseEvent){}).OnScroll(...).OnKeyDown(...).OnTextInput(...)
  .Overlay(ui.BelowStart).OverlayGap(4)    // float next to the parent: never clipped, drawn on top,
                               // hit first; flips to the other side if it doesn't fit
  .OnClickOutside(fn)          // any button pressed outside (an Overlay's parent counts as inside);
                               // on an Overlay, Escape also runs fn (see Actions and key bindings)
  .Tooltip("Save")             // label below after 500 ms of hover; Button has .Tooltip too
  .Key("stable-id").Child(el).Children(els...)

ui.Text("hi").Size(15).Semibold().Italic().Color(c).Truncate().MaxLines(2).Align(ui.Center)
ui.Textf("Count: %d", n)
ui.Icon(icons.Folder).Size(16).Color(c)          // color defaults to the inherited text color
                                                 // icons.All lists every icon; see the icon list below
ui.Image(src).Size(ui.Px(34), ui.Px(34)).Radius(17).Fit(ui.FitCover)
ui.Button("Save").Icon(icons.Check).Secondary().Small().OnClick(fn)   // Primary|Secondary|Ghost|Danger
ui.IconButton(icons.Bell)
ui.TextInput(signal).Placeholder("Search…").Icon(icons.Search).OnSubmit(fn).OnChange(fn)
ui.TextArea(signal).Placeholder("Start writing…").MinRows(10)    // multi-line editor
  // both: .Borderless() .AutoFocus() .FocusHandle(h) .Style(ui.Style{FontSize: 28})
  // TextInput also: .Small() (28 px, like Button.Small)
  // Style on a field sizes and decorates its frame (Width, Height, Padding,
  // Radius, Background, Border...); font fields style the text inside.
ui.Divider(), ui.Spacer(), ui.If(cond, el), ui.Map(items, fn)

// Focus from code (shortcuts, focusing a field you just created):
h := ui.NewFocusHandle()
ui.TextInput(query).FocusHandle(h)
h.Focus()    // takes effect on the next frame, after the change that created the field
saved := ctx.FocusedHandle()  // whatever has focus now (or nothing), to restore later
saved.FocusNow()              // focuses it at once; does nothing and returns false if its element is gone
ui.Div().FocusWithin(ui.Style{BorderColor: c})  // style while focus is inside
ui.Div().CapturePointer()                       // keep getting moves while dragging
```

The command palette restores focus this way. It takes `ctx.FocusedHandle()`
when it opens and calls `FocusNow()` when it closes, so a command it runs
acts on the field the user was typing in, not on the palette.

Text fields support selection with the mouse (drag, double-click a word,
triple-click a paragraph) and with Shift+arrows, plus Ctrl+arrow word
movement, clipboard (Ctrl+C/X/V), grouped undo/redo (Ctrl+Z/Y), Home/End,
PageUp/PageDown and a blinking caret. Text areas keep the caret visible
inside the nearest scroll container. Text is shaped with HarfBuzz (via
`go-text/typesetting`), so ligatures, right-to-left scripts, complex scripts
and font fallback for other writing systems all work. Input-method windows
open at the caret.

App code reads and writes the clipboard's plain text with `ctx.Clipboard()`,
which returns the text and whether there is any, and `ctx.SetClipboard(s)`.
Text fields use the same clipboard. Only plain text is supported for now;
images and HTML are future work.

### Gradients

```go
ui.Div().BackgroundGradient(ui.LinearGradient(135, from, to))  // replaces Background
ui.LinearGradient(90, from, to).Stops(0.2, 0.8)  // the colors sit at 20% and 80% of the gradient line
ui.LinearGradient(135, from, to).InOklab()       // interpolate in Oklab
```

- The angle is CSS's: 0 points up (the gradient runs from bottom to top), 90
  points right, 180 points down, 135 points to the bottom right. The gradient
  line passes through the center of the box, and its length is the box
  projected onto it, so the corners get the end colors.
- Colors are mixed in sRGB by default, as in GPUI. `InOklab()` mixes in Oklab,
  which stays bright between saturated colors: blue to yellow passes through
  green rather than gray.
- Opacity fades both colors. Borders and shadows stay solid. A gradient is also
  a `Style` field, so `Hover`, `Pressed` and `Focused` may set one, and it
  replaces `Background` when both are set.
- Gradient colors do not animate. `Transition` does not ease them, so a hover
  that changes the gradient snaps to the new one.

### Borders and rounded clipping

```go
ui.Div().Border(1, c).Radius(12)                          // one width on every side
ui.Div().BorderEdges(ui.Edges{Bottom: 2}, accent)         // an underline: a width per side
ui.Div().BorderEdges(ui.Edges{Left: 3}, accent).Radius(8) // a left accent bar; its corners curve into the sides
ui.Div().Radius(14).ScrollY()                             // children are clipped to the rounded shape
```

- `BorderEdges(e, c)` takes the width of each side (top, right, bottom, left);
  a zero side has no border. One color serves every side. `Border` and
  `BorderEdges` replace each other: the one set last wins, which is also how
  `Style.Merge` treats the `BorderWidth` and `BorderEdges` fields.
- A border side takes up space like padding, so the content moves in by its
  width, and each corner's inner edge curves from one side into the next. The
  inner radius of a corner is its radius less the average of its two sides'
  widths: exact for a uniform border, as in CSS, and an approximation for
  mixed widths.
- Border widths are layout, so they do not ease with `Transition`. The color
  does.
- A container with a `Radius` that clips (`ClipContent`, `ScrollY`, `ScrollX`)
  clips its children to the rounded shape inside its border. Edges are
  anti-aliased, and text, images and gradients are clipped the same way.
- Only the innermost rounded clip keeps its corners. Where a rounded container
  sits inside another rounded one, the inner corners are applied and the outer
  ones are not, so content can show past the outer corners near them. Avoid
  nesting rounded clips closely, or leave the outer container square.
- Hit testing and scrolling still use the rectangle, so a press in a corner
  that the rounding hides reaches the content there.

### Focus and the keyboard

There are three kinds of focus style:

- `Focused(s)` applies whenever the element has keyboard focus, including
  after a click (as GPUI's `.focus()` does).
- `FocusVisible(s)` applies only while focus is keyboard-driven: it arrived by
  Tab or a key press, and no click has happened since. Use it for focus rings.
  A click on a focused element removes the ring; the next key press brings it
  back.
- `FocusWithin(s)` applies while the element or a descendant has focus, for a
  field's frame.

```go
ui.Div().Focusable().FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}})
```

Custom elements ask `ctx.KeyboardFocus()` (true when this element has focus and
focus is keyboard-driven) and draw their own ring; the component re-renders
when that changes.

- **Built-in rings.** `Button` and `IconButton` draw a 2 px ring in
  `theme.FocusRing` when keyboard-focused. The ring is a `Shadow` with spread
  and no blur or offset, which is drawn as a hollow border outside the box, so
  it adds no layout shift and shows over a transparent button. Text fields and
  text areas keep their `FocusWithin` frame: they always show focus.
- **Enter and Space** run the focused element's `OnClick`, with no modifiers
  and not for an auto-repeat. Text fields keep those keys. A binding for Enter
  or Space runs instead (see the dispatch order under "Actions and key
  bindings").
- **Tab order.** Elements with a positive `TabIndex` come first, in ascending
  order, with ties in tree order. The rest (index 0) follow in tree order, as in
  HTML, and Shift+Tab walks the order backwards. An element inside an `Overlay`
  takes its place from its anchor. `TabIndex` makes an element focusable. A
  negative index, or `TabStop(false)`, keeps an element focusable by click or
  `FocusHandle` while Tab skips it.
- **Tab reveals.** Tab scrolls the element it moves to into view, in its nearest
  vertically scrolling ancestor.
- **Focus moves in OnBlur.** An `OnBlur` handler may move focus to another
  element. The move takes effect, and the element it left is not blurred a
  second time.

### Rich text

`ui.Rich` joins segments into one text element. It wraps, truncates and
aligns like `ui.Text`, and it is one node, so a row of mixed styles stays one
line when it is `Truncate()`d.

```go
ui.Rich(
    ui.Seg("Alex").Style(ui.Highlight{Weight: ui.FontSemibold}), // or .Bold()
    ui.Seg(" pushed to ").Color(t.TextMuted),
    ui.Link("Project Beta", openProject),   // accent color; underlined on hover; a click runs openProject
    ui.Seg(" main").Mono().Background(t.Surface),
).Truncate()

// Ranges of an existing text, by byte offset (use strings.Index on the same string):
ui.Text(body).Highlight(start, end, ui.Highlight{Background: t.AccentSoft, Color: t.Text})
// Every case-insensitive match of a search query (a blank query does nothing):
ui.Text(body).HighlightMatches(query, ui.Highlight{Background: t.AccentSoft, Color: t.Text})
```

- `Highlight` has `Color`, `Background`, `Weight`, `Italic`, `Underline`,
  `Strikethrough` and `Mono`. Zero fields keep the text's own style. Later
  highlights win field by field over earlier ones and over segments.
  Offsets outside the text, or inside a UTF-8 sequence, panic and name the
  offset and the text.
- `HighlightMatches(query, h)` applies `h` to every case-insensitive match of
  `query`, found by rune comparison so the offsets stay valid for text whose
  lowercase form differs in length (İ, the Kelvin sign). A blank query leaves
  the text as it is. On a Rich text it searches the joined text.
- `Segment.Hover(h)` applies `h` while the pointer is over the segment. Only
  the paint fields (`Color`, `Background`, `Underline`, `Strikethrough`) apply;
  weight, italic and mono in a hover are ignored, so hovering never re-lays
  out the text.
- `OnClick` (and `Link`) make a segment clickable: the pointer becomes a hand
  over it. A link inside a clickable row gets the click; the row's `OnClick`
  runs only for the rest of the row.
- `ui.Text(...).Italic()` and segment `Italic()` use the family's italic face.
  A family without an italic face for that weight uses its upright face: there
  is no synthetic slant. Segoe UI and Consolas have italics on Windows; the
  embedded Go fonts are the fallback elsewhere.
- Rich text is display-only. `TextArea` stays single-style.

### Drag and drop

Drag and drop is typed, as in GPUI: a drag carries a Go value, and a target
receives it only when the value's type matches the target's.

```go
// A source. The drag starts after the pointer moves 4 px with the button held;
// a press that does not move is still a click.
ui.Div().OnDrag(ui.Drag(note).
    Preview(func() ui.Element { return notePill(note) }). // drawn centered on the pointer; optional
    End(func(dropped bool) { ... })).                     // after a drop, a cancel, or a release over nothing
    Dragging(ui.Style{Opacity: 0.5})                      // the source's style while it is dragged

// A target for *store.Note. Each OnDrop takes one payload type; a target may
// have several, and the first on the deepest element that accepts wins.
ui.Div().OnDrop(ui.Drop(func(n *store.Note, e *ui.DropEvent) { ... }).
    Accept(func(n *store.Note) bool { return true }). // like GPUI's can_drop
    Over(ui.Style{BorderColor: accent}).              // merged while an accepted drag is over it
    Move(func(n *store.Note, e *ui.DropEvent) { ... }). // pointer moved over it: e.LocalX, e.LocalY
    Leave(func() { ... }))                            // the drag left it, or ended

// Files dropped from the desktop arrive as ui.Files (their paths).
ui.Div().OnDrop(ui.Drop(func(files ui.Files, e *ui.DropEvent) { ... }))
```

- Matching is by type assertion, so `ui.Drop[fmt.Stringer]` takes any payload
  that implements it, and a `ui.Drop[string]` never takes a `type label string`.
- `DropEvent` has logical pixels: `X`, `Y` in the window, `LocalX`,
  `LocalY` relative to the target, and the target's size `W`, `H`, so a row
  can insert before or after itself with `e.LocalY > e.H/2`.
- Escape, or the window losing focus, cancels a drag; `End` then runs with
  `false`. The source may leave the tree mid-drag; the drop still delivers its
  payload.
- While a drag runs, nothing is hovered or pressed, and the cursor shows
  not-allowed unless the pointer is over a target that accepts the payload.
- A `Preview` is not hit-tested: targets under it are found as if it were
  not there. It starts exactly over the source and follows the pointer.
- Within 32 px of the top or bottom edge of a scroll container (left or
  right for a horizontal one), a drag scrolls it. Nothing animates outside
  that zone, so an idle drag costs no CPU.
- A file drop calls only the `Drop[ui.Files]` target under the pointer. Its
  `Over`, `Move` and `Leave` never run: Windows does not report the pointer
  during a drag from outside the app.

### Animation

Paint-only properties ease with `.Transition`. When `Opacity`, `Offset` or
`Scale` (or a color) changes, it eases over the duration, without re-running
components or re-laying out. The first paint shows the current value at once.

```go
ui.Div().Transition(160 * time.Millisecond).Offset(0, -4).Scale(1.02).Opacity(0.9)
```

`ui.Animate` re-renders a function every frame with eased progress, so any
property can animate, layout included:

```go
ui.Animate("bar", ui.Animation{Duration: 600 * time.Millisecond, Easing: ease.OutCubic},
	func(t float32) ui.Element {
		return ui.Div().Height(ui.Percent(v * 100 * t)).Background(accent)
	})
```

- `ui.Animation{Duration, Easing, Repeat}`. `Easing` maps progress 0..1 to
  0..1; nil is ease-out cubic. `Repeat` loops forever (for `Animate`). An
  `Animate` restarts from 0 when its key changes, and stops asking for frames
  when it ends: after the end its function runs once more with t = 1.
- `github.com/tyler-builds/jamb/ease` has `Linear`, `In`, `Out`, `InOut` (quadratic), `InCubic`,
  `OutCubic`, `InOutCubic`, `Bounce(f)` (out and back) and `Reverse(f)`.
- `.Offset(x, y)` moves an element after layout: its place in the flow does
  not change, and hit testing and `Find` follow the element. `.Scale(s)` draws
  the element and its children about their center. Hit testing ignores scale.
- `.Enter(m, a)` plays when the element first appears: it starts with the look
  of the `Motion` `m` and settles into its normal look over `a`.
  `.Exit(m, a)` keeps the element drawn after it is removed while it moves to
  `m`. The helpers are `ui.Fade()`, `ui.Slide(x, y)` and `ui.Grow(s)`, which
  combine by chaining: `ui.Fade().Slide(0, -6).Grow(0.98)`.

```go
ui.Div().Key("palette").Overlay(ui.BelowCenter).
	Enter(ui.Fade().Slide(0, -6).Grow(0.98), ui.Animation{Duration: 140 * time.Millisecond}).
	Exit(ui.Fade().Grow(0.98), ui.Animation{Duration: 90 * time.Millisecond})
```

Cost: `Opacity`, `Offset`, `Scale`, transitions and enter and exit are paint-only,
so they cost no layout. A property that changes the layout (`Width`,
`Padding`, ...) costs a re-layout of the path to the root on each frame of its
animation. An exiting element leaves the layout at once, so its siblings move
straight away. It is drawn above its former siblings while it exits. It is a
ghost: it cannot be hit, focused or found, and it disappears when its exit ends.
Showing it again during its exit drops the ghost.

### Actions and key bindings

An action is a named command. Elements handle it with `OnAction`, key
bindings run it, and `ctx.Dispatch` runs it from code. Create actions once,
as package variables:

```go
var (
	NewNote = ui.NewAction("notes.new", "New note")
	Palette = ui.NewAction("notes.palette", "Command palette")
)

ui.Div().OnAction(NewNote, a.newNote)                // runs while focus is on or inside this element
ui.Div().KeyContext("Palette").Child(ui.TextInput(q)) // names a key context for BindIn
app.Window{Bindings: []ui.Binding{                   // or rt.AddBindings(...)
	ui.Bind("ctrl+n", NewNote),                      // global; strokes are "ctrl+shift+p", "alt+up", "f2"
	ui.Bind("ctrl+k ctrl+t", ToggleTheme),           // a chord: the first stroke waits up to a second
	ui.BindIn("Palette > Editor", "up", PaletteUp),  // only inside a Palette that holds a field
	ui.BindIn("Editor", "ctrl+a", ui.NoAction),      // turns Ctrl+A off in text fields
	ui.BindIn("Editor", "ctrl+z", ui.Unbind(ui.Undo)), // Ctrl+Z is no longer Undo in fields
}}
ctx.Dispatch(NewNote)                                // runs along the focus path; false if unhandled
ctx.KeysFor(NewNote)                                 // "Ctrl+N", for tooltips
ctx.AvailableActions()                               // actions handled on the focus path, deepest first
```

`action.Hide()` marks an action for menus and command palettes to leave out;
`action.Hidden()` tells them. Hidden actions still run and still appear in
`AvailableActions`.

**Turning keys off.** `ui.NoAction` disables keys. `BindIn("Editor", "ctrl+a",
ui.NoAction)` makes Ctrl+A do nothing in a text field: the key goes on to
`OnKeyDown` and text input, as if nothing had bound it. `ui.Unbind(a)` removes
`a`'s bindings for the same keys, and only those. `BindIn("Editor", "ctrl+z",
ui.Unbind(ui.Undo))` frees Ctrl+Z from Undo in fields, and a global binding for
Ctrl+Z still runs. A `NoAction` or `Unbind` affects only the bindings ranked
below it, so one added earlier, or one in a shallower context, does nothing.
`KeysFor` skips the bindings they turn off.

**Key contexts.** `KeyContext` takes whitespace-separated entries, each an
identifier (`Editor`) or an attribute (`mode=full`), as in GPUI. Identifiers
use letters, digits, `_` and `-`. A malformed spec panics, naming the problem:
`ui.KeyContext("Editor mode="): "mode=" has no value`.

**Expressions.** `ui.BindIn` takes a GPUI context expression. It is parsed when
the binding is made, and a malformed one panics. The operators, from loosest to
tightest, are `>`, `||`, `&&`, `==` and `!=`, and `!`; parentheses group.

| Syntax | Matches when | Example |
|---|---|---|
| `Editor` | some context on the focus path has the identifier | `BindIn("Editor", "ctrl+d", ui.DeleteForward)` |
| `mode == full`, `mode != full` | the attribute is, or is not, `full` (`!=` also matches when it is absent) | `BindIn("mode == full", "f2", rename)` |
| `!Terminal` | no context on the focus path has the identifier | `BindIn("!Terminal", "ctrl+s", save)` |
| `a && b`, `a \|\| b` | both, or either, match | `BindIn("Editor && mode == single_line", "enter", ui.Confirm)` |
| `a > b` | `b` matches inside a context that matches `a` | `BindIn("Palette > Editor", "up", PaletteUp)` |
| `(a \|\| b) > c` | grouping | `BindIn("Pane > (Editor \|\| Terminal)", "ctrl+w", close)` |

**Precedence.** A contextual binding's depth is the depth at which its
expression matches, counting contexts from the root; a global `Bind` has depth
0. Bindings run deepest first, and at equal depth the one added last runs
first. So a binding for the field's own context beats one for its parent, and
any contextual match beats a global binding.

This differs from GPUI on purpose. There, a global binding ranks at the
deepest level, so a later one can take a key that a field uses. In jamb a
global `Bind` ranks below every contextual binding, so a global shortcut never
takes a key that a focused field uses. To override a field's key, bind it in
the field's context, as `BindIn("Editor", ...)` does.

**Escape and overlays.** Escape closes the topmost open overlay that has an
`OnClickOutside`, one overlay per press, and otherwise hides a visible
tooltip. It runs after the bindings and the `OnKeyDown` handlers, so an app's
binding or handler for Escape takes it first; the notes palette binds Escape
to close itself and return focus to the note. While an overlay or tooltip
would take Escape, the field's built-in Cancel (which blurs a text field) does
not run, so one Escape closes an overlay even when its text field has focus.
With nothing open, Escape does what it did before. An overlay without
`OnClickOutside` is not closed by Escape. Holding Escape closes one overlay:
the auto-repeats of the press that closed it are ignored, so they neither close
the next overlay nor blur a field; press again to close the next one.

A menu takes Escape itself: the first press closes its open submenu, and the
next closes the menu (see [Menus](#menus)).

Key presses are handled in this order:

1. the rest of a chord in progress;
2. all bindings, contextual and global, in precedence order: the first whose
   action has a handler on the focus path runs;
3. `OnKeyDown` on the focused element, then its ancestors up to the root;
4. Enter or Space, without modifiers and not an auto-repeat, runs the focused
   element's `OnClick` (text fields are not clickable, so they keep these keys);
5. Tab, which also reveals the element it moves to, and the zoom keys.

A key that is both a binding and the start of a longer chord waits up to one
second for the chord's next stroke. If the next key completes a binding, that
binding runs. If the next key matches nothing, the waiting keys are replayed as
if the chord had never started: the longest run that matches a binding runs as
that binding, and any other key goes through steps 3 and 4 and types its text.
Then the new key is handled from step 1. Moving focus while a chord waits drops
it, without replaying it.

So a chord that begins with a printable key, such as `g g`, delays typing that
key in a field by up to a second. Prefer a modifier (`ctrl+k ctrl+t`) or a
context such as `!Editor`. A key press that a single-stroke binding consumes
types no text when it is printable (letters, digits, space, `=` and `-`). A key
that waits for a chord keeps its text, which is typed if the chord is replayed.

**Keymap files.** Key bindings can also come from a JSON file, so users can
change them without code. `app.Window` takes the file and the actions it may
name:

```go
app.New().Run(app.Window{
	Bindings:   views.Bindings(),
	KeymapFile: "keymap.json", // a missing file is fine; changes are applied while the app runs
	Actions:    views.Actions(), // the actions the file may name
	OnKeymapLoad: func(err error) { // optional: nil on success, else the problems
		// show err in the UI
	},
	// ...
})
```

The file is a JSON array of sections. A section has an optional `context`
(the same expression `BindIn` takes; none means global) and its `bindings`,
which map keys to actions:

```json
[
  {"bindings": {"ctrl+shift+n": "notes.new"}},
  {"context": "Editor", "bindings": {
    "ctrl+d": "editor.delete-forward",
    "ctrl+a": null,
    "ctrl+z": {"unbind": "editor.undo"}
  }}
]
```

A value is an action name, `null` for `NoAction`, or `{"unbind": name}` for
`Unbind`. Later sections and later keys win, as code order does. The file's
bindings come after `Bindings`, so they override the defaults. The text-field
actions (`ui.EditorActions()`) and `NoAction` can always be named.

`keymap.Parse(data, actions)` parses the bytes of a file, and
`keymap.Load(path, actions)` reads one. Both return the valid bindings and
an error that lists every problem, naming its section (counted from 1), its
context and its keys, for example `keymap section 2 (context "Editor"): "ctrl+d":
unknown action "editor.delet-forward"`. `app.Window` prints the problems to
stderr with a `jamb:` prefix and still applies the valid bindings, so one bad
entry costs only itself.

The file is watched, and saving it applies the change while the app runs. A
file that does not parse at all (for example, a save caught half-written)
keeps the previous bindings and reports the error through `OnKeymapLoad`. On
Windows the watch uses the OS's change notifications, so nothing is polled
and an idle app uses no CPU for it. `app.Window` does this for you;
apps that wire it themselves can use a `keymap.File{Path, Actions}` and call
its `Apply(rt)`, which reads the file and replaces its bindings as one layer.
`rt.SetBindings(layer, bindings)` sets other layers. `ui.ParseBinding` makes a
single binding from a keys string and an optional context, returning an error
rather than panicking.

**Text fields.** A `TextInput` has the context `Editor mode=single_line`, and a
`TextArea` has `Editor mode=full`. Their keys are actions, bound by
`ui.DefaultBindings()`, which `ui.NewRuntime` adds first, so a binding of the
app's at the same depth wins. The actions are `ui.MoveLeft`, `MoveRight`,
`MoveWordLeft`, `MoveWordRight`, `SelectLeft`, `SelectRight`, `SelectWordLeft`,
`SelectWordRight`, `MoveUp`, `MoveDown`, `SelectUp`, `SelectDown`, `PageUp`,
`PageDown`, `SelectPageUp`, `SelectPageDown`, `MoveLineStart`, `MoveLineEnd`,
`SelectLineStart`, `SelectLineEnd`, `MoveToStart`, `MoveToEnd`,
`SelectToStart`, `SelectToEnd`, `DeleteBack`, `DeleteForward`,
`DeleteWordBack`, `DeleteWordForward`, `Newline`, `Indent`, `Confirm`,
`SelectAll`, `Copy`, `Cut`, `Paste`, `Undo`, `Redo` and `Cancel`. They are
hidden, so a palette does not list them. Rebind them like any action:

```go
rt.AddBindings(ui.BindIn("Editor", "ctrl+d", ui.DeleteForward))       // a new key
rt.AddBindings(ui.BindIn("Editor", "ctrl+backspace", ui.DeleteBack))  // replaces the default word delete
```

Behavior changes from earlier versions: a key the defaults don't list (Ctrl+Up,
Alt+Left, Shift+Delete, Shift+Enter in a single-line field, Ctrl+Shift+Backspace)
is no longer handled by a field, so it bubbles on. Shift+Tab and Ctrl+Tab are
unbound in fields, so they move focus as before.

**Palettes.** A command palette's search field is an `Editor` inside a
`KeyContext("Palette")`. Its keys therefore belong to `"Palette > Editor"`:
`BindIn("Palette", "up", ...)` would lose to the field's own Up, because the
field's context is deeper, and the caret would move instead. This is GPUI's
rule; Zed writes `Picker > Editor` for the same reason.

Reusable styles are ordinary values. Zero-valued fields mean "unset":

```go
var Card = ui.Style{Padding: ui.All(16), Radius: 12, Background: ui.Hex("#18181b")}
ui.Div().Style(Card)
```

State and re-render boundaries:

```go
count := ui.NewSignal(0)
ui.Component(func(ctx *ui.Context) ui.Element {      // re-runs only when count changes
    return ui.Button(fmt.Sprintf("Count: %d", count.Get())).
        OnClick(func() { count.Set(count.Peek() + 1) })
})

// State that belongs to one component (a draft, an open/closed flag):
ui.Component(func(ctx *ui.Context) ui.Element {
    open := ui.Local(ctx, "open", false)            // *ui.Signal[bool], lives with the component
    ...
})
```

`ui.Local` finds state by name, not by call order, so it is safe inside
conditions and loops. It lives as long as the component and starts fresh if
the component is removed and added again.

Long lists build only the rows on screen. Every row must share one height;
the list measures the first row and positions the rest from it:

```go
h := ui.NewListHandle()
ui.UniformList(len(events), func(i int) ui.Element { return eventRow(events[i]) }).
    Style(ui.Style{Grow: 1}).Handle(h)  // Style sizes the scroll container
h.ScrollToItem(5000)                    // scrolls the least amount that shows row 5000
```

Treat a signal's value as immutable. To change a slice or map, build a new
one and `Set` it. Changing it in place notifies nobody.

### Menus

jamb draws its own menus, so they match the theme, work with a custom title
bar, and need no platform code. A menu is a list of `ui.MenuItem`s:

```go
ui.MenuItem{Label: "Open in window", Icon: icons.ArrowUpRight, OnClick: openWindow}
ui.MenuItem{Action: NewNote}                       // label, shortcut and run come from the action
ui.MenuItem{Label: "Star", Check: ui.MenuCheckBox, Checked: starred, OnClick: toggleStar}
ui.MenuItem{Label: "Paused", Check: ui.MenuCheckRadio, Checked: status == paused, OnClick: setPaused}
ui.MenuItem{Label: "Status", Submenu: []ui.MenuItem{...}} // opens to the right
ui.MenuItem{Label: "Archive", Disabled: true}      // shown, but cannot be chosen
ui.MenuSeparator()
ui.MenuSection("Status")                           // a group label; runs nothing
```

Three elements open a menu. Each takes a function that builds the items when
the menu opens:

```go
// Right-click, or Shift+F10 or the Menu key while the element has focus.
ui.Div().Focusable().ContextMenu(func() []ui.MenuItem { return rowMenu(p) })

// A click, or Enter, Space or Down while the trigger has focus, opens a menu below it.
ui.DropdownMenu(ui.Button("Actions").Secondary(), func() []ui.MenuItem { return actions() })

// A row of top-level menus. Pressing and releasing Alt by itself focuses the bar.
ui.MenuBar(
	ui.TopMenu{Label: "File", Items: fileMenu},
	ui.TopMenu{Label: "Edit", Items: editMenu},
)
```

![A context menu with a submenu open](docs/screenshots/menu-context.png)

- **Actions.** An item with an `Action` shows the action's binding through
  `KeysFor`, so a keymap change shows at once. Choosing it closes the menu, returns
  focus to where it was, and then runs the action through `Dispatch`, so the
  handler on that focus path runs. An item with `OnClick` runs the function the
  same way. If an item has both, the action runs.
- **Keyboard in a menu.** Up and Down wrap and skip separators and disabled
  items. Home and End go to the first and last items. Right opens a submenu and
  Left closes it. Enter and Space choose. Escape closes one level. Tab closes the
  menu and moves focus on. A letter or digit jumps to the next item whose label
  starts with it.
- **Submenus and the pointer.** A submenu opens after the pointer rests on its
  item for 150 ms, and it stays open for the same delay after the pointer leaves
  for a neighboring item. That lets the pointer move diagonally into the submenu.
  This is a timed delay, not a triangle.
- **Placement.** A menu is an `Overlay` below its anchor. It flips above and
  shifts to stay inside the window, as other overlays do. A submenu opens to the
  right of its row and flips to the left when there is no room.
- **Focus.** A menu takes keyboard focus when it opens. Closing it (Escape,
  choosing an item, or a press outside) returns focus to the element that had it
  before the press, or to the focused element for a keyboard open. A press
  outside closes the whole chain. A press on the trigger toggles a dropdown.
- **Menu bar.** A click opens a top-level menu, and while one is open, the pointer
  over another label opens that one. Left and Right move between the top-level
  menus, also from inside an open menu. Down, Enter or Space open the highlighted
  menu from the focused bar. Escape or Tab leaves the bar. The labels are not in
  the Tab order. A window without a menu bar keeps Windows' own Alt behavior,
  which opens the system menu; with one, a lone Alt press is taken by the bar.
- **Look.** Menus use the theme's surface, border, shadow and highlight colors.
  They enter and leave with the same motion as the command palette.
- **Roles.** `item.Role()` returns the WAI-ARIA name of an entry (`RoleMenuItem`,
  `RoleMenuItemCheckbox`, `RoleMenuItemRadio`, `RoleSeparator`, `RoleGroupLabel`),
  so that UI Automation can expose menus later. Nothing reads it yet.
- **Not implemented.** Alt+letter mnemonics. The labels take no `&` marks, and
  Alt followed by another key does not open a menu.

The behavior is covered by tests in `ui/menu_test.go`, and by the demo and notes
packages. Those tests drive the framework's input methods directly. The
Windows keyboard path (the Menu key, and an Alt press and release that the
system would otherwise turn into its own menu) is not covered by them, so
check it by hand.

### Variable-height lists

When rows differ in height, use `ui.ListOf`. It takes the items, a key for each
item and a function that builds a row:

```go
h := ui.NewListHandle()
ui.ListOf(notes, func(n *store.Note) string { return n.ID }, func(n *store.Note) ui.Element {
    return noteRow(n)
}).Style(ui.Style{Grow: 1}).Handle(h)
h.ScrollToItem(i)  // the least scroll that shows item i completely
h.ScrollToEnd()    // the last item
h.AtEnd()          // within a pixel of the end; false before the first paint
```

`AtEnd` reads the scroll position without tracking it, so a component that
shows a control only while the list is away from its end should read the state
through `OnAtEnd` instead. It is called on the first paint and then on each
change, from the paint that makes the change:

```go
atEnd := ui.Local(ctx, "atEnd", false)
ui.ListOf(...).Bottom().Handle(h).OnAtEnd(atEnd.Set) // then: if !atEnd.Get() { show "Jump to latest" }
```

Only the rows near the viewport are built. A row's height is measured once it
is built and remembered by its key while the item stays in the list. A row not yet seen is estimated from the
mean of the measured heights (32 px before any is measured). The scroll position
stays on the item at the top of the viewport, so measuring rows above it, or
inserting and removing items above it, does not make the view jump. Items are
compared by slice identity: a new slice means the items changed, and a slice
changed in place is not detected. Keys must be unique among the items; a
duplicate gets a warning once and falls back to its position.

`Bottom()` is for chat-like lists. Rows shorter than the viewport sit at its
bottom, and a list that is at its end stays there when items are appended or a
row grows.

`ScrollToItem` eases toward its item when the item is within a viewport of the
current position. Further away, the list jumps there at once and builds only
the rows at the destination.

Prefer `UniformList` when every row has the same height. Its scroll step is
cheaper and its scrollbar thumb is exact from the start.

Identity: every element's state (hover, focus, scroll position, the text
input's caret, a component's subtree) is tied to its position in the tree.
That position is the element's key if it has one, otherwise its index among
its siblings, combined with its element type.

- Give items in dynamic lists a `.Key(...)` derived from a stable ID. Then
  reordering, inserting or removing items keeps each item's state with that
  item.
- Without keys, state stays at the position. Inserting at the front shifts
  state to the next item.
- Replacing an element with one of a different type always starts fresh.
- Duplicate keys among siblings print a warning and are kept apart.

`ui/incremental_test.go` pins all of this down.

A dropdown is an Overlay child of its anchor:

```go
ui.Component(func(ctx *ui.Context) ui.Element {
    open := ui.Local(ctx, "open", false)
    t := ctx.Theme()
    btn := ui.Div().Padding(8).OnClick(func() { open.Set(!open.Peek()) }).Child(ui.Text("Menu"))
    if open.Get() {
        btn.Child(ui.Div().Overlay(ui.BelowStart).Padding(4).Background(t.Surface).
            OnClickOutside(func() { open.Set(false) }).
            Child(ui.Text("Item")))
    }
    return btn
})
```

Some event semantics to know:

- `OnClick` fires on the deepest clicked element only. Clicks inside an
  overlay never reach its anchor.
- Events on an overlay bubble through its anchor and the anchor's ancestors.
- Other mouse and key events bubble until `StopPropagation`.
- Key presses go to the bindings (deepest context first, global last), then
  to the focused element and its ancestors (`OnKeyDown`), then to Tab and the
  zoom keys. With nothing focused, the root comes first. See the dispatch order
  under "Actions and key bindings".
- Tab cycles focus in tab order, and Enter or Space clicks the focused
  element (see "Focus and the keyboard").

Theming: `ctx.Theme()` returns semantic colors, and `ctx.SetTheme(ui.LightTheme)`
re-renders everything. `ui.DarkTheme` and `ui.LightTheme` are built in.

Zoom: Ctrl+= (or Ctrl++, numpad +) zooms in, Ctrl+- zooms out, Ctrl+0
resets, and Ctrl+mouse wheel zooms. Levels run 50% to 300% (`ui.ZoomLevels`).
In code: `rt.Zoom()`, `rt.SetZoom(z)`, `rt.ZoomIn()`, `rt.ZoomOut()`,
`rt.ResetZoom()`, `rt.SetZoomShortcuts(false)` to turn the keys off, and
`rt.OnZoomChange(fn)`; components use `ctx.Zoom()` and `ctx.SetZoom(z)`.
`rt.Post(fn)` runs fn on the UI thread at the next frame; call it from
background goroutines to hand them their results. A component that holds its
`ui.Context` can call `ctx.Post(fn)` for the same thing.
`app.Window{Zoom: 1.25, OnZoom: func(z float32) {...}}` sets the initial
level and observes changes. Zoom multiplies the device scale, so text is
shaped and rasterized at the zoomed pixel size and stays sharp at every
level. Layout sees a smaller logical window (window size / zoom).

Errors explain what failed and how to fix it. For example, `ui.Hex("#12")`
panics with `ui.Hex("#12"): expected #rgb, #rrggbb or #rrggbbaa`, and a
missing Root says to set `app.Window.Root`.

### Windows

`app.Run` opens the first window and runs until the app quits. More windows
open with `app.Open`, from event handlers or from goroutines (the window is
created on the UI thread):

```go
app.New().Run(app.Window{
    Title: "Projects", Width: 1200, Height: 760,
    QuitWhenClosed: true, // quit when this window closes, even with others open
    Root:           root,
})

// Later, from a click handler or a goroutine:
h := app.Open(app.Window{
    Title: "Project Beta", Width: 640, Height: 420,
    OnClose:  func() bool { return saved() }, // false keeps the window open
    OnClosed: func() { ... },                 // after the window has closed
    Root:     detailRoot,
})
h.Focus()          // bring it to the front
h.SetTitle("...")  // also Close(); h.Closed() reports whether it is gone
```

- The app quits when its last window closes. A `QuitWhenClosed` window quits
  the app when it closes, even with others open. Before it closes, it asks the
  `OnClose` of every other open window, and then its own. If any returns
  false, the whole close is cancelled and every window stays open. Otherwise
  the other windows close too, each without being asked again, and their
  `OnClosed` runs.
- `X` and `Y` place a window in logical screen pixels. Left at zero, the
  first window is centered and a later one is cascaded 32 px from the last.
- Each window has its own runtime, so its own theme, zoom, focus, key
  bindings and keymap file. The windows share the UI thread, the text system
  and the GPU device. A second window creates a swap chain, not a device.
- Signals are shared across windows. A component that reads a signal
  re-renders in every window that shows it:

```go
// The projects page and the project window both read s.Projects.
ui.Component(func(ctx *ui.Context) ui.Element {
    p, _ := s.Project(id) // reads the signal
    return ui.Button("Star").OnClick(func() { s.ToggleStar(id) })
})
```

Inside a component, `ctx.Window()` controls its window: `Minimize()`,
`ToggleMaximize()`, `Close()`, `SetTitle(s)`, and the reactive reads
`Maximized()` and `Focused()`, which re-render the component when they change.
A headless runtime (in tests) has no window, so the actions do nothing.

### Custom title bars

`app.Window{Chrome: app.ChromeCustom}` removes the operating system's title
bar, and the app draws its own. The window keeps everything native: resize
borders and cursors, Aero Snap, Win+arrows, the drop shadow, the minimize and
maximize animations, the rounded corners of Windows 11, and the maximized
geometry (content stops at the work area, above the taskbar).

Mark the title bar's areas with `WindowArea`:

```go
ui.Div().Row().Height(ui.Px(32)).WindowArea(ui.AreaDrag).Justify(ui.End).
    Child(search).             // interactive children keep working
    Child(ui.TitleBarButtons()) // minimize, maximize or restore, and close
```

- `ui.AreaDrag` moves the window when dragged, and double-clicking maximizes
  it. A point in a drag area is a drag unless an element on top of it takes
  input (`OnClick`, `OnMouseDown`, focus or `OnDrag`), so a search field or
  a button inside the title bar still works. `ui.AreaMinimize`,
  `ui.AreaMaximize` and `ui.AreaClose` act when the mouse is released over
  them.
- `ui.TitleBarButtons()` is the usual set: 46×32 buttons like Windows 11's,
  with hover and pressed styles, a red close button and the restore icon
  while the window is maximized. The buttons are native hit-test areas, which is what
  lets Windows 11 show Snap Layouts over the maximize button.
- Windows with the operating system's title bar ignore areas.

### Native dialogs

`ctx.Window()` opens the system's own dialogs, each owned by the window: a file
open dialog, a file save dialog, a folder picker and a message box. A dialog
returns at once. Its result goes to a callback on the UI thread when the dialog
closes, so a handler does not wait inside the dialog's message loop:

```go
ctx.Window().OpenFiles(dialog.FileOptions{
	Title:    "Import notes",
	Filters:  []dialog.Filter{{Name: "Notes", Patterns: []string{"*.md", "*.txt"}}},
	Multiple: true, // several files; Folder and Name also apply (see the dialog package)
}, func(r dialog.FileResult) {
	switch {
	case r.Err != nil: // the dialog could not open
	case r.Canceled:   // the user closed it without choosing: a result, not an error
	default:           // r.Paths, as the dialog returns them
	}
})

ctx.Window().SaveFile(dialog.FileOptions{Name: "Untitled.md", Extension: "md", Filters: filters}, done)
ctx.Window().PickFolder(dialog.FileOptions{Title: "Choose a folder"}, done)
ctx.Window().MessageBox(dialog.MessageOptions{
	Title: "Delete note", Text: "Delete this note?", Icon: dialog.IconQuestion,
	Buttons: dialog.ButtonsYesNo, Default: dialog.No, // Enter chooses No
}, func(b dialog.Button) { /* dialog.Yes, dialog.No, dialog.OK or dialog.Cancel */ })
```

- Icons are `IconInfo`, `IconWarning`, `IconError` and `IconQuestion`. Button
  sets are `ButtonsOK`, `ButtonsOKCancel`, `ButtonsYesNo` and
  `ButtonsYesNoCancel`. `Default` is the button Enter chooses; zero means the
  first button of the set.
- A callback is not run on a headless runtime, which has no window.
- The owner window is disabled while its dialog is open, as Windows does for
  any modal dialog, so it takes no input. Its frames, and those of the app's
  other windows, keep drawing: a timer on the owner renders them while the
  dialog's own message loop runs.
- The callbacks are asynchronous on purpose. A blocking call would run the
  dialog's loop inside the event that opened it, while the runtime is still
  handling that event. The callback shape also matches a macOS sheet, which
  reports its result the same way.
- Calls go from `ui` through the app to `platform.Window`. `ui` does not import
  `platform`.
- File dialogs use `IFileOpenDialog` and `IFileSaveDialog`, called through
  their vtables from pure Go. Message boxes use `MessageBoxW`: `TaskDialog`
  shows richer boxes, but it needs a common-controls version 6 manifest, which
  `jamb package` adds to a packaged app. A `go run` build does not have that
  manifest. Message boxes can move to `TaskDialog`, with `MessageBoxW` as the
  fallback, without changing the API.
- The notes and demo apps use these dialogs. The Components page of the demo
  shows each one.

### Modal dialogs

`ui.Modal` is a dialog in the app's own look: a dimmed backdrop, and a panel
centered on it with a title, a body and footer buttons. Build it only while it
is open, and place it last in the window's root element, which must fill the
window:

```go
ui.If(open.Get(), ui.Modal("New project", form).
	Footer(cancel, create).
	Default(create).             // Enter runs it; it takes focus if the body has no field
	InitialFocus(h).             // optional: the element that takes focus when it opens
	CloseOnBackdrop().           // optional: a press on the backdrop closes it
	OnClose(closeFn))            // Escape, or a backdrop press when CloseOnBackdrop is set
```

![A modal form over the Projects page](docs/screenshots/modal-new-project.png)

- Focus moves into the modal when it opens: to the first field of the body that
  Tab reaches, else to the default button. Tab and Shift+Tab cycle through the
  modal only. When the modal closes, focus returns to the element that had it,
  unless focus has moved elsewhere.
- Escape closes the modal through the overlay rules (see
  [Escape and overlays](#actions-and-key-bindings)). Enter presses a focused
  button as usual, and otherwise runs the default button. A multi-line field
  takes Enter as a newline. A single-line field with no `OnSubmit` passes Enter
  on to the default button.
- Its placement is `ui.OverCenter`, a new overlay placement that centers the
  panel over its anchor. Colors come from `Theme.Scrim` and the surface colors,
  and the modal enters and leaves with the same motion as the command palette.
- While a modal is open, keys stop at it: the app's bindings, `OnKeyDown`
  handlers and typed text outside the modal don't reach anything behind it, and
  Alt doesn't activate a menu bar. Keys handled inside the modal (a field's
  editing keys, the modal's own bindings) work as usual.

### GPU device loss

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

## Tools for humans and coding agents

The `jamb` command in `cmd/jamb` has two commands: `jamb dev`, the watch-and-restart
runner, and `jamb package`, which builds the Windows release (see [Packaging](#packaging)).

These environment variables work with any jamb app, with no code changes:

| Variable | Effect |
|---|---|
| `JAMB_TRACE=1` | Prints startup timings, per-frame build/layout/paint cost, and frames per second of activity, so you can confirm the app is idle. |
| `JAMB_SCRIPT="idle; shot a.png; click 600 400; type hi; key enter; idle; shot b.png; exit"` | Drives the app with synthetic input and saves screenshots. This is how the demo was verified. Besides `click`, the steps include `rclick X Y` (the right button, which opens a context menu), `down X Y`, `move X Y` and `up X Y` for drags, and `drop X Y PATH...` for files dropped from the desktop. The `drop` step goes through the same handler as a real drop, but the Windows message handling itself is only exercised by a real drop. `window N` makes the following steps, and the `shot`s, target the Nth window opened (1-based); the first window is the default. `type TEXT` types everything after the one space that follows `type`, spaces included. `devicelost` simulates a GPU device loss; see [GPU device loss](#gpu-device-loss). |
| `JAMB_STRESS=rebuild\|paint` | Measures sustained frame cost, allocation and GC, prints the results, then exits. |
| `JAMB_VSYNC=0` | Turns off vsync, for throughput tests. |
| `JAMB_D3D_DEBUG=1` | Enables the Direct3D debug layer. |
| `JAMB_TEXT_AA=gray` | Forces grayscale text anti-aliasing instead of ClearType (Windows). |

`JAMB_SCRIPT` keys are `enter`, `escape` (or `esc`), `tab`, `backspace`,
`delete`, `insert`, `space`, the arrows, `home`, `end`, `pageup`, `pagedown`,
`menu` (the Apps key), `f1` to `f12`, `a` to `z`, `0` to `9`, `=` and `-`, with the modifiers `ctrl`,
`shift`, `alt` and `super`. An unknown key or modifier stops the script with an
error when the script is loaded. `idle` waits for `rt.Settled()`, which ignores
repeating animations, so a screen with a spinner does not hold the script.

The headless tools are:

- `ui.NewRuntime(root, theme, nil)` plus `rt.Frame(time.Now())` runs the
  whole pipeline without a window.
- Scripted actions, each followed by a frame:
  - `rt.ClickKey("save")` clicks the element keyed `save`.
  - `rt.Type("text")` types into the focused element.
  - `rt.Press(input.KeyN, input.ModCtrl)` presses a key.
- `rt.Find(key)` returns an element's rectangle. A component's or text
  field's key belongs to what it renders.
- `rt.Dump(os.Stdout, depth)` prints the laid-out tree, with keys and text
  field contents. It found layout bugs during development.
- `rt.ContentWidth()` checks that nothing overflows at the minimum window size.
- `rt.Settled()` reports whether nothing changed since the last frame and
  nothing animates toward an end. Repeating animations never end, so they do
  not count; `rt.NeedsFrame()` still reports them.
- `go run ./internal/iconsheet icons.png` renders every icon.

See `ui/runtime_test.go`, `ui/editor_test.go`,
`examples/demo/views/dump_test.go` and `examples/notes/views/views_test.go` (the notes app
driven headlessly: Ctrl+N, typing, search, saving).

## Packaging

`jamb package` builds a Windows release of an app from its `jamb.json` (below).
It always writes a zip. Flags add an installer and an MSIX.

```
go run ./cmd/jamb package ./examples/demo                 # zip
go run ./cmd/jamb package --installer ./examples/demo     # zip and setup.exe (needs NSIS)
go run ./cmd/jamb package --msix ./examples/demo          # zip and an unsigned MSIX (needs the Windows SDK)
```

Outputs go to `<app>/dist/`, which git ignores. Each run builds its files in a
temporary folder inside `dist` and moves them there only after every step has
succeeded, so a failed signature leaves no partly signed set.

- `<name>-<version>-windows-<arch>.zip`: the executable at the root, and any
  `LICENSE*` and `NOTICE*` files of the app directory beside it. Always built.
- `<name>-<version>-windows-<arch>-setup.exe`: an installer, built with
  `--installer`. See [Installer](#installer).
- `<name>-<version>-windows-<arch>.msix`: an MSIX package, built with `--msix`.
  It is unsigned unless a sign command signs it. See [Signing](#signing).

`<arch>` is `amd64` or `arm64`.

### jamb.json

Every field is required except `copyright`. An unknown field is an error, so a
misspelled name stops the build, and every problem is reported in one message.

| Field | Meaning |
|---|---|
| `name` | The executable's name without `.exe`: starts with a letter, then letters, digits, `-` and `_`, at most 64 characters. |
| `displayName` | The name users see: Start, the taskbar, the installer and the package. It also names the install folder, so it cannot contain `\ / : * ? " < > \|`. |
| `description` | A short description, shown in file properties, the installer and the package. |
| `version` | A semantic version, `MAJOR.MINOR.PATCH`. Each part is 0 to 65535, and Windows stores it as `MAJOR.MINOR.PATCH.0`. Pre-release and build suffixes have no Windows form and are rejected. |
| `publisher` | The publisher's display name, shown as the company in file properties, the installer and the package. |
| `publisherSubject` | The subject of the signing certificate, starting with `CN=`, such as `CN=Example`. The MSIX publisher must equal the certificate's subject exactly, so for a certificate from a certificate authority copy the whole subject. For the Store, use the publisher value that Partner Center shows for the app. |
| `identityName` | The package identity: 3 to 50 characters of letters, digits, `.` and `-`, starting and ending with a letter or digit. Windows uses it to identify the app, and the installer uses it as its registry key, so it must stay the same across versions that should upgrade one another. For the Store, use the identity name that Partner Center shows for the app. |
| `copyright` | Optional. Shown as the legal copyright in file properties and the installer. |
| `icon` | A square PNG, at least 48 px, relative to the app directory. 256 px or larger is recommended, and packaging warns below that. It becomes the executable's icon, the installer's icon and the MSIX logos. |

File associations and extra capabilities are not supported yet. The manifest
requests only `runFullTrust`.

### Commands and flags

`jamb package [flags] [package dir]`. The directory defaults to `.`, and flags may
come before or after it.

| Flag | Effect |
|---|---|
| `--arch amd64\|arm64` | The target architecture. Default `amd64`. |
| `--installer` | Build the `setup.exe` installer with NSIS. |
| `--makensis PATH` | The `makensis` program to use. Needs `--installer`. Default: found on PATH or in the NSIS install directory. |
| `--msix` | Build the MSIX package. It is unsigned unless `--sign-command` signs it. |
| `--sign-command CMD` | Run this command once for each file that is built, with `{file}` replaced by the file's path. Default: the `JAMB_SIGN_COMMAND` environment variable. See [Signing](#signing). |
| `--sdk-bin DIR` | The directory with `makeappx.exe`. Default: the newest SDK under `C:\Program Files (x86)\Windows Kits\10\bin`. |

`--cert`, `--test-cert`, `--timestamp` and `--no-msix` were removed. Each one is
refused with the replacement to use.

### What the executable contains

- The icon, in 16, 24, 32, 48, 64, 128 and 256 px.
- Version information: file and product version (`version` with `.0`), company,
  product name, description, original and internal file names, and copyright.
- An application manifest with per-monitor v2 DPI awareness, which matches what
  the framework sets at run time; Common Controls version 6, which `TaskDialog`
  needs; Windows 10 and 11 as the supported OS; long path names; and `asInvoker`,
  so the app never asks for elevation.
- A build with `-trimpath -ldflags "-s -w -H=windowsgui"`, `GOOS=windows`,
  `CGO_ENABLED=0`. The GUI subsystem means no console window opens.

The manifest does not set `activeCodePage` to UTF-8. The framework calls only the
wide-character Win32 functions, so the active code page does not affect it, and
leaving the system's code page alone is the safer default for any ANSI call.

While it builds, `jamb package` writes a resource object,
`jamb_package_windows_<arch>.syso`, into the app's directory and removes it when
packaging ends, on failure and on Ctrl+C too. If a file with that name already
exists, packaging stops rather than overwrite it.

### Installer

`--installer` compiles `<name>-<version>-windows-<arch>-setup.exe` with NSIS,
from a script that jamb generates from `jamb.json`. The installer:

- installs for the current user only, to `%LOCALAPPDATA%\Programs\<displayName>`.
  It needs no administrator rights, and it writes only to `HKCU` and the user's
  Start menu and desktop. This is the default because it works on managed PCs
  where standard users cannot write to Program Files, and because each user can
  update the app without elevation. A machine-wide Program Files installer is not
  generated; that would need its own script and an administrator prompt.
- adds a Start menu folder with the app and an uninstaller shortcut;
- offers a desktop shortcut as an optional component, off by default;
- registers the app in Apps & features under `identityName`, with the display
  name, version, publisher, description, icon and estimated size;
- uninstalls cleanly: it removes the installed files, the shortcuts and the
  registry entry. It leaves data the app wrote elsewhere, such as under AppData,
  in place.

Close the app before installing or uninstalling it. Silent installs use `/S`, and
`/D=PATH` (last on the command line) chooses another folder.

### Signing

jamb never needs a signing certificate, and it never reads one or a password. It
is a library shipped as source, so it is not signed. Each app's author chooses how
the app's binaries are signed:

- **Unsigned.** Works. Windows SmartScreen may warn people when they first run the
  app. An unsigned MSIX cannot be installed on a PC.
- **Microsoft Store.** Free for individual developers since September 2025. The
  Store signs the MSIX packages it accepts, so an MSIX can be uploaded unsigned:
  <https://blogs.windows.com/windowsdeveloper/2025/09/10/free-developer-registration-for-individual-developers-on-microsoft-store/>
- **SignPath Foundation.** Free signing for open-source apps with OSI-licensed
  source and automated public builds. Eligibility and setup are at
  <https://signpath.org/>.
- **Azure Artifact Signing, or a purchased certificate.** Paid. Azure Artifact
  Signing is documented at
  <https://learn.microsoft.com/en-us/azure/artifact-signing/overview>.

#### The sign command

Set the command with `--sign-command "<command>"`, or with the `JAMB_SIGN_COMMAND`
environment variable. The flag wins when both are set. The environment variable is
used rather than `jamb.json` because the certificate differs between machines and
CI runners, and a committed file should not hold a thumbprint or a path.

jamb runs the command once for each file it builds, and `{file}` is replaced by
that file's path:

1. the executable, before it goes into the zip, the installer or the MSIX;
2. the installer, after it is compiled;
3. the MSIX, after it is packed.

Rules:

- The command runs directly, with no shell. Put a program path with spaces in
  double quotes. Pipes, `&&` and `%VAR%` expansion do not work. For those, call a
  script: `powershell -NoProfile -ExecutionPolicy Bypass -File C:\sign\sign.ps1 {file}`.
  Do not use a `.cmd` or `.bat` file as the program, since cmd.exe reads `&` and
  `%` in the path.
- If the command exits with an error, packaging stops and nothing is written to
  `dist`. The error does not repeat the command, which may name secrets.
- Keep passwords out of the command. Use the Windows certificate store, or a script
  that reads a secret from the environment.
- The command's output is shown in the console, so a signer that asks for a PIN
  can be answered.

Examples, in PowerShell. Replace the placeholders.

Certificate in the Windows certificate store, found by thumbprint, with signtool
from the Windows SDK:

```
$env:JAMB_SIGN_COMMAND = '"C:\Program Files (x86)\Windows Kits\10\bin\<sdk version>\x64\signtool.exe" sign /fd SHA256 /sha1 <THUMBPRINT> /tr http://timestamp.digicert.com /td SHA256 {file}'
go run ./cmd/jamb package --installer --msix ./examples/demo
```

PFX file with its password in an environment variable. signtool accepts a PFX
password only as its `/p` argument, so the password is visible to other processes
of the same user while signtool runs. The certificate store avoids that. Where a
PFX is required, use a script such as `C:\sign\sign.ps1`:

```
param([string]$File)
& "C:\Program Files (x86)\Windows Kits\10\bin\<sdk version>\x64\signtool.exe" sign /fd SHA256 /f "C:\certs\release.pfx" /p $env:CERT_PASSWORD /tr http://timestamp.digicert.com /td SHA256 $File
exit $LASTEXITCODE
```

```
$env:JAMB_SIGN_COMMAND = 'powershell -NoProfile -ExecutionPolicy Bypass -File C:\sign\sign.ps1 {file}'
```

Azure Artifact Signing uses signtool with the Artifact Signing client's `dlib`
and a metadata file. Take the timestamp URL, the DLL path and the metadata file
from the documentation linked above:

```
$env:JAMB_SIGN_COMMAND = '"<signtool.exe>" sign /fd SHA256 /tr <timestamp URL> /td SHA256 /dlib "<Artifact Signing dlib>" /dmdf "<metadata.json>" {file}'
```

SignPath signs through its own service and build integration. Use the command its
documentation gives, as the sign command. See <https://signpath.org/>.

### Testing the MSIX on this PC

An unsigned MSIX cannot be installed, and jamb does not create certificates. For a
local install test, sign the MSIX with a self-signed certificate whose subject is
`publisherSubject`, and trust that certificate on the PC. Microsoft's steps for
creating and trusting a certificate for package signing are at
<https://learn.microsoft.com/en-us/windows/msix/package/create-certificate-package-signing>.
They use `New-SelfSignedCertificate`, and they import the certificate into the
`LocalMachine\TrustedPeople` store from an administrator session.

Trusting a certificate changes what every user of the PC will accept, so remove it
when the test is done.

### Requirements

- **Zip.** Windows 10 or 11 and Go 1.27 or later. Nothing else.
- **`--installer`.** NSIS 3.x, which provides `makensis`. Install it with
  `winget install NSIS.NSIS`, with `scoop install nsis`, or from
  <https://nsis.sourceforge.io/Download>. jamb looks for `makensis` on PATH, in
  `Program Files (x86)\NSIS` and `Program Files\NSIS`, and at the `--makensis`
  path. If it is missing, jamb prints these instructions, skips the installer with
  a warning, and still writes the zip.
- **`--msix`.** The Windows SDK, with its **Windows SDK Signing Tools for Desktop
  Apps** feature, which provides `makeappx.exe`. Get it from
  <https://developer.microsoft.com/windows/downloads/windows-sdk/>. Use
  `--sdk-bin` if the tool is somewhere else. If it is missing, jamb skips the MSIX
  with a warning and still writes the zip.
- **`--sign-command`.** Whatever the command runs.

NSIS also runs on Linux and macOS, so a host other than Windows can compile the
installer. That path has not been tested.

### Packaging dependencies

`cmd/jamb` is its own Go module, listed in `go.work`, so the framework's `go.mod`
and the modules that import it gain nothing from packaging. Its dependencies are
`github.com/tc-hib/winres` (writes the resource object and the icon; ISC-style
license), `golang.org/x/image` (resizes the icon; BSD-3-Clause) and
`github.com/nfnt/resize` (used by winres; ISC-style license). The Windows SDK
tools and NSIS are external programs and are not redistributed. NSIS is under a
zlib-style license.

## Icons

`icons.All` lists all of them:

> activity, alert, archive, arrow-left, arrow-right, arrow-up-right, bell,
> bookmark, box, calendar, check, chevron-down, chevron-left, chevron-right,
> chevron-up, circle, clock, copy, dashboard, download, eye, file, filter,
> folder, git-branch, hash, heart, home, inbox, info, layers, link, lock,
> maximize, menu, minimize, minus, moon, more-horizontal, pencil, pin, plus,
> refresh, restore, search, settings, star, sun, tag, trash, upload, users, x, zap

The Go names are the CamelCase forms: `icons.ArrowLeft`, `icons.GitBranch`,
and so on.

## The demo app

`examples/demo/` shows the widgets, the layout and the paint features on sample pages.
On the Projects page:

- Drag a project card to reorder projects. Drop it on the upper or lower half
  of another card to place it before or after that card; a 2 px line shows
  where. The order is kept even while a status filter hides some projects.
- Drop a project on Active, Paused or Done in the filter bar to set its
  status.
- On the Activity page, the project name in each row is a link: clicking it
  opens the Projects page with that project selected.
- Tab walks the sidebar navigation (each item shows a focus ring), and Enter
  or Space opens the focused page.
- The Projects page header uses an Oklab gradient on the logo tile.
- Clicking the "Open in window" button on a project card opens the project in
  its own window, which has a comments panel. A card whose window is open
  shows an "Open in window" badge until that window closes.
- The Activity page has a filter field (matches are highlighted in each row),
  rows of different heights in a `ListOf`, and build rows that expand on a
  click, or on Enter or Space when focused.
- Right-click a project card for its menu: Open, Open in window, Star (a check
  mark), a Status submenu with radio items, and a disabled Archive item.
- The Components page (Ctrl+4) shows the controls, overlays, gradients, rounded
  clipping, per-side borders, motion and focus styles, one section each. Its
  Menus section has a dropdown and a context menu with a submenu; their shortcuts
  come from the demo's bindings.
- **New Project** opens a modal form with a name and a description. Create adds
  the project, selects it and shows the Projects page. It is disabled until the
  name has text. Cancel, Escape and a press on the backdrop close the form
  without adding a project.
- The Components page's **Dialogs** section opens each native dialog and shows
  what it returned, by file name only, never by folder. Its Clipboard buttons
  copy and paste plain text, and **Form in a modal** opens a modal form.

![The Dialogs section of the Components page](docs/screenshots/components-dialogs.png)

- Keyboard: Ctrl+1 to Ctrl+5 open the Dashboard, Projects, Activity, Components
  and Settings pages, Ctrl+K moves focus to the search field, and Ctrl+Shift+L
  toggles the theme.

## The notes app

`examples/notes/` is a second, text-heavy application built only from the public
API. It exists to find API gaps; it found three, which became framework
features: focus handles, `AutoFocus`, and `FocusWithin`.

- Notes live in a JSON file in the user config directory, `jamb-notes/notes.json`
  (override with `JAMB_NOTES_FILE`). The directory was called `goui-notes` before
  the rename; the app moves it to `jamb-notes` on first start, so existing notes,
  settings and keymap carry over. A background goroutine saves 400 ms after typing
  stops and reports progress through a signal.
- Every note field is a signal, so typing re-renders only that note's list
  entry, the word count and the editor, never the whole list.
- Shortcuts: Ctrl+N (new note, title focused), Ctrl+F (search; Enter opens
  the first match), Alt+Up/Down (previous/next note), Ctrl+Shift+P (command
  palette: type to filter, Up/Down to choose, Enter runs, Escape closes),
  Ctrl+K Ctrl+T (toggle theme). Enter in a title moves to its body. Escape
  closes the palette and returns focus to the note you were in, and a command
  run from the palette acts on that note.
- `keymap.json` next to the notes file overrides the shortcuts, in the format
  described under "Keymap files". It is applied when you save it, while the
  app runs. Its problems are printed to stderr and shown in the status bar.
- Tab reaches the tag chips and the note rows near the viewport (see the
  limitations), and shows a focus ring on the focused one. Enter opens a note (as a click does)
  and toggles a tag chip's filter.
- Ctrl+= / Ctrl+- / Ctrl+0 zoom in, out and reset. They are the actions
  `ZoomIn`, `ZoomOut` and `ZoomReset`, so the View menu shows them. The level is
  remembered in `settings.json` next to the notes file and shown in the status bar
  when it isn't 100% (click it to reset).
- A menu bar above the notes has File (New note, Import..., Export note...,
  Exit), Edit (undo, redo, cut, copy, paste, select all), and View (toggle theme,
  zoom, command palette). Each item with a key shows that key. Alt by itself
  focuses the bar. Right-click a note in the list for Pin or Unpin, and Delete.
- **Import...** opens the native file dialog with `*.md` and `*.txt` offered and
  several files allowed. The files chosen go through the same import as a drop,
  and the status line reports the count. **Export note...** saves the selected
  note: the file is plain text when the chosen name ends in `.txt`, and Markdown,
  with the title as a heading, otherwise. The dialog offers the title as the
  name. The status line names the file, not its folder.
- Delete in the editor asks for confirmation inline, with Cancel and Delete
  buttons. It does not use a message box, so the notes app shows no native
  dialog for it.
- The seeded "Languages" note shows ten scripts, ligatures and emoji.
- With a search query, matches (case-insensitive) are highlighted in each
  listed note's title and snippet.
- Tags: pills under the title (Enter adds a tag, x removes it) and filter
  chips in the sidebar that wrap onto more lines.
- Drag and drop: drag a note from the sidebar list onto a tag chip to add
  that tag. The chip lights up while the note is over it. Drop `.txt` or
  `.md` files (up to 1 MB) on the window to import each as a note titled with
  its file name; the last one is selected, and the status bar says how many
  were imported and skipped.

### Agent test

To test goal #10 (APIs coding agents can use), a fresh agent got only the
README and the notes app. Its task was to add tags: storage, the editor
pills, the sidebar filter, tests and a screenshot check.

**Result:** it finished in about 4 edit/compile cycles. Its first build
compiled with zero errors, and it never read framework source.

**What changed because of its friction log:**

| Friction | Fix |
|---|---|
| No flex-wrap; had to guess text widths to wrap chips | `.Wrap()` |
| Tests located elements by parsing `Dump` text | `rt.Find`, `rt.ClickKey`, `rt.Type`, `rt.Press` |
| Text fields were anonymous in `Dump` | Keys and field contents now shown |
| No compact text field; unclear what `Style` affects | `TextInput.Small()`, documented `Style` |
| No component-local state | `ui.Local` |
| Icons hard to discover; no tag icon | `icons.All`, icon list, 24 new icons |
| Unclear whether signal values can be mutated | Documented: replace, don't mutate |

The agent's chip-wrapping and test workarounds were then replaced with
these features (`examples/notes/views/tags.go`, `tags_test.go`).

## Milestone status

| Milestone | Status |
|---|---|
| 0: native window, GPU surface, rectangle, text | ✅ |
| 1: Element, Div, Text, style object, tree, row/column layout | ✅ |
| 2: hit testing, hover, buttons, click handlers, signals, redraw scheduling | ✅ (counter and dynamic project cards in the demo) |
| 3: grow/shrink, align, justify, gap, margins, percentages, clipping, vertical and horizontal scrolling | ✅ |
| 4: rounded corners, borders, shadows, icons, images, hover transitions, dark/light themes | ✅ |
| 5: profiling | ✅, see [BENCHMARKS.md](BENCHMARKS.md) |
| Dev runner (`jamb dev`): watch, rebuild only on change, restart, keep window geometry | ✅ (development state is not preserved yet) |
| Multiple windows and custom title bars (Windows) | ✅ see [Windows](#windows) and [Custom title bars](#custom-title-bars) |

## Known limitations and next steps

- **Focus.** Tab reaches only the rows of a virtualized list (`ListOf`)
  that are built, which are the rows near the viewport; rows beyond them
  are reached by scrolling and then Tab. Tab reveals vertically only,
  not horizontally. A borderless text field has no focus indicator of its
  own, since it has no frame to style. After a click on a `TabStop(false)`
  element, Tab starts from the first element of the order and Shift+Tab
  from the last.
- **Overlays.** They animate in and out only when they set `Enter` and `Exit`
  (tooltips and the notes palette do). Escape closes an overlay only when it
  has an `OnClickOutside`; any other overlay handles Escape itself, with
  `OnAction` and a binding or in `OnKeyDown`.
- **Key bindings.** A handler can't pass the key on to the next binding.
  The keymap file's watch is per directory, so other files that change there
  (such as the notes file on each save) cause a cheap re-read of keymap.json,
  which is ignored when its bytes are unchanged. Binding layers are only
  partly like GPUI's (its base, default and user keymaps): the file's layer is
  replaced in place, and otherwise the order in which bindings are added
  decides between them.
- **Platforms.** Only the Windows adapter exists. macOS (Cocoa + Metal) and
  Linux (Wayland/X11 + Vulkan or GL) adapters plug in behind
  `platform.Window` and `render.Renderer`. Since there is no cgo today,
  macOS will need either a small cgo shim or purego-style Objective-C calls.
- **Windows, multiple windows and custom title bars.** Checked on Windows 11
  with real mouse input: the caption buttons (hover, press, maximize and
  restore), double-clicking the title bar, the hit-test answers for the title
  bar, its buttons, the left border and a corner, and the maximized geometry
  (content inside the work area).
  Checked with scripted input: opening a window from a click, sharing a star
  between two windows, closing a window while the app keeps running, and
  closing the last one. The Snap Layouts flyout was checked by hand on Windows
  11 and works, provided the top DWM frame margin stays 0 (see `applyCustomFrame`
  in `platform/windows/chrome.go`). Not checked automatically, so check
  these by hand: dragging the title bar to move the window and to snap it to an
  edge, Win+ arrows, the drop shadow and the rounded corners.
  Images and glyph textures are shared per device and stay resident after
  the window that used them closes. A lost GPU device is rebuilt
  automatically; see [GPU device loss](#gpu-device-loss) for what was checked
  and what was not.
- **Text.** Shaping, bidi display, fallback, selection, clipboard and undo
  work. Still missing:
  - **IME.** Composition text appears in the system's window at the caret,
    not inline in the field.
  - **Paragraph direction.** It is always left-to-right, so an Arabic-only
    paragraph is right-to-left inside but left-aligned. Arrow keys move in
    logical order through mixed-direction text.
  - **Emoji.** Drawn monochrome.
  - **Rendering.** On Windows, glyph masks come from DirectWrite (hinted
    vertically: crossbars and x-height/cap-height edges land on whole pixels
    while stems keep their subpixel position). When the user has ClearType
    on (`SystemParametersInfo` font smoothing of type ClearType), glyphs are
    rasterized as ClearType RGB coverage (BGR panels are honored) into a
    second atlas and drawn with dual-source blending, with an alpha
    correction in the shader driven by the user's DirectWrite gamma,
    contrast and ClearType level. The correction was fitted against
    `IDWriteBitmapRenderTarget` output (`TestClearTypeReference`, about 2.5
    levels mean difference on inked pixels). Grayscale anti-aliasing with
    the contrast-enhancing shader is used when ClearType is off, when the
    device lacks dual-source blending, for icons, and for fonts without a
    file path or on other platforms (the pure-Go rasterizer is unhinted and
    slightly softer). Subpixel text assumes an opaque swap chain and
    pixel-scale drawing, which holds for every window jamb creates.
    `JAMB_TEXT_AA=gray` forces grayscale. Fitted at the default gamma 1.8
    and contrast 0.5; other tuner settings scale the correction rather
    than being fitted.
  - **Spell checking.** Missing. Rich text is display-only; text areas stay single-style.
  - **Shaping cost.** About 0.4 ms per 1,000 characters. It's cached per
    paragraph, so typing in a long note re-shapes only the edited paragraph
    (about 0.1 ms per keystroke at 11,000 characters).
- **Untested paths.** Non-100% DPI is covered by a headless test
  (`TestScaleProducesPhysicalPixels`) but not on real hardware.
  Multi-monitor `WM_DPICHANGED` has not been exercised either.
- **Paint cost is per visible node.** Paint and hit testing skip off-screen
  subtrees (see the frame pipeline), so a long scroll list costs about its
  visible rows: repainting 10,000 nodes with 73 on screen takes about 0.06 ms.
  A tree whose nodes are all on screen still pays the per-node cost, and a
  container with 10,000 children runs a visibility test on each of them. The
  node itself is about 1 KB; the compact per-node paint struct is the next
  step (see BENCHMARKS.md). `ui.UniformList` and `ui.ListOf` avoid the
  per-child cost for long lists by building only the rows in view. A `ListOf`
  has three caveats: the scrollbar thumb is sized from estimated heights until
  rows have been seen; a slice changed in place is not detected (build a new
  one); and a row whose height changes while it is not built keeps its old
  height until it is built again.
- **No race-detector run.** Go's `-race` needs cgo, and neither of this
  machine's C toolchains could build it (MSYS2 GCC 16 fails on
  `runtime/cgo`; Zig's MinGW lacks `synchronization.lib`). The signal and
  instance locking is covered by a concurrent stress test instead.
- **Styling.** Border widths are layout, so they don't ease with `Transition`,
  and all sides share one color. Clipping to a rounded shape is exact for one
  rounded container; a nested one keeps only its own corners, and hit testing
  uses the rectangle (see Borders and rounded clipping). Colors, opacity,
  offset and scale ease with `Transition`. Layout
  properties (sizes, padding and so on) animate only through `ui.Animate`,
  which re-lays out each frame. There are no springs yet, no rotation, and
  scale is not hit-tested.
- **Events.** Bubbling only; there's no capture phase.
- **Drag and drop.** Files dragged in from outside the app get no drag
  feedback: `Over`, `Move` and `Leave` do not run, because OLE drag feedback
  for external drags is not implemented. Nothing can be dragged out of the
  app, and OS drops have no drag image. Cursor kinds are unchanged; a drag
  uses the existing not-allowed cursor. File drops arrive through Win32
  `WM_DROPFILES`, which tests can't produce (the drop handle is only valid
  in the process Explorer gives it to); `JAMB_SCRIPT`'s `drop` step covers
  everything after it. Windows refuses drops from Explorer into an app
  running as administrator.
- **Startup.** The ~180 ms startup floor is Direct3D driver initialization.
  The framework's own startup cost is about 20 ms.
- **Menus.** Alt+letter mnemonics are not implemented. A menu stays open when a
  key binding moves focus elsewhere. A press outside, Escape or Tab closes it.
  Focus loss does not close menus, because a click on the trigger of a focusable
  dropdown would then reopen it. The submenu delay is a timer, not a triangle.
  The menu bar's labels are not in the Tab order. The roles are in place for
  UI Automation, which is not wired.
- **Dialogs and clipboard.** Native dialogs run on Windows only. Message boxes
  use `MessageBoxW`, not `TaskDialog`. `TaskDialog` needs the common-controls
  version 6 manifest, which `jamb package` adds to a packaged app but a `go run`
  build does not have, so the switch (with a `MessageBoxW` fallback) is future
  work. The clipboard is plain text only: images and
  HTML are not supported. On Windows 11 the file, folder and message box dialogs
  were driven through UI Automation and window messages. They opened owned by
  their window, returned their results, and canceled correctly, and the window
  stayed disabled and kept drawing while a dialog was open. Not checked: dialogs
  in other languages, and high-contrast themes.
- **Packaging.** Windows only (see [Packaging](#packaging)). Not done: Store
  submission (the MSIX is uploaded by hand), a machine-wide installer, file
  associations and capabilities beyond `runFullTrust`, localized resources, the
  wide and splash logos and target-size logos of the MSIX, and automatic updates.
  The installer has been run only on Windows 11, and its upgrade over an older
  version has not been tested. The arm64 build is compiled but has not been run.
  The MSIX install steps are for a person to run; the SDK tools check the
  package's structure.
- **Accessibility.** Not available yet; screen readers see nothing. The design
  and plan are in [docs/accessibility.md](docs/accessibility.md).
- **Not attempted (non-goals).** Tray,
  CSS grid, and the optional utility-class shorthand
  (`.Class("flex col gap-2")`). The shorthand was skipped in favor of typed
  styles.

## License

jamb is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE)
and [NOTICE](NOTICE).
