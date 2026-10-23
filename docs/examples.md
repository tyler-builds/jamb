# Examples

The demo app, the notes app, the Git app and the monitor in this repository.

## The demo app

`examples/demo/` shows the widgets, the layout and the paint features on sample pages.
On the Projects page:

- Drag a project card to reorder projects. Drop it on the upper or lower half
  of another card to place it before or after that card; a 2 px line shows
  where. The order is kept even while a status filter hides some projects.
- The status filter on the Projects page is a segmented tab list (All, Active,
  Paused, Done). Drop a project on Active, Paused or Done to set its status; the
  drop target is the tab's content, drawn by `RenderTab`.

![The Projects page with its segmented status filter](screenshots/projects.png)
- On the Activity page, the project name in each row is a link: clicking it
  opens the Projects page with that project selected.
- Tab walks the sidebar navigation (each item shows a focus ring), and Enter
  or Space opens the focused page.
- The Projects page header uses an Oklab gradient on the logo tile.
- Clicking the "Open in window" button on a project card opens the project in
  its own window, which has an Overview tab and a Comments tab. A card whose
  window is open shows an "Open in window" badge until that window closes.
- The Activity page has a filter field (matches are highlighted in each row),
  rows of different heights in a `ListOf`, and build rows that expand on a
  click, or on Enter or Space when focused.
- Right-click a project card for its menu: Open, Open in window, Star (a check
  mark), a Status submenu with radio items, and a disabled Archive item.
- The Components page (Ctrl+4) shows the controls, overlays, gradients, rounded
  clipping, per-side borders, motion and focus styles, one section each. A filter
  bar above the sections stays in view as they scroll: it shows only the sections
  whose title or keywords (the API names they show, such as `Stepper` or `Kbd`)
  contain every word typed, and Ctrl+F focuses it. Its
  Drawing and layout section has a canvas chart that follows the pointer and zooms
  with a pinch, SVG
  images, selectable text and a form laid out as a grid. Its More widgets section
  has a sidebar, breadcrumbs, pagination, avatars, tags, skeletons, a color
  picker and a date range picker. Its Clipboard images section copies a sample
  image with transparency for pasting into another program, and pastes images
  with a button or with Ctrl+V into a text area (`OnPasteImage`), showing them
  over a checkerboard. Its
  Menus section has a dropdown and a context menu with a submenu; their shortcuts
  come from the demo's bindings.
- The Components page's **Form controls** section shows each form control in its
  normal and disabled states, with a readout of its value, and a notification
  preferences form that uses several of them together. Its "All alerts" check box
  is the parent of the three alerts, and is mixed when only some are on.
- The Components page's **Feedback** section shows progress bars (determinate, with
  a range, and indeterminate), busy rings in three sizes, a toast button for each
  variant, a toast with an Undo action, and the badges. Its toasts appear bottom-right.
- Creating a project shows a "Project created" toast with an Undo action, which
  removes the project. The bell in the header counts the unread notifications; it
  lists the recent ones, and "Mark all read" clears the count. Every toast the app
  shows is also in the list.
- The sidebar's Projects item shows the project count as a badge. The project cards
  and the project window show each project's progress as a progress bar, and each
  status as a badge. A teammate's reply in a project's comments shows a "typing"
  busy ring until it arrives.

![The Form controls section of the Components page, with its number inputs and the Notification preferences below it](screenshots/form-controls.png)

![The open select in the Form controls section](screenshots/select-open.png)

- The Components page's **Navigation and layout** section shows tabs (plain, one
  with a disabled tab, and one with icons), a collapsible section, an accordion
  with one section open at a time, a small project file tree and a tree of 10,000
  generated folders. The large tree builds only the rows in view.

![The Navigation and layout section of the Components page](screenshots/navigation-layout.png)

![The tree views in the Navigation and layout section](screenshots/tree-view.png)

- The project window has its details on an **Overview** tab and its thread on a
  **Comments** tab.

![The Overview tab of a project window](screenshots/project-window-tabs.png)

![The Comments tab of a project window](screenshots/project-window-comments.png)

- The Components page's **Customizing** section shows how a built-in widget is
  styled without rebuilding it: the demo's theme is a copy of the built-in theme, with a
  3 px line under every underline tab, one tab list has an instance override in
  Success, and the segmented and stacked variants and two render slots (a count
  in a tab, and a color swatch in a select) are next to them. See
  [docs/styling.md](styling.md).

![The Customizing section of the Components page, with the stacked spin buttons under the pointer](screenshots/customizing.png)

