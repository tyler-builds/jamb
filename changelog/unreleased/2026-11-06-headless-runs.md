### Added

- Headless runs: `JAMB_HEADLESS=1` runs an app with no window on the screen. Its windows exist in
  memory and the CPU renderer draws them, so nothing takes the focus or appears on the desktop, and
  `JAMB_SCRIPT`, scenes, the control channel and `jamb dev` work as in a window. A headless run uses
  the default platform settings and touches nothing of the system's: the clipboard is the process's
  own, and no dialog, tray icon, notification or jump list is shown or registered.
  `JAMB_HEADLESS_SCALE` sets its scale. See docs/tools.md, "Headless runs".
- `JAMB_SCRIPT` steps aim at an element by key with `#KEY` in place of `X Y` (`click #save`), and
  the `tree PATH` step writes the accessibility tree as JSON.

### Fixed

- `jamb dev` stops the app when it gets SIGTERM, as it does on Ctrl+C; the app kept running.
