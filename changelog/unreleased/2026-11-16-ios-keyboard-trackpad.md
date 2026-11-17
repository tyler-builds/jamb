### Fixed

- On an iPhone or an iPad, the keyboard's trackpad (a long press of the space bar) moves a text
  field's caret, along a line and from line to line. It used to put the caret at the start of
  the text and leave it there: iOS finds the caret's place at the finger's point in the focused
  textarea, and jamb's hidden textarea was a pixel wide and took no touches, so the point fell on
  the canvas. An editable element now takes the textarea's place on iOS, lies over the field's
  text and takes the touches there for the app. It is built from the field's own layout: each
  piece in the face the field draws it in, from the font files jamb holds, with the field's
  kerning, hidden Markdown syntax at no width, tabs at their width and lines that wrap where the
  field's do, so the caret lands on the character jamb draws under the finger, in styled text as
  in plain. Autocorrect, suggestions, the double-space period, dictation, composition and paste
  go through it as they went through the textarea. A page must load the `jamb.js` of this
  version; a page without it keeps the textarea at the caret.
