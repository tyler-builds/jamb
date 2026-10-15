# Getting started

What jamb needs, how to run the bundled examples, and how to add jamb to an app.

## Quick start

Windows 10+ and Go 1.27+. Nothing else.

```
go run ./examples/demo                         # run the demo
go run ./examples/notes                        # run the notes app
go run ./cmd/jamb dev ./examples/notes         # rebuild + restart on every save, window position preserved
go run ./cmd/jamb package --installer ./examples/demo   # Windows zip and setup.exe in examples/demo/dist (see Packaging)
go test ./... ./cmd/jamb/... ./examples/demo/... ./examples/notes/...   # headless tests; JAMB_LIVE_TESTS=1 adds the few that show a window
go build -trimpath -ldflags "-s -w -H windowsgui" -o jamb-demo.exe ./examples/demo   # ~17 MB release build
```

Results against the performance targets are in [BENCHMARKS.md](../BENCHMARKS.md):

- Builds take ~0.3–0.8 s after an edit.
- Startup to the first frame is ~200 ms.
- An idle window draws nothing. The demo used no CPU over 10 s; the hello app in the GPUI
  comparison used 0.16–0.29% of a core, mostly in the graphics driver's own thread.
- A full re-render costs ~0.3 ms of CPU per frame.

**Compared with GPUI:** [docs/comparison-gpui.md](comparison-gpui.md) measures the same hello-world app built with jamb and with GPUI (Zed's Rust UI framework): project size, binary size, build times, startup, idle CPU and memory.

## Using jamb in your app

The module is `github.com/tyler-builds/jamb`, and the latest release is `v0.2.0`. A minimal
`main.go` is a window with a greeting and a button that counts its clicks:

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

In the module that holds it, add jamb and run the app. `go get` on a module path records only
that module's checksum, so `go mod tidy` adds the checksums of jamb's own dependencies:

```
go get github.com/tyler-builds/jamb@v0.2.0
go mod tidy
go run .
```

Start with the [API guide](concepts.md#api-guide). The `jamb` command is its own module, tagged `cmd/jamb/v0.2.0`
for the same release, and it is installed with:

```
go install github.com/tyler-builds/jamb/cmd/jamb@v0.2.0
```

- `jamb dev ./myapp` rebuilds and restarts the app on each save, keeping its window position (see
  [Tools for humans and coding agents](tools.md#tools-for-humans-and-coding-agents)).
- `jamb package` builds the Windows zip, installer and MSIX (see [Packaging](packaging.md#packaging)).

An app that uses `codeedit` also needs the grammars module, whose latest release is `v0.1.0`:

```
go get github.com/tyler-builds/jamb-grammars@v0.1.0
go mod tidy
```

See [Code editor](codeedit.md#code-editor) for the grammars and what `codeedit` adds to a binary.
