### Added

- `app.Window.Corners` chooses rounded, slightly rounded or square window corners. On Windows 11 it
  sets DWM's corner preference.
- Linux: on GNOME, which leaves window frames to apps, windows get rounded corners, a shadow that
  deepens while the window has the focus, and a thin outline, as GNOME's own apps do. The shadow is
  outside the window: snapping and placement leave it out, and the pointer resizes the window from
  just outside its edge. Maximized, fullscreen and snapped edges stay square.
