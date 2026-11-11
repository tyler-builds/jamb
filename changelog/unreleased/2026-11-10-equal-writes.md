### Added

- `Signal.EqualFunc` and `Computed.EqualFunc` set how a new value is compared with the current one,
  such as `slices.Equal` for a slice of strings.

### Changed

- A write that leaves a signal's value as it was re-renders nothing: `Signal.Set` and
  `Signal.Update` compare the new value with the current one by `==`, and skip the readers when they
  are the same. Values that `==` cannot compare (slices, maps, functions, and structs holding them)
  still notify on every write. Setting the same pointer again no longer re-renders the readers; build
  a new value instead of changing one in place, or give the signal an `EqualFunc` that returns false.
- A `Computed` re-renders its readers only when its new result differs from the last one. When a
  signal it read changes, its readers learn that it may have changed, and before the next frame it
  computes again and re-renders them only if the result changed. Rows that read "is this row
  selected" through a Computed each now re-render two rows when the selection moves, not all of
  them. In a list of 3,000 rows, writing a row's label with the value it has went from 4.3 ms to
  90 ns, and moving the selection from 69 ms to 33 ms.
