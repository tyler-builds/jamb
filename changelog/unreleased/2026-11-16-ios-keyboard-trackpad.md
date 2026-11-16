### Fixed

- On an iPhone or an iPad, the keyboard's trackpad (a long press of the space bar) moves a text
  field's caret, along a line and from line to line. It used to put the caret at the start of
  the text and leave it there: iOS finds the caret's place at the finger's point in the focused
  textarea, and jamb's hidden textarea was a pixel wide and took no touches, so the point fell on
  the canvas. The textarea now lies over the field's text, laid out as the field lays it out, and
  takes the touches there for the app. A page must load the `jamb.js` of this version.
