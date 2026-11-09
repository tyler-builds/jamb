# Web

A jamb app can run in a web page. This is experimental: file dialogs and message boxes are not
there yet.

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

`scripts/web/browser.sh` checks the demo in Chromium, as CI does: it draws the same frame with
WebGL 2 as on the CPU, and its text fields take typing.

An app's code does not change for the web: `app.Run` opens its window as a canvas that fills
the page, or in a `<canvas id="jamb">` when the page has one.

## What a page does

- **Window.** A page holds one window. It is as large as the page or its canvas, and it does
  not minimize, maximize, move or close itself, so a title bar the app draws shows no caption
  buttons. `Window.OpenURL` opens a new tab.
- **Input.** The mouse and touchpad (clicks, double clicks, the wheel), the keyboard (keys by
  their position, and the text they type), and touch: a tap is a click, a drag scrolls with
  momentum, and a drag that starts on a slider, a split divider or another element that takes
  the drag moves it.
  The browser keeps its own shortcuts, such as reloading the page or closing the tab. On a
  Mac, iPhone or iPad, Command is Ctrl, so Command+C copies.
- **Text input.** While a text field has focus, a hidden textarea of the page has it too, at
  the field's caret: a phone shows its keyboard, and input methods (IME) compose in the field
  and put their window by the caret.
- **Clipboard.** Copying writes the system's clipboard, and pasting into a text field with
  Ctrl+V (or the browser's paste) reads it. A paste chosen in the app's own context menu, and
  Ctrl+V outside a text field, paste the text last copied in the app: a page reads the
  system's clipboard only in the browser's paste event.
- **Settings.** The page follows the system's dark mode, reduced motion and forced colors.
- **Drawing.** WebGL 2 draws the frames, with the shaders of the other GPU renderers. A browser
  without WebGL 2 draws them on the CPU, as `JAMB_RENDERER=cpu` asks for; a page has no
  environment, so it sets `window.jambEnv = {JAMB_RENDERER: "cpu"}` before it loads the app.
  When the browser takes the GPU away (a GPU reset, or too many pages using it), the app
  draws again once the browser gives it back, keeping its state.
- **Fonts.** The embedded Go fonts and the fonts the app registers with `App.Font`; a page
  reads no font files.
- **Frames** are paced by the browser's display frames, so a page in a background tab draws
  nothing.
- **Panics** end the app, and the page says so: `recover` does not catch a panic in a TinyGo
  build.

## Not there yet

File dialogs, message boxes, second windows, pinch zoom, and images on the clipboard.
