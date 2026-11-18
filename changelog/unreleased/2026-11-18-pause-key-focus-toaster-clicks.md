### Added

- `input.KeyPause`, the Pause/Break key, on Windows, Linux and the web. Bindings name it `pause`,
  or `break` (`ui.Bind("alt+break", cancel)`), and menus show it as Pause. Ctrl+Pause, which
  Windows and Linux report as Break, is `KeyPause` with Ctrl held.
- `Binding.Primary` marks the binding that menus, tooltips and the command palette show for an
  action bound to several keys. `KeysFor` shows a primary binding where its context applies,
  and otherwise the binding added last, as before.
- `ctx.HasToaster()` reports whether the window has a `Toaster` mounted, and re-renders the
  component when the first one is mounted or the last one removed.
- `ctx.Focused()` reports whether the component's element has focus, however it got it.
  `FocusHandle.Focused` re-renders a component that reads it in its render when it changes, and
  the new `FocusHandle.FocusWithin` reports focus on the element or inside it, the same way.
- `uitest`: `Element.ClickWith(mods)`, `Element.ClickAt(dx, dy, mods...)` and
  `UI.ClickAt(x, y, mods...)` click with modifiers held or at a point, as `Click` does.

### Fixed

- Alt+Pause no longer focuses the menu bar when Alt is released before Pause.
