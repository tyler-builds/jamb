# Translations

Languages and translated strings.

## Languages

Every word that jamb shows or announces is in a `ui.Strings` value: the names and
labels of the controls, the picker words, the validation messages and the
announcements. Errors and diagnostics for the developer stay English. By default a
window uses the display language of Windows (its first preferred UI language, not the
region format, which the date and time pickers follow). A language without a pack is
English.

```go
app.Window{
    Title:   "Notes",
    Root:    root,
    Strings: ui.StringsFor("it-IT"), // nil: the Windows display language
}
```

`ui.StringsFor` returns the pack for a BCP 47 tag: `zh-CN` (Simplified Chinese, the
default for any other `zh` tag), `zh-TW` and `zh-HK` (Traditional Chinese for Taiwan,
and for Hong Kong and Macao), and `it`. The packs translate the short words and
phrases that have an unambiguous translation, such as the page buttons, "Required"
and the window buttons, and some short messages that take a value, such as "Remove
Sales". The longer sentences, such as the date and range messages, are English in
every language. A field that a pack leaves empty is English, so a partial pack is
never incomplete.

The words of the charts (package `charts`) are the `Chart…` fields: the names of the
chart types, the summary that names a chart for assistive technology, the
announcements of the keyboard walk, the lines of the tooltips, the legend's name and
the headers of `DataTable`. No pack translates them, so they are English in every
language until an app sets them. The summary is built from whole clauses, such as
`ChartSeriesSummary` ("Line chart, 2 series, 7 points each") and
`ChartPositionRange`, which takes the summary so far and adds "from Monday to
Sunday", so a language sets its own order, punctuation and plurals. The values in
them (numbers, dates, series names) come from the chart's data and its `Format`.

The words that the Markdown view (package `markdown`) adds to a document are the
`Markdown…` fields: the name of an image without alternative text, the placeholder
of an image that is not shown, the name of a task's check box, and the names of
tables, code blocks and the list of footnotes. They are English in every language
until an app sets them too. The text of the document itself is the app's.

To change a message, start from `ui.EnglishStrings()` and set the fields you need.
Each message with values is a function, so each language can set its own word order
and plurals. A field that is empty or nil shows the English text. Use
`ctx.Strings()` inside a render function to read the text, and `Runtime.SetStrings` to
change it at run time:

```go
s := ui.EnglishStrings()
s.Required = "Please fill in this field."
s.PageOf = func(page, pages int) string { return fmt.Sprintf("%d / %d", page, pages) }
// in app.Window: Strings: s
```

The month and weekday names of the date pickers come from Windows, in the user's
locale, and are not in `ui.Strings`. Strings translated by gpui-component are credited
in the NOTICE file.
