# Code editor

The `codeedit` package is a code editor for source files: a multi-line text area with a
line-indexed buffer, undo and redo, several selections, find and replace, folding by indentation,
soft wrap, syntax highlighting for 76 languages, input-method composition, diagnostics and the
hooks of a language server, and an accessibility Text pattern. It is pure
Go and draws on the Canvas of the `ui` package. It is built with
`codeedit.Editor(codeedit.NewDocument(text))`. Its keys are actions with default bindings, so they
work without setup, and an app can rebind them. The sections below cover the keys, the document,
highlighting, the limits and the measured performance.

Highlighting is built in. The package carries
[@pierre/highlights](https://github.com/pierrecomputer/pierre/tree/main/packages/highlights), the
syntax highlighter of the Pierre Computer Company, with a lexer for each of 76 languages in one
WebAssembly module of about 170 KB. A language is named as a code fence names it:

```go
codeedit.Editor(doc).Language("go")
```

The module runs in wazero, which adds about 1.1 MB to an app's binary. wazero uses Go's SHA-256, which links Go's
FIPS crypto module and its 32 MB zero-filled buffer, so an app that imports `codeedit` uses about
32 MB more private memory than one that does not (see BENCHMARKS.md). Apps that do not import
`codeedit` link none of this.

## Building an editor

```go
doc := codeedit.NewDocument(source)
ui.Div().Child(codeedit.Editor(doc).
    Label("main.go").
    Language("go").
    CommentPrefix("// ").
    TabSize(4).
    Height(ui.Px(400)))
```

The editor's keys work without any setup: the package registers them as default bindings of
every window (`ui.RegisterDefaultBindings`). An app's own bindings win over them, so it can bind
a command to another key, or turn a key off:

```go
rt.AddBindings(ui.BindIn("CodeEditor", "ctrl+k", codeedit.CmdFind))
rt.AddBindings(ui.BindIn("CodeEditor", "ctrl+d", ui.Unbind(codeedit.CmdAddNextOccurrence)))
```

`Editor` returns an element. Its options chain: `Key`, `Label`, `LineNumbers`, `ReadOnly`,
`TabSize` (4 by default), `CommentPrefix`, `Size`, the sizing methods (`Width`, `Height`,
`MinWidth`, `MinHeight`, `MaxWidth`, `MaxHeight`, `Grow` and `GridSpan`, as on the `ui` widgets;
Full by 300 pixels by default), `Styles`, `Language`, `Highlighter`, `SoftWrap`, `WrapColumn`
and `WrappingIndent` (see Soft wrap), and `Hover`, `HoverDelay`, `Completions`, `Definitions` and
`OnOpenLocation` (see Language server hooks).

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
| `SetTabSize(n)` | The width of a tab stop, for the commands that depend on columns. The editor sets it from `TabSize`. |
| `OnChange(fn)`, `OnEdit(fn)`, `Version()` | Listen to changes. `OnEdit` reports each replaced byte range, last part first. |
| `TextVersion()` | A number that changes with the text only, for the requests to a language server. |
| `SetDiagnostics(ds)`, `SetDiagnosticsFor(v, ds)`, `Diagnostics()`, `DiagnosticsAt(off)` | The diagnostics (see Diagnostics). |
| `UTF16Pos(off)`, `OffsetUTF16(row, col)` | Convert between byte offsets and rows with UTF-16 columns, as the Language Server Protocol counts them. |

Positions are byte offsets. Selections move with the text: a caret before an insertion moves
after it, and a caret inside a replaced range goes to its start.

`Version` changes on every change of the text, the selections or the folds, whether it comes
from typing, an edit, undo or redo, or `SetText`. Read during a component's render it is a tracked
read, like `Signal.Get`: the component renders again after the next change, with no `OnChange` of
its own. Read anywhere else it is a plain read.

```go
saved := ui.Local(ctx, "saved", doc.Version()) // the version at the last save
if doc.Version() != saved.Get() {              // renders again on each change
	title += " (modified)"
}
```

## Keys

The keys are the actions in the `CodeEditor` key context. Each action is a `codeedit.Cmd...`
value, so an app can rebind it with `ui.BindIn("CodeEditor", keys, action)` or turn it off with
`ui.Unbind`.

| Key | Action |
|---|---|
| Arrows, Home, End, Ctrl+Left/Right, Ctrl+Home/End, PageUp, PageDown | Move the caret. Up and Down keep the screen column, counting tab stops and wide characters. Home goes to the first non-blank character, then to column 0. With soft wrap, Up, Down and the page keys move by the rows shown, and Home and End go to the edges of the segment first (see Soft wrap). |
| Shift with any of those | Extend the selection. |
| Ctrl+A | Select all. |
| Escape | Keep the primary caret only. |
| Enter | New line, with the indent of the line; one more indent after an opening bracket, and a line of its own between a pair. |
| Tab, Shift+Tab | Indent or outdent the selected rows; a caret inserts spaces to the next tab stop. |
| Typed brackets and quotes | Close the pair, step over a closer that is already there, and wrap a selection. An apostrophe after a letter does not pair. |
| Backspace, Delete, Ctrl+Backspace, Ctrl+Delete | Delete. Backspace between an empty pair deletes both. |
| Ctrl+Z, Ctrl+Y, Ctrl+Shift+Z | Undo and redo. |
| Ctrl+C, Ctrl+X, Ctrl+V | Copy, cut and paste through the UI clipboard. With several selections and as many lines in the clipboard, each selection takes one line. |
| Ctrl+/ | Toggle the line comment prefix set with `CommentPrefix`. |
| Ctrl+Shift+[ | Fold the row of the caret. |
| Ctrl+Shift+] | Unfold the row of the caret. |
| Ctrl+D | Select the word at the caret, then add the next occurrence of the selection. |
| Ctrl+Alt+Up, Ctrl+Alt+Down | Add a caret above or below the topmost or bottommost caret. |
| Alt+click | Add a caret. Shift+click extends the primary selection. Double-click selects a word; triple-click selects a row. Ctrl+click goes to the definition, with `Definitions`. |
| Ctrl+F, Ctrl+H | Find, and find and replace. |
| F3, Shift+F3 | Next and previous match. |
| Ctrl+G | Go to line. |
| F8, Shift+F8 | Select the next or the previous diagnostic, open its hover, and announce it. |
| Ctrl+K Ctrl+I | Open the hover at the caret, and announce it. |
| F12 | Go to the definition at the caret, with `Definitions`. |
| Ctrl+Space | Open the completion list; Up, Down, PageUp and PageDown move its highlight, and Enter and Tab accept it. |

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

## Soft wrap

`SoftWrap` wraps the lines that are too long for the editor:

```go
codeedit.Editor(doc).SoftWrap(codeedit.WrapViewport)                     // at the editor's width
codeedit.Editor(doc).SoftWrap(codeedit.WrapBounded).WrapColumn(100)      // at 100 columns, or narrower
codeedit.Editor(doc).SoftWrap(codeedit.WrapColumn).WrappingIndent(codeedit.IndentNone)
```

| Mode | Wraps at |
|---|---|
| `WrapNone` | Nowhere; long lines scroll horizontally. The default. |
| `WrapViewport` | The width of the editor's text, less one cell for the caret, and at least 8 cells. |
| `WrapColumn` | The column set with `WrapColumn` (80 by default). A narrower editor scrolls horizontally to it. |
| `WrapBounded` | The column, or the width of the editor when that is narrower. |

A line is cut after the last space or punctuation that fits, or else between two characters; a tab
or a wide character is never cut. Each segment of a wrapped line is drawn on a row of its own. The
continuation segments start under the line's first non-blank character (`IndentSame`, the default),
at the left edge (`IndentNone`), or one tab stop further (`IndentOneLevel`); an indent that would
leave fewer than 8 cells of text is dropped. The line number and the fold marker are on the first
segment, and the row of a caret is highlighted on every segment. With `WrapViewport` and
`WrapBounded` the editor does not scroll horizontally.

The wrap belongs to the editor, not to the document: two editors can show one document at different
widths, and the text, its line count and the document's moves do not change. Up, Down, PageUp and
PageDown move by the rows shown and keep the column within the segment. Home goes to the start of
the segment, and on the first segment it toggles between the first non-blank character and column 0
as on an unwrapped line; a second Home on a later segment goes to the line's first non-blank
character. End goes to the end of the segment, and a second End to the end of the line. A caret at
the edge of two segments shows at the end of the first after End or a click past the end of a
segment, and at the start of the next otherwise. Ctrl+Alt+Up and Ctrl+Alt+Down add a caret on the
row shown above or below.

Each edit wraps again only the rows it touches, and a change of the width in cells wraps every row
again, keeping the row at the top of the view where it was. Folding hides the segments of the rows
under the header, and the header keeps its own. The Text pattern reports each segment as a line and
each row as a paragraph.

## Diagnostics

Diagnostics are the errors, warnings and notes that a language server or a linter reports. They
belong to the document, so every editor that shows it draws them:

```go
text, ver := doc.Text(), doc.TextVersion() // the version is recorded with the request
go func() {
	ds := lint(text)
	ctx.Post(func() { doc.SetDiagnosticsFor(ver, ds) }) // dropped if the text changed since
}()
```

A `Diagnostic` has a byte range `[From, To)`, a `Severity` (`SeverityError`, `SeverityWarning`,
`SeverityInfo`, `SeverityHint`; zero is an error), a `Message`, and the `Source` and `Code` that
reported it. An empty range marks the character at `From`. `SetDiagnostics` replaces the set;
`SetDiagnosticsFor` does the same only when the text is still the one of the given `TextVersion`,
and reports whether it did. A move of the caret does not make a set stale. The diagnostics follow
the text through edits, undo and redo: an insertion before one moves it, typing at either of its
ends does not grow it, and a range that is replaced becomes an empty one at the start of the
replacement. `SetText` removes them.

Each diagnostic is drawn as a wavy underline, on every display row it covers; an empty one, or one
about the line feed, takes one cell. Once a document has had diagnostics set, the gutter has a
column of markers, one on each row where a diagnostic starts, for the most serious of them: a
filled circle for an error, a triangle for a warning, an "i" for information and a dot for a hint,
so that the severity does not rest on color. The column stays while the set is empty, so that the
text does not shift as diagnostics come and go. F8 and Shift+F8 select the next or the previous
diagnostic, wrapping around the text, scroll to it, open the hover there, and announce it politely, such as "Error on
line 3: undefined: x" (`ui.Strings.CodeEditorDiagnostic` and the names of the severities,
`CodeEditorError`, `CodeEditorWarning`, `CodeEditorInfo` and `CodeEditorHint`).

