### Added

- `DivElement.LayoutSpring` and `Style.LayoutSpring` move an element to a new place in the layout as a
  spring: a list reordered, a row added or removed above it, a parent that grew. The element keeps
  where it is drawn and its velocity, so a move mid-flight bends the motion. It is paint-only: nothing
  is laid out again while it moves, and hit testing and `Find` follow it. A scroll, a window resize,
  zoom, text scale and reduced motion never make it spring, and a change of size is not animated.
  `DivElement.ChildrenLayoutSpring` gives the spring to a parent's direct children. Debug builds warn
  about a springing element with no key. See Layout springs in docs/animation.md.
- `MotionTokens.Reflow` is the layout spring of the built-in widgets, 350 ms with no bounce by
  default, and `MotionTokens.ReflowSpring` returns it for an app's own lists.

### Fixed

- When a toast leaves, the toasts that stay glide to their new places instead of jumping there in one
  frame, and a new toast appears in its place while the others move up to make room.
