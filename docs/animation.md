# Animation

Paint-only properties ease with `.Transition`. When `Opacity`, `Offset` or
`Scale` (or a color) changes, it eases over the duration, without re-running
components or re-laying out. The first paint shows the current value at once.

```go
ui.Div().Transition(160 * time.Millisecond).Offset(0, -4).Scale(1.02).Opacity(0.9)
```

`ui.Animate` re-renders a function every frame with eased progress, so any
property can animate, layout included:

```go
ui.Animate("bar", ui.Animation{Duration: 600 * time.Millisecond, Easing: ease.OutCubic},
	func(t float32) ui.Element {
		return ui.Div().Height(ui.Percent(v * 100 * t)).Background(accent)
	})
```

- `ui.Animation{Duration, Easing, Repeat}`. `Easing` maps progress 0..1 to
  0..1; nil is ease-out cubic. `Repeat` loops forever (for `Animate`). An
  `Animate` restarts from 0 when its key changes, and stops asking for frames
  when it ends: after the end its function runs once more with t = 1.
- An animation whose element is out of view asks for no frames either: clipped
  or scrolled out of the viewport, hidden by `Display(false)`, or in a window
  that is minimized or covered by another window. The progress is time based,
  so when the element is drawn again the animation shows the progress of that
  moment and runs on from there, with no jump.
- `github.com/tyler-builds/jamb/ease` has `Linear`, `In`, `Out`, `InOut` (quadratic), `InCubic`,
  `OutCubic`, `InOutCubic`, `Bounce(f)` (out and back) and `Reverse(f)`.
- `.Offset(x, y)` moves an element after layout: its place in the flow does
  not change, and hit testing and `Find` follow the element. `.Scale(s)` draws
  the element and its children about their center. Hit testing ignores scale.
- `.Enter(m, a)` plays when the element first appears: it starts with the look
  of the `Motion` `m` and settles into its normal look over `a`.
  `.Exit(m, a)` keeps the element drawn after it is removed while it moves to
  `m`. The helpers are `ui.Fade()`, `ui.Slide(x, y)` and `ui.Grow(s)`, which
  combine by chaining: `ui.Fade().Slide(0, -6).Grow(0.98)`.

```go
ui.Div().Key("palette").Overlay(ui.BelowCenter).
	Enter(ui.Fade().Slide(0, -6).Grow(0.98), ui.Animation{Duration: 140 * time.Millisecond}).
	Exit(ui.Fade().Grow(0.98), ui.Animation{Duration: 90 * time.Millisecond})
```

Cost: `Opacity`, `Offset`, `Scale`, transitions and enter and exit are paint-only,
so they cost no layout. A property that changes the layout (`Width`,
`Padding`, ...) costs a re-layout of the path to the root on each frame of its
animation. An exiting element leaves the layout at once, so its siblings move
straight away. It is drawn above its former siblings while it exits. It is a
ghost: it cannot be hit, focused or found, and it disappears when its exit ends.
Showing it again during its exit drops the ghost.

With Windows' Animation effects turned off (Settings, Accessibility, Visual
effects), jamb asks for reduced motion (`ctx.ReducedMotion()`). Then `Transition`,
`Enter` and `Exit` jump to their end, and smooth scrolling jumps to its target. An
`Animate` that is finite jumps to its last frame, and a repeating one stops at its
first frame, so nothing asks for more frames. Apps should check `ctx.ReducedMotion()`
before they run their own motion.
