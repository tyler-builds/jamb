### Changed

- Widget styles merge in place, through pointers, instead of as copies. A frame full of widgets
  builds about a third faster (`BenchmarkWidgetFrameNoInspector`: 1.99 to 1.34 ms), with the same
  allocations. Under TinyGo, which turns each copy of a large struct into code for every field, the
  change takes 1.2 MB of code out of a build, so its optimizer can run on jamb.
- `HighContrastTheme` and `Theme.Clone` copy the theme as a whole.
