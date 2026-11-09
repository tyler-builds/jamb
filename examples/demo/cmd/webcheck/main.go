//go:build js

// Command webcheck renders every page of the demo in a js build, for the web
// build's CI job (scripts/web/check.sh). The pages hold most of jamb's widgets,
// the Markdown view and the code editor, so a TinyGo build that crashes, hangs
// or draws nothing on one of them fails the job. It exits with status 1 when a
// page draws nothing.
//
// It needs no window and no app package: it drives a ui.Runtime directly and
// draws with the CPU renderer.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tyler-builds/jamb/examples/demo/views"
	"github.com/tyler-builds/jamb/internal/render"
	"github.com/tyler-builds/jamb/internal/render/soft"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

type host interface {
	Frame(time.Time) *scene.Scene
}

func main() {
	const w, h = 1200, 760
	s := views.NewStore()
	rt := ui.NewRuntime(views.App(s), views.DemoDarkTheme, nil)
	rt.SetViewport(w, h, 1)
	hst := uihost.Of(rt).(host)
	r := soft.New(nil)
	now := time.Unix(1_000_000, 0)

	failed := false
	for _, p := range []struct {
		name string
		page views.Page
	}{
		{"Dashboard", views.PageDashboard}, {"Projects", views.PageProjects}, {"Activity", views.PageActivity},
		{"Components", views.PageComponents}, {"Settings", views.PageSettings}, {"Markdown", views.PageMarkdown},
	} {
		s.Page.Set(p.page)
		// A page's entrance and its first highlighting take a few frames.
		const frames = 30
		t0 := time.Now()
		for range frames {
			now = now.Add(16 * time.Millisecond)
			if err := render.Draw(r, hst.Frame(now), w, h); err != nil {
				fmt.Printf("page %s: %v\n", p.name, err)
				os.Exit(1)
			}
		}
		ms := float64(time.Since(t0).Microseconds()) / 1000 / frames
		nodes, colors := rt.Stats().Nodes, distinctColors(r.Image().Pix)
		fmt.Printf("page %-10s %5d nodes %5d colors %6.2f ms per frame\n", p.name, nodes, colors, ms)
		if nodes == 0 || colors < 16 {
			fmt.Printf("page %s drew nothing\n", p.name)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

// distinctColors counts the different RGBA values in pix, up to 1000.
func distinctColors(pix []byte) int {
	seen := map[[4]byte]bool{}
	for i := 0; i+4 <= len(pix) && len(seen) < 1000; i += 4 {
		seen[[4]byte(pix[i:i+4])] = true
	}
	return len(seen)
}
