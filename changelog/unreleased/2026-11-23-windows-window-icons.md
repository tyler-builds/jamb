### Added

- `app.Window.Icon` gives a window an icon from an image on Windows, drawn at each size Windows
  shows. Nil uses the executable's icon.

### Fixed

- On Windows, every window now has an icon: in its title bar, on its taskbar button and the
  button's thumbnail, and in Alt+Tab. Windows had none, so the taskbar showed a blurry, stretched
  fallback and the thumbnail's title row no icon. The small and big icons come from the icon that
  `jamb package` embeds, each at the exact size of the window's DPI, and are made again when the
  window moves to a monitor with another scale. A program without an icon resource runs as before.
- `jamb package` embeds the icon at the in-between sizes too (20, 28, 36, 40, 56, 72, 80 and 96
  px), so no display scale up to 300% stretches it.
