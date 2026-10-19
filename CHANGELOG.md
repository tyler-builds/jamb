# Changelog

All notable changes to jamb are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
version numbers follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- A `markdown` package: `markdown.View(src)` shows CommonMark with GitHub's tables, task lists,
  strikethrough, autolinks and footnotes, parsed with goldmark. Headings, links, lists and task
  states are accessible, text is selectable per block, and raw HTML is shown as text. Links open with
  the system or call `OnLink`; images come from an app's loader or local files under `BaseDir`, and
  the view never downloads anything. It is styled through `markdown.Style`, a theme slot and
  `Styles`. The guide is [docs/markdown.md](docs/markdown.md).
- `markdown/mdcode` colors fenced code blocks with the code editor's tree-sitter grammars. It is a
  separate package, so an app that does not import it links neither the editor nor wazero.
- Markdown tables are data grids that screen readers navigate by row and column, with the header row
  as their column headers, and list items report their position ("2 of 5") and level.
- `AccessGrid`, `AccessCell`, `AccessPosition` and `AccessLevel` on `Div` let any element describe a
  table, a cell, or an item's place in a set and its nesting level to assistive technology.
- `Tabs.OnReselect` runs when the selected tab is activated again, and `Tab.Popup` shows any
  content the app builds in a floating panel under the tab. It opens on a second click on the
  selected tab, or with Down, Shift+F10 or the Menu key, and closes on Escape, a click outside or
  its `close` func.
- `Tabs.Reorderable` lets the user move tabs by dragging them or with Ctrl+Shift+Left and Right,
  with the new position announced, and `OnReorder` reports the new order. The drop line is the
  `DropLine` part of `TabsStyle`.
- `ui.Alert` is a message box in the flow of the page, with a title, a description, an icon per
  variant (`Info`, `Success`, `Warning`, `Danger`), an optional action and an optional close button.
  It is a group named by its title; it is not announced, so an app calls `Announce` when it should be.
- `ui.DescriptionList` shows terms with their values in a grid of `Columns(n)`, with the term beside
  the value or above it (`Vertical`), optionally `Bordered`. Each item is a group named by its term.
- `ui.GroupBox` is a titled section for related content, outlined or filled (`Fill`), with an
  optional description. It is a group named by its title.
- `ui.Kbd` shows a keyboard shortcut as key caps, from keys written as for `Bind`, with the key
  names a menu shows. It is one static text for assistive technology, such as "Ctrl+Shift+P".
- `ui.Toggle(label, on)` is a button that stays pressed while it is on, such as a Bold button in a
  toolbar, with the sizes and the Ghost or Secondary look of a button, an optional icon and
  `IconOnly`. Assistive technology reads it as a toggle button.
- `ui.ToggleGroup(value, options...)` is a row of joined toggles of which one is selected. It takes
  the options of a radio group, which now have an `Icon`, and behaves as a radio group for the
  keyboard, for assistive technology and in a `Field`.
- `ui.Rating(label, value)` is a row of stars for a whole number from 0 to `Max` (5 by default):
  the pointer previews and a click sets the value, a click on the current star clears it, and
  the arrows, Home, End and the digit keys set it. `ReadOnly` shows it without input. Assistive
  technology reads it as a slider with a value such as "3 of 5 stars" (`Strings.RatingValue`).
- `ui.Calendar(value)` is the month of a date picker's calendar on its own, inline on the page,
  and `ui.CalendarRange(from, to)` the month of a date range picker's, choosing a range in two
  steps. They take the pickers' `Min`, `Max`, `DisableDays` and `FirstDay`, their keys (Enter
  chooses, and Escape and Tab are left to the page) and their accessible grid, and are styled
  through `ui.CalendarStyle` and `Theme.Calendar`, with the part names of the pickers' calendars.
- `ui.LinkSegs` makes one link of several styled segments.
- `ui.Stepper(current, steps...)` shows the progress through the steps of a process: complete
  steps show a check mark, the current one is highlighted, and the later ones are muted. It lays
  out horizontally or with `Vertical()`, and `OnSelect` makes the steps choosable with a click or
  the keyboard. Each step is named for assistive technology by its position, title and state, such
  as "Step 2 of 4: Shipping, current". It is styled through `ui.StepperStyle` and `Theme.Stepper`.
