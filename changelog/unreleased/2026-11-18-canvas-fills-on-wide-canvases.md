### Fixed

- Canvas fills no longer get slower as the canvas gets wider. A fill works only on the pixels its
  shape covers, and a rectangle with sides along the axes, the shape a grid draws most, is filled
  faster still: 400 small cells take about the same time on a canvas 200 or 3,200 pixels wide, and
  a 1,600 by 900 grid redraws in about 2 ms rather than 50. `Painter.Clip` no longer allocates a
  mask the size of the canvas on each call; it keeps only the clip's bounds and reuses the memory.
  What is drawn does not change.
