# Markdown

The `markdown` package shows Markdown as jamb elements: a read-only view of a
document in CommonMark, with the GitHub extensions for tables, strikethrough, task
lists and autolinks, and footnotes. The text is parsed with
[goldmark](https://github.com/yuin/goldmark), and the view builds elements from its
syntax tree; nothing is rendered to HTML. The package doc
(`go doc github.com/tyler-builds/jamb/markdown`) is the reference for every
method; this page covers how the pieces fit.

## Showing a document

```go
ui.Div().Grow(1).ScrollY().
	Child(markdown.View(readme).BaseDir(dir).MaxWidth(ui.Px(820)))
```

`markdown.View(src)` returns a `*markdown.ViewElement`, which is a `ui.Element` (it
embeds `ui.Composite`), so it goes straight into a tree. It is a column of blocks
and does not scroll by itself: put it in a scrolling container, as above. Give it a
`Key` when its place in the tree may change, since it keeps its state (the parsed
text, the loaded images, the selections of its text).

The view parses its text once, and again only when the text changes. A view built
on each render of its component costs a parse only when the text is new, as in a
preview that follows an editor (for a large document, see
[Live previews of large documents](#live-previews-of-large-documents)):

```go
ui.Component(func(ctx *ui.Context) ui.Element {
	return markdown.View(body.Get()).Label("Preview")
})
```

The package imports only `ui`, `access`, the standard library and goldmark. It links
no `crypto` package (the test in `app/nocrypto_test.go` checks it), no `net/http`
and no part of `codeedit`.

## What each element becomes

| Markdown | Element | For assistive technology |
| --- | --- | --- |
| Heading, `#` to `######` | a box with the heading's text | a heading of its level |
| Paragraph | a selectable rich text | text |
| `*em*`, `**strong**`, `~~strike~~`, `` `code` `` | italic, bold, struck and monospace segments of the paragraph's text | part of the text |
| Hard line break (two spaces or `\` at the end of a line) | a line break in the text | |
| Link, reference link, autolink, `www.` link | link segments: the `Link` color, underlined under the pointer; a link whose text mixes styles is still one link (see `ui.LinkSegs`) | a link named by its whole text, one Tab stop, which Enter follows |
| Bullet and ordered list, nested | a column of items with bullets or numbers (from the list's start number) | a list of list items, each with its position ("2 of 5") and its level (see Lists below) |
| Task list item, `- [x]` | an item with a check box | a list item with its position and level, holding a check box, checked or not, named by the task's text |
| Block quote, nested | a box with a line on its left | its blocks |
| Thematic break, `---` | a rule | a separator |
| Fenced or indented code block | monospace text that does not wrap, in a box that scrolls sideways | a group named "Code, go" (from the fence) with the code |
| Table | a grid, with the header row in bold and each column aligned as its delimiter row says | a table of rows and columns (see Tables below) |
| Image | the image at its size, at most as wide as the view | an image named by its alternative text |
| Footnote reference, `[^1]` | a link `[1]` to the note | a link |
| Footnotes | a rule and a numbered list at the end | a list named "Footnotes", each note with its position |
| HTML block or inline HTML | the HTML as it is written, as text | text |

A tight list spaces its items by the `List` part's gap; a loose list (with blank
lines between its items) spaces them as the document spaces its blocks.

### Lists

Each item of a list reports its position in its own list and the number of items in
it, so a screen reader says "2 of 5" as it moves through the items, and its level: 1
for a list that is not nested, 2 for a list inside an item of it, and so on. The
position counts the items, whatever number an ordered list starts at; the item's
number stays part of its text. A task is a list item with its position and level,
and its check box is a check box inside it, with its checked state.

### Tables

A table is a data grid with UI Automation's Grid and Table patterns, and its cells
have the GridItem and TableItem patterns: the patterns that a screen reader's table
navigation (NVDA's Ctrl+Alt+arrow keys, Narrator's table keys) reads to move by row
and column and to say each cell's column header.
The header row's cells are header items, the table's column headers; the other cells
are data items, each at its row and column, named by its text. The header row is not
counted among the table's rows, as in a `ui.DataGrid`: a table with a header and
three rows has 3 rows, and its first row of data is row 1. A link in a cell is still
a link of its own and a Tab stop. The table has neither a name nor a description: a
screen reader says that it is a table, and its size, from the Grid pattern.

## Options

| Method | Does |
| --- | --- |
| `Key(k)` | gives the view an identity among its siblings |
| `Label(s)` | names the view for assistive technology, as a group of that name |
| `Styles(markdown.Style{...})` | sets the parts of this view (see Parts) |
| `OnLink(func(url string))` | handles every link, in place of the view's own handling |
| `OnTaskToggle(func(index int, checked bool))` | makes the tasks' check boxes operable |
| `Images(func(src string) (*ui.ImageSource, error))` | sets the image loader |
| `BaseDir(dir)` | sets the directory of the default image loader |
| `Code(h markdown.CodeHighlighter)` | colors the code blocks (see Code blocks) |
| `Width`, `Height`, `MinWidth`, `MinHeight`, `MaxWidth`, `MaxHeight`, `Grow`, `GridSpan` | size the view's box, as the ui widgets' sizing methods do (see [Sizing](styling.md#sizing)); they win over the `Document` part |

`OnTaskToggle` calls its function with the task's index (0 for the first task of the
document, in the order of the text) and the state that a click asks for. The view
does not change the text: the function does, and the view shows the new text. The
demo's Markdown page rewrites the task's `[ ]` or `[x]` in its source. Without
`OnTaskToggle`, the check boxes only show whether each task is done, and do not
take focus.

## Links

A link is a segment of its paragraph's rich text. Tab stops on each link, in the
order of the text, a focus ring is drawn around its words, and Enter follows it; a
click follows it too. To assistive technology it is a link inside its text, also
inside a heading, a list item or a table cell.

Without `OnLink`, following a link:

- opens an `http`, `https` or `mailto` URL with the system, through
  `ui.Context.OpenURL`. Every other URL, relative paths and `file:` URLs included,
  is refused, and the link does nothing. A headless runtime opens nothing.
- moves to the heading for a `#name` link, when a heading of the document has that
  anchor. Anchors are made as GitHub makes them: the heading's text in lower case,
  without punctuation, with a hyphen for each space, and `-1`, `-2` after the second
  and later headings with the same anchor. The heading takes keyboard focus, which
  scrolls it into view; Tab does not stop on headings.
- moves to the footnote, for a footnote reference.

`OnLink` replaces the first two: its function gets every link's destination as it is
written in the text, `#name` anchors and relative paths included, and decides what is
safe to open. A footnote reference still moves to its note.

## Images

The view reads images with a loader, which is given each image's source as it is
written. The loader runs on a goroutine of its own, and the view shows a placeholder
with the alternative text until it returns. Each source is loaded once and kept by
the view.

The default loader reads local files only:

- A relative path is read from `BaseDir`, or from the working directory when no
  `BaseDir` is set. Its query and fragment are dropped (`logo.png?raw=true`), and
  its percent escapes are decoded (`my%20logo.png`).
- A source with a URL scheme is refused: `http`, `https`, `data`, `file` and the
  others. The view downloads nothing.
- A network path is refused: `\\server\share\a.png`, `//server/share/a.png`, and the
  device paths `\\?\` and `\\.\`. Windows opens a network path by connecting to its
  server, so refusing them keeps the loader off the network.
- Only regular files of up to 32 MB are read, in the formats of
  `ui.ImageFromBytes`: PNG, JPEG, GIF and WebP. A device, such as `CON`, is refused.

An image that cannot be shown is replaced by a box with "Image not shown:" and its
alternative text. An app that wants images from elsewhere, such as from its own
assets or a cache, sets its loader with `Images`; the demo's Markdown page returns
its embedded banner. A loader that downloads is the app's choice.

An image is shown at its size in pixels, as logical pixels, and is never wider than
the view. Images in the same paragraph, such as a row of badges, wrap in a row; an
image between words splits its paragraph, since a rich text holds no images. An image
inside a link is a link of its own, named by the image's alternative text, which Tab
reaches.

## Code blocks

Code blocks are in the monospace font (the `CodeBlock` part's `FontFamily` is
`"mono"`), keep their lines, scroll sideways when a line is longer than the view,
and are selectable. They are plain unless the view has a highlighter:

```go
import "github.com/tyler-builds/jamb/markdown/mdcode"

var code = mdcode.Highlight() // one for the app

markdown.View(readme).Code(code)
```

`mdcode` colors a block whose fence names a language that the code editor's
highlighter knows, by its name or an alias, in any case: one of the 76 languages of
@pierre/highlights, such as `go` or `golang`, `python` or `py`, `typescript` or `ts`,
`rust` or `rs`, `bash` or `sh`, `yaml` or `yml`, `csharp` or `c#`, `json` and `jsonc`
(`highlight.Languages` lists the names). The colors are the theme's `Syntax` styles,
which the code editor uses, matched by the token name and then by its shorter forms
("function.method" falls back to "function"). Under high contrast the theme's syntax
colors are the system colors.

Each block is lexed while the view renders, the first time a view shows it: a block of
a few kilobytes takes some microseconds, and a megabyte about 10 ms. Its colors are kept
by the highlighter, for up to 1024 blocks, so a render of the same text, or a new text
that keeps the block, lexes nothing again. The first block of all also compiles the
highlighter's module, once per process; the code editor keeps the compiled code in a
cache directory, so later runs start faster.

`mdcode` is a separate package so that only the apps that import it link the code
editor's highlighting: its WebAssembly module, the runtime wazero, which runs it, and
the `crypto` packages that wazero imports. An app that imports `markdown` alone
links none of them; `go list -deps` on it lists no `codeedit`, `wazero` or `crypto`
package.

An app can write its own highlighter: `markdown.CodeHighlighter` has one method,
`HighlightCode(ctx context.Context, lang, code string, ready func()) ([]markdown.CodeSpan, bool)`,
which the view calls while it renders. It returns at once: the ranges, or false while
it works on them, and then it calls `ready` from any goroutine, and the view asks
again. `ctx` is done when the view no longer shows the block: when the view goes
away, or when its text changes and the new text does not have the block. The
highlighter can then drop the work.

## Parts

The parts of a view are set with `ViewElement.Styles(markdown.Style{...})`, or for
every view of a theme with `ui.SetThemeStyle(theme, markdown.Style{...})` (see
[Styles of other widgets on a theme](theming.md#styles-of-other-widgets-on-a-theme)).
A part's style merges over the built-in defaults, then over the theme's
`markdown.Style`, then over the view's `Styles`, field by field. Under high contrast
the colors are the system colors, whatever the theme or the view sets; sizes,
spacing and fonts still apply. The inspector names the layers "built-in default",
"theme", "instance Styles" and "high contrast".

```go
for _, t := range []*ui.Theme{light, dark} { // set it on each theme the window uses
	ui.SetThemeStyle(t, markdown.Style{Heading1: ui.Style{FontSize: 30}})
}
view.Styles(markdown.Style{Paragraph: ui.Style{LineHeight: 1.6}})
```

| Part | Applies to | Default |
| --- | --- | --- |
| `Document` | the column of blocks | a gap of 12 between blocks |
| `Paragraph` | the text of paragraphs and HTML | line height 1.5 |
| `Heading1` to `Heading6` | the heading boxes and their text | 2, 1.5, 1.25, 1, 0.9 and 0.85 times the theme's text size, semibold; a line under levels 1 and 2; level 6 muted |
| `Link`, `LinkHover` | link text, and link text under the pointer (`TextColor`, `Background`) | the theme's `Link` color; underlined under the pointer |
| `InlineCode` | code in a line (`TextColor`, `Background`) | the text color on `SurfaceActive` |
| `CodeBlock` | the box and text of code blocks | the code editor's colors, a border, the theme's radius, padding 12, monospace |
| `Quote` | the box of block quotes | a 3-pixel line on the left, muted text |
| `List` | the column of a tight list's items | a gap of 4 |
| `ListMarker` | bullets and numbers | muted, at least 18 wide, aligned to the end |
| `Task`, `TaskChecked` | a task's check box, and its look when done (its `TextColor` is the check mark's) | a 14-pixel box with a border; the `Accent` fill when done |
| `Table` | the table's box; its `BorderColor` and `BorderWidth` are also the lines between cells | a 1-pixel `Border` line |
| `TableHeader`, `TableCell` | the header cells (over `TableCell`), and every cell | the header semibold on `Surface`; cells padded 10 by 6 |
| `Rule` | thematic breaks, a box of the rule's `Height` and `Background` | 1 pixel of `Border` |
| `ImagePlaceholder` | the box shown in place of an image | a bordered box with muted text |

No part changes the weight of its text for a state: hover, focus and selection
change colors only. Headings and strong text are bold by their content.

The parts are documented here and in the package doc. They are not listed in
`docs/styling.md`, so `TestEveryPartIsApplied` does not cover them.

## Words the view adds

The words that the view adds to a document are the `Markdown…` fields of the
window's `ui.Strings`: `MarkdownImage` (the name of an image without alternative
text), `MarkdownImageNotShown` (the placeholder's text), `MarkdownTask` (the name of
a task's check box when the task has no text), `MarkdownCode` and
`MarkdownFootnotes`. They are English in every built-in pack (see
[Translations](translations.md)).

## Performance

Measured on 2026-10-15 on the development machine (Intel Core i5-11600K, Windows 11),
with `go test -bench . -benchtime 2s ./markdown`, run three to five times; the ranges
are the lowest and highest results of those runs. The document is a generated README-like text with
headings, paragraphs, lists, tasks, tables, quotes and code blocks; 2,000 lines are
about 35 KB.

| Benchmark | Time | What it measures |
| --- | --- | --- |
| `BenchmarkParse` | 1.6 to 2.5 ms | goldmark's parse of 2,000 lines |
| `BenchmarkParseAndBuild/lines=300` | 2.0 to 2.2 ms | a new text of 300 lines: the parse, the build of every block's elements, their layout and a paint of a 900 by 700 window |
| `BenchmarkParseAndBuild/lines=2000` | 12.2 to 21.9 ms | the same for 2,000 lines; most runs took 12 to 15 ms, and one 21.9 ms |
| `BenchmarkScroll` | 0.055 to 0.063 ms | one frame of scrolling 2,000 lines by 120 pixels: nothing is parsed or built, and only the blocks in view are painted |

Scrolling a long document stays far below a frame's budget. A change of the text
builds the whole document again: about 2 ms for a 300-line text, and 12 ms or more
for a 2,000-line one, which is most of a frame at 60 Hz.

### Live previews of large documents

A preview that follows an editor should show a new text after a pause in typing, not
on every key, so typing in a long document stays smooth. Show the text at once when
the preview opens or shows another document, and wait about 150 ms after the last
edit otherwise. `ctx.After` runs a function on the window's clock, and its cancel
function restarts the pause. Keep the view in a component of its own, which reads
only the text it shows, so that a key re-renders the small watcher and not the view:

```go
func preview(body *ui.Signal[string]) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		shown := ui.Local(ctx, "shown", body.Peek()) // at once when the preview opens
		watcher := ui.Component(func(ctx *ui.Context) ui.Element {
			text := body.Get() // each key re-renders this component only
			pause := ui.Local(ctx, "pause", &struct{ cancel func() }{func() {}}).Peek()
			pause.cancel() // a pause that was pending starts again
			pause.cancel = func() {}
			if text != shown.Peek() {
				pause.cancel = ctx.After(150*time.Millisecond, func() { shown.Set(body.Peek()) })
			}
			return nil
		})
		return ui.Div().ScrollY().Child(watcher).Child(ui.Component(func(ctx *ui.Context) ui.Element {
			return markdown.View(shown.Get())
		}))
	})
}
```

The notes example does this (`examples/notes/views/editor.go`, with its test in
`preview_test.go`), and cancels the pending pause in the watcher's `OnDispose`. The
highlighter's colors are kept by language and code, so an update after a pause
parses only the code blocks that changed.

## Limits

- Selection is per block: a drag selects within one paragraph, heading, cell or
  code block, not across blocks.
- An image keeps its height when the view is narrower than the image, and is
  letterboxed in its box. Image titles are not shown.
- An image inside a paragraph splits the paragraph's text at the image.
- HTML is never interpreted: blocks and inline HTML are shown as written, and HTML
  comments are left out.
- The way back from a footnote to its reference (the "↩" of GitHub) is not shown.
- Without `OnLink`, relative links do nothing, since the view does not know what to
  open them with. A `#name` link moves only to a heading of the same document.
- The view does not edit its text: `OnTaskToggle` reports the click, and the app
  changes the text.
- The extensions are those of GitHub listed above, and footnotes. Definition lists,
  math and front matter are not parsed: front matter is read as Markdown, so its
  `---` lines are rules or heading underlines.
