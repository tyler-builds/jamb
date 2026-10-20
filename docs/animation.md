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

## Keyframes (Play)

`.Play(key, frames, a)` runs an element through keyframes: at each stop's `At`
(0..1 of `a.Duration`) it has the look of the stop's `Motion`, and between stops
it moves from one look to the next. `Motion.Opacity(o)` lets a stop rest at a
partial opacity.

```go
ui.Div().Play("pulse", []ui.Keyframe{
	{At: 0.5, Motion: ui.Grow(1.06).Opacity(0.8)},
}, ui.Animation{Duration: 600 * time.Millisecond, Repeat: true})
```

- A missing stop at 0 or 1 is the element's normal look, so the pulse above
  rises from the element as it is and returns to it.
- `Keyframe.Easing` shapes the segment that arrives at that stop; nil uses
  `a.Easing` (and nil there is ease-out cubic).
- It plays when the element first appears and again whenever `key` changes
  (`key` must be comparable, as a map key is). `Repeat` loops it.
- It is paint-only, like `Enter`: it costs no layout, and an element that is not
  drawn asks for no frames. The progress is time based, so an element drawn
  again shows the progress of that moment, with no jump.
- Keyframes animate opacity, offset and scale only; colors and shadows do not
  take keyframes.

## Sequences (Timeline)

A `Timeline` arranges `Animation`s in time, and `ui.AnimateTimeline` re-renders
a function with the progress of each of them, as `Animate` does with one:

```go
tl := ui.Sequence(
	ui.Animation{Duration: 200 * time.Millisecond}, // p[0]
	ui.Animation{Duration: 300 * time.Millisecond, Easing: ease.Spring(0.3)}, // p[1]
).Then(ui.Parallel(
	ui.Sequence(ui.Animation{Duration: 150 * time.Millisecond}), // p[2]
	ui.Sequence(ui.Animation{Duration: 400 * time.Millisecond}), // p[3]
))
ui.AnimateTimeline("intro", tl, func(p []float32) ui.Element {
	return ui.Div().Opacity(p[0]).Width(ui.Px(100 + 100*p[1]))
})
```

- `Sequence(steps...)` runs the steps one after another; `Parallel(parts...)`
  runs timelines together and lasts as long as the longest; `t.Then(next)` runs
  `next` after `t`; `t.Repeat()` loops it when `AnimateTimeline` plays it (a
  timeline made from it with `Then` or `Parallel` plays it once);
  `t.Duration()` is how long it lasts.
- `p[i]` is the eased progress of the i-th `Animation` given, in declaration
  order, whatever the nesting: 0 before it starts and 1 once it has ended. Each
  step's `Easing` shapes its own progress.
- It is an `Animate` underneath, with the timeline's duration, so it pauses out
  of view, restarts when its key changes and settles as `Animate` does.

## Delay and stagger

`Animation.Delay` holds an animation back before it starts. Meanwhile it shows
its first frame: an entrance has the look of its `Motion`, keyframes their
first stop, an `Animate` its frame at 0, and an exit the element's normal look.
A delay asks for no frames: the window is woken when the delay ends. `Settled`
is false until then, so `uitest.Settle` waits through it. A delayed entrance
that fades is invisible but can still be hit while it waits. In a timeline a
step's `Delay` adds to its start, and the timeline waits without frames until
its first step starts.

`Animation.Stagger`, on an `Enter`, delays the elements that enter together:
siblings that appear in the same render, under one parent and with the same
`Stagger`, start `Stagger` apart in the order they were built, up to 16
`Stagger`s (the 17th and later start with the 16th). A row appended later
enters at once, and a virtual list staggers only the rows it realizes.

```go
for i, item := range items {
	list.Child(row(item).Keyf("row%d", i).
		Enter(ui.Fade().Slide(0, 8), ui.Animation{Duration: 160 * time.Millisecond, Stagger: 30 * time.Millisecond}))
}
```

## Completion

`Animation.OnDone` runs once when an animation ends by itself, `Delay` plus
`Duration` after it started, whether or not it is drawn then. It runs on the UI
thread at a frame, never while rendering, so it can set signals.

- It does not run for an animation that restarts first (its key changed, or
  the element entered again) or whose element is removed first, nor for a
  repeating one.
- An `Exit`'s `OnDone` runs once the element has finished exiting. An element
  removed without its exit (off screen, or with reduced motion) has finished at
  once, and its `OnDone` runs at the next frame. A ghost dropped because the
  element was shown again does not run it.
- In a timeline each step's `OnDone` runs when that step ends.

```go
ui.Div().Exit(ui.Fade(), ui.Animation{Duration: 90 * time.Millisecond, OnDone: func() { closed.Set(true) }})
```

## Theme motion tokens

`Theme.Motion` sets the timing of the built-in widgets' motion for the whole
app. A zero field keeps the built-in default, so the zero `MotionTokens` is
today's timing:

| Field | Default | Used by |
| --- | --- | --- |
| `Hover` | 110 ms | the color transitions of the built-in controls on hover and press |
| `Enter` | 140 ms; 120 ms | menus, lists, dialogs and drawers; popovers, hover cards and tooltips |
| `Exit` | 90 ms; 80 ms | the same, as they go |
| `Spring` | unset | when set, the opacity, offset and scale of those controls move as this spring |

```go
t := ui.DarkTheme()
t.Motion.Enter = 200 * time.Millisecond // every built-in entrance, whatever its default
t.Motion.Exit = 120 * time.Millisecond
```

Setting `Enter` or `Exit` gives every built-in entrance or exit that duration.
Reduced motion still turns the motion off.

## Cost

`Opacity`, `Offset`, `Scale`, transitions, springs, enter and exit and keyframes
are paint-only, so they cost no layout. A timeline costs what `Animate` costs:
a render of its function per frame, and a re-layout if what it changes is
layout. A property that changes the layout (`Width`,
`Padding`, ...) costs a re-layout of the path to the root on each frame of its
animation. An exiting element leaves the layout at once, so its siblings move
straight away. It is drawn above its former siblings while it exits. It is a
ghost: it cannot be hit, focused or found, and it disappears when its exit ends.
Showing it again during its exit drops the ghost. A delay costs nothing while
it waits: no frames are drawn until it ends.

## Reduced motion

With Windows' Animation effects turned off (Settings, Accessibility, Visual
effects), jamb asks for reduced motion (`ctx.ReducedMotion()`). Then `Transition`,
`Enter` and `Exit` jump to their end, a spring jumps to its target and stops, and
smooth scrolling jumps to its target. An `Animate`, `Play` or `AnimateTimeline`
that is finite jumps to its last frame (the last stop, every step at 1), and a
repeating one stops at its first frame, so nothing asks for more frames. Delays
and stagger are ignored, and an `OnDone` runs at the next frame. Apps should check `ctx.ReducedMotion()`
before they run their own motion.
