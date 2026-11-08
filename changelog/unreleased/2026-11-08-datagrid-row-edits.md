### Changed

- A `DataGrid` handles a change to its rows faster when it has many of them. It keeps a copy of the
  rows it made keys for, finds the rows a new slice changed by comparing it with that copy, and calls
  the key function for those rows alone. It no longer makes a table of every row's position on each
  change, and checks for a repeated key only among the rows that changed. Inserting and removing a
  row among 100,000 takes a quarter of the time it did. The copy costs memory as large as the rows,
  for rows of up to 128 bytes; larger rows are keyed again on each change, as before.
