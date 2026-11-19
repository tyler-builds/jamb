### Fixed

- A text area that holds a large text, such as a paste of a few hundred kilobytes of Markdown, no
  longer runs a web page out of memory or stops the app for seconds at each key. The text system
  keeps at most about a megabyte of text in the layouts and paragraphs it caches, beyond those of
  the last frames, where it kept a layout of all of the text for each edit of the last 240 frames.
  A text area draws only the lines in view, lays its text out once for each edit, and styles its
  marks in time that grows with the text's length rather than with its square; `markdown.EditMarks`
  reads a text once for the functions made for it in successive builds of the UI.
