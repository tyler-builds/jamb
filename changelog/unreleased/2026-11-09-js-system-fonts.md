### Fixed

- A `js` build no longer reads system font files. Under Node it read the host's, so the same build
  drew with different fonts there than in a browser; it now uses the embedded Go fonts and the fonts
  the app registers everywhere.
