# Code editor

The `codeedit` package is a code editor for source files: a multi-line text area with a
line-indexed buffer, undo and redo, several selections, find and replace, folding by
indentation, tree-sitter syntax highlighting, input-method composition, and an accessibility
Text pattern. It is pure Go, and renders on the Canvas of the `ui` package.

## Building an editor

```go
doc := codeedit.NewDocument(source)
ui.Div().Child(codeedit.New(doc).
    Label("main.go").
    Language(golang.Grammar).
    Comment("// ").
    TabSize(4).
    Height(ui.Px(400)))
```

Add the editor's keys to the runtime once, with the other bindings:

```go
rt.AddBindings(codeedit.Bindings()...)
```

`New` returns an element. Its options chain: `Key`, `Label`, `LineNumbers`, `ReadOnly`,
`TabSize` (4 by default), `Comment`, `Size`, `Height`, `Styles`, `Language` and `Highlighter`.
Word wrap is not offered (see Limits).

## Document

A `Document` holds the text, the selections, the folds and the undo history. Several editors
can show one document: a change made in one appears in the others, and each editor keeps its
own scroll position.

| Method | Purpose |
|---|---|
| `NewDocument(text)`, `SetText(text)` | Load text. `SetText` clears the undo history and is not a step. |
| `Text()`, `Len()`, `LineCount()`, `Line(row)` | Read the text. Line feeds only: a CR LF pair is read as one line feed. `Text` copies the whole text; the editor does not call it on its paint or keystroke path. |
| `Edit(changes...)`, `Replace(from, to, s)` | Change the text as one undo step. Ranges are bytes and must start and end on character boundaries. |
| `Undo()`, `Redo()`, `CanUndo()`, `CanRedo()` | The history. Typing and deleting within a second of the same kind form one step. |
| `Selections()`, `SetSelections(...)`, `Primary()`, `Caret()` | The selections; the last one set is the primary. |
| `SetTabWidth(n)` | The width of a tab stop, for the commands that depend on columns. The editor sets it from `TabSize`. |
| `OnChange(fn)`, `OnEdit(fn)`, `Version()` | Listen to changes. `OnEdit` reports each replaced byte range, last part first. |

Positions are byte offsets. Selections move with the text: a caret before an insertion moves
after it, and a caret inside a replaced range goes to its start.

## Keys

The keys are the actions in the `CodeEditor` key context. Each action is a `codeedit.Cmd...`
value, so an app can rebind it with `ui.BindIn("CodeEditor", keys, action)` or turn it off with
`ui.Unbind`.

| Key | Action |
|---|---|
| Arrows, Home, End, Ctrl+Left/Right, Ctrl+Home/End, PageUp, PageDown | Move the caret. Up and Down keep the screen column, counting tab stops and wide characters. Home goes to the first non-blank character, then to column 0. |
| Shift with any of those | Extend the selection. |
| Ctrl+A | Select all. |
| Escape | Keep the primary caret only. |
| Enter | New line, with the indent of the line; one more indent after an opening bracket, and a line of its own between a pair. |
| Tab, Shift+Tab | Indent or outdent the selected rows; a caret inserts spaces to the next tab stop. |
| Typed brackets and quotes | Close the pair, step over a closer that is already there, and wrap a selection. An apostrophe after a letter does not pair. |
| Backspace, Delete, Ctrl+Backspace, Ctrl+Delete | Delete. Backspace between an empty pair deletes both. |
| Ctrl+Z, Ctrl+Y, Ctrl+Shift+Z | Undo and redo. |
| Ctrl+C, Ctrl+X, Ctrl+V | Copy, cut and paste through the UI clipboard. With several selections and as many lines in the clipboard, each selection takes one line. |
| Ctrl+/ | Toggle the line comment prefix set with `Comment`. |
| Ctrl+Shift+[ | Fold the row of the caret. |
| Ctrl+Shift+] | Unfold the row of the caret. |
| Ctrl+D | Select the word at the caret, then add the next occurrence of the selection. |
| Ctrl+Alt+Up, Ctrl+Alt+Down | Add a caret above or below the topmost or bottommost caret. |
| Alt+click | Add a caret. Shift+click extends the primary selection. Double-click selects a word; triple-click selects a row. |
| Ctrl+F, Ctrl+H | Find, and find and replace. |
| F3, Shift+F3 | Next and previous match. |
| Ctrl+G | Go to line. |

