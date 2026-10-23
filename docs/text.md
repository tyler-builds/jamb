# Text

Rich text, selectable text, fonts and emoji. Text fields and input methods are in [input.md](input.md).

## Rich text

`ui.Rich` joins segments into one text element. It wraps, truncates and
aligns like `ui.Text`, and it is one node, so a row of mixed styles stays one
line when it is `Truncate()`d.

```go
ui.Rich(
    ui.Seg("Alex").Style(ui.Highlight{Weight: ui.FontSemibold}), // or .Bold()
    ui.Seg(" pushed to ").Color(t.TextMuted),
    ui.Link("Project Beta", openProject),   // link color; underlined on hover; a click or Enter runs openProject
    ui.Seg(" main").Mono().Background(t.Surface),
).Truncate()

// Ranges of an existing text, by byte offset (use strings.Index on the same string):
ui.Text(body).Highlight(start, end, ui.Highlight{Background: t.AccentSoft, Color: t.Text})
// Every case-insensitive match of a search query (a blank query does nothing):
ui.Text(body).HighlightMatches(query, ui.Highlight{Background: t.AccentSoft, Color: t.Text})
```

- `Highlight` has `Color`, `Background`, `Weight`, `Italic`, `Underline`,
  `Strikethrough` and `Mono`. Zero fields keep the text's own style. Later
  highlights win field by field over earlier ones and over segments.
  Offsets outside the text, or inside a UTF-8 sequence, panic and name the
  offset and the text.
- `HighlightMatches(query, h)` applies `h` to every case-insensitive match of
  `query`, found by rune comparison so the offsets stay valid for text whose
  lowercase form differs in length (İ, the Kelvin sign). A blank query leaves
  the text as it is. On a Rich text it searches the joined text.
- `Segment.Hover(h)` applies `h` while the pointer is over the segment. Only
  the paint fields (`Color`, `Background`, `Underline`, `Strikethrough`) apply;
  weight, italic and mono in a hover are ignored, so hovering never re-lays
  out the text.
- `OnClick` (and `Link`) make a segment clickable: the pointer becomes a hand
  over it. A link inside a clickable row gets the click; the row's `OnClick`
  runs only for the rest of the row.
- A clickable segment is a link for the keyboard and for assistive technology.
  Tab stops on each one, in the order of the text, a focus ring is drawn around
  its words, and Enter runs its `OnClick`. Screen readers find it as a link
  inside the text, also when the text is inside a control that is named by its
  text, such as a heading or a list item.
- `ui.LinkSegs(fn, segs...)` makes one link of several segments, for a link whose
  text mixes styles: `ui.LinkSegs(open, ui.Seg("the "), ui.Seg("guide").Bold())`.
  Placed one after another in a `Rich` text, they are one Tab stop, one focus ring,
  one link named "the guide" for assistive technology, and they underline together
  under the pointer.
- `ui.Text(...).Italic()` and segment `Italic()` use the family's italic face.
  A family without an italic face for that weight uses its upright face: there
  is no synthetic slant. Segoe UI and Consolas have italics on Windows; the
  embedded Go fonts are the fallback elsewhere.
- An app ships its own fonts with `app.New().Font(family, weight, data)`, once per
  face, and uses them with `Font(family)` on a `Text` or `FontFamily` in a `Style`:

  ```go
  //go:embed fonts/Inter-Regular.ttf
  var interRegular []byte

  app.New().Font("Inter", ui.FontRegular, interRegular).Run(app.Window{...})
  ```

  The fonts are parsed while the first window opens; data that is not a font makes
  `RunE` return an error. A registered family has upright faces only, so italic text
  in it uses the upright face.
- A weight the family has no face for uses the nearest face, and the heavier
  one on a tie, as Windows' DirectWrite does. Medium (500) in Segoe UI, which
  has Regular and Semibold faces, is drawn Semibold. The family's own faces
  always come before the embedded Go fonts, so text never switches typeface
  because of its weight.
- Rich text is display-only. `TextArea` stays single-style.

## Selectable text

Text and rich text are not selectable by default, as labels in Windows apps are
not. `Selectable()` makes one selectable, for error messages, details panes and
anything users copy from:

```go
ui.Text(err.Error()).Selectable()
ui.Rich(ui.Seg("Order "), ui.Seg(id).Mono(), ui.Seg(" shipped")).Selectable()
```

