### Changed

- A `Modal`, `Drawer` or `AlertDialog` can be declared anywhere in the tree, such as next to the
  button that opens it, rather than only as the last child of the root. Its backdrop is now a
  window-level overlay: it covers the whole window and is drawn above everything else, the panel is
  centered in the window, a press outside the panel reaches only the backdrop, and the dialog is a
  child of the window in the accessibility tree, after the app's other content. A non-modal dialog is placed in the window the same
  way. Of two open modals, the one opened last is on top.

### Fixed

- A press in a modal no longer closes another modal open under it.
