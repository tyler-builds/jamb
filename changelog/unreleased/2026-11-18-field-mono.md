### Added

- `TextInputElement.Mono` and `TextAreaElement.Mono` show a field's text in the monospace
  font, as `TextElement.Mono` does. Each is the same as
  `Styles(ui.FieldStyle{Text: ui.Style{FontFamily: "mono"}})`.

### Fixed

- docs/codeedit.md's example of binding Ctrl+K in the editor now unbinds the editor's
  Ctrl+K Ctrl+I chord first. Without that, Ctrl+K waits up to a second for the chord's
  second key; docs/input.md says so where it explains chords.
