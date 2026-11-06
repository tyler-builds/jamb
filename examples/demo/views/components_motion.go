package views

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ease"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// motionTimingSamples are the samples of the motion API beyond the transitions and
// the easing curves: springs, keyframes, timelines, delays, staggers and completion.
// Each plays when it is first shown; its button plays it again. Its state is kept in
// the component, so only that sample re-renders.
func motionTimingSamples(t *ui.Theme) []ui.Element {
	return []ui.Element{
		springSample(t),
		reorderSample(t),
		expandCardSample(t),
		matchedSample(t),
		keyframeSample(t),
		timelineSample(t),
		loopSample(t),
		staggerSample(t),
		delaySample(t),
	}
}

// springSample is two tiles that move to the right and back as springs: one settles
// without overshoot, and the other bounces past its target. A tile that is moved while
// it is still moving keeps its speed, so the second press bends the motion.
func springSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		right := ui.Local(ctx, "motion.springRight", false)
		x := float32(0)
		if right.Get() {
			x = 120
		}
		still := tile(t, "Bounce 0").Spring(ui.Spring{Duration: 420 * time.Millisecond}).Offset(x, 0)
		bouncy := tile(t, "Bounce 0.5").Spring(ui.Spring{Duration: 420 * time.Millisecond, Bounce: 0.5}).Offset(x, 0)
		return sample(t, "Spring: Move, then Move back", ui.Div().Column().Gap(10).Width(ui.Px(240)).
			Child(still).
			Child(bouncy).
			Child(ui.Button("Move").Key("motion-move").Secondary().Small().OnClick(func() { right.Set(!right.Peek()) })))
	})
}

// reorderSample is a list whose rows glide to their new places when it is
// reordered: ChildrenLayoutSpring gives each row the theme's Reflow spring. The
// layout moves a row at once, and the row is drawn where the spring has brought
// it, so a second press mid-flight bends the motion. Each row is keyed, so it
// keeps its identity as it moves.
func reorderSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		order := ui.Local(ctx, "motion.order", []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"})
		items := order.Get()
		list := ui.Div().Key("motion-reorder").Column().Gap(4).Width(ui.Px(240)).
			ChildrenLayoutSpring(t.Motion.ReflowSpring())
		for i, name := range items {
			list.Child(ui.Div().Key("motion-row-"+name).Row().Align(ui.Center).Gap(8).Height(ui.Px(32)).
				PaddingX(10).Radius(8).Background(t.Surface).Border(1, t.Border).
				Child(ui.Div().Grow(1).Child(ui.Text(name).FontSize(12))).
				Child(ui.IconButton(icons.ArrowUp).Label("Move " + name + " up").Small().Disabled(i == 0).OnClick(func() {
					next := slices.Clone(order.Peek())
					if j := slices.Index(next, name); j > 0 {
						next[j-1], next[j] = next[j], next[j-1]
						order.Set(next)
					}
				})))
		}
		// The shuffles are the same in every run of the demo, so its tests see them.
		rng := ui.Local(ctx, "motion.rng", (*rand.Rand)(nil))
		shuffle := func() {
			if rng.Peek() == nil {
				rng.Set(rand.New(rand.NewPCG(1, 2)))
			}
			r := rng.Peek()
			next := slices.Clone(order.Peek())
			for slices.Equal(next, order.Peek()) {
				r.Shuffle(len(next), func(i, j int) { next[i], next[j] = next[j], next[i] })
			}
			order.Set(next)
		}
		return sample(t, "Layout spring: reorder", ui.Div().Column().Gap(10).Width(ui.Px(240)).
			Child(list).
			Child(ui.Button("Shuffle").Key("motion-shuffle").Secondary().Small().OnClick(shuffle)))
	})
}

// expandCardSample is a column of cards that grow to show their details. Each card
// is 52 px tall, clipping what it holds, until its button lets it take the height
// of its content. ChildrenLayoutSpring gives the cards the theme's Reflow spring,
// so a card's box springs to its new height, revealing its details with its
// bottom edge, and the cards below it glide down with it. The details are laid
// out once, at the card's final size: the text never scales or rewraps.
func expandCardSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		open := ui.Local(ctx, "motion.cardOpen", "")
		cards := ui.Div().Key("motion-cards").Column().Gap(6).Width(ui.Px(240)).
			ChildrenLayoutSpring(t.Motion.ReflowSpring())
		for _, c := range []struct{ name, detail string }{
			{"Build", "Compiles the app and its examples for each platform, then runs the tests headless."},
			{"Release", "Tags the version, writes the changelog from its fragments and publishes the notes."},
			{"Docs", "Checks that every exported name has a comment and that the guides build."},
		} {
			on := open.Get() == c.name
			label, icon := "Show details for "+c.name, icons.ChevronDown
			if on {
				label, icon = "Hide details for "+c.name, icons.ChevronUp
			}
			card := ui.Div().Key("motion-card-"+c.name).Column().Gap(6).Padding(10).Radius(8).
				Background(t.Surface).Border(1, t.Border).ClipContent().
				Child(ui.Div().Row().Align(ui.Center).Height(ui.Px(30)).Shrink(0).
					Child(ui.Div().Grow(1).Child(ui.Text(c.name).FontSize(13).FontWeight(ui.FontMedium))).
					Child(ui.IconButton(icon).Label(label).Small().OnClick(func() {
						if open.Peek() == c.name {
							open.Set("")
						} else {
							open.Set(c.name)
						}
					}))).
				Child(ui.Text(c.detail).FontSize(12).TextColor(t.TextMuted))
			if !on {
				card.Height(ui.Px(52)) // the header, with the details below it clipped
			}
			cards.Child(card)
		}
		return sample(t, "Layout spring: expanding card", cards)
	})
}

