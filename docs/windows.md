# Windows and dialogs

Windows, zoom, custom title bars and native dialogs.

## Zoom

Zoom: Ctrl+= (or Ctrl++, numpad +) zooms in, Ctrl+- zooms out, Ctrl+0
resets, and Ctrl+mouse wheel zooms. Levels run 50% to 300% (`ui.ZoomLevels`).
In code: `rt.Zoom()`, `rt.SetZoom(z)`, `rt.ZoomIn()`, `rt.ZoomOut()`,
`rt.ResetZoom()` and `rt.SetZoomShortcuts(false)` to turn the keys off;
components use `ctx.Zoom()` and `ctx.SetZoom(z)`, and `app.Window.OnZoom` runs
when the zoom changes.

`app.Window{Zoom: 1.25, OnZoom: func(z float32) {...}}` sets the initial
level and observes changes. Zoom multiplies the device scale, so text is
shaped and rasterized at the zoomed pixel size and stays sharp at every
level. Layout sees a smaller logical window (window size / zoom).

## Windows

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
h.Focus()          // bring it to the front; h is a ui.Window
h.SetTitle("...")  // also Close(); h.Closed() reports whether it is gone
```

`app.Open` returns a `ui.Window`, the same type as `ctx.Window()` inside a
component, so either one can be moved, resized, made fullscreen, focused or
closed (see below). Its methods may be called from any goroutine. An action
runs on the UI thread, at once when it is called there and in call order
otherwise, so an action right after `Open` runs once the window exists. A read
from another goroutine waits for the UI thread to answer it.

- The app quits when its last window closes. A `QuitWhenClosed` window quits
  the app when it closes, even with others open. Before it closes, it asks the
  `OnClose` of every other open window, and then its own. If any returns
  false, the whole close is cancelled and every window stays open. Otherwise
  the other windows close too, each without being asked again, and their
  `OnClosed` runs.
- `X` and `Y` place a window in logical screen pixels. Left at zero, the
  first window is centered and a later one is cascaded 32 px from the last.
- `OnGeometry` is called on the UI thread as the window moves, resizes or changes
  its scale, with its position and content size in logical pixels. It is called
  for each step of a move, so save from it only once the window is still.
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

Inside a component, `ctx.Window()` controls its window: `Focus()`, `Minimize()`,
`ToggleMaximize()`, `Close()`, `SetTitle(s)`, and the reactive reads
`Maximized()` and `Focused()`, which re-render the component when they change.
`Position()`, `Size()`, `SetPosition(x, y)` and `SetSize(w, h)` read and change
the window's place and content size in logical pixels, as `app.Window` takes them
(a maximized or fullscreen window is restored first). `SetFullscreen(on)` makes the
window cover its monitor without a frame, and puts it back where it was;
`Fullscreen()` reports which. `Show()` and `Hide()` show and hide the window and its
taskbar button, and `Visible()` reports whether it is shown: it has opened, is not
hidden by `Hide` or by a `CloseToTray` close, and has not closed. A minimized window
is visible. A headless runtime (in tests) has no window, so the actions do nothing,
`Size()` is the viewport's, and `Visible()` is true.

## Custom title bars

`app.Window{Chrome: app.ChromeCustom}` removes the operating system's title
bar, and the app draws its own. The window keeps everything native: resize
borders and cursors, Aero Snap, Win+arrows, the drop shadow, the minimize and
maximize animations, the rounded corners of Windows 11, and the maximized
geometry (content stops at the work area, above the taskbar).

Mark the title bar's areas with `WindowArea`:

```go
ui.Div().Row().Height(ui.Px(32)).WindowArea(ui.WindowAreaDrag).Justify(ui.End).
    Child(search).             // interactive children keep working
    Child(ui.TitleBarButtons()) // minimize, maximize or restore, and close
```

- `ui.WindowAreaDrag` moves the window when dragged, and double-clicking maximizes
  it. A point in a drag area is a drag unless an element on top of it takes
  input (`OnClick`, `OnMouseDown`, focus or `OnDrag`), so a search field or
  a button inside the title bar still works. `ui.WindowAreaMinimize`,
  `ui.WindowAreaMaximize` and `ui.WindowAreaClose` act when the mouse is released over
  them.
- `ui.TitleBarButtons()` is the usual set: buttons 46 px wide like Windows 11's,
  as high as the bar they are in (32 px in a bar sized by its content), so they reach
  the top and bottom of a taller bar, with hover and pressed styles, a red close button and the restore icon
  while the window is maximized. The buttons are native hit-test areas, which is what
  lets Windows 11 show Snap Layouts over the maximize button.
- Windows with the operating system's title bar ignore areas.

### TitleBar

`ui.TitleBar()` is a window's own title bar as a container: a row whose children
go at its start, the left, in order, with the caption buttons at its end. It marks
itself as a drag area, and its content dims while the window is not focused (the
caption buttons keep full strength). Under high contrast it does not dim.

```go
ui.TitleBar().Height(ui.Px(32)).
    Child(ui.Icon(icons.GitBranch)).Child(crumbs).  // the start, left to right
    Center(search).                                  // centered on the window
    Trailing(refresh, themeToggle)                   // the end, before the caption buttons
```

- `Center(el)` puts `el` in the middle of the bar's whole width, not between the sides.
  It gets the room between the two sides, so it never covers them: the wider side is
  kept clear on both ends, and when the sides leave too little room the element is
  clipped to what is left. The sides are measured from the last layout, and the bar
  renders again when a side changes width.
- The start side shrinks and clips when the window is narrow, so a long title
  truncates. Put a `Truncate` text in a wrapper with `Shrink(1)` and `MinWidth(0)`.
- `Trailing(els...)` adds items at the end, before the caption buttons. The end
  is as wide as its items, and the caption buttons fill the bar's height.
- `Buttons(false)` leaves out the caption buttons, for a window that has none. They
  are shown by default, as `TitleBarButtons`.
- `Height` is 40 px at 100 percent Text size by default. Windows' own caption buttons
  are 32 px, so a bar lower than 32 px cannot hold them.
- Children that take input (a search field, a button) stay clickable inside the bar,
  as with `WindowAreaDrag` above.
- With the operating system's title bar (`ChromeNative`) the operating system draws
  the caption buttons, so a window that uses a `TitleBar` there passes
  `Buttons(false)`. The bar still lays out without them.
- The look is `ui.TitleBarStyle` with `Bar` (the strip, with its height, fill, bottom
  border and text) and `Center` (the centered element's box), set with `.Styles(...)`
  or `.Style(s)` for the strip. By default the strip has no fill and no border, so it
  is part of the window's content, as a Windows 11 title bar is; a band of its own is
  `Style(ui.Style{Background: t.Panel, BorderEdges: ui.Edges{Bottom: 1}, BorderColor: t.Border})`.

## Native dialogs

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
  dialog's own message loop runs. The same timer runs while the window's system
  menu (F10, or Alt alone in a window without a menu bar) or a move or size is in
  progress, so posted work still runs during those loops.
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
