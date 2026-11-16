### Added

- `DivElement.SelectAcross` makes the selectable texts inside an element one selection. A drag
  selects across them, Shift+click extends into another text, `Ctrl+A` selects all of them, and
  `Ctrl+C` copies the whole selection, with blank lines between blocks, line breaks between list
  items and table rows, and tabs between cells. Dragging past the edge of a scroll container
  scrolls it. `OnTextSelect` reports the whole selection.

### Changed

- A `markdown.View` is one selection: a drag selects across its paragraphs, headings, lists, quotes,
  code blocks and tables, where it used to stop at the edge of the block it started in.
