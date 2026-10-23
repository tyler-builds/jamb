# Feedback widgets

Feedback and status indicators, alerts, empty states, avatars, tags, skeleton placeholders and
keyboard shortcuts.

## Feedback

Progress bars, busy rings, toasts and badges. They use the theme's colors, follow
high contrast, the Text size and reduced motion, and show their state by a shape or
a word as well as by a color. Their looks are `ui.ProgressBarStyle`, `ui.ProgressRingStyle`,
`ui.BadgeStyle` (per variant in `Theme.Badges`), `ui.ToastStyle` (per kind in
`Theme.Toasts`, and per toast with `Toast.Styles`) and `ui.TooltipStyle`.

```go
ui.ProgressBar(0.45).Label("Upload").Color(accent)    // 0 to 1, or set .Min(lo).Max(hi); Color is the fill
ui.BadgeText("Active").Variant(ui.BadgeAccent).StatusDot() // a word with a dot in front
ui.ProgressBar(3).Min(0).Max(8).Label("Setup steps").
	Format(func(v float64) string { return fmt.Sprintf("%.0f of 8", v) })
ui.ProgressBar(0).Indeterminate().Label("Searching")      // work of unknown length
ui.ProgressRing().Size(ui.RingSmall).Label("Sending reply") // busy: RingSmall, RingMedium, RingLarge
ui.Badge(6).Noun("items").Variant(ui.BadgeAccent)     // a count; above Max (99) it shows 99+
ui.BadgeText("New").Variant(ui.BadgeSuccess)          // a word
ui.BadgeDot().Variant(ui.BadgeDanger)                 // a dot only
ui.Div().Label("Projects").Badged(ui.Badge(6).Noun("items")) // a badge on an element
ui.Toaster().Key("toaster")                           // once, as the last child of the window's root
ctx.Toast(ui.ToastSuccess, "Project created").Action("Undo", undo) // in a handler
ctx.ToastWithID("upload", ui.ToastInfo, "Uploading 20%").Title("Upload") // updates the shown "upload" toast
ctx.DismissToast("upload")                                              // removes it, if it is shown
```

- **Progress bar.** A determinate bar is a read-only range: its value is clamped to
  its range, and its readout is the percentage of the range, or what `Format` returns.
  An indeterminate bar slides across its track and repeats. With reduced motion it
  stays in the middle, and it never asks for frames.
  `Color` sets the fill for an app that has a color of its own; the default is the
  theme's accent. The default fill is 3:1 or more against the track in both themes.
  An app color is the app's responsibility: the fill is a graphical object and needs
  3:1 against the track (WCAG 1.4.11). Under high contrast the fill is Highlight, and
  `Color` is not used.
- **Busy ring.** Eight dots, four of them lit, turn around the ring. It has no value,
  so it has no range. With reduced motion it is drawn still: dimmed dots, or solid ones
  under high contrast. Its name is "Loading…" unless `Label` gives another. Its sizes
  (`RingSmall`, `RingMedium`, `RingLarge`) are fixed in pixels, as icon sizes are, so
  they do not grow with the Text size; an app that needs a larger ring passes a larger
  size.
- **Badge.** A count of zero draws nothing, so a badge can stay in place while its
  count is empty. `Variant` sets the color: `BadgeNeutral`, `BadgeAccent`,
  `BadgeSuccess`, `BadgeWarning` or `BadgeDanger`. Under high contrast a badge is
  outlined, and its text carries the meaning. A badge's text is dark on the bright
  Success and Warning fills, and light on the others, in both themes. Every variant's
  text is 4.5:1 or more against its fill in both themes, and under both high contrast
  schemes. `StatusDot()` puts a dot in front of a text badge, as a status chip has one; the dot
  is hidden from assistive technology.
- **Badge on an element.** `Badged` adds the badge to the end of the element and adds
  its text to the element's accessible name, as in "Projects, 6 items". The badge is
  then hidden from assistive technology, so the element is read once, with its count.
  A badge that stands alone is a group named by its text, such as "3 unread".
