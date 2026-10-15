# Selection and date widgets

The combobox and the date and time pickers.

## Combobox

A combo box is an editable text field with a list of suggestions below it: `ui.Combobox`.
As the user types, the list opens with the suggestions for the text. The text is the
signal the field reads and writes, as `TextInput` does. The suggestions come from a
function of the query, or from a background function with `SuggestAsync`.

```go
// Suggestions for the text, computed on the UI thread at each change.
ui.Combobox(city, func(q string) []ui.Option[string] { return citiesMatching(q) }).
	Label("City").Placeholder("Type a city").
	OnPick(func(v string) { ... }) // v is the value of the suggestion chosen

// Only the suggestions can be chosen: the text reverts on blur (see AllowFree).
ui.Combobox(fruit, fruits).Label("Fruit").AllowFree(false).Validate(ui.Required[string]())

// A background search with a debounce, and the spinner while it runs.
ui.Combobox[string](q, nil).Label("Address").
	SuggestAsync(func(ctx context.Context, q string) ([]ui.Option[string], error) { ... }).
	Debounce(150 * time.Millisecond).MinChars(2)

// The header search of the demo: values are project IDs, and each row is a render slot.
ui.Combobox(s.Query, searchOptions(s)).Label("Search projects").
	RenderOption(searchOption(s, t)).OnChange(...).OnPick(func(id int) { ... })
```

The options are `ui.Option[T]`, as they are for a select: the value is what `OnPick`
receives, the label is what the text shows and what the suggestions match. The methods:

| Method | What it does |
| --- | --- |
| `OnPick(fn func(T))` | runs with the value of the suggestion chosen by a click or by Enter; the text is set to its label first |
| `OnChange(fn func(string))` | runs with the text after each change the user makes (not for a chosen suggestion) |
| `AllowFree(bool)` | free text is allowed (the default); `AllowFree(false)` chooses only from the suggestions (see below) |
| `MinChars(n)` | the list stays closed until the text has n characters (default 0) |
| `Debounce(d)` | waits for the user to stop typing before asking for suggestions (default: at once) |
| `SuggestAsync(fn)` | finds the suggestions in the background; see below |
| `MaxVisible(n)` | the list is at most n rows high (default 8) and scrolls to the rest; the highlighted row is scrolled into view |
| `ErrorText(s)` | the text of the row shown when an async suggest function fails (default "Couldn't load suggestions") |
| `EmptyText(s)` | the text of the empty slot (default "No results") |
| `RenderOption(fn)` | replaces the label of each suggestion; the label stays the accessible name, and the content is hidden from assistive technology |
| `Validate(rules...)` | the validators of the text, as TextInput has them; see Forms |
| `Placeholder`, `Icon`, `Label`, `LabelledBy`, `Key`, `Disabled`, `FocusHandle` | as for TextInput |
| `Width`, `Height`, `MinWidth`, `MinHeight`, `MaxWidth`, `MaxHeight`, `Grow`, `Styles` | the sizing methods and the look of the parts (see Sizing and Styling) |

The match of the query in each label is highlighted (`HighlightMatches`), so the
label shows which part matched. Its look is the `Match` part of `ui.ComboboxStyle`.
A suggestion that is disabled (`Disabled: true`) is shown and cannot be chosen.

