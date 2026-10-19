# Overlays and menus

Menus, dropdowns, popovers, drawers, modal dialogs and the command palette.

## Menus

jamb draws its own menus, so they match the theme, work with a custom title
bar, and need no platform code. A menu's look is `ui.MenuStyle` (`Theme.Menu`, or
`.Styles(...)` on a `DropdownMenu`), and a menu bar's labels are `ui.MenuBarStyle`
(`Theme.MenuBar`, or `.Styles(...)` on the bar). A menu is a list of `ui.MenuItem`s:

```go
ui.MenuItem{Label: "Open in window", Icon: icons.ArrowUpRight, OnClick: openWindow}
ui.MenuItem{Action: NewNote}                       // label, shortcut and run come from the action
ui.MenuItem{Label: "Star", Check: ui.MenuCheckBox, Checked: starred, OnClick: toggleStar}
ui.MenuItem{Label: "Paused", Check: ui.MenuCheckRadio, Checked: status == paused, OnClick: setPaused}
ui.MenuItem{Label: "Status", Submenu: []ui.MenuItem{...}} // opens to the right
ui.MenuItem{Label: "Archive", Disabled: true}      // shown, but cannot be chosen
ui.MenuSeparator()
ui.MenuSection("Status")                           // a group label; runs nothing
```

Three elements open a menu. Each takes a function that builds the items when
the menu opens:

```go
// Right-click, or Shift+F10 or the Menu key while the element has focus.
ui.Div().Focusable().ContextMenu(func() []ui.MenuItem { return rowMenu(p) })

// A click, or Enter, Space or Down while the trigger has focus, opens a menu below it.
ui.DropdownMenu(ui.Button("Actions").Secondary(), func() []ui.MenuItem { return actions() })

// A row of top-level menus. Pressing and releasing Alt by itself focuses the bar.
ui.MenuBar(
	ui.TopMenu{Label: "File", Items: fileMenu},
	ui.TopMenu{Label: "Edit", Items: editMenu},
)
```

![A context menu with a submenu open](screenshots/menu-context.png)

- **Actions.** An item with an `Action` shows the action's binding through
  `KeysFor`, so a keymap change shows at once. Choosing it closes the menu, returns
  focus to where it was, and then runs the action through `Dispatch`, so the
  handler on that focus path runs. An item with `OnClick` runs the function the
  same way. If an item has both, the action runs.
- **Keyboard in a menu.** Up and Down wrap and skip separators and disabled
  items. Home and End go to the first and last items. Right opens a submenu and
  Left closes it. Enter and Space choose. Escape closes one level. Tab closes the
  menu and moves focus on. A letter or digit jumps to the next item whose label
  starts with it.
- **Submenus and the pointer.** A submenu opens after the pointer rests on its
  item for 150 ms, and it stays open for the same delay after the pointer leaves
  for a neighboring item. That lets the pointer move diagonally into the submenu.
  This is a timed delay, not a triangle.
- **Placement.** A menu is an `Overlay` below its anchor. It flips above and
  shifts to stay inside the window, as other overlays do. A submenu opens to the
  right of its row and flips to the left when there is no room.
- **Focus.** A menu takes keyboard focus when it opens. Closing it (Escape,
  choosing an item, or a press outside) returns focus to the element that had it
  before the press, or to the focused element for a keyboard open. A press
  outside closes the whole chain. A press on the trigger toggles a dropdown.
- **Menu bar.** A click opens a top-level menu, and while one is open, the pointer
  over another label opens that one. Left and Right move between the top-level
  menus, also from inside an open menu. Down, Enter or Space open the highlighted
  menu from the focused bar. Escape or Tab leaves the bar. The labels are not in
  the Tab order. A window without a menu bar keeps Windows' own Alt behavior,
  which opens the system menu; with one, a lone Alt press is taken by the bar.
  F10 by itself activates the bar the same way, as Windows does for its menu bars:
  the first label takes focus. Without a menu bar F10 does nothing. Shift+F10 is the
  context-menu key, not the bar's. The application takes F10 from Windows, so that
  Windows' menu mode does not swallow the keys that follow.
