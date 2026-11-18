### Added

- `Tabs.Small()` and `Tabs.Large()` size the tabs as `Button.Small` and `Button.Large` size a
  button, as a `ToggleGroup`'s sizes do.
- `TabsStyle.Root`, the column of the tab list and the panel. Its `Gap` is the space between them,
  12 px by default; `Root: ui.Style{Clear: ui.ClearGap}` puts the panel right under the tabs.

### Changed

- The segmented `Tabs` track is 34 px high by default, as tall as a `Button`; it was 36 px.
- While a `Tabs` shows its bar (a prefix, a suffix, the scroll buttons or the overflow menu), the
  line under the tabs of `TabsUnderline` is the bar's bottom border rather than the strip's, so it
  also runs under the bar's buttons. A border set on `Bar` no longer doubles up with the strip's,
  and `List: {Clear: ui.ClearBorder}` is no longer needed with it.
- A `DropdownMenu` is laid out as its trigger would be: its parent aligns it, rather than it always
  going to the top, and in a column it keeps the trigger's width.

### Fixed

- Shift+F10 or the Menu key on a `Tree`, `ListBox` or `DataGrid` opens the context menu below the
  selected row or the focused cell, named after it, as Windows places a menu opened from the
  keyboard. It opened at the bottom of the whole control, named "Actions" or after the control.
- `Tree.RenderRow` content keeps its width beside a node's description, so its text no longer wraps
  and rows no longer overlap. The description grows from no width (`Basis` 0) and takes the room
  left. Row content is a single line: content wider than the tree is clipped.
- The overflow menu's button of `Tabs` sits on the middle of the bar, as a `DropdownMenu` in any
  centered row now does; it was 4.5 px too high.