- `ui.OTPInput(label, code, length)` takes a one-time code in a row of single-character boxes, as
  one control with a caret: typing moves on, Backspace and Delete clear, the arrows move, and a
  paste fills the boxes with the characters it takes. It has `Masked`, `Alphanumeric`, `Groups`,
  `OnComplete` and `Validate`, works in a `Field` and a `Form`, and is one text field for assistive
  technology, reported as a password field when masked. It is styled through `ui.OTPInputStyle`
  and `Theme.OTPInput`.
- `ui.CommandPalette(open)` is a searchable list of commands, open while its signal is true. It
  lists the actions available where focus was (or what `Commands` returns), without hidden ones,
  and keeps those whose names contain every word typed, with the matches highlighted and each
  command's key binding beside it. Up, Down, PageUp and PageDown move the highlight, Enter or a
  click runs the command where focus was and gives focus back, and Escape closes it. Assistive
  technology sees a dialog with a combo box whose highlighted command is the focus. It is styled
  through `ui.CommandPaletteStyle` and `Theme.CommandPalette`, and its texts are
  `Strings.CommandPalette`, `CommandPalettePlaceholder` and `NoMatchingCommands`.
- `ui.StatusBar()` is the bar along the bottom of a window, with items at its `Start` and its
  `End`, a top border and small muted text. Assistive technology sees a status bar (the new
  `access.RoleStatusBar`, UI Automation's StatusBar control type). It is styled through
  `ui.StatusBarStyle` and `Theme.StatusBar`. The demo's main window has one.
- `ui.Toolbar(label)` is a row of controls that is one Tab stop: Left and Right move focus
  between its controls, Home and End go to the first and the last, and Tab comes back to the
  control that had focus last. Controls that use those keys, such as a text field, keep them.
  `ui.ToolbarSeparator()` divides groups of controls. Assistive technology sees a tool bar
  named by its label (the new `access.RoleToolBar`, UI Automation's ToolBar control type). It
  is styled through `ui.ToolbarStyle` and `Theme.Toolbar`.
- `ui.Empty(title)` is an empty state: an icon (an inbox by default), a title, an optional
  `Description` and an optional `Action`, centered and muted, for `DataGrid.Empty` and lists
  with no items. It is a group named by its title. It is styled through `ui.EmptyStyle` and
  `Theme.Empty`. The demo's Projects page uses it for its cards and its table.
- `ui.HoverCard(trigger, content)` shows a floating card of rich content once the pointer
  rests on the trigger or keyboard focus reaches it (`OpenDelay`, 500 ms by default). It stays
  open while the pointer is over the trigger or the card, closes `CloseDelay` (300 ms) after it
  leaves, and closes at once on Escape, a click outside or focus leaving. It never takes focus;
  assistive technology reads it as the trigger's description, as a tooltip. It has the look of
  a popover's panel, through `ui.HoverCardStyle` and `Theme.HoverCard`, and `Placement`.
- `Context.After` runs a function on the UI thread after a delay on the window's clock, which
  `ui/uitest` can advance.
- The notes example shows a Markdown preview beside the editor (Ctrl+Shift+V), and the demo has a
  Markdown page.

### Changed

- Links in rich text (`ui.Link` and segments with `OnClick`) are Tab stops: the focused link shows a
  focus ring and Enter follows it. Links inside headings and list items are kept in the
  accessibility tree.
- The predefined themes are converted from GPUI Kit v0.8.0 (formerly gpui-component). Panel and
  border colors of Asciinema and macOS Classic, and the syntax colors of macOS Classic Dark,
  follow upstream. The Matrix theme, which GPUI Kit no longer has, stays.
- The visual inspector has a new look: a toolbar with the pick toggle (Ctrl+Shift+C), the inspected
  window's title and a filter for the tree, over a resizable split of the element tree and the
  details. Tree rows show the kind, key, part, text and the component's short name on one line. The
  details show the element's component and ancestors, a box model diagram, the accessibility node
  with its warnings as alerts, the style as a table with the overridden values struck through, and
  frame tiles over a sparkline with the 16.7 ms budget.
- The inspector outlines the element of the tree row under the pointer in the inspected window,
  more lightly than the selection. The selected element shows its margin, border and padding as
  tinted bands and its content box filled, as browsers' developer tools do, under an accent outline
  and a label with its kind, key and size.

### Fixed

- The fonts parsed for a window stayed in memory after the window closed, because a process-wide
  map held every parsed font. A closed window, such as a dock window or the inspector, now gives
  its fonts back, and the `ui` tests peak at about 0.5 GB instead of 8 GB.
- An element selected in the inspector's tree is outlined in the inspected window at once, not only
  after the pointer or the focus reaches that window.
- The inspector marks an element with no width or no height with a 2-pixel line and its label,
  where it drew nothing visible before. Its label stays inside the window.
- The inspector shortens long texts by characters, so it no longer cuts a character in two.
- The inspector's pick button follows pick mode when Escape in the inspected window ends it.

## [0.2.0] - 2026-10-15

The additions from the API review of v0.1.0. No API is removed or renamed; the changes below are
the only ones that can affect existing code.

### Added

- `Window.Visible()` reports whether a window is shown, and `Tray.Shown()` whether a tray icon is
  in the notification area. `ui.Window` gains the method; fakes that embed `ui.NopWindow` get it.
- `PinchEvent` carries the handler element's `Width` and `Height`, as `MouseEvent` does.
- `ColorPicker` takes `Disabled`, `FocusHandle`, `LabelledBy` and `Tooltip`, with a
  `TriggerDisabled` style part.
- `DateRangePicker.Validate`, with rules on the new `ui.DateRange` type; a range picker in a `Field`
  shows and announces its error as `DatePicker` does.
- `Tooltip` on `Pagination`, `Breadcrumbs`, `Sidebar`, `Tag` and `Avatar`.
- `SVG` takes `Opacity`, `Fit` and `Radius`, as `Image` does. Without `Fit`, an SVG still follows its
  own `preserveAspectRatio`.
- Charts and the code editor take `Width`, `MinWidth`, `MinHeight`, `MaxWidth`, `MaxHeight`, `Grow`
  and `GridSpan`.
- `Series.Color` sets a series' color, and `CandleSeries.Rising` and `Falling` the candles' colors.
  High contrast ignores them, and pies and Sankey diagrams keep the palette.
- The words and summaries that charts show and announce are `ui.Strings` fields (`Chart...`), so an
  app can translate them. The built-in packs leave them English.
- `ui.SetThemeStyle` and `ui.ThemeStyle` hold the style of any widget type on a `Theme`, so charts,
  the code editor and widgets outside `ui` can be styled app-wide. A chart's or editor's style is the
  built-in default, then the theme's, then the instance's `Styles`; under high contrast their colors
  are ignored.
- `Document.Version()` in `codeedit` is a tracked read: a component that shows it re-renders when the
  document changes, with no subscription by hand.
- `ui/uitest` drives a UI in headless tests: find elements by key, name or role, click, type and press
  keys, step time, use an in-process clipboard, and read text, accessibility states and the drawn
  style. `Runtime.StyleOf` reports how the latest paint drew an element. The guide is
  [docs/testing.md](docs/testing.md).
- `jamb package` checks that the app's `App.AppID` matches `identityName` in jamb.json, and stops an
  installer or MSIX build when they differ. It runs the built app with an internal flag, and skips
  the check with a warning when the app can't run on this machine or predates the flag.

### Changed

- `charts.DataTable` returns a `*DataTableElement`, which has `Key` and the sizing methods. Code that
  stores the result as a `ui.Element` still compiles.
- Chart summaries use the singular for a count of one ("1 link", "1 slice").
- The code editor's `Size`, `Width` and `Height` win over a width or height in its `Box` style, as
  the sizes of the `ui` widgets and charts do. The `Box` style still wins over the default size.

### Fixed

- The tray icon docs named the wrong UUID version for the icon's GUID.

## [0.1.0] - 2026-10-15

jamb is a small, declarative, GPU-rendered desktop UI framework written in plain Go, licensed under
the Apache License, Version 2.0. Windows is the first platform: windows use Win32, and rendering
uses Direct3D 11. An app is a tree of elements built in render functions, and state lives in
signals. This is the first release, and the API may change before 1.0.

### Added

#### Windows and platform

- Native windows, with several windows in one app, and window moves, resizes and fullscreen set from code.
- Custom title bars that the app draws, with the platform still handling the caption buttons.
- Recovery from GPU device loss.
- Following Windows settings: dark and light themes, high contrast, text scale, reduced motion and the caret.
- Opening http, https and mailto links with the default handler.

#### Rendering and text

- Canvas for drawing shapes, a CPU path rasterizer, and SVG images.
- Text shaping and layout, with emoji drawn in color, rich text, and selectable text.
- Fonts registered by the app with `App.Font`, and WebP and animated GIF images.
- Gradients, borders with rounded clipping, shadows, and transitions.
- Frames are drawn only when something changes, so an idle window draws nothing and runs no timers.

#### Layout and styling

- Row and column layout with grow, shrink, align, justify, gap, margins, percentages, clipping and scrolling.
- Grid layout with `Div.Grid`, sidebars, split panes with sized panes, and docking with tab groups,
  splits and floating panels.
- Typed style objects for every built-in widget, with theme-wide styles for each kind of widget.
- Predefined color themes, converted from the theme sets of gpui-component (see NOTICE).

#### Widgets

- Buttons, text fields, text areas, number inputs, check boxes, switches, radio groups and sliders.
- Forms with fields, validation and submit, which moves focus to the first field that needs attention.
- Combo boxes with suggestions, date pickers, date range pickers, time pickers and color pickers.
- Tabs, collapsible sections, accordions and tree views.
- Menus, context menus and a menu bar.
- Variable-height lists, and virtualized data grids with sorting, selection and editable or interactive cells.
- Popovers, drawers, modal dialogs, progress bars, progress rings, toasts, badges, tooltips, avatars,
  tags, skeletons, breadcrumbs and pagination.
- Touchpad and touchscreen pinch gestures, and drag and drop.

#### Accessibility

- A UI Automation tree built from the element tree, with roles, names, states, actions and live updates.
- The Text pattern for text fields and for elements that draw their own text.
- Platform-neutral `access` package, with helpers for tests (`access/accesstest`).
- Development warnings for accessibility problems, such as an interactive element with no name or an
  image with no alternative text, and checks with Axe.Windows.

#### Input and IME

- Keyboard focus through focus handles on every interactive control.
- Input method (IME) composition, drawn in text fields and in elements that draw their own text.
- Actions with default key bindings, contextual bindings, and keymap files in JSON.
- Clipboard text and images.

#### Theming and translations

- Light and dark themes, and a high contrast theme built from the system colors.
- Built-in text translated into Italian and Chinese (Simplified, Traditional and Hong Kong variants),
  with `ui.Strings` for an app to change any of the text.

#### OS integration

- Jump lists with tasks and recent documents, and a single-instance mode that hands command-line
  arguments to the running instance.
- Notification area (tray) icons with menus, and hiding a window to the tray.
- Taskbar progress and badges.
- Windows notifications with activation, and a Start menu shortcut for a notifying app.
- Native dialogs: open, save, folder picker and message box.

#### Developer tools

- `jamb dev`, which watches the app, rebuilds it on change, restarts it and keeps the window geometry.
- A visual inspector in its own window, which shows the layers that set each style value and can
  edit values live.
- Headless tests by default; `JAMB_LIVE_TESTS=1` adds the tests that show a window.
- Environment variables for tracing, accessibility warnings and the inspector, which need no code changes.

#### Packaging

- `jamb package`, which builds a Windows zip, an NSIS installer (`setup.exe`) and an MSIX package.
- A sign command for the release files, and a `jamb.json` file for the package settings.

#### Charts

- A `charts` package drawn with Canvas: line, area, bar (grouped, stacked and horizontal), scatter,
  pie, donut, radar, candlestick and Sankey charts.
- Keyboard navigation and tooltips for each chart, and `DataTable`, which shows a chart's values as a table.
- Lines and areas are downsampled before they are drawn, so large series stay fast.

#### Code editor and grammars

- A `codeedit` code editor with undo and redo, several selections, find and replace, folding by
  indentation, and tree-sitter highlighting that runs in WebAssembly.
- The grammars are in their own module, `github.com/tyler-builds/jamb-grammars`, released with v0.1.0,
  with packages for Go, JavaScript, TypeScript, Python, Rust, C, C++, C#, Java, HTML, CSS, JSON, YAML,
  TOML, Markdown and Bash.

#### Performance

- Lists and data grids build only the rows that are in view.
- Text is held in a persistent rope with a line index.
- Startup and idle figures are measured in [BENCHMARKS.md](BENCHMARKS.md).

[Unreleased]: https://github.com/tyler-builds/jamb/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/tyler-builds/jamb/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/tyler-builds/jamb/releases/tag/v0.1.0
