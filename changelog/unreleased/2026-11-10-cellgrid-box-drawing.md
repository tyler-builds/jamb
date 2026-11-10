### Fixed

- A `CellGrid` draws box drawing (U+2500 to U+2570, U+2574 to U+257F), block elements (U+2580 to
  U+259F) and braille (U+2800 to U+28FF) as shapes that fill their cells, as terminals do, so the
  lines of frames, tables and graphs no longer break between rows where the cell is taller than the
  font's glyphs. Rounded corners are drawn as arcs, and the diagonals are left to the font.