The Document is not safe for concurrent use: an app that gets diagnostics on another goroutine sets
them on the UI thread, with `ui.Context.Post`. `UTF16Pos` and `OffsetUTF16` convert between byte
offsets and the positions of the Language Server Protocol, whose columns count UTF-16 code units.

## Language server hooks

The editor asks an app's providers about the text, as a language server answers an editor. Each
provider is an interface with a function type that implements it, and runs on a goroutine of its
own: it gets a `context.Context` and a `Query`, which holds a copy of the text as it was when the
request started, its `TextVersion` and a byte offset. The answer is applied on the UI thread with a
frame (through `ui.Context.Post`), and only if it is still wanted: a newer request of the same kind,
an edit of the text, the loss of focus, another document and the removal of the editor each cancel
the context, and an answer that arrives after that is dropped. A provider that talks to a language
server converts the offsets with `UTF16Pos` and `OffsetUTF16`.

### Hover

```go
codeedit.Editor(doc).Hover(codeedit.HoverFunc(func(ctx context.Context, q codeedit.Query) (codeedit.Hover, error) {
	return codeedit.Hover{Text: describe(q.Text, q.Offset)}, nil
}))
```

When the pointer rests on the text for `HoverDelay` (500 milliseconds by default), or on Ctrl+K
Ctrl+I at the caret, the editor asks the provider about the offset and opens a popup under the text,
with the messages of the diagnostics there first, each with its severity and its source. A `Hover`
has the range it is about (`From`, `To`; empty for the word under the pointer), a `Text`, and an
optional `Content` that builds an element to show instead of the text, such as rendered Markdown.
The diagnostics show at once; the provider's answer joins them when it comes. Without a provider
the popup shows the diagnostics alone. Escape, an edit, the loss of focus, a click outside the popup
and a move of the pointer off the range close it. A hover opened from the keyboard is announced
politely. The popup is a tooltip for assistive technology and does not take the focus.

