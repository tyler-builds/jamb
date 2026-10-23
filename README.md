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
go get github.com/tyler-builds/jamb@v0.4.0
go mod tidy    # records the checksums of jamb's own dependencies
go run .
```

The `jamb` command is a separate module, whose latest release is `cmd/jamb/v0.4.0`:

```
go install github.com/tyler-builds/jamb/cmd/jamb@v0.4.0
```

The repository includes five examples. From a clone of the repository, run them with:

```
go run ./examples/hello    # the smallest complete app: one window, a signal and a button
go run ./examples/demo     # a tour of the widgets, layouts, and pages
go run ./examples/notes    # a note-taking app
go run ./examples/git      # a Git history browser with a commit graph
go run ./examples/monitor  # a system monitor with live charts and a process table
```

The [getting started guide](docs/getting-started.md) covers the commands for building and packaging,
and the [concepts](docs/concepts.md) page introduces the API.

## Benchmarks

The same hello-world app (a window, a greeting and a click counter) built with jamb and with GPUI, the
UI framework from Zed, on one Windows machine. The figures are from 2026-10-23 (the v0.4.0 release), except
the rows marked 2026-10-12. Lower is better in every row, and the better figure is in bold. The windows were on a
secondary 144 Hz monitor, never activated. The machine was not dedicated to benchmarking: other
applications were running. The [docs/comparison-gpui.md](docs/comparison-gpui.md) has the method, the ranges of each measurement and
the setup, and the earlier passes.

| Measure | jamb | GPUI |
|---|---|---|
| Startup to first frame, warm launch | **229 ms** | 337 ms |
| Startup to first frame, first launch (scanned by the antivirus) | **552.5 ms** | 587.3 ms |
| Window visible, first launch | **538.2 ms** | 552.3 ms |
| Idle CPU, window in the background, 60 s | **0.05%** of one core | 1.48% |
| Idle context switches per second | **71** | 426 |
| Memory, private bytes | 76.1 MB | **73.2 MB** |
| Memory, working set | 56.0 MB | **49.5 MB** |
| GPU memory, dedicated | **13.7 MB** | 15.5 MB |
| Threads | 39 | **32** |
| Handles | 483 | **462** |
| CPU while the pointer moves over the window | **0.8%** | 4.2% |
| CPU while the window is resized | **7.8%** | 8.6% |
| Context switches per second while resizing | 1,932 | **1,520** |
| Build after an edit (release, text change) | **1.45 s** | 3.13 s (release), 4.27 s (debug) |
| Clean build (2026-10-12) | **9 s** | 138 s (release), 100 s (debug) |
| Binary size, release build | **8.25 MB** | 9.22 MB |
| Binary size, GPUI's size-optimized build profile (2026-10-12) | | **4.31 MB** |

Against 2026-10-22, jamb's resize CPU rose from 2.5% to 7.8%, and its context switches while resizing
from 1,210 to 1,932 a second, above GPUI's. v0.4.0 keeps the swap chain's spare room only while the user
drags the window's edge, which fixed a window shown zoomed in after a maximize and a restore, and this
benchmark resizes the window from code, not by a drag. Both apps started more slowly than on 2026-10-22
(jamb 215 ms and GPUI 296 ms warm then), which points to a busier machine; the pointer CPU, which varies
from run to run, was 1.4% for jamb and 2.5% for GPUI then. jamb's binary grew by 0.23 MB: v0.4.0's
Lucide icons link an SVG engine. The focused idle CPU was not measured on 2026-10-21, 2026-10-22 or
2026-10-23. On 2026-10-12 the hello app used 0.29% of a core with the window focused, and GPUI 1.85%.
jamb's frames on the secondary monitor followed the primary monitor's refresh rate, a known issue listed
in [BENCHMARKS.md](BENCHMARKS.md).

The demo app (four pages), from [BENCHMARKS.md](BENCHMARKS.md), measured on 2026-10-22 unless the row says
otherwise. The figures in the middle column are the 2026-10-22 results, with the 2026-10-21 figure in
brackets where it changed:

| Measure | Result | What it means |
|---|---|---|
| Startup to first frame, warm | 221 ms (261 ms to the first painted frame) [215 ms, 252 ms] | About 180 ms of it is the graphics driver creating the Direct3D device (2026-10-12). Both figures are higher than on 2026-10-21, which is not explained. |
| Idle CPU, unfocused, 60 s | 0.08% of one core [0.13%] | An idle window draws nothing. About 66 context switches a second; the 2026-10-12 analysis traced most of an idle window's wake-ups to the graphics driver's thread. |
| Full re-render of every component | 0.81 ms of CPU per frame on average, 165 FPS at vsync, on a 310-element page [0.97 ms, 164 FPS] | A full re-render is a stress case; normal updates are incremental. It allocates 465 KB a frame [936 KB], still above the 110 KB of 2026-10-06. |
| One key press in a 10,000-node tree | 0.47 µs [273 µs] | The top modal is cached per build, so a key press no longer searches the tree. |
| One signal change in a 10,000-element tree | 0.32 ms [0.31 ms] | Only the component that read the signal renders again. |
| One scroll step in a 100,000-row list | 0.15 ms in a `UniformList`, 0.07 ms in a `ListOf` [0.15 ms] | Only the rows in view are built. |
| Expanded tree, 10,100 rows, one frame | 2.0 ms [80.5 ms] | A tree or data grid without a height shows the rows that fit its parent. |
| Page switch, first visit (click to Present, p95) | 37.7 ms [26.7 ms] | The first visit to a page builds it. This got worse in the live run, which is not explained. |
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
and a keymap file. `examples/git` browses the history of Git repositories, with a commit graph, branches,
tags and diffs. `examples/monitor` shows the machine's load, memory, network and processes as they
change, in panels that can be rearranged and floated. [docs/examples.md](docs/examples.md) describes
the demo, notes, Git and monitor apps.

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
go vet ./... ./cmd/jamb/... ./examples/demo/... ./examples/notes/... ./examples/hello/... ./examples/git/... ./examples/monitor/...
go test ./... ./cmd/jamb/... ./examples/demo/... ./examples/notes/... ./examples/hello/... ./examples/git/... ./examples/monitor/...
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
