### Added

- A `DataGrid` copies its selection: Ctrl+C (and Ctrl+Insert) puts it on the clipboard as tab-separated text that spreadsheets paste into cells, and Ctrl+Shift+C puts the columns' headers first. The rows and columns are copied as shown (moved, pinned, sorted and filtered), a selection that is not a rectangle keeps each cell under its column, and cells with tabs, line breaks or quotes are quoted. The keys are the actions `actions.GridCopy` and `actions.GridCopyWithHeaders` (`ui.DataGridActions`), bound in the `DataGrid` key context. They do not run while a control or an editor in a cell has focus (its cell has the context `DataGridCell`), so the control's own keys win. `DataColumn.CopyText`, `DataGrid.CopyText` and `DataGridHandle.Copy` change the text or copy from code.

### Changed

- A right-press on a `DataGrid` row focuses the grid before its context menu opens, so that the menu's actions run on the grid.
