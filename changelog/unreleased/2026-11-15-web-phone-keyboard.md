### Added

- `TextInput.Purpose` and `TextArea.Purpose` say what a text field holds, for a phone's keyboard:
  prose (`ui.PurposeText`, the default), which it corrects and capitalizes, text typed as it is
  (`PurposeVerbatim`), an email address, a web address, a number or a phone number, or a search.

### Fixed

- In a web page, a phone's keyboard corrects words, capitalizes sentences, suggests and completes
  words, types a period for a double space, expands text replacements and takes dictation in a
  text field, and its cursor movement, as a long press of an iPhone's space bar, moves the
  field's caret. The page's hidden textarea now holds the field's text around the caret, with its
  selection, and its edits reach the field as edits of its text. A masked field, a field with a
  format and a code editor still correct nothing. A page needs the `jamb.js` of this version.