**Keys.** The field follows the combo box pattern of the WAI-ARIA Authoring Practices
(https://www.w3.org/WAI/ARIA/apg/patterns/combobox/). Focus stays in the text while
the list is open, and the highlighted suggestion is the one the keys have reached.

- Typing filters the suggestions and opens the list, with nothing highlighted.
- Down opens the list when it is closed, and moves the highlight to the next suggestion,
  without wrapping. Up moves it back, and from the first suggestion returns to the text.
- Enter chooses the highlighted suggestion and closes the list. With no suggestion
  highlighted, Enter keeps the text as it is and closes the list. When the list is
  closed, Enter is the field's own Enter (in a Form, it submits).
- Escape closes the list. When the list is closed and the text is not empty, Escape
  clears the text (the APG permits this as an option); when the text is empty, Escape
  leaves the field, as it does in a text field.
- Tab closes the list and does not choose the highlighted suggestion. The APG does not
  define Tab while the list is open; the choice here is that Enter is the only key that
  chooses, so a Tab through the field never changes the text.
- Alt+Down opens the list without moving the highlight, and Alt+Up closes it.
- Home and End move in the text.
- A click on a suggestion chooses it and keeps focus in the text. A click in the text opens
  the list when it is closed, and never closes it: the press places the caret.
- Home and End are not keys of the combo box in the APG, so they edit the text. Page Up and
  Page Down are not defined for the combo box either, and are not bound. The list scrolls
  with the highlight, and with the mouse wheel.

The keys are actions of the field (see Actions and key bindings). Up and Down are the
field's while it is enabled, since they open the list. Enter is the field's only while
the list is open, and Escape while the list is open or the text is not empty; otherwise
they are the text field's own keys.

**Free text and choices.** With free text (the default), the text stays as the user
leaves it, and `OnPick` runs only for a suggestion that is chosen. With `AllowFree(false)`,
the field chooses only from its suggestions: when the text loses focus, a text that is
not the label of the suggestion last chosen is reverted to that label, or cleared when
nothing was chosen. A suggestion whose label matches the text exactly (ignoring case) is
chosen in its place. The revert happens when the field loses focus, so a Form submitted
from another control checks the reverted text, and `Required` makes an unchosen
restricted field an error. The change is announced politely and shown under the field, as
"Not in the list; cleared" or "Not in the list; reverted to Apple"; the same text is the
field's description. A text that is empty when the field is left is reverted without a message. A Form submitted by Enter in the field checks the text as it
is at that moment.

**Background suggestions.** `SuggestAsync` runs the function on its own goroutine, with
a context that is canceled when a newer query starts. Its results are shown only when
they are for the latest query; an older result is dropped, so a slow answer cannot
replace a newer one. While a request runs, the list shows a `ProgressRing`. An error
shows the error row (`ErrorText`), and the text is announced politely. The field stays
usable: its text is still free, and editing it asks again. There is no retry button in the
list, since the keys do not reach it. Return promptly when the context is canceled.

**Accessibility.** The field is a ComboBox with ExpandCollapse, its text is an Edit with
the Value pattern, and the suggestions are a List of ListItems with SelectionItem. Expand and
Collapse from assistive technology run the field's own handlers, not its click. Every
suggestion is in the list and in the accessibility tree: the rows are not virtualized, so a
suggest function should narrow its results to a few dozen. The
highlighted suggestion is reported as the focused element, so a screen reader announces
it while the focus stays in the text (the active descendant of APG; see
docs/accessibility-design-phases-i-m.md, Phase L). When the suggestions change, their count is announced
politely, after the changes settle: "5 suggestions", "1 suggestion", or "No results".
Microsoft's AutoSuggestBox guidance (https://learn.microsoft.com/windows/apps/develop/ui/controls/auto-suggest-box)
describes keyboard navigation through the suggestion list, with the text box updated
from the highlighted item. It does not describe the APG's model, in which DOM focus
stays in the combobox; the APG model is the one used here.

## Date and time

`ui.DatePicker` is a text field for a day, with a calendar below it, and `ui.TimePicker` is a
text field for a time of day, with a list of times below it. Both read and write a
`*ui.Signal[time.Time]`. The text is in the short date and clock format of the user's
locale, and the value is set when the text is a date or a time that can be chosen.

```go
var due = ui.NewSignal(time.Time{}) // a zero time is no date

ui.DatePicker(due).Label("Due date").Placeholder("MM/DD/YYYY").
	Min(today).Max(today.AddDate(0, 0, 90)).   // the days that can be chosen
	DisableDays(func(d time.Time) bool { return d.Weekday() == time.Saturday || d.Weekday() == time.Sunday })

var start = ui.NewSignal(time.Time{}) // a zero time is no time; only the clock is read
ui.TimePicker(start).Label("Start time").MinuteStep(15). // the list has a time every 15 minutes
	Min(eight).Max(eighteen)

// A date and a time on one value: each picker keeps the part it does not set.
ui.DatePicker(when).Label("Meeting date")
ui.TimePicker(when).Label("Meeting time")
```

The value of a date picker is a `time.Time` whose day is the chosen day. Its time of day is
kept when the day changes, and a date picker set from the signal shows only the day. The value
of a time picker is a `time.Time` whose clock is the chosen time. Its day is kept when the time
changes, and a new time takes today's day when the value has none. So a date picker and a time
picker on one signal are a date and time picker, with no third widget. Set the value with
`Signal.Set`; `OnChange` runs only for a change the user makes.

| Method | What it does |
| --- | --- |
| `DatePicker`: `Min(t)`, `Max(t)` | the earliest and the latest day that can be chosen; the time of day is ignored (a zero time is no limit) |
| `DatePicker`: `DisableDays(fn)` | rules out the days for which `fn` returns true; they are drawn dimmed, marked unavailable, and refused when typed |
| `DatePicker`: `FirstDay(d)` | the first day of the week in the calendar (default: the locale's) |
| `TimePicker`: `MinuteStep(n)` | the minutes between the times of the list, and the step of Up and Down (default 15) |
| `TimePicker`: `Min(t)`, `Max(t)` | the earliest and the latest time that can be chosen; only the clock is used |
| both: `Validate(rules...)` | validators of the value, such as `ui.Required[time.Time]()`; a zero time is no date or time |
| both: `OnChange(fn)`, `Placeholder(s)`, `Label(s)`, `LabelledBy(keys...)`, `Disabled(bool)`, `Key(k)`, `Styles(st)` | as for the other fields |
| both: `Width`, `Height`, `MinWidth`, `MinHeight`, `MaxWidth`, `MaxHeight`, `Grow` | the sizing methods (see Sizing in docs/styling.md); a date picker is 180 px and a time picker 140 px at the Text size of 100 percent |

`DisableDays` is the name of the rule because `Disabled(bool)` disables the whole field,
as it does for every control.

**Typing.** A date is read in the order of the locale's short date (month, day and year in
the US, day, month and year in most of Europe, year first in some), with any punctuation
between the numbers. A year of two digits is read in the century of the current year, or the
one before when the result would be more than 20 years ahead. A date with no year is in the
current year. Eight digits, such as `10132026`, are read in the same order. Month names are not
read. A time is read with its hour and minute separated by punctuation or a space, or with the
digits run together, such as `930` or `1430`; `AM` and `PM` (or `a` and `p`) make the hour a
12 hour one, and without one the hour is on the 24 hour clock. Enter applies a text that is
valid; a text that is not is kept with a message that gives an example, and the value does not
change. Leaving the field applies a valid text and otherwise shows the value again, as a number
field does, so a form never submits a text the field does not hold. An empty text is no date
or time.

**The calendar and the list.** Alt+Down and F4 open the calendar of a date picker, with the
keys on the chosen day, or on today when there is none. Its keys follow the WAI-ARIA date
picker dialog pattern (https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/examples/datepicker-dialog/):

| Key | Moves or does |
| --- | --- |
| Left, Right | one day back or forward |
| Up, Down | one week back or forward |
| Home, End | the first and the last day of the week (the week starts on the locale's first day) |
| Page Up, Page Down | one month back or forward; the day stays, or is the last day of a shorter month |
| Shift+Page Up, Shift+Page Down | one year back or forward |
| Enter, Space | choose the day, and close the calendar; focus returns to the field |
| Escape | close the calendar without a change; focus returns to the field |
| Tab | close the calendar without a change, and move on from the field |

A day that cannot be chosen is still reached by the keys, so the screen reader can say it is
unavailable, but Enter does not choose it. The month changes with the keys, and its name is
announced politely. The navigation buttons of the calendar are for the pointer; the keys above
move a month and a year.

For a time picker, Up and Down step the time by the minute step while the list is closed, and
move the highlight through the list while it is open (Down to the next row, as the rows show).
Alt+Down and F4 open the list with the time the field holds highlighted. Enter chooses the
highlighted time, or applies the typed text when no time is highlighted. Escape and Tab close
the list. Focus stays in the text, as it does in a combo box.

**Locale.** On Windows, the short date pattern, the first day of the week, whether the clock is
12 or 24 hours, the AM and PM designators, the time separator, and the names of the days and the
months come from the user's locale settings, read with `GetLocaleInfoEx` and `GetDateFormatEx`.
Elsewhere, and where those settings cannot be read, the formats are those of en-US. The locale is
read once. The month and day names of the calendar and of the accessible names follow it.

**Accessibility.** A date picker and a time picker are ComboBoxes for assistive technology, with
Expand and Collapse for the calendar or the list, and a Value that is the text. The calendar is a
Calendar (`RoleCalendar`, UI Automation's Calendar control type, with the Grid and Table
patterns). Its weekday names are header items, each day is a data item in a grid of six week
rows and seven columns, named by its long date (such as "Tuesday, October 13, 2026"), and the
chosen day is selected. A day that cannot be chosen is disabled, and today has the description
"Today". See docs/accessibility-design-phases-i-m.md, Phase M.

**Styling.** `DatePickerStyle` and `TimePickerStyle` are the looks of the parts, as the other
widgets' styles are, and `Theme.DatePicker` and `Theme.TimePicker` are the defaults of every
picker in the app. The parts are in docs/styling.md. Under high contrast the selected day is
drawn in the system Highlight colors, today and the focused day are outlined, and no state is
shown by a change of weight.

**Limits.** Months are not read from names in the text. Only the calendar's month and year
buttons are for the pointer, and the calendar closes on Tab rather than keeping focus. The
date picker has no time of day of its own, so a time is set with a time picker on the same
signal. The Today description is in English. Text areas for dates and times are not provided.

**Date ranges.** `ui.DateRangePicker` is a text field for a range of days, with a calendar below it. It
reads and writes two signals, the start and the end, each a `*ui.Signal[time.Time]`; a zero time is no
day, and a zero start is no range.

```go
var start = ui.NewSignal(time.Time{}) // the first day of the range
var end = ui.NewSignal(time.Time{})   // the last day of the range

ui.DateRangePicker(start, end).Label("Trip dates").
	Min(today).Max(today.AddDate(0, 6, 0)). // the days that can be chosen
	DisableDays(func(d time.Time) bool { return d.Weekday() == time.Sunday })
```

- The first day chosen is the start, and the calendar stays open; the second is the end, and it closes
  the calendar. The days are at local midnight. If the end is before the start, the two are swapped.
  Hovering or moving with the keys shades the range before the end is chosen.
- The signals change only when the end is chosen. Escape, Tab and a click outside the calendar keep the
  range that was there, and `OnChange(fn)` runs with both days only then.
- Typed text is `date – date` or `date - date`, each date in the locale's short date format. Each date is
  read as a date picker reads one, with the same messages, and reversed dates are swapped.
- A day that `Min`, `Max` or `DisableDays` rules out cannot be an end, and typing it is refused. A range
  may include such a day.
- The ends are filled with the accent. The days between them are a band with a line above and below and
  square corners, so the range is not shown by its fill alone; under high contrast the lines are the text
  color.
- The field is a combo box for assistive technology, with the range as its value. Each day is named by its
  long date, and its description is "Start of range", "End of range" or "In range".
- The keys are those of `DatePicker`. `Placeholder`, `Disabled`, `FirstDay`, `Key`, `Label`, `Small`,
  `Large`, `Tooltip`, `Styles` and the sizing methods are as for a date picker; a range is 280 px wide at
  the Text size of 100 percent. Its look is `DateRangePickerStyle`, and `Theme.DateRangePicker` sets it for
  every range picker (see docs/styling.md).
- The calendar shows one month at a time. A range has no `Validate`; a form reads the two signals.