- **Look.** Menus use the theme's surface, border, shadow and highlight colors.
  They enter and leave with the same motion as the command palette.
- **Roles.** Entries take the WAI-ARIA menu roles in the access tree: menu item,
  check and radio menu items (with their checked state), and separator.
- **Not implemented.** Alt+letter mnemonics. The labels take no `&` marks, and
  Alt followed by another key does not open a menu.

The behavior is covered by tests in `ui/menu_test.go`, and by the demo and notes
packages. Those tests drive the framework's input methods directly. The
Windows keyboard path (the Menu key, and an Alt press and release that the
system would otherwise turn into its own menu) is not covered by them, so
check it by hand.

## Dropdowns

A dropdown is an Overlay child of its anchor:

```go
ui.Component(func(ctx *ui.Context) ui.Element {
    open := ui.Local(ctx, "open", false)
    t := ctx.Theme()
    btn := ui.Div().Padding(8).OnClick(func() { open.Set(!open.Peek()) }).Child(ui.Text("Menu"))
    if open.Get() {
        btn.Child(ui.Div().Overlay(ui.BelowStart).Padding(4).Background(t.Surface).
            OnClickOutside(func() { open.Set(false) }).
            Child(ui.Text("Item")))
    }
    return btn
})
```

## Popovers and drawers

`ui.Popover(trigger, content)` opens a floating panel of any content next to its
trigger, such as filters or a short form. It opens and closes itself, like a
`DropdownMenu`: a click, Enter or Space on the trigger toggles it, and Escape or a
click outside closes it. Focus moves to the panel's first field or button when it
opens, and Escape returns it to the trigger. Assistive technology sees the trigger
as a button that opens a dialog, and the panel as that dialog.

```go
ui.Popover(ui.Button("Filters").Icon(icons.Filter), func() ui.Element {
	return filterForm()                 // runs while the panel is open
}).Placement(ui.BelowStart).            // default; any Placement, flipped when it does not fit
	OnChange(func(open bool) { ... })
```

`ui.Drawer(title, body)` is a modal panel that slides in from an edge of the
window: settings, details or filters that need more room. It behaves as a
`ui.Modal` does (build it only while open, last in the window's root element;
focus moves in and returns, Tab stays inside, Escape closes it), and a press on the
backdrop also closes it. A left or right drawer is 360 px wide and as tall as the
window; a top or bottom one is 280 px high and as wide as the window.

```go
ui.If(open.Get(), ui.Drawer("Filters", filters).
	Side(ui.DrawerRight).               // DrawerRight (default), DrawerLeft, DrawerTop, DrawerBottom
	Width(ui.Px(420)).                  // across its side
	Footer(reset, apply).Default(apply).
	OnClose(func() { open.Set(false) }))
```