// matchedPhotos are the pictures of the shared-element sample: a sky, the
// ground under it, and a sun, drawn with colored boxes.
var matchedPhotos = []struct {
	name             string
	sky, ground, sun ui.Color
	sunX             float32 // the sun's center, as a share of the picture's width
}{
	{"Dusk", ui.Hex("#f2a07b"), ui.Hex("#3a285c"), ui.Hex("#ffecaa"), 0.3},
	{"Lake", ui.Hex("#78c8ff"), ui.Hex("#1e5a46"), ui.Hex("#ffffeb"), 0.42},
	{"Haze", ui.Hex("#c496ff"), ui.Hex("#281e46"), ui.Hex("#ffc8f0"), 0.54},
	{"Dune", ui.Hex("#ffd678"), ui.Hex("#783c1e"), ui.Hex("#fffadc"), 0.66},
}

// matchedSample is a grid of four pictures. Clicking one opens it on the whole
// stage; Back, Escape or a click on it closes it. The tile in the grid and the
// picture on the stage are two elements with the same Matched id, each laid
// out where it is: when one replaces the other, it flies from the other's
// place to its own, scaled to cover its moving frame, so the picture zooms
// without distortion. The grid fades and shrinks out under it, and back in.
func matchedSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		open := ui.Local(ctx, "motion.photo", -1)
		last := ui.Local(ctx, "motion.photoLast", -1) // the picture last opened, which takes the focus back
		close := func() { open.Set(-1) }
		const w, h = 320, 208
		picture := func(i int, pw, ph float32) *ui.DivElement {
			p := matchedPhotos[i]
			id := "motion-photo-" + p.name
			sun := ph * 0.22
			return ui.Div().Key(id).Matched(id).Size(ui.Px(pw), ui.Px(ph)).Radius(10).ClipContent().
				Background(p.ground).Column().Focusable().Role(access.RoleButton).
				Child(ui.Div().Height(ui.Px(ph * 0.62)).Shrink(0).Background(p.sky)).
				Child(ui.Div().Absolute().Left(pw*p.sunX-sun/2).Top(ph*0.4-sun/2).
					Size(ui.Px(sun), ui.Px(sun)).Radius(sun / 2).Background(p.sun)).
				Child(ui.Div().Absolute().Left(10).Bottom(8).
					Child(ui.Text(p.name).FontSize(12).FontWeight(ui.FontMedium).TextColor(ui.Hex("#ffffff"))))
		}
		stage := ui.Div().Key("motion-stage").Size(ui.Px(w), ui.Px(h))
		if i := open.Get(); i >= 0 {
			name := matchedPhotos[i].name
			stage.Child(picture(i, w, h).Label("Close " + name).AutoFocus().OnClick(close).
				OnKeyDown(func(e *ui.KeyEvent) {
					if e.Key == input.KeyEscape {
						close()
					}
				}))
		} else {
			const gap = 8
			grid := ui.Div().Key("motion-grid").Row().Wrap().Gap(gap).Size(ui.Px(w), ui.Px(h)).
				Enter(ui.Fade().Grow(0.96), ui.Animation{Duration: 200 * time.Millisecond}).
				Exit(ui.Fade().Grow(0.96), ui.Animation{Duration: 200 * time.Millisecond})
			for j, p := range matchedPhotos {
				tile := picture(j, (w-gap)/2, (h-gap)/2).Label("Open " + p.name).OnClick(func() {
					last.Set(j)
					open.Set(j)
				})
				if j == last.Peek() {
					tile.AutoFocus()
				}
				grid.Child(tile)
			}
			stage.Child(grid)
		}
		back := ui.Button("Back").Key("motion-photo-back").Secondary().Small().Disabled(open.Get() < 0).OnClick(close)
		return sample(t, "Shared-element motion: Matched", ui.Div().Column().Gap(10).
			Child(stage).Child(back))
	})
}

// keyframeSample is a tile that pulses through two keyframes with Play. Play starts
// when the tile first appears, and again whenever its key changes.
func keyframeSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		plays := ui.Local(ctx, "motion.plays", 0)
		frames := []ui.Keyframe{
			{At: 0.3, Motion: ui.Grow(1.12).Opacity(0.7)},
			{At: 0.6, Motion: ui.Slide(0, -10)},
		}
		return sample(t, "Keyframes: Play", ui.Div().Column().Gap(10).Width(ui.Px(240)).
			Child(tile(t, "Pulse").Play(plays.Get(), frames, ui.Animation{Duration: 800 * time.Millisecond, Easing: ease.OutCubic})).
			Child(ui.Button("Play").Key("motion-play-keyframes").Secondary().Small().OnClick(func() { plays.Set(plays.Peek() + 1) })))
	})
}

