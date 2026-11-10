### Added

- `ui.Style.MergeFrom`, `ui.StyleLayerRef` and `ui.ThemeStyleRef` do what `Style.Merge`,
  `ui.StyleLayer` and `ui.ThemeStyle` do, through pointers and without copying the style. A
  widget whose style is a struct of several `ui.Style` parts uses them to stay small in a web
  build (see "Styles of other widgets on a theme" in docs/theming.md).

### Changed

- `charts`, `markdown` and `codeedit` merge their styles through pointers. A web build of the
  demo, which uses all three, is 2.13 MB in Brotli, down from 2.31 MB.
