### Changed

- A `DataGrid` that renders again keeps the rows that did not change instead of building every row
  in view: a row with the same text in every cell, in the same state (selected, holding the cursor,
  open), keeps its elements and only takes its new position. A click, a move of the cursor, a new
  selection or a new slice of rows builds the rows it changed. Inserting and removing a row among
  10,000 takes less than half the time it did, and moving one less than a third. A grid whose
  columns have `Cell`, `Edit`, `Editor` or `Interactive`, or that selects cells, builds every row as
  before.
