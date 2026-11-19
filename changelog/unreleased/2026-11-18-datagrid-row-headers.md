### Added

- `DataGrid.RowHeaders` shows a header with the row's number at the start of each row, and a corner that selects every cell. A click on a row's header selects the row, Ctrl+click adds or removes it, Shift+click extends, and a drag down the headers selects the rows it crosses. The keys reach the headers: Left from the first column, then Space, Shift+Up and Shift+Down; Up from the first row's header reaches the corner. Row headers are header items of their rows for assistive technology.
- `DataGrid.HeaderSelects` makes a click on a column's header select the column in a grid that selects cells, with Ctrl, Shift and drags as for the rows; the sort moves to the header's arrow and to Enter. The default is unchanged: a header click sorts.
- A drag over the cells of a grid that selects cells selects the rectangle it covers. A drag of a selection (over the cells, down the row headers or across the column headers) held past an edge of the rows in view scrolls the rows or columns toward the pointer and selects what comes into view.
- `DataGridStyle` has the parts `RowHeader`, `RowHeaderSelected`, `HeaderCellSelected` and `Corner`.
- `AccessCell(row, 0)` on a header item makes it the header of the row, which the Windows Table pattern reports as a row header.
