### Changed

- A `DataGrid`'s selected rows and cells have a stronger default fill (`RowSelected`, `CellSelected`): the accent at 0.28 alpha on a dark theme and 0.20 on a light one, in place of `AccentSoft`. A selected range now stands at about 1.36:1 against the rows under it, where it was 1.2:1, while text keeps more than 10:1 and the focused cell's ring 3:1 on it. The headers of a row or a column selected whole are the solid accent with `AccentText`. A theme's `DataGrid` or an instance's `Styles` still replace them.