// timelineSample runs a fade and a spring slide one after the other, then a glow and a
// pop together, with AnimateTimeline. Its progress values are in the order the steps
// are given: fade, slide, glow and pop.
func timelineSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		runs := ui.Local(ctx, "motion.runs", 0)
		fade := ui.Animation{Duration: 300 * time.Millisecond, Easing: ease.OutCubic}
		slide := ui.Animation{Duration: 400 * time.Millisecond, Easing: ease.Spring(0.3)}
		glow := ui.Animation{Duration: 500 * time.Millisecond, Easing: ease.InOutCubic}
		pop := ui.Animation{Duration: 300 * time.Millisecond, Easing: ease.OutCubic}
		tl := ui.Sequence(fade, slide).Then(ui.Parallel(ui.Sequence(glow), ui.Sequence(pop)))
		stage := ui.AnimateTimeline(fmt.Sprintf("motion-timeline-%d", runs.Get()), tl, func(p []float32) ui.Element {
			return ui.Div().Row().Align(ui.Center).Gap(12).Height(ui.Px(56)).
				Child(tile(t, "Timeline").Opacity(p[0]).Offset(40*(1-p[1]), 0).Scale(0.9 + 0.1*p[3])).
				Child(ui.Div().Opacity(p[2]).Child(ui.Text("Done").FontSize(12).TextColor(t.Accent)))
		})
		return sample(t, "Timeline: Sequence, Then, Parallel", ui.Div().Column().Gap(10).Width(ui.Px(240)).
			Child(stage).
			Child(ui.Button("Play").Key("motion-play-timeline").Secondary().Small().OnClick(func() { runs.Set(runs.Peek() + 1) })))
	})
}

// loopSample is a timeline that repeats forever: a tile goes out and comes back, as
// Repeat keeps the timeline playing while it is drawn.
func loopSample(t *ui.Theme) ui.Element {
	out := ui.Animation{Duration: 700 * time.Millisecond, Easing: ease.InOutCubic}
	back := ui.Animation{Duration: 700 * time.Millisecond, Easing: ease.InOutCubic}
	tl := ui.Sequence(out, back).Repeat()
	return sample(t, "Repeat: a timeline that loops", ui.Div().Width(ui.Px(240)).Height(ui.Px(56)).
		Child(ui.AnimateTimeline("motion-loop", tl, func(p []float32) ui.Element {
			return ui.Div().Height(ui.Px(56)).Child(tile(t, "Loop").Offset(120*p[0]*(1-p[1]), 0))
		})))
}

// staggerSample is five tiles that enter one after another, 80 ms apart. Its Play
// button builds the row again under a new key, so the tiles enter again.
func staggerSample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		runs := ui.Local(ctx, "motion.staggerRuns", 0)
		row := ui.Div().Key(fmt.Sprintf("motion-stagger-%d", runs.Get())).Row().Wrap().Gap(6).Width(ui.Px(240))
		for i := range 5 {
			row.Child(tile(t, fmt.Sprintf("Item %d", i+1)).Enter(ui.Slide(0, 12).Fade(),
				ui.Animation{Duration: 240 * time.Millisecond, Stagger: 80 * time.Millisecond}))
		}
		return sample(t, "Stagger: siblings enter one after another", ui.Div().Column().Gap(10).Width(ui.Px(240)).
			Child(row).
			Child(ui.Button("Play").Key("motion-play-stagger").Secondary().Small().OnClick(func() { runs.Set(runs.Peek() + 1) })))
	})
}

// delaySample is a tile that enters after a 600 ms delay. Its OnDone runs when the
// entrance ends, and says so under the tile.
func delaySample(t *ui.Theme) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		runs := ui.Local(ctx, "motion.delayRuns", 0)
		done := ui.Local(ctx, "motion.delayDone", "")
		status := done.Get()
		if status == "" {
			status = "The tile waits 600 ms, then fades in."
		}
		return sample(t, "Delay and OnDone", ui.Div().Column().Gap(10).Width(ui.Px(240)).
			Child(ui.Div().Key(fmt.Sprintf("motion-delay-%d", runs.Get())).
				Child(tile(t, "Delayed").Enter(ui.Fade(), ui.Animation{Duration: 300 * time.Millisecond,
					Delay: 600 * time.Millisecond, OnDone: func() { done.Set("Finished: the tile is in place.") }}))).
			Child(ui.Text(status).FontSize(12).TextColor(t.TextMuted)).
			Child(ui.Button("Play").Key("motion-play-delay").Secondary().Small().OnClick(func() { done.Set(""); runs.Set(runs.Peek() + 1) })))
	})
}
