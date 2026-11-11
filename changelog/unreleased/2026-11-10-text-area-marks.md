### Added

- `TextAreaElement.Marks` styles a text area's text as it is edited, from `ui.TextMark`s that the app
  computes for each value. A mark can be syntax, such as the `**` around bold text, which hides
  unless the caret or selection touches its construct, so text reads as formatted until the caret
  comes to edit it. The value keeps the syntax: copy, undo and assistive technology see the text as
  typed.
- `markdown.EditMarks` gives those marks for Markdown: bold, italic, strikethrough, inline code and
  links, with their syntax hidden away from the caret, and headings and code blocks, whose syntax
  stays in view.
