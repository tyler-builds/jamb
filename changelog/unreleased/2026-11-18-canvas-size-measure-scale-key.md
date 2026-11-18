### Added

- `Div.OnResize(fn)` reports the size of an element's content box when it is first laid out and
  each time it changes. What `fn` sets is built and laid out before the frame is painted, so a view
  drawn on a canvas never shows the rows of the size before.
- `ctx.MeasureText(s, format)` measures canvas text in a render, with the same result as
  `Painter.MeasureText`, to size a grid's columns before it is drawn.
- `Painter.PixelScale()` and `Painter.Snap(x, y)` draw lines that are crisp at any display scale:
  docs/drawing.md shows a 1 px line that stays on one row of pixels at 125%.
- `Div.CanvasKey(key)` keeps a canvas's drawing while the key is unchanged, instead of drawing
  again on each render of its component. It panics on a key that cannot be compared.

### Changed

- `Painter.Clip` clips canvas text as well as shapes, to the clip's bounding box, so the text of a
  grid's cell stays in the cell. Text is still drawn over all the canvas's shapes.
- A canvas draws at the display scale in both directions. At a scale such as 125%, `draw` gets
  the content size rounded to whole display pixels, and the drawing is no longer stretched by up
  to a pixel to fill them, so its pixels line up with the rest of the window's.