![The Customizing section in the light theme, with the stacked spin buttons under the pointer](screenshots/customizing-light.png)

![The Customizing section under high contrast, with the stacked spin buttons under the pointer](screenshots/customizing-hc.png)

- The Settings page (Ctrl+5) groups its settings in tabs: Appearance, with switches
  and a slider for the UI zoom from 80% to 150% (which changes the whole interface);
  Notifications; and Keyboard shortcuts.
- **New Project** opens a modal form with a name and a description. Create adds
  the project, selects it and shows the Projects page. It is disabled until the
  name has text. Cancel, Escape and a press on the backdrop close the form
  without adding a project.
- The Components page's **Dialogs** section opens each native dialog and shows
  what it returned, by file name only, never by folder. Its Clipboard buttons
  copy and paste plain text, **Form in a modal** opens a modal form, and **Open
  command palette** opens the command palette.

![The Dialogs section of the Components page](screenshots/components-dialogs.png)

- The **Markdown** page (Ctrl+6) shows a document with every element of the
  `markdown` package: headings, emphasis, links, nested and ordered lists, tasks,
  quotes, a rule, code blocks colored by `markdown/mdcode`, a table, images from
  the demo's own loader (and one it refuses, shown by its alternative text), a
  footnote and an anchor link. A click on a task's box marks the task in the text,
  through `OnTaskToggle`. See [docs/markdown.md](markdown.md).
- Keyboard: Ctrl+1 to Ctrl+6 open the Dashboard, Projects, Activity, Components,
  Settings and Markdown pages, Ctrl+K moves focus to the search field, Ctrl+F
  to the Components page's filter (the code editor keeps Ctrl+F for its find
  bar), Ctrl+Shift+L toggles the theme, and Ctrl+Shift+P opens a command palette
  (`ui.CommandPalette`) that lists the actions available where focus is: type to
  filter, Up/Down to choose, Enter runs, Escape closes.

## The notes app

`examples/notes/` is a second, text-heavy application built only from the public
API. It exists to find API gaps; it found three, which became framework
features: focus handles, `AutoFocus`, and `FocusWithin`.

- Notes live in a JSON file in the user config directory, `jamb-notes/notes.json`
  (override with `JAMB_NOTES_FILE`).
- The sidebar and the editor are a `ui.Split` with a pixel position. The sidebar's
  width is saved in `settings.json` next to the notes file 400 ms after the divider
  stops moving (and as the window closes), with the zoom, and it is restored on the
  next start. The directory was called `goui-notes` before
  the rename; the app moves it to `jamb-notes` on first start, so existing notes,
  settings and keymap carry over. A background goroutine saves 400 ms after typing
  stops and reports progress through a signal.
- Every note field is a signal, so typing re-renders only that note's list
  entry, the word count and the editor, never the whole list.
- Shortcuts: Ctrl+N (new note, title focused), Ctrl+F (search; Enter opens
  the first match), Alt+Up/Down (previous/next note), Ctrl+Shift+P (command
  palette, a `ui.CommandPalette`: type to filter, Up/Down to choose, Enter runs,
  Escape closes),
  Ctrl+K Ctrl+T (toggle theme), Ctrl+Shift+V (Markdown preview). Enter in a title moves to its body. Escape
  closes the palette and returns focus to the note you were in, and a command
  run from the palette acts on that note.
- `keymap.json` next to the notes file overrides the shortcuts, in the format
  described under "Keymap files". It is applied when you save it, while the
  app runs. Its problems are printed to stderr and shown in the status bar.
- Tab reaches the tag chips and the note rows near the viewport (see the
  limitations), and shows a focus ring on the focused one. Enter opens a note (as a click does)
  and toggles a tag chip's filter.
- Ctrl+= / Ctrl+- / Ctrl+0 zoom in, out and reset. They are the actions
  `ZoomIn`, `ZoomOut` and `ZoomReset`, so the View menu shows them. The level is
  remembered in `settings.json` next to the notes file and shown in the status bar
  when it isn't 100% (click it to reset).
- A menu bar above the notes has File (New note, Import..., Export note...,
  Exit), Edit (undo, redo, cut, copy, paste, select all), and View (toggle theme,
  Markdown preview, zoom, command palette). Each item with a key shows that key. Alt by itself
  focuses the bar. Right-click a note in the list for Pin or Unpin, and Delete.
- **Import...** opens the native file dialog with `*.md` and `*.txt` offered and
  several files allowed. The files chosen go through the same import as a drop,
  and the status line reports the count. **Export note...** saves the selected
  note: the file is plain text when the chosen name ends in `.txt`, and Markdown,
  with the title as a heading, otherwise. The dialog offers the title as the
  name. The status line names the file, not its folder.
