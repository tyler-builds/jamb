### Fixed

- The selection of selectable rich text shows over a segment with a background, such as inline code
  in a `markdown.View`, rather than being hidden under it. The selection of a text field shows over
  the background of a mark in the same way. Backgrounds are drawn first, then the selection, then
  the text, as the code editor draws them.
