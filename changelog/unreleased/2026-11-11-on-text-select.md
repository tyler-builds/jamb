### Added

- `DivElement.OnTextSelect` reports the text the user selects in a `Selectable` text inside the
  element, with where it is drawn relative to the element, as a `ui.TextSelection`, again as it
  scrolls, and with the zero value once it is cleared, loses focus or scrolls out of view. What the
  function changes is drawn in the same frame, so a control placed by the selection, such as a
  "Quote" button, follows it.
- `Context.ClearTextSelection` clears the selection of the focused selectable text.
