### Added

- Text fields, selectable text and `codeedit` editors have a context menu: Undo, Redo, Cut, Copy,
  Paste and Select all (Copy and Select all for read-only text). A right click focuses the text and
  keeps a selection it lands in, or else moves the caret to it.
- `Segment.ContextMenu` gives a rich text segment, such as a link, its own context menu, which a
  right click opens, and Shift+F10 or the Menu key while the link has focus.
- `markdown.ViewElement.OnLinkMenu` gives every link of a Markdown view a context menu for its
  destination.

### Changed

- While a context menu has focus, the element it returns focus to no longer gets `OnBlur`; it runs
  when the menu closes, only if focus went elsewhere.
- A right press runs the `OnMouseDown` handlers under the pointer before the context menu opens,
  and presses inside an open context menu no longer reach the `OnMouseDown` of the element it hangs
  from.