Their looks are `ui.PopoverStyle` (`Panel`) and `ui.DrawerStyle` (the modal's parts),
set in `Theme.Popover` and `Theme.Drawer` or with `.Styles(...)`.

## Modal dialogs

`ui.Modal` is a dialog in the app's own look: a dimmed backdrop, and a panel
centered on it with a title, a body and footer buttons. Its look is `ui.ModalStyle`
(`Theme.Modal` or `.Styles(...)`). Build it only while it
is open, and place it last in the window's root element, which must fill the
window:

```go
ui.If(open.Get(), ui.Modal("New project", form).
	Footer(cancel, create).
	Default(create).             // Enter runs it; it takes focus if the body has no field
	InitialFocus(h).             // optional: the element that takes focus when it opens
	CloseOnBackdrop().           // optional: a press on the backdrop closes it
	OnClose(closeFn))            // Escape, or a backdrop press when CloseOnBackdrop is set
```

![A modal form over the Projects page](screenshots/modal-new-project.png)

![The New project dialog after a failed submit, with a character typed in the Name field: the error clears and the field keeps focus](screenshots/modal-new-project-typing.png)

- Focus moves into the modal when it opens: to the first field of the body that
  Tab reaches, else to the default button. Tab and Shift+Tab cycle through the
  modal only. When the modal closes, focus returns to the element that had it,
  unless focus has moved elsewhere.
- Escape closes the modal through the overlay rules (see
  [Escape and overlays](input.md#actions-and-key-bindings)). Enter presses a focused
  button as usual, and otherwise runs the default button. A multi-line field
  takes Enter as a newline. A single-line field with no `OnSubmit` passes Enter
  on to the default button.
- The panel is at most the window's height less 24 px, as the window is when the modal
  opens (a window resized while a modal is open keeps its size until the modal is built
  again). The title and the footer stay on screen, and a body that does not fit scrolls.
  The footer's buttons wrap when they do not fit the panel's width. A focus the code asks
  for (`FocusHandle.Focus`) scrolls the element into view, as Tab does.
- Its placement is `ui.OverCenter`, a new overlay placement that centers the
  panel over its anchor. Colors come from `Theme.Scrim` and the surface colors,
  and the modal enters and leaves with the same motion as the command palette.
- While a modal is open, keys stop at it: the app's bindings, `OnKeyDown`
  handlers and typed text outside the modal don't reach anything behind it, and
  Alt doesn't activate a menu bar. Keys handled inside the modal (a field's
  editing keys, the modal's own bindings) work as usual.

## Command palette

`ui.CommandPalette(open)` is a searchable list of commands, as in an editor's
Ctrl+Shift+P: a field, and under it the commands whose names match what is typed,
each with its key binding. It is open while `open` is true, and sets it to false
when it closes itself. Place it last in the window's root element, which must fill
the window, as a modal is placed; its panel floats 48 px below the top of the
window, centered. Its look is `ui.CommandPaletteStyle` (`Theme.CommandPalette` or
`.Styles(...)`).

```go
open := ui.NewSignal(false)
ui.Div().Size(ui.Full, ui.Full).
	OnAction(showPalette, func() { open.Set(true) }). // bound to "ctrl+shift+p"
	Child(page).
	Child(ui.CommandPalette(open).
		Placeholder("Search commands").   // default: Strings.CommandPalettePlaceholder
		MaxRows(10).                      // rows shown before the list scrolls; 8 by default
		Commands(func() []*ui.Action {    // optional; runs each time the palette opens
			return appCommands
		}))
```

- When it opens it lists the actions available where focus is
  (`ctx.AvailableActions()`), or what `Commands` returns, without hidden actions,
  and its field takes focus. The action that opens it should be hidden, so that
  the palette does not list itself.
- A command matches when every word of the query occurs in its name, ignoring case
  and in any order: "note new" finds "New note". The matches are highlighted in the
  names, as in a `Combobox`.
- Up and Down move the highlight and stop at the ends of the list; PageUp and
  PageDown move it by `MaxRows`. Home and End stay with the field. Typing moves the
  highlight back to the first match. The pointer highlights the row it is over.
- Enter, or a click on a row, closes the palette, gives focus back to the element
  that had it before the palette opened, and runs the command there (see
  `ctx.Dispatch`), so a command that acts on the focused element acts on that one.
  A command that no element there handles does nothing.
- Escape closes the palette and gives focus back. A click outside closes it and
  leaves focus where the click put it.
- It enters and leaves with the motion of a menu (a fade with a short slide), and
  without motion when the system asks for reduced motion.
- Assistive technology sees a dialog named "Command palette"
  (`Strings.CommandPalette`) holding a combo box: the field keeps focus, and the
  highlighted command is reported as the focused, selected item of its list, as in
  a `Combobox`. Each command is read by its name, with its key binding as the
  description. The empty message is `Strings.NoMatchingCommands`.
- Its keys are the actions bound in `"CommandPalette > Editor"` (see
  [Palettes](input.md#actions-and-key-bindings)).
