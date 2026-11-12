### Fixed

- A `TextArea` with a `Height` or `MaxHeight`, or one that grows into a bounded room, scrolls its
  text when the text is taller than the box, instead of drawing it past the box and over what comes
  after. It clips the text to its frame, scrolls with the wheel, shows a scroll bar, and keeps the
  caret in view as it moves, inside a page that scrolls too. A text area with no height bound still
  grows with its text.
