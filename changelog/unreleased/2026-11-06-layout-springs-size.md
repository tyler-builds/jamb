### Added

- A layout spring now springs an element's size too. When the layout resizes an element with
  `LayoutSpring` (its own, or from `ChildrenLayoutSpring`), its own box (background, border, shadows,
  corners and focus ring) springs to the new size, keeping its velocity when the size changes
  mid-flight. What it holds keeps its final layout and moves with it, so text never scales, and is
  clipped to the drawn box only when the element clips. Hit testing, `Find`, `DrawnStyle.Bounds` and
  the accessibility bounds follow the drawn box. It is paint-only, and snaps where a move does: on
  first appearance, a window resize, zoom, text scale and reduced motion. See Layout springs in
  docs/animation.md.
- The demo's Motion section has an expanding card sample.

### Changed

- `Collapsible` and `Accordion` spring open and closed, and `Sidebar` springs to its new width as it
  collapses and expands, with the theme's `Motion.Reflow`. The box clips its content while it moves,
  so the content is revealed and covered rather than popping in and out; what the widget removes is
  drawn inside the box, fading, until it comes to rest. With reduced motion they still open and close
  at once. Content still being revealed can only be clicked where it is drawn.
