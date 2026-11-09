# Web

A jamb app can run in a web page. This is experimental: the page draws on the CPU, and text
input methods, the system clipboard, file dialogs and message boxes are not there yet.

## Building an app for the web

`scripts/web/build.sh` builds an app as a page:

```
sh scripts/web/build.sh examples/hello . site/hello
python3 -m http.server -d site/hello
```

The first argument is the app's module directory, the second its package, and the third the
directory for the page: `index.html`, which loads the app (`app.wasm`) with `wasm_exec.js`.
Serve it with any static file server; one that sends `.wasm` files as `application/wasm`
lets the browser compile the app while it downloads.

The build uses TinyGo, which runs from its official image in docker or podman (see
`scripts/web/tinygo.sh` for its settings). `-go` before the arguments builds with Go
instead, which is quicker to build and makes a larger, slower page.

An app's code does not change for the web: `app.Run` opens its window as a canvas that fills
the page, or in a `<canvas id="jamb">` when the page has one.

## What a page does

- **Window.** A page holds one window. It is as large as the page or its canvas, and it does
  not minimize, maximize, move or close itself, so a title bar the app draws shows no caption
  buttons. `Window.OpenURL` opens a new tab.
- **Input.** The mouse and touchpad (clicks, double clicks, the wheel), the keyboard (keys by
  their position, and the text they type), and touch: a tap is a click and a drag scrolls.
  The browser keeps its own shortcuts, such as reloading the page or closing the tab.
- **Settings.** The page follows the system's dark mode, reduced motion and forced colors.
- **Fonts.** The embedded Go fonts and the fonts the app registers with `App.Font`; a page
  reads no font files.
- **Frames** are paced by the browser's display frames, so a page in a background tab draws
  nothing.
- **Panics** end the app, and the page says so: `recover` does not catch a panic in a TinyGo
  build.

## Not there yet

Input methods (IME), the system clipboard (copy and paste work within the app), file dialogs,
message boxes, second windows, pinch zoom, and drawing on the GPU.