Drag selects, a double-click selects a word, a triple-click a paragraph, and
Shift+click extends. A click puts keyboard focus on the text, without making it a
Tab stop, so `Ctrl+A` selects all and `Ctrl+C` or `Ctrl+Insert` copies; the actions
are `text.select-all` and `text.copy` in the `SelectableText` key context. The
selection is drawn in the text fields' selection colors and cleared when focus
moves away. A click on a link in selectable rich text still follows the link.
Selection stays within one text element; it does not run across paragraphs.

## Relative time

`ui.FormatRelative(t, now)` returns how far `t` is from `now` in the user's language,
such as "3 days ago" or "in 3 hours". A time within a minute is "now", and the
calendar days before and after today are "yesterday" and "tomorrow". Minutes and
hours count from the time; days count by the calendar, so a clock change does not
shift a day. A week starts at 7 days, a month at 30 and a year at 365.

`ui.TimeAgo(t)` shows the same text from the window's clock and renders again as time
passes: every minute under an hour, every hour under a day, and daily after that. Its
tooltip and accessible description are the full date and time (`ui.FormatDate` and
`ui.FormatTime`). An off-screen one keeps no timer, and it catches up the next time its
parent renders it.

```go
ui.TimeAgo(commit.When).Key("when")
ui.Text(ui.FormatRelative(commit.When, time.Now())) // a fixed text, not refreshed
```

The words are in `ui.Strings`: `RelativeNow`, `RelativeYesterday`, `RelativeTomorrow`,
and the functions `RelativeAgo` and `RelativeIn`, which take a `RelativeUnit`
(`RelativeMinute` to `RelativeYear`) and the count. The English, Italian and Chinese
forms follow the CLDR relativeTime long forms (CLDR 48.2). Chinese has no plural, so
its words do not change with the count. `TimeAgo` uses the window's `Strings`;
`FormatRelative` uses the system's language (see `ui.SystemStrings`).

## Numbers and sizes

`ui.FormatNumber(v, decimals)` returns `v` rounded to `decimals` places, with the group
separators, the decimal point and the minus sign of the user's locale: `12,345.68` in
en-US and `12.345,68` in de-DE. It rounds as Windows does: half away from zero, on the
digits of the shortest decimal form of `v`, so `1.005` at two places is `1.01`. A value
that rounds to zero has no minus sign. A NaN is the empty string, and an infinity is `∞`
or `-∞`.

`ui.FormatCurrency(amount)` returns an amount of money in the currency of the user's
region settings, as Windows formats it: rounded to the currency's digits, with the
decimal point and group separator of amounts of money, and the symbol and the sign
where the settings put them. That is `$1,234.56` and `-$5.00` in en-US, `1.234,56 €` and
`-5,00 €` in de-DE, `¥1,235` in ja-JP, or `($5.00)` where the settings show negative
amounts in parentheses; Windows has four positions for the symbol and sixteen shapes
of a negative amount, and each is followed. `ui.CurrencySymbol()` returns the symbol and
whether it goes after the amount. Elsewhere than on Windows the currency is the US
dollar. A field for an amount of money is a `TextInput` with
`NumberFormat(NumberOptions{Currency: true})` (see [Form controls](widgets-forms.md)).

`ui.FormatBytes(n)` returns a size in bytes as Windows Explorer and Task Manager show
it, in 1024-based units with three significant digits: `512 bytes`, `1.00 KB`,
`12.3 MB`, `123 MB` and `1.23 GB`. A size of 1000 units or more is shown in the next
unit, as Windows does, so `1024000` is `0.97 MB`, and the digits are cut, not rounded, so
`1048575` is `0.99 MB`. The sizes go up to PB. The decimal point is the locale's, as in
`FormatNumber`. Unlike Windows, one byte is `1 byte`.

```go
ui.Text(ui.FormatBytes(p.ResidentBytes) + " in use")
ui.Text(ui.FormatNumber(float64(p.Handles), 0) + " handles")
```

The words are in `ui.Strings`: `Byte`, `Bytes` and `KB` to `PB`. Every built-in pack sets
them to the English words for now. `FormatBytes` uses the system's language, as
`FormatRelative` does (see `ui.SystemStrings`).
