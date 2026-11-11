### Fixed

- `ui.Announce` is spoken even when nothing on screen changes with it, such as a chart's repeat of
  the point that a keyboard walk past its end stays on. It asks for a frame, and the frame publishes
  the announcement with the accessibility tree as it was; before, it waited for the next change.