### Completion

```go
type completer struct{}

func (completer) Triggers() string { return "." }
func (completer) Complete(ctx context.Context, q codeedit.CompletionQuery) (codeedit.CompletionList, error) {
	return codeedit.CompletionList{From: -1, Items: []codeedit.CompletionItem{{Label: "Println", Detail: "func(a ...any)"}}}, nil
}

codeedit.Editor(doc).Completions(completer{})
```

The list opens at the caret when one of the provider's `Triggers` is typed (`TriggerCharacter`,
with the character in `CompletionQuery.Char`), and on Ctrl+Space (`CmdTriggerSuggest`,
`TriggerInvoked`). It shows that it is loading until the answer comes. A `CompletionList` has the
items, at most 500 of which are kept, and `From`, the byte offset of the query's text where the
replaced word starts (-1 for the word before the caret). A `CompletionItem` has a `Label`, a
`Detail` shown after it, a `Doc`, the `Insert` text (the label when empty), the `Filter` text that
the typed word is matched against (the label when empty), a `Kind`, and `Edits`: more changes made
with it, such as an import, in the offsets of the query's text.

Typing filters the list: an item matches when the characters of the typed word appear in its filter
text in order, ignoring case, and the items that start with the word come first. A list marked
`Incomplete` is asked for again as the word grows (`TriggerIncomplete`), 100 milliseconds after the
last keystroke; a keystroke while a request is on its way cancels it and asks again at once. Up,
Down, PageUp and PageDown move the highlight, Enter and Tab accept it, and Escape, a click, the
loss of focus, an undo, and a move of the caret before the word or past where typing left it close
the list. Accepting replaces the word before every caret with the item's text, makes its edits,
mapped through the text typed since the request, and is one undo step; an edit that overlaps the
replaced word is dropped. A read-only editor has no completions. The `Doc` of the highlighted item
shows under the list.

