//go:build js

// Command webcheck draws a page of cards in a js build and prints a hash of its
// pixels, for the web build's CI job (scripts/web/check.sh). The job runs it
// built with Go and with TinyGo, and fails when the hashes differ, which catches
// a TinyGo build that computes something differently.
//
// It needs no window and no app package: it drives a ui.Runtime directly and
// draws with the CPU renderer. The page holds no clock and no random values, so
// its pixels are the same on every run.
package main

import (
	"fmt"
	"hash/fnv"
	"runtime"
	"time"

	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/soft"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

type host interface {
	Frame(time.Time) *scene.Scene
	InvalidateAll()
}

func main() {
	for _, v := range []struct{ w, h, scale int }{{1200, 760, 1}, {390, 844, 3}} {
		rt := ui.NewRuntime(cards, nil, nil)
		rt.SetViewport(float32(v.w), float32(v.h), float32(v.scale))
		h := uihost.Of(rt).(host)
		r := soft.New(nil)
		now := time.Unix(1_000_000, 0)
		pw, ph := v.w*v.scale, v.h*v.scale
		draw := func() {
			now = now.Add(16 * time.Millisecond)
			if err := render.Draw(r, h.Frame(now), pw, ph); err != nil {
				panic(err)
			}
		}
		draw()
		sum := fnv.New64a()
		sum.Write(r.Image().Pix)
		fmt.Printf("pixels %dx%d@%d %x\n", v.w, v.h, v.scale, sum.Sum64())

		// The time of a full rebuild and draw, for the job's log only.
		const frames = 20
		t0 := time.Now()
		for range frames {
			h.InvalidateAll()
			draw()
		}
		fmt.Printf("timing %dx%d@%d (%s): %.1f ms per rebuilt frame\n", v.w, v.h, v.scale,
			runtime.Compiler, float64(time.Since(t0).Microseconds())/1000/frames)
	}
}

var done = ui.NewSignal(false)

// cards is a page shaped like the demo's Projects page: a sidebar, a header and
// a column of cards with text, a badge, a progress bar, a checkbox and buttons.
func cards(ctx *ui.Context) ui.Element {
	t := ctx.Theme()
	side := ui.Div().Column().Gap(4).Padding(16).Width(ui.Px(240)).Background(t.Surface)
	for _, n := range []string{"Dashboard", "Projects", "Activity", "Components", "Markdown", "Settings"} {
		side.Child(ui.Div().Padding(8).Radius(6).Child(ui.Text(n).TextColor(t.Text)))
	}
	list := ui.Div().Column().Gap(12).Padding(24).Grow(1)
	list.Child(ui.Div().Row().Gap(12).Child(ui.Text("Projects").FontSize(24).TextColor(t.Text)).
		Child(ui.Button("New project")))
	for i := range 6 {
		list.Child(ui.Div().Column().Gap(8).Padding(16).Radius(8).Background(t.Surface).Border(1, t.Border).
			Child(ui.Div().Row().Gap(8).
				Child(ui.Text(fmt.Sprintf("Project %c", 'A'+i)).FontSize(16).TextColor(t.Text)).
				Child(ui.BadgeText("Active"))).
			Child(ui.Text("Native GPU renderer and the scene graph that feeds it, with layout and text.").TextColor(t.TextMuted)).
			Child(ui.ProgressBar(float64(i+1) / 7)).
			Child(ui.Div().Row().Gap(8).
				Child(ui.Checkbox("Starred", done)).
				Child(ui.Button("Open")).
				Child(ui.Text(fmt.Sprintf("%d open tasks", 3*i+2)).TextColor(t.TextMuted))))
	}
	return ui.Div().Row().Background(t.Background).Child(side).Child(list)
}
