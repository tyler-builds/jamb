### Added

- An app runs in a web page, experimentally: a js build opens its window as a canvas that fills the
  page (or `<canvas id="jamb">`), draws on the CPU, and takes the page's mouse, wheel, keyboard and
  touch input and its dark mode, reduced motion and forced colors settings. `scripts/web/build.sh`
  builds an app as a page, with TinyGo or with Go. Input methods, the system clipboard and file
  dialogs are not there yet (see docs/web.md).
