### Fixed

- A `DataGrid`'s header takes the pointer where it is drawn once the rows have scrolled down by more
  than its height. A click on a header cell sorts again, its edge resizes and fits the column, a
  movable header can be dragged, and a right-press opens the header's menu, rather than the press
  going to the row scrolled under the header. The header is no longer culled on such a scroll, and
  the access tree reports the header items at their places on screen rather than off screen.
