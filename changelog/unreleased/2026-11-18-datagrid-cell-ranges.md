### Added

- `DataGrid.CellRanges` keeps a grid's cell selection as blocks of cells (`ui.CellRange`, `ui.KeySet`): whole rows, whole columns and rectangles together. Ctrl+A, Ctrl+Space and Shift+Space add one range whatever the number of rows, so Ctrl+A on 100,000 rows of 4 columns takes 0.4 ms and 0.4 MB, where it took 49 ms and 38 MB.
- Ctrl+Shift with a click or a move extends a cell selection from the anchor and keeps the rest of it, and Shift extends whole columns after Ctrl+Space and whole rows after Shift+Space.

### Changed

- `DataGrid.CellSelection` is now filled from the grid's ranges, and can be given with `CellRanges`: a value the app sets in either selects those cells, and the grid sets the other to match. Ctrl+A no longer builds a set of every cell for the grid's own use, so it is about 9 times faster with `CellSelection` too (49 to 5.7 ms on 100,000 rows of 4 columns).