The punctuation keys name their physical position as the US layout prints it: `ctrl+/` is the
slash key, and `ctrl+shift+[` is the key that prints `[` on a US keyboard (Shift makes it `{`,
and the binding still matches). A keyboard with another layout has its keys in the same places.

Every command is an action, so menus and a command palette can run it. The movement and typing
actions are hidden from a palette, as the ui's own text field actions are.

## Find and replace

The find bar is a real element above the text. It has a query, options for case, whole words
and regular expressions (Go's RE2 syntax), and a count such as "3 of 12" or "No results". An
invalid pattern shows its own message. Matches are highlighted in the visible rows.

Replace replaces the current match and selects the next one. Replace all replaces every match
as one undo step, and the expansion of `$1` and `${name}` applies to regular expressions.

`Finder` is the search, for an app that does not use the bar: `NewFinder(query, FindOptions{...})`
and `Matches(doc)`, cached until the text changes. A search scans the whole text, so with the bar
open a search runs again after each change (see Limits).

## Folding

A row with lines under it that are indented past it has a fold marker in the gutter. A click on
the marker folds or unfolds it, and Ctrl+Shift+[ and Ctrl+Shift+] do the same at the caret. Folded
rows are not drawn, and the caret and Up and Down skip them. A caret inside a fold moves to the
end of its header. Folds follow the text when lines are inserted above them.

## Highlighting

Highlighting uses the tree-sitter session in `codeedit/highlight`. The grammars are in their own
module, [github.com/tyler-builds/jamb-grammars](https://github.com/tyler-builds/jamb-grammars), with
one package per language (`golang`, `python`, `typescript` and others), each providing a
`highlight.Grammar`:

```go
import "github.com/tyler-builds/jamb-grammars/golang"

codeedit.New(doc).Language(golang.Grammar)
```

A grammar is a WebAssembly module: the tree-sitter runtime, the grammar, and a shim that the
editor calls. `highlight.ABI` is the version of that interface, and each grammar reports the
version it was built with; a grammar of another version is refused with an error that names both,
so jamb and jamb-grammars must be versions that match. `highlight.NewGrammar` makes a
`highlight.Grammar` from a module built with jamb-grammars' build command, for a language it does
not include.

`Language` starts a session on the document's text when the editor first renders. The session is
closed when the language or the document changes, and when the editor's component is disposed (the
close is registered with `ui.Context.OnDispose`). Each change goes to the session at once, with the
bytes it replaced, so a keystroke never waits for a parse. The editor sends the rows shown to the
session as the viewport, so the rows on screen are parsed first. The session's worker asks for a
frame when it has new spans. A document over the highlight limit (`highlight.ErrTooLarge`) is
shown without colors.

A capture takes the theme's style of its closest name: `string.escape.go` takes the style of
`string.escape`, and then of `string` (`highlight.Closest`). Where spans overlap, the later one
wins. A capture whose style is italic is drawn in the italic face of the monospace family, such
as Consolas Italic on Windows.

`Highlighter(h)` takes any value with the session's methods (`Edit`, `SetViewport`, `Spans`,
`CaptureNames`, `Close`), for an app that runs its own highlighter.

## Input method composition

An input method composes text before it is typed, as Chinese, Japanese and Korean input do. The
editor shows the composition inline at the primary caret, underlined, with the active clause
marked. The composition is not part of the document until it is committed. The editor commits it
as one typed edit when the pointer clicks in the editor, when a key the editor handles is pressed,
or when the editor loses focus; a composition that ends empty is cancelled. A read-only editor
shows no composition.

