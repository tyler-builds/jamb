# Changelog

All notable changes to jamb are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the
version numbers follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
- Frames are drawn only when something changes, so an idle window uses no CPU.

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

[0.1.0]: https://github.com/tyler-builds/jamb/releases/tag/v0.1.0
