# jamb

A declarative, GPU-accelerated desktop UI framework for Go. Tested only on Windows.

jamb is a small, declarative, GPU-rendered desktop UI framework written in plain Go. An app is a tree
of elements built by components, and its state lives in signals, so a change re-renders only the
parts that depend on it.

> **Status:** jamb is pre-1.0. The API may change between versions. It runs on Windows 10 and 11
> today; other platforms are planned.

## Highlights

- Widgets: buttons, split buttons and toggles, text fields (masked and clearable), one-time code
  inputs, checkboxes, switches, sliders and range sliders, ratings, selects, comboboxes, date and time
  pickers, calendars, color pickers, menus, a command palette, tabs, trees, steppers, carousels, data
  grids, forms with validation, alerts, toasts, hover cards, and modal dialogs.
- Layout: flexbox, grid, sidebars, toolbars, status bars, split panes, docking, and variable-height
  and uniform lists.
- Theming: light and dark themes that follow the system's light or dark mode, 37 converted predefined
  themes in 22 families, and automatic high contrast.
- Accessibility: a UI Automation tree for screen readers, keyboard navigation, reduced motion, and a
  text size setting.
- Text: shaping with HarfBuzz, fallback fonts, rich and selectable text, and input method (IME) composition.
- Graphics: a canvas, SVG images, gradients, and charts.
- Code editing: a `codeedit` package with syntax highlighting for 76 languages, undo and redo, and find and replace.
- Markdown: a `markdown` package that shows CommonMark with the GitHub extensions as accessible, selectable elements.
- OS integration: jump lists, single instance, a notification area icon, taskbar progress, and OS notifications.
- Development tools: `jamb dev` rebuilds and restarts an app on each save, and a visual inspector shows the
  elements of a running window.
- Packaging: `jamb package` builds a Windows zip, an installer, and an MSIX.

## Install

jamb needs Windows 10 or 11 and Go 1.27.1 or later. A minimal `main.go` opens a window with a
greeting and a button that counts its clicks:

```go
package main

import (
	"fmt"

	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/ui"
)

func main() {
	clicks := ui.NewSignal(0)

	app.New().Run(app.Window{
		Title:  "Hello",
		Width:  800,
		Height: 600,
		// Root is a component, so it runs again whenever clicks changes.
		Root: func(ctx *ui.Context) ui.Element {
			return ui.Div().Column().Center().Gap(24).
				Child(ui.Text("Hello, world!").FontSize(24)).
				Child(ui.Button(fmt.Sprintf("Clicked %d times", clicks.Get())).
					OnClick(func() { clicks.Update(func(n int) int { return n + 1 }) }))
		},
	})
}
```

In the module that holds it, add jamb and run the app:

```
go get github.com/tyler-builds/jamb@v0.3.0
go mod tidy    # records the checksums of jamb's own dependencies
go run .
```

The `jamb` command is a separate module, whose latest release is `cmd/jamb/v0.2.0` (it is unchanged in v0.3.0):

```
go install github.com/tyler-builds/jamb/cmd/jamb@v0.2.0
```

The repository includes three examples. From a clone of the repository, run them with:

```
go run ./examples/hello    # the smallest complete app: one window, a signal and a button
go run ./examples/demo     # a tour of the widgets, layouts, and pages
go run ./examples/notes    # a note-taking app
```

The [getting started guide](docs/getting-started.md) covers the commands for building and packaging,
and the [concepts](docs/concepts.md) page introduces the API.

## Benchmarks

The same hello-world app (a window, a greeting and a click counter) built with jamb and with GPUI, the
UI framework from Zed, on one Windows machine. The figures are from 2026-10-21, except the two rows
marked 2026-10-12. Lower is better in every row, and the better figure is in bold. The windows were on
a secondary 144 Hz monitor, never activated, while a video played on the primary monitor. The
[docs/comparison-gpui.md](docs/comparison-gpui.md) has the method, the ranges of each measurement and
the setup.

| Measure | jamb | GPUI |
|---|---|---|
| Startup to first frame, warm launch | **222 ms** | 304 ms |
| Startup to first frame, first launch (scanned by the antivirus) | **487 ms** | 493 ms |
| Window visible, first launch | 468 ms | **460 ms** |
| Idle CPU, window in the background, 60 s | **0.05%** of one core | 1.74% |
| Idle context switches per second | **64** | 410 |
| Memory, private bytes | 78.8 MB | **73.2 MB** |
| Memory, working set | 58.4 MB | **49.0 MB** |
| GPU memory, dedicated | **13.7 MB** | 15.5 MB |
| Threads | 39 | **32** |
| Handles | 483 | **462** |
| CPU while the pointer moves over the window | **1.1%** | 3.6% |
| CPU while the window is resized | **11.8%** | 13.9% |
| Context switches per second while resizing | 2,175 | **1,531** |
| Build after an edit (release, text change) | **1.36 s** | 2.57 s (release), 3.60 s (debug) |
| Clean build (2026-10-12) | **9 s** | 138 s (release), 100 s (debug) |
| Binary size, release build | 9.36 MB | **9.22 MB** |
| Binary size, GPUI's size-optimized build profile (2026-10-12) | | **4.31 MB** |

The focused idle CPU was not measured on 2026-10-21. On 2026-10-12 the hello app used 0.29% of a core
with the window focused, and GPUI 1.85%. jamb's frames on the secondary monitor followed the primary
monitor's refresh rate, a known issue listed in [BENCHMARKS.md](BENCHMARKS.md).