- **Toasts.** `ctx.Toast` shows a notification in the window's `Toaster`. It takes a
  variant: `ToastInfo`, `ToastSuccess`, `ToastWarning` or `ToastError`, each with its
  icon. The toast can add an `Action` button (running it dismisses the toast), set its
  own time with `Duration`, stay until dismissed with `Sticky`, or go with `Dismiss`.
  By default an info toast stays 5 seconds, a success toast 4, a warning toast 8, and an
  error toast until it is dismissed. Its time does not run while the pointer is over it
  or keyboard focus is in it. A shown toast asks for no frames while its time
  runs: the window is woken when the toast runs out. The stack shows 3 toasts at most (`Toaster().Max(n)`).
  When a new toast would pass the limit, the oldest toast that is neither hovered nor
  holding keyboard focus goes. A toast a user is reading or using is kept, and if every
  toast is in use the stack grows for the time being. The stack sits in the window's
  bottom-right corner over the content, so put the `Toaster` last in the root element.
  Its enter and exit motion jumps to its end under reduced motion. A window with no
  `Toaster` shows no toast; the first toast there is reported as a development warning.
- **Toasts and focus.** A toast does not take focus. F6 moves focus into the toast
  region, named "Notifications", and F6 again returns it to the element that had it,
  as Windows moves between the regions of a window
  ([Keyboard accessibility](https://learn.microsoft.com/en-us/windows/apps/design/accessibility/keyboard-accessibility)).
  Shift+F6 does the same. Tab reaches the actions and close buttons of the toasts.
  Escape on a toast dismisses it, and focus moves to the next toast or back to the
  element it had. Escape in the region returns focus. F6 toggling is what the framework
  does for now: full cycling through several regions (the sidebar, the header, the
  content, the toasts), as Microsoft's guidance describes, is future work.
- **Toast announcements.** Each toast is announced: politely for info and success,
  and assertively for warning and error. A toast is announced once per frame, so the
  updates made in one handler are read as their last text. Its group is named by its
  text, or by its title when it has one, and described by its text.
- **Keyed and titled toasts.** `ToastWithID` shows a toast under an ID the app
  chooses, or, when a toast with that ID is shown, replaces it in place: the toast
  keeps its place in the stack, its element, its hover and its focus, and its time
  starts again. Its text, variant, title and actions are those of the new call, so
  chain them again on each update. An update does not take room in the stack, and it
  is not announced again when its text is unchanged. The same kind of toast is
  announced at most once every 2 seconds: an update within that gap is read when the
  gap ends, and the latest text is the one read. A toast with an ID announces from
  its own source, so a newer message replaces the earlier ones still waiting to be
  read (Windows UI Automation's most-recent processing). `DismissToast` removes the
  toast with the ID, as its close button does, and does nothing when none is shown.
  `Toast.Title` gives a toast a semibold title above its text. `Toast.Dismiss` moves
  the focus on when the focus was in the toast, as the close button does.

## Alerts

An alert is a message box in the flow of the page, such as a note above a form or the
result of an action: an icon, a title, an optional description and action, and an
optional close button. Its look is `ui.AlertStyle`: per app in `Theme.Alert`, per variant
in `Theme.Alerts`, or per instance with `Styles`.

```go
ui.Alert("Sync paused").Description("Reconnect to resume.") // info, the default
ui.Alert("Payment failed").Danger().Description("Check the card details.").
	Action(ui.Button("Retry").Small().Secondary().OnClick(retry))
ui.Alert("Update ready").Success().OnClose(func() { shown.Set(false) }) // a close button
ui.Alert("Read only").Icon(icons.Lock)              // another icon; Icon(nil) shows none
```

- **Variants.** `Info()` (the default), `Success()`, `Warning()` and `Danger()`, or
  `Variant(v)`. Each has its icon: info, a check, and the alert icon for warning and danger.
  The box is tinted with the variant's color (Accent, Success, Warning or Danger), and the
  icon is drawn in that color. The title and the description are 4.5:1 or more on every
  variant's fill in both themes, and the icon 3:1 or more. Under high contrast the box is
  the window color outlined in the text color, and the icon is the text color, so the
  icon's shape and the text tell the variant.
- **Close.** `OnClose` adds a close button at the end, named with the `Close` text of the
  window's Strings. It works with the mouse, Enter and Space. The app removes the alert in
  `fn`, by changing the state it is built from.
- **Accessibility.** The alert is a group named by its title, with the description as its
  description. An alert with no title is named by its description. An alert is not a live
  region: it is not announced when it appears. An app that shows one after an action the
  user should hear about announces it too, with `ui.Announce`.

## Empty states

`ui.Empty(title)` is what a list, a table or a page shows when it has nothing to show: an
icon, a title, an optional description and an optional action, centered and muted. It
fits the empty slots of the widgets, such as `DataGrid.Empty(...)`, and the place of a
list or a set of cards with no items. Its look is `ui.EmptyStyle`: per app in
`Theme.Empty`, or per instance with `Styles`. It follows high contrast and the Text size.

```go
ui.Empty("No projects").Description("Create a project to get started.").
	Action(ui.Button("New project").OnClick(create))
ui.DataGrid(rows, key, cols...).
	Empty(ui.Empty("No matches").Icon(icons.Search).Description("Try another search."))
ui.Empty("Nothing here").Icon(nil)                  // no icon
```

- **Look.** The parts are centered in a column with 24 pixels of padding: a 32-pixel icon
  (an inbox by default; `Icon` replaces it) in the muted text color, the title semibold,
  the description muted and centered, and the action under them. Under high contrast the
  icon and the description are the text color.
- **Accessibility.** The empty state is a group named by its title, with the description as
  its description, and its action inside it. The icon is not exposed. An empty state is
  not announced when it appears; an app that empties a list after an action, such as a
  search, can say so with `ui.Announce`.

## Avatars, tags and skeletons

An avatar shows a person's initials, or an image, in a circle. Its fill is picked from its
name, so a name keeps its color. A tag is a small chip for a label or a filter, and its remove
button takes the tag away. A skeleton is a placeholder that pulses while content loads. They
use the theme's colors, follow high contrast, the Text size and reduced motion. Their looks are
`ui.AvatarStyle`, `ui.TagStyle` and `ui.SkeletonStyle`: per app in `Theme.Avatar`, `Theme.Tag`
and `Theme.Skeleton`, or per instance with `Styles`.

```go
ui.Avatar("Dana Lee")                                // "DL" on a color picked from the name
ui.Avatar("Dana Lee").Image(photo).Size(48)          // an image cropped to the circle
ui.Avatar("Dana Lee").AccessHidden()                   // beside the name, which is read already
ui.Tag("Sales").Icon(icons.Folder).OnRemove(remove)  // a chip with a button named "Remove Sales"
ui.Skeleton().Lines(3)                               // placeholder lines while content loads
ui.Skeleton().Circle(32)                             // a placeholder for an avatar
```

- **Avatar.** The initials are the first letter of the first and last words of the name, or one
  letter for a name of one word. An empty name shows an empty circle, named "Avatar". Without
  `AccessHidden`, the avatar is an image named by the name, so it is read as the person's name.
  `Tooltip(s)` shows s over the circle, such as the name in full, since an avatar has no visible
  text; assistive technology reads it as the description unless it is the name.
  `ui.AvatarInitials(name)` and `ui.AvatarColor(name)` return the initials and the fill an
  avatar of that name uses, for an app that shows the same person elsewhere.
- **Tag.** A tag is as wide as its text by default. Under `MaxWidth`, a long text ends with an
  ellipsis. `Color(c)` tints it for a category: a fill of c at about 14% alpha, a border of c
  at about 32%, and the icon in c; the text keeps the theme's text color, and high contrast
  ignores it. `OnRemove` adds a button that works with the mouse, Enter and Space. `Tooltip(s)`
  shows s over the chip, such as the full text of a truncated tag, and is read as its description.
- **Skeleton.** It is hidden from assistive technology. The app marks the region that is loading
  as busy, or labels it "Loading". It pulses about once every 1.2 seconds, between full and half
  opacity, and it is still under reduced motion and high contrast.

## Keyboard shortcuts

`Kbd` shows a keyboard shortcut as key caps, such as a hint beside a command or in a help
page. It takes the keys as `Bind` does, and names them as a menu shows its shortcuts, so
`"ctrl+shift+p"` draws the caps Ctrl, Shift and P, and reads "Ctrl+Shift+P". Its look is
`ui.KbdStyle`: per app in `Theme.Kbd`, or per instance with `Styles`.

```go
ui.Kbd("ctrl+shift+p")                 // three caps: Ctrl, Shift, P
ui.Kbd("ctrl+k ctrl+t")                // a sequence: the strokes are spaced apart
ui.Kbd("f2").Tooltip("Rename")         // read as the shortcut's description
```

- **Keys.** Each key is a cap of its own (the `Key` part). The caps of a stroke sit close
  together, and the `Gap` of the `Kbd` part spaces the strokes of a sequence. The key names
  are the ones `KeysFor` gives a menu, in English, in the order Ctrl, Alt, Shift, Super.
  A malformed keys string panics, as `Bind` does.
- **Look.** The caps are small, in the surface color, with muted text and a border that is
  heavier at the bottom. Under high contrast they are outlined in the text color.
- **Accessibility.** The shortcut is one static text named by the keys, such as
  "Ctrl+Shift+P". Its caps are hidden from assistive technology, so the shortcut is not
  read key by key.