- The **Markdown preview** (Ctrl+Shift+V, the eye button in the toolbar, or View >
  Toggle Markdown preview) shows the note's body as a `markdown.View` beside the
  editor. It shows the note at once when it opens or another note is selected, and
  follows the edits 150 ms after typing pauses, so a long note does not slow typing.
  The note is still stored as the text that is typed.
- Delete in the editor asks for confirmation inline, with Cancel and Delete
  buttons. It does not use a message box, so the notes app shows no native
  dialog for it.
- The seeded "Languages" note shows ten scripts, ligatures and emoji.
- With a search query, matches (case-insensitive) are highlighted in each
  listed note's title and snippet.
- Tags: pills under the title (Enter adds a tag, x removes it) and filter
  chips in the sidebar that wrap onto more lines.
- Drag and drop: drag a note from the sidebar list onto a tag chip to add
  that tag. The chip lights up while the note is over it. Drop `.txt` or
  `.md` files (up to 1 MB) on the window to import each as a note titled with
  its file name; the last one is selected, and the status bar says how many
  were imported and skipped.

### Agent test

To test goal #10 (APIs coding agents can use), a fresh agent got only the
[README](../README.md) and the notes app. Its task was to add tags: storage, the editor
pills, the sidebar filter, tests and a screenshot check.

**Result:** it finished in about 4 edit/compile cycles. Its first build
compiled with zero errors, and it never read framework source.

**What changed because of its friction log:**

| Friction | Fix |
|---|---|
| No flex-wrap; had to guess text widths to wrap chips | `.Wrap()` |
| Tests located elements by parsing `Dump` text | `rt.Find`, `rt.ClickKey`, `rt.Type`, `rt.Press` |
| Text fields were anonymous in `Dump` | Keys and field contents now shown |
| No compact text field; unclear what `Style` affects | `TextInput.Small()`, documented `Style` |
| No component-local state | `ui.Local` |
| Icons hard to discover; no tag icon | `icons.All`, icon list, 24 new icons |
| Unclear whether signal values can be mutated | Documented: replace, don't mutate |

The agent's chip-wrapping and test workarounds were then replaced with
these features (`examples/notes/views/tags.go`, `tags_test.go`).

## The Git app

`examples/git/` browses the history of Git repositories. It reads them with the
`git` command, which must be on PATH, and never changes them. Like the notes
app, it is built only from the public API to find gaps in it.

- The sidebar lists the repositories in a `ui.ListBox`, so the arrow keys move
  between them. The + button (Ctrl+O) adds one with the folder picker, and a
  repository's context menu refreshes it, copies its path or removes it from the list. Ctrl+PageUp and Ctrl+PageDown move between them. Each
  repository keeps its own view, hidden with `Display` while another is shown, so
  its scroll position, selection and search survive the switch. On the first run
  the list starts with the repository the app was started in, if any.
- Below the repositories, a `ui.Tree` lists the branches, the remote branches by
  remote, and the tags. A branch's description names its upstream, and its row ends
  with HEAD's check and how far it is ahead of and behind (`RenderTrailing`). Choosing
  a ref selects its commit, and its context menu copies its name or hash.
- The history is a `ui.DataGrid` of every branch's commits in topological order:
  the graph, the message with its branch and tag pills, the author, the date and
  the hash. The `graph` package gives each commit a lane and each row the lines
  that cross it, and each row's graph cell is a `ui.Canvas` that draws its slice,
  with the author's initials in the node, a dot for a merge and a ring on HEAD. The
  rows have no border, so the slices meet. The grid loads 1,500 commits at a time
  as it nears its end (`LoadMore`), and the lanes carry on from page to page.
- The search field in the title bar (Ctrl+F) fades the commits that do not match
  the words typed (in the message, the author or the start of the hash), so the
  graph stays whole. Enter and F3 go to the next match, Shift+F3 to the previous.
- The detail pane shows the selected commit's message, author and committer (with
  `ui.TimeAgo`), hash (with a `ui.CopyButton`), parents (each selects its commit) and
  refs, and the files it changed with their line counts, in a list box. A merge is
  compared with its first parent. The details and each diff load with `ui.Query`, so
  a commit seen a moment ago shows at once. Choosing a file shows its diff in place of
  the history, in a read-only `codeedit` editor with the diff highlighting; Escape
  goes back.
- The Tags page (Ctrl+2) is a sortable table of the tags, annotated or
  lightweight, newest first. Enter or a double click shows a tag's commit.
