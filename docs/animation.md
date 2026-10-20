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
  `OutCubic`, `InOutCubic`, `Bounce(f)` (out and back), `Reverse(f)` and
  `Spring(bounce)` (see Springs).
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

## Springs

`.Spring(s)` moves changes of `Opacity`, `Offset` and `Scale` as a damped
spring instead of a timed ease. The spring has a velocity, so a target that
changes mid-flight (a hover that ends halfway, a drag that reverses) bends the
motion rather than restarting it.

```go
ui.Div().Spring(ui.Spring{Duration: 300 * time.Millisecond, Bounce: 0.2}).
	Hover(ui.Style{Scale: 1.04, OffsetY: -2})
```

- `Duration` is the perceived length of the response, the spring's natural
  period: a critically damped spring is close to its target after one
  `Duration`.
- `Bounce` is how far it overshoots: 0 (the default) is critically damped and
  never passes its target; larger values swing more. It is clamped to 0..0.9.
- The step is exact for any frame gap, so the motion does not depend on the
  frame rate. The spring rests (and asks for no more frames) once it is within
  0.001 of its target (0.05 px for offsets) and nearly still.
- Opacity stays in 0..1 and scale at 0 or more. Colors never overshoot: they
  keep easing as a `Transition` does, over the `Transition` when one is set and
  over the spring's `Duration` otherwise. The first paint shows the target at
  once, as with `Transition`.
- `Style.Spring` sets it in a style; a zero `Spring` is unset, so it merges as
  `Transition` does.

`ease.Spring(bounce)` is the spring's shape as an easing function, for timed
motion: `Enter`, `Exit`, `Animate` and keyframe segments. It is not an engine:
it has no velocity, and it ends at the animation's `Duration`, exactly at 1.

```go
ui.Div().Enter(ui.Grow(0.9), ui.Animation{Duration: 400 * time.Millisecond, Easing: ease.Spring(0.3)})
```

## Cost

`Opacity`, `Offset`, `Scale`, transitions, springs and enter and exit are paint-only,
so they cost no layout. A property that changes the layout (`Width`,
`Padding`, ...) costs a re-layout of the path to the root on each frame of its
animation. An exiting element leaves the layout at once, so its siblings move
straight away. It is drawn above its former siblings while it exits. It is a
ghost: it cannot be hit, focused or found, and it disappears when its exit ends.
Showing it again during its exit drops the ghost.

## Reduced motion

With Windows' Animation effects turned off (Settings, Accessibility, Visual
effects), jamb asks for reduced motion (`ctx.ReducedMotion()`). Then `Transition`,
`Enter` and `Exit` jump to their end, a spring jumps to its target and stops, and
smooth scrolling jumps to its target. An
`Animate` that is finite jumps to its last frame, and a repeating one stops at its
first frame, so nothing asks for more frames. Apps should check `ctx.ReducedMotion()`
before they run their own motion.
