### Changed

- A web page loads `jamb.js`, the host script that `build.sh` writes beside the app, and gives it
  the app's instance (`jambHost.attach(result.instance)`) before it runs the app. The script
  delivers the display's frames and the pointer, wheel, key and text events by calling functions
  the app exports. A page that does not load it still runs, and says so in the console.

### Fixed

- A web page no longer pauses to collect memory every few frames while the pointer moves, the
  page scrolls or text is typed. Each call from the page into Go through `syscall/js` made
  references to JavaScript values, and TinyGo collects after every 32 of them, for a time in
  proportion to the app's memory. In the demo, a frame of scrolling the Components page took
  8.8 ms of work on average, with a pause of over 50 ms every seventh frame; it now takes 1 ms,
  with none over 3 ms.
