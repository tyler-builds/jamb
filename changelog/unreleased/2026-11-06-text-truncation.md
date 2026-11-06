### Added

- `TruncateMiddle()` keeps both ends of a one-line text that does not fit, with "…" in place of the
  middle, for file paths and names, and `TruncateHead()` keeps its end. The cuts fall between
  characters and refit as the text resizes; selecting, copying and the accessible name keep the whole
  text. The last of `Truncate`, `TruncateMiddle`, `TruncateHead` and `MaxLines` wins. The git example
  shows file paths cut in the middle.