The keyboard focus stays in the editor, as with a combobox: the list is a list for assistive
technology (`ui.Strings.Suggestions`), its highlighted item is the selected one, the number of
items is announced when the answer comes (`SuggestionCount`), and each item the highlight moves to
is announced politely. The list shows up to 10 items at a time around the highlighted one.

### Definition

```go
codeedit.Editor(doc).
	Definitions(definer). // a codeedit.DefinitionProvider
	OnOpenLocation(func(l codeedit.Location) { openTab(l.URI, l.Row, l.Col) })
```

F12 (`CmdGoToDefinition`) and Ctrl+click ask the provider where the symbol at the caret or the click
is defined, and go to the first `Location` it gives. A location with an empty `URI` is in this
document: its bytes `[From, To)` are selected and scrolled into view. Any other is handed to the
function set with `OnOpenLocation`, with the `Row` and `Col` the provider gave; without one it is not
opened. While Ctrl is held, the word under the pointer is underlined and the pointer is a hand.

## Highlighting

Highlighting uses the session in `codeedit/highlight`, which runs @pierre/highlights. `Language`
takes the name of a language or a common alias, in any case: `go`, `python`, `ts`, `tsx`, `rust`,
`c#`, `sh`, `yml` and so on. `highlight.Languages` lists the names, and `highlight.Supported`
checks one. A language the highlighter does not know is shown without colors
(`highlight.ErrUnknownLanguage`).

The highlighter lexes. Each language's lexer classifies the text from the tokens near it and from
the state that it carries from one line to the next, without building a syntax tree. The session
keeps that state at the end of each line, so a change relexes the lines it touches and the lines
after them only until the state is again what it was: relexing after a keystroke inside a line takes about 12 µs (see BENCHMARKS.md),
and typing `/*` recolors the file down to where the comment ends. A lexer can color some code
differently from a parser: in Go, for one, `pkg.Name` may take the color of a property where a
parser knows that it is a type. An app that wants colors from a syntax tree can plug in the
tree-sitter highlighter of jamb-grammars (see `Highlighter` below).

