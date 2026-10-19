# Documentation

The guides for building apps with jamb. [Getting started](getting-started.md) shows how to add
jamb to an app; the other pages cover one topic each.

## Building an app

- [Getting started](getting-started.md): requirements, the bundled examples, and adding jamb to an app.
- [Concepts](concepts.md): the element tree, the builder API, state and keys, the UI thread, and errors.
- [Layout](layout.md): shrinking, grid layout, split panes, and docking.
- [Input](input.md): keyboard and focus, text fields and the clipboard, actions and key bindings, events, drag and drop, and gestures.
- [Windows and dialogs](windows.md): windows, zoom, custom title bars, and native dialogs.
- [OS integration](os-integration.md): jump lists and single instance, the notification area, OS notifications, and opening URLs.
- [Translations](translations.md): languages and translated strings.
- [Examples](examples.md): the demo app and the notes app in this repository.

## Looks

- [Theming](theming.md): themes, the predefined themes, the style parts of the built-in widgets, and system settings.
- [Styling](styling.md): styling the built-in widgets, with their parts, states, variants, sizing, and guardrails.
- [Drawing](drawing.md): gradients, borders, the canvas, charts, SVG images, and icons.
- [Animation](animation.md): transitions and enter and exit animations.
- [Text](text.md): rich text, selectable text, fonts, and emoji.

## Widgets

- [Form widgets](widgets-forms.md): form controls, the color picker, and forms with validation and submission.
- [Selection and date widgets](widgets-selection.md): the combobox and the date and time pickers.
- [Navigation widgets](widgets-navigation.md): tabs, collapsible sections, trees, the sidebar, group boxes, and description lists.
- [Feedback widgets](widgets-feedback.md): feedback and status indicators, alerts, avatars, tags, skeleton placeholders, and keyboard shortcuts.
- [Data widgets](widgets-data.md): variable-height lists, uniform lists, and data grids.
- [Overlays and menus](widgets-overlays.md): menus, dropdowns, popovers, drawers, modal dialogs, and the command palette.
- [Charts](charts.md): building charts, data and axes, interaction, and accessibility.
- [Code editor](codeedit.md): the `codeedit` package, with its keys, highlighting, and limits.
- [Markdown](markdown.md): the `markdown` package, a read-only view of Markdown, with its links, images, code highlighting, parts, and limits.

## Accessibility

- [Accessibility](accessibility.md): the guide for app authors.
- [Accessibility design](accessibility-design.md): the design and the decision log, with the phases in separate files: [B and C](accessibility-design-phases-b-c.md), [D and E](accessibility-design-phases-d-e.md), [F to H](accessibility-design-phases-f-h.md), and [I to M](accessibility-design-phases-i-m.md).
- [Accessibility testing](accessibility-testing.md): checks by hand with Narrator and NVDA.

## Tools and reference

- [Testing your app](testing.md): headless UI tests with `ui/uitest`, which find elements, act on them, and read their text, states and drawn style.
- [Tools for humans and coding agents](tools.md): `jamb dev`, the visual inspector, environment variables, tracing, scripted runs, headless testing, and accessibility checks.
- [Packaging](packaging.md): `jamb package`, `jamb.json`, the installer, MSIX, signing, and the licenses of what your app ships.
- [Architecture and internals](architecture.md): the repository layout, the architecture and frame pipeline, and GPU device loss.
- [Comparison with GPUI](comparison-gpui.md): the same hello-world app built with jamb and with GPUI, measured.
- [Status and known limitations](status.md): the milestone status, the known limitations, and the next steps.
