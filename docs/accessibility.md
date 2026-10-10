# Accessibility design

Status: accepted. Phase A (Foundation) is done, with the changes listed under
"Phase A: what changed from this design". Phases B to E have not started. This
document describes how jamb exposes its UI
to assistive technology (screen readers, magnifiers, voice control) and the
order in which the work is done.

## Goals

- Apps built with jamb are usable with a screen reader by default: built-in
  widgets report the right role, name and state with no app code.
- Custom elements can describe themselves with a small API.
- No cost when no assistive technology is running.
- One platform-neutral model, with one adapter per operating system, so macOS
  and Linux support later reuse everything above the adapter.
- Developers are warned about the common mistakes (an icon button without a
  label, an image without a description) while they develop.

Accessibility standards (for example Section 508, which references WCAG 2.0 AA,
or EN 301 549) apply to finished apps, not to a toolkit. jamb's job is to make
meeting them achievable and the default; whether an app conforms depends on the
app.

## Prior art

jamb draws its own UI, like egui, Bevy and GPUI. Toolkits of this kind generally
follow [AccessKit](https://github.com/AccessKit/accesskit): the toolkit
describes its UI as a tree of nodes, and a platform adapter translates the tree
to the operating system's accessibility API. GPUI depends on AccessKit; its
Windows support is not complete yet
([zed-industries/zed#41138](https://github.com/zed-industries/zed/issues/41138)).

AccessKit is written in Rust; using it from Go would require cgo, and jamb is
pure Go. jamb therefore follows AccessKit's architecture in Go instead of
binding to it.

## Architecture

```
ui runtime ──builds──▶ access tree (package access) ──snapshot──▶ platform adapter ──▶ OS API
   ▲                                                                  │
   └──────────────────── actions (click, focus, expand…) ◀────────────┘
```

### The access package

A platform-neutral package with no dependencies. It defines:

- **Node**: a stable ID, a role, a name, an optional description and value,
  states (focused, disabled, checked, mixed, expanded, selected, read-only,
  required, invalid), bounds in window coordinates, the actions it supports,
  its children, relations (labelled by, described by, controls), heading level,
  and list position (position in set and set size, so a row of a virtualized
  list can report "40 of 10,000" without the other rows existing).
- **Roles**: window, group, button, check box, switch, radio button, text
  field, multi-line text field, static text, image, link, list, list item,
  menu bar, menu, menu item (plain, check, radio), separator, dialog,
  tooltip, scroll area, heading, tab list, tab, tab panel.
- **Actions**: click (invoke), focus, expand, collapse, toggle, select, set
  value, scroll into view, increment, decrement.
- **Tree updates**: the nodes that changed and the focused node. Updates are
  incremental.
- **Announcements**: text to speak, polite or assertive, for status messages
  that have no focused element ("Saved", "3 results").

### Building the tree

The ui runtime builds the tree from its own nodes:

- Built-in widgets map to roles automatically: buttons, text fields and text
  areas, menus and menu bars, modals (dialog, named by the title), lists,
  tooltips, links in rich text, images and icons.
- Plain text becomes the name of the element that contains it, or static text
  when it stands alone. Purely visual elements (decorative icons, dividers,
  layout containers without semantics) are left out of the tree.
- Virtualized lists (`UniformList`, `ListOf`) expose the rows that are built,
  each with its position in the full list and the list's size.
- Overlays are attached where they belong in reading order, not at the end of
  the window.

The tree is built only after an assistive technology first asks for it, and is
then updated from the runtime's existing change tracking, so frames cost nothing
extra when no assistive technology is running.

### Public API

Small and explicit, on the elements that already exist, for example:

- `Div().Role(access.RoleCheckBox).Label("Notifications").Checked(on)`
- `Div().Description("…")`, `Div().Hidden()` (leave out of the tree),
  `Div().Heading(2)`
- `Image(src).Alt("…")`, `IconButton(icon).Label("…")`
- `ui.Announce(text, access.Polite)`

The exact names are settled during implementation and documented in the
README.

### Windows adapter (UI Automation)

- The window answers `WM_GETOBJECT` with a fragment root provider; nodes are
  fragment providers (navigation, bounds, focus, runtime IDs).
- Control patterns by role and state: Invoke, Toggle, ExpandCollapse,
  Selection and SelectionItem, Value, RangeValue, Scroll and ScrollItem, and
  Text (phase C).
- Events: focus changed, property changed (name, value, toggle and expand
  state), structure changed, menu opened and closed, and notification events
  for announcements.
- Implemented in pure Go with the COM calling technique jamb already uses for
  Direct3D and the file dialogs.
- Threading: UI Automation may call from other threads. The adapter answers
  from an immutable snapshot of the tree, and posts actions back to the UI
  thread, as AccessKit does.

## Developer tooling

- A dump of the access tree (a `JAMB_SCRIPT` step and a runtime method), for
  tests and debugging.
- Development warnings for unlabeled interactive elements and images without
  alt text.
- Test helpers to assert roles, names and states in unit tests.

## Phases

| Phase | Delivers |
|---|---|
| A. Foundation | access package, tree building, Windows fragment providers (navigation, names, roles, bounds, focus), Invoke for buttons, focus events, tree dump, development warnings |
| B. Controls | Toggle, ExpandCollapse, Selection, Value for single-line fields, Scroll, virtualized list positions, dialog and menu events, announcements, headings |
| C. Text | UI Automation Text pattern for text fields and text areas: caret, selection, reading by character, word and line, text change events |
| D. System settings | Windows high contrast, the Text size setting, reduced animations, a contrast audit of the built-in themes, caret tracking for magnifiers |
| E. Verification | Accessibility Insights for Windows checks on the examples, a manual NVDA and Narrator script, automated tests that read the real UI Automation tree |

## Testing

- Unit tests on the access tree built from ui elements.
- Automated checks that read the live UI Automation tree of the examples the
  way a screen reader does.
- [Accessibility Insights for Windows](https://accessibilityinsights.io/)
  (free, from Microsoft) for automated rule checks.
- Manual passes with NVDA (free) and Narrator (built into Windows).

## Phase A: what changed from this design

Phase A is in place on Windows (`access`, `ui/access.go`,
`platform/windows/uia*.go`). These are the places where the work differed from
the design above, and why.

- **The tree is rebuilt when something changed, not per subtree.** The runtime
  marks the tree dirty when a component re-renders, when layout or paint runs
  (paint moves nodes when they scroll), and when focus moves. The next frame
  then builds the whole tree and publishes it with a `Diff` against the last
  one, so only changed nodes are reported. Building per subtree would need the
  runtime to track which nodes changed, which it does not do yet. The cost is
  measured: about 18 µs and 281 allocations for the demo's dashboard (about 47
  nodes). A frame of the paint stress test takes about 300 µs, and with a client
  attached each painted frame builds the tree once. That is cheap at this size,
  but it grows with the tree, so per-subtree updates are the next step if large
  trees need it.
- **One provider object per node.** The adapter answers from the immutable
  snapshot, as the design says. It also keeps one COM object per node in a
  registry, so that UI Automation sees the same element each time, and releases
  the registry's reference when the node leaves the tree. The design did not
  specify this.
- **Per-window and per-snapshot caches.** The host provider of a window is made
  once and held by the registry, and the root node is rebuilt when the snapshot
  changes, not on each call. Navigation and bounds no longer allocate on the hot
  paths; `uia_alloc_test.go` holds them to zero.
- **The window is the host of its fragment root.** The root's
  `get_HostRawElementProvider` returns `UiaHostProviderFromHwnd`. Without it UI
  Automation placed the fragment root one level below a native element for the
  window, so every node sat one level too deep in a client's tree. Found by
  walking the tree from PowerShell.
- **`IRawElementProviderFragmentRoot` derives from `IUnknown`**, as the SDK
  headers declare, not from `IRawElementProviderFragment`. Its vtable is
  `QueryInterface`, `AddRef`, `Release`, `ElementProviderFromPoint`, `GetFocus`,
  and the fragment methods are on the fragment view.
- **`ElementProviderFromPoint` takes its coordinates in XMM registers.** Its two
  doubles do not reach a `syscall.NewCallback` function, so a small amd64
  assembly stub moves them into the integer registers first (no cgo). The live
  check covered it: a client's `ElementFromPoint` over a button returned that
  button.
- **Public API.** Added `Checked` on `DivElement`, which the design's example
  uses and a switch needs. `Heading` is defined in `access` but has no public
  method yet, as the design allows. Icon elements have no `Label` or `Alt`:
  they are decorative, and an icon's meaning comes from its button's name.
- **Clickable elements with no role are buttons.** A `DivElement` with `OnClick`
  and no role is exposed as a button, because a clickable element that a screen
  reader calls a group cannot be operated.
- **The placeholder is not a name.** A text field's name comes from `Label`
  alone, so a field with only a placeholder is reported by the warnings.
- **Caption buttons have a click handler** that calls the window's minimize,
  maximize and close, so that UI Automation can invoke them. The native hit test
  still handles the mouse.

Open after Phase A:

- **Memory growth with a client walking the tree.** Private memory of the demo
  rises about 0.05 MB per full walk of its 46 nodes (109 to 138 MB over 600
  walks in 27 seconds, with and without a client-side collection). With no client
  the idle demo stays at 106 MB for two minutes. The Go heap is not the cause
  (its live size is flat after each collection) and the adapter's own objects are
  bounded (47 live objects for the demo). Per-call garbage on the hot paths was
  removed and is guarded by a test (`uia_alloc_test.go`). The rest tracks the
  number of new element objects that a client creates: a loop that re-reads one
  element's property stays flat, while a loop that makes a new element each time
  grows about 1.2 KB per element. That points at per-client-element state inside
  UI Automation or OLE, which the provider cannot release. The same walk against
  Windows Notepad grows at the same rate (101.8 to 135.3 MB over 600 walks of its
  49 nodes, about 1.1 KB per element read), so this is how UI Automation behaves
  under repeated full-tree walks, not a leak in jamb. Screen readers read what
  changes rather than re-walking the whole tree, so the growth seen in practice
  is far smaller.
- **`E_UNEXPECTED` in a client walk.** Reproduced once, when a walk continued
  after the demo's window had closed: `FindAll` failed with `E_UNEXPECTED`. Three
  runs in which the walk stayed inside the window's life had no errors. The
  likely source is the disconnect at `WM_DESTROY` (`UiaDisconnectProvider`), so
  a client that outlives its window gets an error from UI Automation. A walk that
  races a page change can also get `UIA_E_ELEMENTNOTAVAILABLE` for a node that
  just left the tree, which is the expected answer for a removed node.
