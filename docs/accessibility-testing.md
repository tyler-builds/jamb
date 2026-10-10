# Accessibility testing by hand

This page is the manual pass for jamb's examples with a screen reader. It is for
a person to run. The automated checks are in `scripts/a11y-check.ps1` (see the
Tools section of the [README](../README.md#tools-for-humans-and-coding-agents)),
and the design is in [accessibility.md](accessibility.md).

The expected results say what the screen reader should speak. Its wording
depends on its version and settings, so match the meaning (the name, the role,
the state), not the exact words. Record any difference.

Whether an app meets a standard such as Section 508 or WCAG depends on the app
and on the contract.

## Before you start

- Build the examples: `go run ./examples/demo` and `go run ./examples/notes`.
- Run the notes app with a temporary notes file, never your own notes:
  `set JAMB_NOTES_FILE=%TEMP%\jamb-notes-test.json` in the same command window
  before `go run ./examples/notes`.
- Close other screen readers, so that only one speaks.

## Narrator (built into Windows)

- **Start:** Windows key + Ctrl + Enter. Finish the first-run setup, or skip it.
- **Stop:** Windows key + Ctrl + Enter again, or Narrator's own exit command.
- **Narrator key:** Caps Lock or Insert, as set in Narrator's settings. Narrator + F1
  lists its commands.
- Focus changes are spoken without a command. Use Tab and Shift + Tab to move.

## NVDA (free, from [nvaccess.org](https://www.nvaccess.org/download/))

- **Start:** Ctrl + Alt + N, or the NVDA item in the Start menu. The first run asks
  about the install or portable copy; either works.
- **Stop:** NVDA key + Q (the NVDA key is Insert, or Caps Lock on a laptop layout).
- **Read the focused item again:** NVDA key + Tab.
- **Read the whole window from the focus:** NVDA key + Down.

## Tasks in the demo

Start `go run ./examples/demo`. It opens on the Projects page.

| # | Task | Expected |
|---|---|---|
| 1 | Press Tab until the sidebar opens. Use Down to move through "Dashboard", "Projects", "Activity", "Components", "Settings". | Each item is spoken with its name and "button"; the current page is spoken as selected. |
| 2 | Press Enter on "Activity". | The page changes. The summary "10,000 events from 5 people" can be read with the NVDA key + Down. |
| 3 | Go to "Projects" and Tab to a project card. | The card is spoken as a list item with its name (the project, its status and its summary), and its position, for example "1 of 6". |
| 4 | On the card, press Shift + F10 (or the Applications key). | A menu opens and is spoken by the card's name. Down moves through Open, Open in window, Star, Status, Archive. Archive is spoken as dimmed or unavailable. |
| 5 | On "Status", press Right. Then Down to "Paused". | "Status" is spoken as expanded. The three statuses are spoken as radio items, one checked, with the position "x of 3". |
| 6 | Escape twice. Tab to "New Project" and press Enter. | A dialog named "New project" opens. Focus goes into it. |
| 7 | In the dialog, Tab to "Create project". | It is spoken as dimmed or unavailable, because the name is empty. Type a name in "Name": "Create project" is then spoken as enabled. Escape closes the dialog and focus returns to "New Project". |
| 8 | Go to "Settings". Tab to the "Dark mode" switch and press Space. | The switch is spoken with its name "Dark mode" and its state, on or off. The theme changes with it. Press Space again to restore it. |
| 9 | Go to "Activity". Tab to "Filter events" and type `build`. Stop typing and wait about one second. | The count "3,000 matching events" (the number may differ) is spoken once, when typing pauses. It is not spoken for each letter. |
| 10 | Tab to a build row, for example one that says "finished a build of Dev Runner". Press Enter. | The row is spoken as "expanded", and the details appear. Press Enter again: "collapsed". |
| 11 | Go to "Components". Scroll down to "Menus" with the mouse wheel or the Page Down key. Tab to "Project actions" and press Enter. | The menu opens and is spoken by "Project actions". Escape closes it, and focus returns to the button. |
| 12 | Go to "Components", "Buttons". Tab to "Primary" and press Enter. | "Primary pressed" is spoken. |

## Tasks in the notes app

Start `go run ./examples/notes` with `JAMB_NOTES_FILE` set as above.

| # | Task | Expected |
|---|---|---|
| 1 | Press Alt. Use Right to move to "File", and press Enter. | "File" is spoken as a menu item, expanded, and its items are spoken as they are reached. Escape closes it. |
| 2 | Tab to the search field "Search notes" and type `jamb`. | The field is spoken as an edit field named "Search notes". The list narrows. |
| 3 | Tab to the note list. Move with Down to a note. | Each note is spoken as selected or not, with its position, for example "2 of 6". |
| 4 | Tab to "Pin note" and press Enter. | "Pin note" is spoken as a toggle button, pressed or not pressed. The state changes with each Enter. |
| 5 | Tab to "Title". Read it with the NVDA key + Tab. | "Title, edit". The text of the title is spoken. |
| 6 | Tab to "Body". Press Ctrl + Home, then Ctrl + Right three times. | The caret starts at the first letter of "A small note-taking app". Each Ctrl + Right moves to the next word, which is spoken: "small", then "note", then "-" (the hyphen). |
| 7 | At the start of the body, press Ctrl + Shift + Right. | The word after the caret is selected, and the screen reader speaks it. Keep the selection for the next step. |
| 8 | Press Down in the body. | The line is spoken. Repeat to read the next line. |
| 9 | Type a letter at the end of the body. | "Saved" is spoken when the save finishes, once per save. It is not repeated for each keystroke. |
| 10 | Tab to "New tag", type `draft`, and press Enter. | A tag chip "draft" appears, with its button "Remove tag draft". |
| 11 | Press Ctrl + Shift + P. | The command palette opens. Its search field is spoken as an edit field named "Command". Down moves through the commands, each with its shortcut, for example "New note Ctrl+N". |
| 12 | Escape. Press Ctrl + Shift + P, type `theme`, and press Enter. | The command runs and the theme changes. Focus returns to the body. |

## Keyboard-only use

Do these tasks with the mouse unplugged or not touched, in both apps.

- Every control is reached with Tab and Shift + Tab, in the order it appears on
  screen. Focus never disappears behind a closed dialog or menu.
- Each focused control shows a visible focus ring.
- Enter and Space activate buttons and toggles. Arrow keys move through lists,
  menus and radio groups. Escape closes menus, the palette and dialogs.
- Record any control that cannot be reached or activated from the keyboard.

## High contrast and text size

These checks show what jamb does today. jamb does not yet follow the Windows
high contrast theme or the Text size setting; that is Phase D of the design
([accessibility.md](accessibility.md#phases)). Record what you see, so that Phase D
can be checked against it.

- **High contrast:** turn on a contrast theme (Settings, Accessibility, Contrast
  themes). Note whether the text, the focus ring and the controls remain visible.
- **Text size:** set Settings, Accessibility, Text size to the largest value. Note
  whether the demo's text and the notes text change size, and whether anything is
  cut off.
- **Magnifier:** turn on Magnifier (Windows key + Plus). In the notes body, type
  and move the caret. The view should follow the caret. The caret is drawn by jamb,
  and only one caret appears.

## Recording a result

For each run, note the date, the Windows version, the screen reader and its
version, the app and the task number, and whether the result matched. For a
difference, write what was spoken and what was expected, and the step that led
there.
