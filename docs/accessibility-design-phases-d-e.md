# Accessibility design: phases D and E

The decisions of phases D and E. The goals, the architecture and the phase index are in [accessibility-design.md](accessibility-design.md).

## Phase D: what changed from this design

Phase D reads four Windows settings and applies them to every window, and checks
the built-in themes' contrast. The settings are read, never written. Sources are
linked where each setting is described.

### Settings and where they are read

| Setting | Read with | Source |
|---|---|---|
| High contrast on, and its colors | `SystemParametersInfo(SPI_GETHIGHCONTRAST)` (HCF_HIGHCONTRASTON); `GetSysColor` for the 15 colors in `platform.SystemColors` | [SystemParametersInfo](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-systemparametersinfoa), [GetSysColor](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getsyscolor) |
| Text size, 100 to 225% | `RegGetValueW` on `HKCU\Software\Microsoft\Accessibility`, value `TextScaleFactor` (DWORD) | [Accessible text requirements](https://learn.microsoft.com/en-us/windows/apps/design/accessibility/accessible-text-requirements) describes the setting and its range. The registry location is not in the official documentation: it comes from community answers, so verify it on a PC (see the manual steps in [accessibility-testing.md](accessibility-testing.md#high-contrast-text-size-reduced-motion-and-the-caret)). |
| Animation effects off | `SPI_GETCLIENTAREAANIMATION` (BOOL) | [SystemParametersInfo](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-systemparametersinfoa) |
| Caret blink time (INFINITE = no blink) | `GetCaretBlinkTime` | [GetCaretBlinkTime](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getcaretblinktime) |
| Text cursor thickness | `SPI_GETCARETWIDTH` (DWORD, pixels; default 1) | [SystemParametersInfo](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-systemparametersinfoa) |

The settings are read at start, and again on `WM_SETTINGCHANGE` and
`WM_SYSCOLORCHANGE`, which the window procedure turns into
`platform.EventSettingsChanged`. The app package (`app/settings.go`) converts the
platform's values, applies the dev overrides, and gives the result to
`ui.Runtime.SetSystemSettings`. Values are clamped: text scale to 100 to 225, caret
width to 1 to 20. `internal/platform/linux` and `internal/platform/darwin` report
`platform.DefaultSettings()`, since no adapter reads the settings there yet.

### API

- `ui.SystemSettings` (HighContrast, Colors, TextScale, ReducedMotion, CaretBlinkOff,
  CaretBlink, CaretWidth) and `rt.SetSystemSettings`, `rt.SystemSettings`.
- `ctx.HighContrast()`, `ctx.ReducedMotion()`, `ctx.TextScale()` (percent), and
  `rt.HighContrast()`, `rt.ReducedMotion()`, `rt.TextScale()`.
- `ui.HighContrastTheme(colors, base)`, `ui.HighContrastBlack()` and
  `ui.HighContrastWhite()` (representative schemes, used by the dev override; not
  read from a system).
- `Theme.HighContrast` (true for the high contrast theme) and `ctx.AppTheme()` /
  `rt.AppTheme()`, the theme the app set with `SetTheme`. Under high contrast the
  active theme is the high contrast one, so the app's own selection must use
  `AppTheme`.

### High contrast

Microsoft's guidance for contrast themes is at
[Contrast themes](https://learn.microsoft.com/en-us/windows/apps/design/accessibility/high-contrast-themes):
the system colors are the palette, text uses WindowText, selected and interacted
UI uses Highlight with HighlightText, buttons use ButtonText and ButtonFace,
hyperlinks use HotLight, disabled UI uses GrayText, and borders keep their
boundaries. The guidance also says not to hard-code colors in a contrast theme.

`ui.HighContrastTheme` maps the tokens this way:

| Token | System color |
|---|---|
| Background, Panel, Surface, SurfaceHover | Window |
| SurfaceActive (selection, pressed) | Highlight |
| Text, TextMuted, TextSubtle, Border, BorderStrong, Scrollbar, Success, Warning, Danger | WindowText (status is shown by text and icons) |
| Accent, AccentHover, AccentActive, FocusRing | Highlight |
| AccentText | HighlightText |
| AccentSoft | Window (no tint) |
| Scrim | Window (opaque) |
| Link (hyperlinks) | HotLight |
| Disabled (text, icons, borders of disabled controls) | GrayText |

Radius and FontSize come from the app's theme. High contrast is automatic and
has no opt-out: no clear reason for one was found, and an app that picks its own
colors can use `ctx.HighContrast()`.

Painting follows the same rules automatically: a blurred or offset shadow is not
drawn, a crisp ring (a focus ring) is; a gradient is drawn as its first color, solid;
an icon takes the text color (`fg`) instead of its own color; images stay images.

The demo adds three things that the framework cannot know: the Dashboard's banner
picture is replaced by a plain panel with theme text, since its gradient cannot
hold the text's contrast; chart bars use the solid color; and chips use the
surface instead of a tint (`tintOf`, `barFill` in `examples/demo/views/kit.go`).
The selected navigation item uses `AccentText` for its label, and the theme switch
selects the app's theme (`ctx.AppTheme()`).

The parts of the built-in widgets (`Tabs`, `Select`, `Slider`, `NumberInput`) follow
the same rules. The theme's and an instance's part styles lose their colors,
gradients, shadows and opacity under high contrast, and keep their sizes, spacing,
fonts, radius and border widths, so the built-in system colors and focus ring stay.
A render slot's content is hidden from assistive technology, so it cannot change
what the widget announces. See [styling.md](styling.md).

Custom graphics that show a state were checked under high contrast, since a color
that carries a state has to be replaced by something that does not depend on color.
Fixed: the progress bar (an outlined empty track, a solid Highlight fill, the
percentage as text); the switch (on: a Highlight track with an AccentText knob; off:
an outlined track with a text-colored knob; both before, the off track was the same
Highlight as the on one); a starred project and a pinned note (a filled star, in the
demo's `starIcon` and the notes' pin, so the state shows in the shape); the own
message in a comment thread (a Highlight edge); the selected filter (an outline); the
Danger button's label (the window color, since its fill is the text color). Not
changed: the chart bars, badges, activity icons and stat tiles, whose meaning is in
their height, text or position, not in their color.

Verified in screenshots (black and white schemes) and by
`ui/system_settings_test.go`: the theme mapping, shadows and gradients dropped while
a crisp ring stays, icons take the text color, and the switch back to the app's theme. Tints are replaced in the demo (tintOf), not by the framework, since a framework rule would also hide chart bars and selections.

### Contrast audit of the built-in themes

`ui/contrast_test.go` computes WCAG 2 contrast ratios for each pair the built-in
widgets draw, in both themes, and fails when an enforced pair falls below its
minimum: 4.5:1 for text (WCAG 2.1
[1.4.3](https://www.w3.org/TR/WCAG21/#contrast-minimum)) and 3:1 for interactive
graphics (WCAG 2.1 [1.4.11](https://www.w3.org/TR/WCAG21/#non-text-contrast)).
Translucent tokens are composited over their surface first. Disabled text is
recorded but exempt (WCAG 1.4.3 exempts inactive components). Decorative lines are
recorded without a minimum. A pair that is a known gap fails if it starts to pass,
so the entry is removed.

Token changes (before, then after, with the minimum):

| Pair | Theme | Before | After | Minimum | Change |
|---|---|---|---|---|---|
| TextSubtle on Background, Panel, Surface, SurfaceHover, SurfaceActive | dark | 4.04, 3.90, 3.67, 3.40, 3.08 | 6.01, 5.80, 5.45, 5.05, 4.58 | 4.5 | TextSubtle #71717a to #8e8e96 |
| TextSubtle on the same surfaces | light | 2.46, 2.33, 2.56, 2.33, 2.02 | 5.79, 5.50, 6.04, 5.50, 4.76 | 4.5 | TextSubtle #a1a1aa to #62626a |
| Placeholder (TextSubtle) on Surface | dark / light | 3.67 / 2.56 | 5.45 / 6.04 | 4.5 | (same token) |
| AccentText on Accent | dark | 4.47 | 4.56 | 4.5 | Accent #6366f1 to #6265ed |
| AccentText on AccentHover | dark | 3.73 | 5.00 | 4.5 | AccentHover #7376f4 to #5b5ee7 |
| AccentText on AccentActive | dark | 5.50 | 5.50 | 4.5 | none |
| White label on Danger, hover, pressed | dark | 3.76, 3.46, 4.53 | 5.01, 4.56, 5.94 | 4.5 | Danger #ef4444 to #d62828 |
| White label on Danger, hover, pressed | light | 4.83, 4.43, 5.74 | 5.15, 4.66, 6.10 | 4.5 | Danger #dc2626 to #d12a2a |
| BorderStrong (control border) on Background, Panel, Surface | dark | 1.73, 1.67, 1.57 | 3.43, 3.31, 3.11 | 3 | BorderStrong #3a3a41 to #66666d; controls now draw BorderStrong at rest |
| BorderStrong on the same surfaces | light | 1.42, 1.34, 1.48 | 3.28, 3.12, 3.43 | 3 | BorderStrong #d4d4d8 to #8a8a92 |
| Scrollbar thumb at rest on Surface | dark | 1.21 | 3.23 at alpha 0.35, 5.24 at 0.5 | 3 | Scrollbar #ffffff33 to #ffffff; rest alpha 0.35 to 0.5 (scrollbar.go) |
| Scrollbar thumb at rest on Surface | light | 1.21 | 2.44 at alpha 0.35, 3.98 at 0.5 | 3 | Scrollbar #0000003d to #000000; rest alpha 0.5 |
| Link text on Background, Panel, Surface | dark | Accent: 4.38, 4.22, 3.97 | Link token: 9.80, 9.46, 8.89 (all five surfaces 7.47 to 9.80) | 4.5 | new Link token (#a5b4fc); Accent kept for fills |
| Link text on the light surfaces | light | Accent passed | Link token: 6.23 to 7.90 | 4.5 | new Link token (#4338ca) |
| Disabled text | both | TextSubtle (no change in normal themes) | Disabled token; GrayText under high contrast | exempt (1.4.3) | normal themes dim with Opacity as before |
| Demo badge text (Accent.Lighten(0.3) on AccentSoft) | light | 2.84 | text now Text (passes) | 4.5 | demo badge uses t.Text |
| Input and secondary-button border at rest | both | Border (1.22 to 1.30) | BorderStrong; hover TextMuted | 3 | widgets.go |

The values above are the test output (the ratios listed in the test file), rounded.


Changed after the audit, at the user's direction:

- **Links** use their own token, `Theme.Link` (dark #a5b4fc, light #4338ca; under
  high contrast HotLight). The audit's link pairs, on every surface, reach 6.2:1 or
  more in both themes (see the `Link_on_*` rows of the test). Accent stays the
  primary button fill.
- **Disabled controls** use `Theme.Disabled`: in the normal themes it equals
  TextSubtle and disabled controls keep their `Opacity` dimming (WCAG 1.4.3 exempts
  inactive components). Under high contrast it is GrayText, and disabled buttons and
  menu items are drawn in it with a GrayText outline, not dimmed.

Still recorded, not enforced: Danger, Success and Warning as text (the built-in
widgets do not use them as text; Danger text on the dark Surface is 3.54), and the
Scrim and the hovered scrollbar thumb (informational).

Both themes keep their character: the dark theme stays dark, and the light theme
stays light. AccentHover in the dark theme is now darker than Accent, since a lighter
hover cannot hold a white label at 4.5:1.

### Text size

The Text size setting scales text only. `Runtime.textStyle` multiplies the font
size by `textFactor` (percent / 100), and line heights follow, since they are
multiples of the size. Zoom and the display scale are separate and multiply as before:
a test checks that the logical text height is the same at any zoom and device scale
for the same text size, and that 225% is 2.25 times 100%.

Fixed heights became minimum heights, so content grows instead of overlapping.
Changed in the framework (`ui/widgets.go`, `ui/menu.go`, `ui/menubar.go`): buttons,
text fields, menu rows and menu bar items. Scaled in the framework:
`ui/vlist.go` estimates an unmeasured row at 32 px times the text factor, and a
multi-line field's minimum height scales with `ctx.TextScale()`. The rows of
`ListOf` and the virtual list are measured, so they follow their content.

Changed in the examples, with `layoutScale` (the text size, capped at 150%) for
wide controls:

- Demo: the sidebar width, the search width, navigation rows and badges are minimum
  heights; the title bar is a minimum height; the header wraps onto a second line
  when it is narrow (breadcrumb first, then search, bell and New Project), so no label
  is cut.
- Notes: the header bars and the menu bar are minimum heights; the menu bar's
  height and the content's top padding scale together; the "Add tag" field widens.

Checked at 150% and 225% in screenshots (`docs/screenshots`): demo Components,
Projects, Activity, Settings and Dashboard; notes. Known: a single-line field
(the notes title) scrolls its text horizontally when the text is wider than the
field, as any input does. Recorded as a limitation in [the known limitations](status.md#known-limitations-and-next-steps).

### Reduced motion

With reduced motion (`ctx.ReducedMotion()`):

- A `Transition` jumps to its target (`animate` in `ui/paint.go`).
- `Enter` and `Exit` are not run; an element that leaves is not kept as a ghost
  (`retireNode`, `motion`).
- A scroll jumps to its target (`scrollStep`); the hover is refreshed after a jump.
- A finite `Animate` renders its end value and asks for no frame.
- A repeating `Animate` renders its first frame (progress 0) and asks for no frame,
  so it stops. Apps that want a different still frame can check `ctx.ReducedMotion()`.

Caret: the blink uses the system's blink time (`caretVisible`), and the blink
stays solid when the system's blink is off. The caret width is 1.5 logical pixels
times the system's cursor thickness; the system value is in device pixels, and the
scale of the display applies to it as to the rest of the UI.

### Dev overrides

`JAMB_HIGH_CONTRAST`, `JAMB_TEXT_SCALE` and `JAMB_REDUCED_MOTION` are documented
in `app/dev.go` and in [tools.md](tools.md). They simulate the settings for one run and
change no system setting. Normal use reads the real settings only; a variable
replaces its setting only when it is set.

### Part 1: the idle step

The Phase E open item below ("The `idle` script step") is fixed. The cause was not in
the idle step: the Components page never settles because a hover feedback loop
keeps the runtime animating. A hover style that lifts an element (here the
"Hover: scale and lift" card) also moved its hit box, since hit testing used the
painted box (`node.abs`, which includes the element's offset). When the pointer
rests on the lifted card's edge, the card leaves the pointer, the hover ends, the
card drops back under the pointer, and the hover begins again. The transition
never ends, and `Settled()` stays false.

The fix: a separate hit box (`node.hit`), set in `paintNode` to the layout box
moved only by the element's own base offset (`n.div.style`), not by its hover,
focus, press styles, transitions or motion. `hit()` tests that box. A static
`Style.Offset` still moves the hit area, so a click where the element is painted
still reaches it. Tests: `ui/hit_test.go` (the hover does not move the hit box and
the runtime settles; a static offset does move it) and
`examples/demo/views/components_idle_test.go` (the Components page settles after a
scroll with the pointer at the card's edge). Both fail without the fix.

Evidence: the script from the task (`wait 1500; key ctrl+4; wait 800; scroll 700 500 600`
three times, `idle`, `shot`, `exit`) completes. The probe found the loop at pointer
(600, 400) in the real app: the script timed out at 25 s before the fix and completes
in about 3 s after it. A sweep of 84 pointer positions in the headless runtime settled
at every point after the fix. With a trace (`JAMB_TRACE=1`) after the idle step, the
frames are the five repeating Motion bars only (`rendered=5`, no layout, no build),
at the display's rate, as repeating animations require.

### Keyboard context menus (Shift+F10)

Shift+F10 and the Menu key open a context menu from the keyboard. Two faults stopped
a keyboard or screen reader user from moving through it, and both are fixed.

- **F10 was handled by Windows, not the app.** F10 is a system key. The window
  passed its key-down to `DefWindowProc`, which turns F10 into the menu bar's keyboard
  mode, a native menu loop. That loop took the keys that followed (Down, Tab) and
  left only on Escape, so the app never saw them: after Shift+F10 the menu opened,
  but Down did nothing. Found with `GetGUIThreadInfo` (the thread's `menuOwner` was
  the window) and the window's key messages. The fix is in `internal/platform/windows/window.go`
  (`ownsF10`): F10 without Alt is the application's, and the default handling is not
  run for it. Alt+F10 keeps the system's handling.
- **The focus did not move to the menu's items.** The menu's highlight lived in the
  menu's own state, so the access tree reported the menu as focused, with no item. A
  screen reader then heard the menu's name, not the item. Now a menu that has focus
  passes it to its highlighted row (`highlightedRow` in `ui/access.go`, the flag is
  set by the menu row). The adapter raises a focus event for each change of focused
  node, so each Down and Up is announced. Dropdown menus and the menu bar use the same
  rows, so they behave the same way.

Tests: `ui/menu_focus_test.go` (Shift+F10 focuses the first item; Down and Up move the
focus item by item; a pointer-opened menu is named and focused; a dropdown follows the
same rule; Escape returns the focus to the element). They fail without the change.
`scripts/a11y-check.ps1` has a live check that focuses a project card, presses
Shift+F10 and Down through `SendKeys`, and checks by UI Automation that the focused
element is "Open", then "Open in window", then "Escape" returns it to the card. It
failed on the build without the F10 fix.

The Axe state 11 (the card context menu with the Status submenu) is reached by the
same keys: Shift+F10, Down three times to Status, Right.

### Phase D verification

AxeWindowsCLI scans of the 18 Phase E states, with no overrides, with
`JAMB_HIGH_CONTRAST=1`, and with `JAMB_TEXT_SCALE=225`. Errors per state; zero in
all 54 scans.

| State | Normal | High contrast | Text 225% |
|---|---|---|---|
| 1 Dashboard | 0 | 0 | 0 |
| 2 Projects (default) | 0 | 0 | 0 |
| 3 Activity, build row expanded, filter "build" | 0 | 0 | 0 |
| 4 Components, top | 0 | 0 | 0 |
| 5 Components, scrolled about 600 px | 0 | 0 | 0 |
| 6 Components, scrolled about 1200 px | 0 | 0 | 0 |
| 7 Components, scrolled about 2400 px (clamped to the bottom) | 0 | 0 | 0 |
| 8 Components, dropdown open | 0 | 0 | 0 |
| 9 Settings | 0 | 0 | 0 |
| 10 New Project dialog | 0 | 0 | 0 |
| 11 Card context menu, Status submenu (Shift+F10, Down three times, Right) | 0 | 0 | 0 |
| 12 Project detail window (both top-level windows scanned) | 0 | 0 | 0 |
| 13 Notes default | 0 | 0 | 0 |
| 14 Command palette | 0 | 0 | 0 |
| 15 File menu | 0 | 0 | 0 |
| 16 Edit menu | 0 | 0 | 0 |
| 17 View menu | 0 | 0 | 0 |
| 18 Tag editor, New tag focused | 0 | 0 | 0 |

Scroll positions are approximate pixel targets, set by the scroll percentage of
the live viewport; the 2400 px state is clamped to the bottom of the page. The
scan driver is not in the repository.

`scripts/a11y-check.ps1` (20 checks: the 18 of Phase E, plus the Shift+F10 check on
a project card and the F10 check on the notes menu bar), each run with two Axe scans:
normal `Summary: 20 passed, 0 failed`; `-HighContrast` `Summary: 20 passed, 0 failed`;
`-TextScale 225` `Summary: 20 passed, 0 failed`.

Tests: `gofmt -l .` prints nothing; `go vet ./... ./examples/demo/... ./examples/notes/... ./cmd/jamb/...`
is clean; `go test -count=1 ./... ./examples/demo/... ./examples/notes/... ./cmd/jamb/...`
passes in every package; `GOOS=linux go build ./access/ ./ui/ ./app/ ./text/` builds.

The Axe scans ran before the plain-F10 menu bar change. That change only adds a
case for F10 with no modifiers, which none of the 18 states presses.

## Phase E: what changed from this design

Phase E's automated parts are in place: the Axe.Windows rules (the engine of
Accessibility Insights) run on both examples in every significant state through
AxeWindowsCLI, the errors they found are fixed, a live
UI Automation check runs in `scripts/a11y-check.ps1`, and the manual screen reader
script is in [accessibility-testing.md](accessibility-testing.md). The NVDA and
Narrator pass is for a person to run, and is pending.

### Tools

- **AxeWindowsCLI 2.4.2** (the Axe.Windows rules engine), from the official
  release at github.com/microsoft/axe-windows. The winget package
  `Microsoft.AxeWindows` is that release's MSI. Its install needs elevation, because
  the MSI's .NET 6 runtime dependency is installed machine-wide, so the signed zip
  was used instead, unpacked to `%LOCALAPPDATA%\Programs\AxeWindowsCLI`. Every
  executable and DLL in it carries a valid Microsoft or .NET Authenticode
  signature.
- **Accessibility Insights for Windows 1.1.2924.1**, the latest stable release
  from github.com/microsoft/accessibility-insights-windows. It is not in winget.
  Its MSI is a machine-wide install that needs elevation, so the MSI was extracted
  with an administrative install to `%LOCALAPPDATA%\Programs\AccessibilityInsights`.
  The extracted app is signed by Microsoft and starts. It is not registered: there
  is no Start menu entry and no uninstall entry.

The scans use AxeWindowsCLI. The Accessibility Insights app was installed and
started, but its own assessment and FastPass tests were not run, since they are
interactive. The CLI runs the same rules non-interactively.

Axe.Windows 2.4.2 has 154 rules: 76 at Error, 16 at Warning and 62 at
NeedsReview. The CLI reports errors. No scan log had warning or needs-review
output.

### Scan results

Errors per state, as reported by AxeWindowsCLI, before and after the fixes. The
demo's states are on the Projects page unless noted.

| State | Before | After |
| --- | --- | --- |
| Demo, Dashboard | 1 | 0 |
| Demo, Projects (default) | 7 | 0 |
| Demo, Activity (one build row expanded, filter "build") | 1 | 0 |
| Demo, Components, top | 7 | 0 |
| Demo, Components, scrolled 600 px | 7 | 0 |
| Demo, Components, scrolled 1200 px | 7 | 0 |
| Demo, Components, scrolled 2400 px | 7 | 0 |
| Demo, Components, dropdown open | 8 | 0 |
| Demo, Settings | 1 | 0 |
| Demo, New Project dialog | 8 | 0 |
| Demo, context menu with the Status submenu open | 8 | 0 |
| Demo, project detail window (comments) | 8 | 0 |
| Notes, default | 2 | 0 |
| Notes, command palette open | 2 | 0 |
| Notes, File menu open | 3 | 0 |
| Notes, Edit menu open | 3 | 0 |
| Notes, View menu open | 3 | 0 |
| Notes, tag editor (New tag field focused) | 2 | 0 |

Three rules produced all the errors before the fixes:

- **ButtonInvokeAndTogglePatterns** (Error, WCAG 4.1.2): a toggle button exposed
  both Invoke and Toggle. It hit every star, the Pin note toggle and the Toggle
  theme button.
- **ButtonShouldHavePatterns** (Error, WCAG 4.1.2): a button had none of Invoke,
  Toggle or ExpandCollapse. It hit the disabled Create project button and the
  disabled Components sample, the Components sample buttons with no action, and
  the Notifications bell, which had no action.
- **NameNotNull** (Error, Section 508 502.3.1): a focusable menu panel had no name.
  It hit the Components dropdown menu, the context menu and the notes menu bar's
  File, Edit and View popups.

### Fixes

Framework (the generic problems):

- `internal/platform/windows/uia.go`: a toggle has Toggle and not Invoke. A disabled button
  keeps its Invoke pattern, which the design already says a disabled element does.
  The pattern is still absent for an enabled element with no action.
- `ui/menu.go`, `ui/menubar.go` and `ui/widgets.go`: a menu panel takes a name. A
  dropdown's menu takes its trigger's name, a menu bar popup takes its menu's
  name, and a context menu takes the name of the element it hangs from, or
  "Actions" when that element has none. `ButtonElement.accessName` gives the
  trigger's name, in the same order the access tree uses: Label, text, tooltip.

Examples (app content):

- `examples/demo/views/app.go`: the Notifications bell announces "No new
  notifications" when it is pressed. The demo has no notifications feature, so the
  text is a placeholder to confirm.
- `examples/demo/views/components.go`: the five enabled sample buttons (Primary,
  Secondary, Ghost, Danger, Small) announce "<name> pressed" through a `pressed`
  helper. They had no action. The text is a placeholder to confirm.

Regression tests: `ui/access_controls_test.go` (a request on a disabled button does
nothing, menu panels are named, a context menu is named by its element);
`internal/platform/windows/uia_controls_test.go` (`TestInvokeFollowsTogglesAndDisabledButtons`).
Two existing tests looked up unnamed menus, and now look up "Actions", the name
a context menu gets. A request on a disabled button already does nothing, since a
disabled element has no request target (the rule that makes targets, in
`ui/access.go`, leaves out a disabled element that has no actions), so that test
needed no code change.

### Justified exceptions

None. Every error the rules raised was a real defect, and each is fixed above. The
two placeholder actions are content decisions, listed under open items.

### Live checks

`scripts/a11y-check.ps1` builds both examples, runs them (the notes app with a
temporary notes file), checks them through UI Automation and scans them with
AxeWindowsCLI. It ran 18 checks, all passing, on this tree:

- Names and roles on the navigation, the toolbar and the notes menu bar.
- Invoke opens a page; a toggle has Toggle and no Invoke (the star and the Pin
  note).
- Expand and collapse on an Activity build row, the dropdown trigger and the File
  menu, with the menu named after its trigger.
- Selection on the filter chips, with the group reporting its one selection.
- Value on the search field and the note title, with SetValue.
- Focus moves to the search field, and a focus event is raised for it.
- Scroll: the Components page scrolls to 50 percent with SetScrollPercent.
- A disabled button keeps Invoke (Create project); Cancel closes the dialog.
- Settings switches toggle with Toggle.
- The note body: its text holds the welcome sentence; FindText finds "small";
  Select selects it; the selection and the word unit are "small"; the word has one
  bounding rectangle.
- AxeWindowsCLI reports no errors on the demo's Settings page and on the notes
  editor.

The script was also run against the commit before this phase (extracted from git,
with the same script). It failed seven checks: the star and Pin toggles (Invoke
present), the dropdown and File menus (not named), Create project (no Invoke), and
the two Axe scans (2 and 3 errors). Those are the defects the fixes address.

### Open items

- **Manual passes.** NVDA and Narrator are for a person to run, with
  [accessibility-testing.md](accessibility-testing.md).
- **The two placeholder actions.** The Notifications bell and the sample buttons
  now announce something. Confirm the wording, or give the bell a real action.
- **The `idle` script step.** Fixed in phase D (see "Part 1: the idle step"). The
  cause was a hover feedback loop, not the idle step.
- **Scanner coverage.** The CLI reports errors only, and this phase did not run
  the Accessibility Insights assessment, which needs a person.
