### Changed

- A `ProgressRing` (and a loading button's ring) keeps turning under reduced motion, at half its
  speed, such as when Windows' Animation effects are off. Its turning is essential motion: drawn
  still, with every dot dimmed, it read as a static glyph rather than a busy indicator. Only the
  dots' opacity changes, so nothing moves across the screen.