The demo app (four pages), from [BENCHMARKS.md](BENCHMARKS.md), measured on 2026-10-21 unless the row says
otherwise:

| Measure | Result | What it means |
|---|---|---|
| Startup to first frame, warm | 215 ms | About 180 ms of it is the graphics driver creating the Direct3D device (2026-10-12). |
| Idle CPU, unfocused, 60 s | 0.13% of one core | An idle window draws nothing. About 66 context switches a second; the 2026-10-12 analysis traced most of an idle window's wake-ups to the graphics driver's thread. |
| Full re-render of every component | 0.97 ms of CPU per frame on average, 164 FPS at vsync, on a 310-element page | A full re-render is a stress case; normal updates are incremental. It allocates 936 KB a frame, up from 110 KB (2026-10-06), which is under investigation. |
| One signal change in a 10,000-element tree | 0.31 ms | Only the component that read the signal renders again. |
| One scroll step in a 100,000-row list | 0.15 ms | Only the rows in view are built. |
| Rebuild after an edit (2026-10-06) | 0.25 to 0.84 s | Edit, build and run stays under a second; `jamb dev` shows the new window 1.05 to 1.36 s after a save. |

## How it works

jamb is plain Go, with no cgo. On Windows it uses Win32 for windows and input, and draws with a
Direct3D 11 renderer that it reaches through COM vtables; text is shaped with HarfBuzz (through
go-text) and the accessibility tree is exposed through UI Automation. Each frame, components whose
signals changed render again, the layout updates what moved, and the renderer submits the scene to
the GPU; nothing is drawn while nothing changes. [docs/architecture.md](docs/architecture.md) has the
details.

## Examples

The examples are in [examples/](examples/). `examples/hello` is the minimal app shown above.
`examples/demo` is a tour of the widgets and layouts. It is also the app that `jamb package` builds in
the packaging guide. `examples/notes` is a note-taking app with persistence, a command palette,
and a keymap file. [docs/examples.md](docs/examples.md) describes the demo and notes apps.

## Documentation

The full documentation is in [docs/](docs/README.md). Each page covers one topic:

- [Getting started](docs/getting-started.md): requirements, the examples, and adding jamb to an app.
- [Concepts](docs/concepts.md): the element tree, the builder API, state and keys, the UI thread, and errors.
- [Layout](docs/layout.md): shrinking, grid layout, split panes, and docking.
- [Styling](docs/styling.md): the parts, states, variants, and sizing of the built-in widgets.
- [Theming](docs/theming.md): themes, predefined themes, widget style parts, and system settings.
- [Drawing](docs/drawing.md): gradients, borders, the canvas, charts, SVG images, and icons.
- [Animation](docs/animation.md): transitions and enter and exit animations.
- [Text](docs/text.md): rich text, selectable text, fonts, and emoji.
- [Input](docs/input.md): keyboard and focus, text fields, the clipboard, actions and key bindings, drag and drop, and gestures.
- [Windows and dialogs](docs/windows.md): windows, zoom, custom title bars, and native dialogs.
- [OS integration](docs/os-integration.md): jump lists, the notification area, OS notifications, and opening URLs.
- [Translations](docs/translations.md): languages and translated strings.
- [Form widgets](docs/widgets-forms.md), [selection and date widgets](docs/widgets-selection.md),
  [navigation widgets](docs/widgets-navigation.md), [feedback widgets](docs/widgets-feedback.md),
  [data widgets](docs/widgets-data.md), and [overlays and menus](docs/widgets-overlays.md).
- [Charts](docs/charts.md), the [code editor](docs/codeedit.md), and the [Markdown view](docs/markdown.md).
- [Accessibility](docs/accessibility.md): the guide for app authors. The design and the decision log are in
  [docs/accessibility-design.md](docs/accessibility-design.md), and the checks by hand are in
  [docs/accessibility-testing.md](docs/accessibility-testing.md).
- [Tools](docs/tools.md): `jamb dev`, the visual inspector, environment variables, tracing, scripted runs,
  headless testing, and accessibility checks.
- [Packaging](docs/packaging.md): `jamb package`, `jamb.json`, the installer, MSIX, and signing.
- [Architecture](docs/architecture.md): the repository layout, the rendering pipeline, and GPU device loss.
- [Comparison with GPUI](docs/comparison-gpui.md): the hello-world app built with jamb and with GPUI.
- [Status and known limitations](docs/status.md): the milestone status and the known limitations.

## Contributing

Development needs Go 1.27.1 or later and Windows. Run the headless tests with:

```
gofmt -l .
go vet ./... ./cmd/jamb/... ./examples/demo/... ./examples/notes/... ./examples/hello/...
go test ./... ./cmd/jamb/... ./examples/demo/... ./examples/notes/... ./examples/hello/...
```

`gofmt -l .` should print nothing. The `go test` runs need no display. A few tests open a window; set
`JAMB_LIVE_TESTS=1` to run them as well. The module layout, the packages, and the internals are in
[docs/architecture.md](docs/architecture.md).

[CONTRIBUTING.md](CONTRIBUTING.md) covers issues, pull requests and the rules the code follows. Report
security problems privately, as [SECURITY.md](SECURITY.md) describes.

## Changelog

Release notes are in [CHANGELOG.md](CHANGELOG.md).

## License

jamb is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

An app built with jamb ships third-party code too. The list of notices to include with a distributed
app is in [Licenses of what your app ships](docs/packaging.md#licenses-of-what-your-app-ships).
