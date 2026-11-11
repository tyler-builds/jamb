### Changed

- Text that is drawn again where it was, as in each frame of a vertical scroll, draws faster. Each
  text keeps its layout and where its glyphs are in the glyph atlas from its last paint, and draws
  from them without looking each glyph up. Scrolling a 10,000-row DataGrid by a few pixels went
  from 147 µs to 72 µs a frame, and changing one label among 3,000 rows from 3.3 ms to 1.9 ms.
  Text that moves sideways draws as before.
