# jamb

A declarative, GPU-accelerated desktop UI framework for Go. Pure Go, no cgo; Windows first.

jamb is a small UI framework written in plain Go. It has no Electron, WebView, HTML, CSS, JavaScript,
Node, or Rust. On Windows it uses pure-Go Win32 windowing and a Direct3D 11 renderer reached through
COM vtables. An app is a tree of elements built by components, and its state lives in signals, so a
change re-renders only the parts that depend on it.

> **Status:** jamb is pre-1.0. The API may change between versions. It runs on Windows 10 and 11
> today; other platforms are planned.

## Highlights

- Widgets: buttons, text fields, checkboxes, switches, sliders, selects, comboboxes, date and time
  pickers, color pickers, menus, tabs, trees, data grids, forms with validation, toasts, and modal dialogs.
- Layout: flexbox, grid, sidebars, split panes, docking, and variable-height and uniform lists.
- Theming: light and dark themes that follow Windows, 37 converted predefined themes in 22 families, and
  automatic high contrast.
- Accessibility: a UI Automation tree for screen readers, keyboard navigation, reduced motion, and a
  text size setting.
- Text: shaping with HarfBuzz, fallback fonts, rich and selectable text, and input method (IME) composition.
- Graphics: a canvas, SVG images, gradients, and charts.
- Code editing: a `codeedit` package with tree-sitter highlighting, undo and redo, and find and replace.
- OS integration: jump lists, single instance, a notification area icon, taskbar progress, and OS notifications.
- Development tools: `jamb dev` rebuilds and restarts an app on each save, and a visual inspector shows the
  elements of a running window.
- Packaging: `jamb package` builds a Windows zip, an installer, and an MSIX.

## Install

jamb needs Windows 10 or 11 and Go 1.27.1 or later. The module is `github.com/tyler-builds/jamb`.
The repository is private for now, so Go must fetch it directly rather than through the public module
proxy, and git must be able to clone it:

```
go env -w GOPRIVATE=github.com/tyler-builds/*
go get github.com/tyler-builds/jamb@v0.1.0
```

A minimal `main.go` opens a window with a greeting and a button that counts its clicks:

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

The `jamb` command is a separate module, tagged `cmd/jamb/v0.1.0` for the same release:

```
go install github.com/tyler-builds/jamb/cmd/jamb@v0.1.0
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

These are headline numbers from [BENCHMARKS.md](BENCHMARKS.md) and
[docs/comparison-gpui.md](docs/comparison-gpui.md). Each row names the app it was measured on, and the
two documents give the dates, the method, and the full results. The GPUI column compares against
GPUI, the UI framework from Zed.

| Measure | jamb | GPUI | Notes |
|---|---|---|---|
| Build time after an edit | 2 s (hello app); demo: 250–370 ms for edits that keep the APIs, 550–840 ms for other edits | 4 s (release), 6 s (debug) (hello app) | Incremental builds. The demo figures are from BENCHMARKS.md. |
| Startup to first frame | 266 ms on a warm launch (hello app); about 195 ms for the demo | 387 ms on a warm launch (hello app) | Process start to first frame on screen. |
| Idle CPU | 0.29% of one core, focused (hello app); 0 ms of CPU in 10 s (demo) | 1.85% of one core, focused (hello app) | Median of the measurements. |
| Frame cost | About 0.3 ms per frame for a full re-render of every component; about 0.13 ms for a repaint of the retained tree (demo) | Not measured | CPU time for build, layout, paint, and GPU submit. |
| Binary size | 8.4 MB, stripped release build (hello app) | 9.21 MB, release build (hello app) | Built with `-trimpath -ldflags "-s -w"` for jamb. |

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
- [Charts](docs/charts.md) and the [code editor](docs/codeedit.md).
- [Accessibility](docs/accessibility.md): the guide for app authors. The design and the decision log are in
  [docs/accessibility-design.md](docs/accessibility-design.md), and the checks by hand are in
  [docs/accessibility-testing.md](docs/accessibility-testing.md).
- [Tools](docs/tools.md): `jamb dev`, the visual inspector, environment variables, tracing, scripted runs,
  headless testing, and accessibility checks.
- [Packaging](docs/packaging.md): `jamb package`, `jamb.json`, the installer, MSIX, and signing.
- [Architecture](docs/architecture.md): the repository layout, the rendering pipeline, and GPU device loss.
- [Comparison with GPUI](docs/comparison-gpui.md): the hello-world app built with jamb and with GPUI.
- [Status and known limitations](docs/status.md): the milestone status and the known limitations.

## Examples

The examples are in [examples/](examples/). `examples/hello` is the minimal app shown above.
`examples/demo` is a tour of the widgets and layouts. It is also the app that `jamb package` builds in
the packaging guide. `examples/notes` is a note-taking app with persistence, a command palette,
and a keymap file. [docs/examples.md](docs/examples.md) describes the demo and notes apps.

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

## Changelog

Release notes are in [CHANGELOG.md](CHANGELOG.md).

## License

jamb is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

An app built with jamb ships third-party code too. The list of notices to include with a distributed
app is in [Licenses of what your app ships](docs/packaging.md#licenses-of-what-your-app-ships).