The candidate window opens at the caret. The editor reports the caret rectangle (a `ui.Rect`, in
logical pixels from the element's top left) of its primary caret through
`ui.DivElement.CompositionCaret`, and the host places the window there. An element of an app outside `codeedit` can take composition
the same way: `OnComposition` receives a `ui.Composition` (the text, the input method's cursor, and
the clause), and `OnCommitComposition` receives the confirmed text.

## Accessibility

The editor's canvas has the role of a multi-line text area and the name given by `Label`. The
find bar's fields and buttons are named through `ui.Strings` (the `CodeEditor...` fields).

The canvas has a Text pattern through `ui.DivElement.TextPattern`. The editor's model of its text
reads an immutable snapshot of the rope, so assistive technology can read the text from any
thread. The model gives the points as bytes, and computes characters, words, lines and paragraphs
locally around a position; a query never copies the whole document. `GetText` reads only the
requested range, cut to the requested length. Bounding rectangles are given for the visible part
of a range: a line outside the painted rows has none, which the pattern allows.

The pattern is writable. Assistive technology can select a range (`Select`, or setting the
selection), which sets the editor's primary selection, and ScrollIntoView scrolls the editor to
its caret. The hooks are `ui.DivElement.TextSelect` and `ui.DivElement.TextReveal`.

The font name the pattern reports is the name the platform knows the monospace face by, read from
the font file (`ui.Painter.FontName`).

## Caret blink

The caret blinks at the system's caret blink rate, from the time of its last edit, move, click,
focus gain, undo, redo or select all. It stays solid under reduced motion, when the system turns
blinking off, and after about 15 seconds without activity. It is drawn only while the editor has
the keyboard focus, and it schedules its next frame only then. Canvas drawings can use the same
rule through `ui.Painter.CaretVisible`.

## Styling

`codeedit.Style` has the parts of the editor. A part's style merges over the defaults, which come
from `Theme.Editor` and `Theme.Syntax`, and then over `Styles`:

| Part | What it is |
|---|---|
| `Box` | The frame: background, border, radius and padding. |
| `Text` | The text: `TextColor`, `FontSize`. |
| `Gutter` | The line numbers and fold markers: `TextColor`, `FontSize`, `Background`. |
| `ActiveLine` | The rows of the carets. |
| `Selection` | The selected text. |
| `Match`, `CurrentMatch` | The find matches. |
| `Find` | The find and replace bar. |

Under high contrast the colors are the system colors (the theme's `Editor` colors), whatever is set
in `Styles`; sizes and spacing still apply.

## Performance

Measured headlessly with `go test -bench` in `codeedit` (the runtime builds and draws each frame,
without a window), with 20 iterations:

| Benchmark | Time |
|---|---|
| Open a document of 100,000 lines, with the first frame | about 5.7 ms |
| Type a character at 50,000 lines, with the frame | about 0.8 ms |
| Scroll by a few rows at 50,000 lines, with the frame | about 0.7 ms |

The paint draws only the rows shown. The text of a row is shaped by the `ui` text system, which
caches shaped paragraphs; the editor does not keep its own shaping cache. Frame time with
highlighting depends on the spans shown and is not included in these figures.

## Limits

- No word wrap. The option is not offered; long lines scroll horizontally.
- With the find bar open, a change runs the search again over the whole text. The bar is the only
  path that copies the text on a keystroke.
- A wide character takes two cells in the editor's grid. If the font draws it at another width,
  the columns after it are drawn off the grid.
- CR LF text loads as LF, and is saved as LF by `Text()`.
- Translations: the find bar's messages are translated for Italian and Chinese where the
  translation is certain. The remaining messages are English in every language, as the `ui.Strings`
  packs describe.
- Binary size and memory: importing `codeedit` links wazero, the WebAssembly runtime that runs
  tree-sitter (about 1.1 MB of code), and each jamb-grammars package an app imports adds its
  grammar (0.15 to 5.5 MB; Go is 0.36 MB). wazero hashes modules with SHA-256, which links Go's
  FIPS crypto module and its 32 MB zero-filled buffer, so an app that imports `codeedit` reports
  about 32 MB more private bytes than one that does not (see BENCHMARKS.md).