`Language` starts a session on the document's text when the editor first renders. The session is
closed when the language or the document changes, and when the editor's component is disposed (the
close is registered with `ui.Context.OnDispose`). Each change goes to the session at once, with the
bytes it replaced, so a keystroke never waits for the highlighter. The editor sends the rows shown
to the session as the viewport: the session's worker relexes up to the last row shown, publishes
the spans of the rows, and then relexes the rest of the document in slices of about a millisecond.
The worker asks for a frame when it has new spans. A document over the highlight limit (16 MB by
default, `highlight.ErrTooLarge`) is shown without colors.

The tokens are named as the syntax styles of Zed's themes, which jamb's themes use: `keyword`,
`keyword.control`, `string.escape`, `function.method`, `type.builtin` and so on
(`highlight.CaptureNames`). A token takes the theme's style of its closest name:
`keyword.control` takes the style of `keyword` when the theme has none of its own
(`highlight.Closest`). A few names that Zed's themes do not style take the style of another:
CSS's `selector` that of `tag`, `selector.class` and `selector.id` that of `property`, and
`selector.pseudo` that of `attribute`. A token whose style is italic is drawn in the italic face of
the monospace family, such as Consolas Italic on Windows.

`Highlighter(h)` takes any value with the session's methods (`Edit`, `SetViewport`, `Spans`,
`CaptureNames`, `Close`), for an app that runs its own highlighter. The editor sends it each change
and the viewport, and paints its spans, where a later span wins over an earlier one that overlaps
it. It does not close a highlighter that the app gives it. The `treesitter` package of
[github.com/tyler-builds/jamb-grammars](https://github.com/tyler-builds/jamb-grammars) is one: it
runs tree-sitter grammars in WebAssembly, for an app that wants colors from a syntax tree.

## Input method composition

An input method composes text before it is typed, as Chinese, Japanese and Korean input do. The
editor shows the composition inline at the primary caret, underlined, with the active clause
marked. The composition is not part of the document until it is committed. The editor commits it
as one typed edit when the pointer clicks in the editor, when a key the editor handles is pressed,
or when the editor loses focus; a composition that ends empty is cancelled. A read-only editor
shows no composition.

The candidate window opens at the caret. The editor reports the caret rectangle (a `ui.Rect`, in
logical pixels from the element's top left) of its primary caret through `ui.IME.Caret`, and the
host places the window there. An element of an app outside `codeedit` can take composition the same
way: `ui.IME.Update` receives a `ui.Composition` (the text, the input method's cursor, and the
clause), and `ui.IME.Commit` types the confirmed composition, set through `ui.DivElement.IME`.

## Accessibility

The editor's canvas has the role of a multi-line text area and the name given by `Label`. The
find bar's fields and buttons are named through `ui.Strings` (the `CodeEditor...` fields).

The canvas has a Text pattern through `ui.DivElement.TextAccess`. The editor's model of its text
reads an immutable snapshot of the rope, so assistive technology can read the text from any
thread. The model gives the points as bytes, and computes characters, words, lines and paragraphs
locally around a position; a query never copies the whole document. `GetText` reads only the
requested range, cut to the requested length. Bounding rectangles are given for the visible part
of a range: a line outside the painted rows has none, which the pattern allows.

The pattern is writable. Assistive technology can select a range (`Select`, or setting the
selection), which sets the editor's primary selection, and ScrollIntoView scrolls the editor to
its caret. The hooks are `ui.TextAccess.Select` and `ui.TextAccess.Reveal`.

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
from `Theme.CodeEditor` and `Theme.Syntax`, then over the theme's `codeedit.Style`, and then over
`Styles`, field by field. The theme's style is set for every editor of a theme with
`ui.SetThemeStyle` (see [Styles of other widgets on a theme](theming.md#styles-of-other-widgets-on-a-theme));
set it on each theme the window uses. The inspector names the layers "built-in default", "theme",
"instance Styles" and "high contrast".

```go
for _, t := range []*ui.Theme{light, dark} {
	ui.SetThemeStyle(t, codeedit.Style{Box: ui.Style{Radius: 4}, Gutter: ui.Style{FontSize: 12}})
}
codeedit.Editor(doc).Styles(codeedit.Style{Box: ui.Style{Radius: 8}}) // this editor: 8, gutter 12
```

The parts are:

| Part | What it is |
|---|---|
| `Box` | The frame: background, border, radius and padding. |
| `BoxFocused` | The frame while the editor has focus. Unset by default: as in code editors, the caret shows focus and the frame does not change. Under high contrast it is a border of the system's Highlight color. |
| `Text` | The text: `TextColor`, `FontSize`. |
| `Gutter` | The line numbers and fold markers: `TextColor`, `FontSize`, `Background`. |
| `ActiveLine` | The rows of the carets. |
| `Selection` | The selected text. |
| `Match`, `CurrentMatch` | The find matches. |
| `Find` | The find and replace bar. |
| `Popup`, `PopupItem`, `PopupItemSelected` | The panel of the hover and of the completions, one entry of it, and the completion that Enter accepts. |
| `DiagnosticError`, `DiagnosticWarning`, `DiagnosticInfo`, `DiagnosticHint` | The marks of the diagnostics: the `TextColor` of the underline and of the gutter marker. By default the theme's `Danger`, `Warning`, `Accent` and `TextMuted`. |

Under high contrast the colors are the system colors (the theme's `CodeEditor` colors), whatever is set
in the theme's `codeedit.Style` or in `Styles`; sizes and spacing still apply.

## Performance

Measured headlessly with `go test -bench` in `codeedit` (the runtime builds and draws each frame,
without a window), with 20 iterations:

| Benchmark | Time |
|---|---|
| Open a document of 100,000 lines, with the first frame | about 5.7 ms |
| Type a character at 50,000 lines, with the frame | about 0.8 ms |
| Scroll by a few rows at 50,000 lines, with the frame | about 0.7 ms |

With soft wrap at the editor's width, in a window 360 pixels wide (about 34 columns, where one row
in six wraps), measured on 2026-10-20 on an Intel Core i5-11600K, with 20 iterations:

| Benchmark | Time |
|---|---|
| Change the width at 100,000 lines: wrap every row again, with the frame | about 4.2 ms, of which the wrap takes about 3 ms |
| Type a character at 50,000 lines, with the frame | about 0.8 ms |
| Scroll by a few rows at 50,000 lines, with the frame | about 0.7 ms |

The paint draws only the rows shown. The text of a row is shaped by the `ui` text system, which
caches shaped paragraphs; the editor does not keep its own shaping cache. Frame time with
highlighting depends on the spans shown and is not included in these figures.

## Limits

- Soft wrap wraps an edited line again from its start, so each keystroke on a line of several
  megabytes measures the whole line. A width change wraps every row at once, which takes about 3 ms
  at 100,000 lines (see Performance) and grows with the text.
- While an input method composes on a wrapped line, the line keeps the rows it had: the composed
  text is wrapped within them, and the last of them shows the rest. The rows are wrapped again when
  the composition is typed.
- The column that Up and Down keep is kept by the document, so two editors at different widths that
  show one document share it.
- With the find bar open, a change runs the search again over the whole text. The bar is the only
  path that copies the text on a keystroke.
- A wide character takes two cells in the editor's grid. If the font draws it at another width,
  the columns after it are drawn off the grid.
- CR LF text loads as LF, and is saved as LF by `Text()`.
- Translations: the find bar's messages are translated for Italian and Chinese where the
  translation is certain. The remaining messages are English in every language, as the `ui.Strings`
  packs describe.
- Binary size and memory: importing `codeedit` links wazero, the WebAssembly runtime (about 1.1 MB
  of code), and the highlighter's module (0.17 MB, all 76 languages). The session's copy of a
  document takes about five times its size in memory. wazero hashes modules with SHA-256, which links Go's
  FIPS crypto module and its 32 MB zero-filled buffer, so an app that imports `codeedit` reports
  about 32 MB more private bytes than one that does not (see BENCHMARKS.md).
