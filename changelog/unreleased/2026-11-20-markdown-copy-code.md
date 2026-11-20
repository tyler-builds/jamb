### Added

- `markdown.ViewElement.CopyCode(true)` gives each code block a copy button in its top right
  corner, which copies the block's code without the fence, the language or a trailing line break.
  Its icon turns into a check for a moment after a copy. It always shows, so a touch reaches it,
  and moves neither the block nor its text. It is off by default. The `CodeCopy` part of
  `markdown.Style` styles the box behind it, and `ui.Strings.MarkdownCopyCode` is its tooltip.
