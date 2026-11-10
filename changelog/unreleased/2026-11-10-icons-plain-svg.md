### Changed

- An app that draws the icons of package `icons`, and parses no SVG of its own with `icons.New`
  or `ui.SVGFromBytes`, no longer links the XML decoder: the icons are written in a plain form
  that a few lines of code read. The smallest web app is 947 KB in Brotli, down from 958 KB.