- The window draws its own title bar with `ui.TitleBar`, with the search centered on
  the window. The themes are the app's own light and
  dark themes; they follow Windows until the sun or moon button picks one. The
  repositories, the pane widths, the theme and the zoom are kept in
  `jamb-git\settings.json` in the user's configuration folder.
- Shortcuts: Ctrl+P (go to a branch, tag or commit, a quick open built on the command
  palette's items; a target further back than the history loaded so far loads the history until it is there), F5 or Ctrl+R (refresh), Ctrl+H (go to HEAD), Ctrl+Shift+C (copy
  the selected commit's hash), Ctrl+1 and Ctrl+2 (history and tags), Ctrl+K or
  Ctrl+Shift+P (command palette), Ctrl+Shift+I (the visual inspector).
- The tests in `examples/git/views` build a small repository with `git` in a
  temporary folder and drive the app headlessly; they skip when `git` is missing.

## The monitor

`examples/monitor/` shows what the machine is doing as it changes: the load of each
processor, the memory, the network traffic, the processes' I/O and the running
processes. Like the notes and Git apps it is built only from the public API, and it
found five gaps that became library features: `charts.Axis.Hidden` for its
sparklines, `ui.FormatNumber` and `ui.FormatBytes`, floating dock windows that
follow the window of their dock, tree rows in `ui.DataGrid` for its groups, and
`app.FileIcon` for the programs' icons.

- A goroutine samples the machine every second (0.5, 2 or 5 seconds with the interval
  menu in the title bar, or Ctrl+[ and Ctrl+]) and sets the store's signals. On Windows
  it reads the processes with one call to `NtQuerySystemInformation`; elsewhere, and
  with `-synthetic`, it shows a made-up machine. The processor load is
  `% Processor Utility`, as Task Manager shows it: the busy time scaled by the
  processors' speed, so it reads higher than the busy time on a processor in turbo.
  The processes' shares are scaled to it, and their memory is the private working
  set, Task Manager's Memory column.
- The tiles above the panels show the processor, memory, network and I/O now, each
  with a sparkline of the last minute: a `charts.Area` with both axes hidden.
- The panels are a `ui.Dock`: Processor (the load over time and each logical
  processor), Memory, Network, Process I/O, Processes, Details and Alerts. Drag a tab to
  rearrange them, or out of the window to float it. A floated panel keeps the window's
  theme and zoom, and the shortcuts work in it. The layout is kept in the settings.
  Ctrl+1 to Ctrl+7 show each panel, and reopen a closed one.
- The process table is a `ui.DataGrid` that is sampled again every second. By default
  it groups the processes as Task Manager does, with tree rows: the parts of Windows
  in one group (a process with no executable file, such as System or Memory
  Compression, or one whose executable is in the Windows folder and published by
  Microsoft, read from its version information; drivers' programs are not), and each program that runs more than one process in a group with their
  sums. Each row shows its program's icon, read once for each executable with
  `app.FileIcon` by a goroutine of its own. "All processes" lists them one a row, and
  a filter always does. Its sort,
  focus, open groups and selection follow each row by its key as the rows move.
  The CPU column is shaded by load. Its columns can be moved and resized, and are kept.
  Ctrl+F filters it by name or PID. Right-click a row for its menu, and Delete asks
  before it ends the selected process, through an `AlertDialog`. A process is ended
  only if it is still the one that was selected: one that took over its PID is not.
- The Details panel shows the selected process: its path (loaded with `ui.Query`), its
  parent (a button selects it), when it started (`ui.TimeAgo`), and charts of its
  processor and memory use. For Memory Compression and Secure System it says what
  their memory is. A selected group shows its sums and its members that use the most
  memory.
- The Alerts panel holds two rules, for the processor load and the memory, each with
  a limit and a time. An alert is listed in the panel, shown as a Windows notification
  (a click shows the alerts), and marked on the taskbar button with a badge while the
  value stays above its limit.
- The notification area icon shows the processor load as a bar, and its tooltip the
  load and the memory. Its menu shows the window, pauses the sampling, or quits. The
  Alerts panel's switch closes the window to the notification area instead of
  quitting.
- Shortcuts: Ctrl+P (pause), Ctrl+F (filter), Ctrl+Shift+L (theme), Ctrl+K or
  Ctrl+Shift+P (command palette), Ctrl+Shift+I (the visual inspector).
- The settings are kept in `jamb-monitor\settings.json` in the user's configuration
  folder. The tests in `examples/monitor/views` drive the app headlessly over the
  made-up machine.
