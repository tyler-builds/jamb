### Added

- `uitest.UI.Record` records the frames a test runs, with how every element was drawn in each, and
  `Recording.Check` fails the test on motion a person would notice: an element that pops in or out
  with no transition, is cut off partway through its exit fade, jumps in one frame, overshoots, or
  keeps moving long after the last input. `Recording.Findings` only reports, and `uitest.Allow`
  accepts an intended finding with its reason. See the Motion section of docs/testing.md.
