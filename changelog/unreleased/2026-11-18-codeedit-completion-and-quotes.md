### Added

- `codeedit.CompletionList.KeepOrder` keeps a provider's ranking: typing filters the items but no
  longer puts the ones that start with the typed word first.

### Changed

- A completion list that opened on a trigger character closes when nothing matches, rather than
  showing "No results"; a list opened with Ctrl+Space still says so.

### Fixed

- The code editor's completion list closes when a space, a comma, a parenthesis or another
  character that ends the word leaves nothing matching, instead of staying open with "No results"
  while the next word is typed.
- A completion item's edit that inserts text at the start of the replaced word, such as `dbo.`
  before a table's name, is made before the item's text instead of being dropped.
- Typing a quote in the code editor steps over the same quote right after the caret, so typing
  `'abc'` gives `'abc'` rather than `'abc''`.
