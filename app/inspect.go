package app

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// The visual inspector is a development window that shows the elements of
// another window: its element tree, the resolved style and layout of the
// selected element with where each style value came from, its accessibility
// node, live style edits, and the frame timings of the window. It is a window of
// its own, so it never changes the layout of the window it inspects. It is on
// under `jamb dev`, with JAMB_INSPECT=1, and in an app that sets App.DevTools;
// Ctrl+Shift+I opens and closes it for the focused window.
//
// The inspector runs on the UI thread with the windows it inspects, and reads
// them through the inspect.Target that the ui package gives their runtimes.

const (
	// inspectRefresh is how often the inspector reads its window again while the
	// window draws. A change the inspector makes is shown at once.
	inspectRefresh = 250 * time.Millisecond
	// inspectHistory is how many frames the timing sparkline keeps.
	inspectHistory = 120
)

// inspector is the inspector of one window: the state of its window and the data
// it shows. It is used on the UI thread only.
type inspector struct {
	target *host           // the inspected window
	tg     inspect.Target  // the target side of the inspected window's runtime
	win    ui.Window       // the inspector's own window
	perf   *inspect.Perf   // the frame timings of the inspected window
	due    time.Time       // when the inspector reads the target again, at the latest
	tick   *ui.Signal[int] // changes to make the inspector read the target again
	sel    *ui.Signal[string]
	field  *ui.Signal[string] // the style field the edit row sets
	value  *ui.Signal[string] // the value the edit row sets it to
	status *ui.Signal[string] // the outcome of the last edit or copy
}

// newInspector returns the state of an inspector for tg, the target side of the
// runtime of target.
func newInspector(target *host, tg inspect.Target) *inspector {
	first := ""
	if fields := tg.EditableFields(); len(fields) > 0 {
		first = fields[0]
	}
	return &inspector{
		target: target, tg: tg,
		perf:   inspect.NewPerf(inspectHistory),
		tick:   ui.NewSignal(0),
		sel:    ui.NewSignal(""),
		field:  ui.NewSignal(first),
		value:  ui.NewSignal(""),
		status: ui.NewSignal(""),
	}
}

// startInspector opens the inspector of a window at start, when JAMB_INSPECT=1
// asks for it. It runs after the window is created, on the UI thread.
func (h *host) startInspector() {
	if h.primary && h.dev.inspect {
		postUI(func() { openInspector(h) })
	}
}

// inspectKey handles the inspector's shortcut, Ctrl+Shift+I, which opens the
// inspector of the window, or closes it if it is open. It reports whether k is
// the shortcut, which then does not reach the window.
func (h *host) inspectKey(k input.Key, mods input.Modifiers) bool {
	if !h.devTools || h.inspected != nil || k != input.KeyI || mods != input.ModCtrl|input.ModShift {
		return false
	}
	if h.insp != nil {
		h.insp.win.Close()
	} else {
		openInspector(h)
	}
	return true
}

// openInspector opens the inspector of the window h, unless h has one already.
func openInspector(h *host) {
	if h.insp != nil || h.rt == nil || h.handle.closed.Load() {
		return
	}
	tg, ok := uihost.Of(h.rt).(inspect.Target)
	if !ok {
		return
	}
	ins := newInspector(h, tg)
	h.insp = ins
	tg.Attach(ins.changed)
	handle := &appWindow{}
	ins.win = handle
	cfg := withDefaults(Window{
		Title:    "Inspector: " + h.cfg.Title,
		Width:    1100,
		Height:   760,
		Strings:  h.cfg.Strings,
		Root:     ins.root,
		OnClosed: ins.close,
	})
	hi, err := openWindow(cfg, handle)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: inspector:", err)
		ins.close()
		return
	}
	// The inspector's own window has no inspector of its own, so Ctrl+Shift+I
	// there does nothing.
	hi.inspected = h
	hi.devTools = false
}

// frame records the timings of the inspected window's frame, and asks the
// inspector to read the window again when the refresh interval has passed.
// It runs after every frame of the window, on the UI thread.
func (ins *inspector) frame(rt *ui.Runtime, now time.Time) {
	st := rt.Stats()
	if st.Built || st.LaidOut || st.Painted {
		ins.perf.Add(inspect.Sample{
			Build: st.BuildTime, Layout: st.LayoutTime, Paint: st.PaintTime,
			Nodes: st.Nodes, Rendered: st.Rendered, Quads: st.Quads, Glyphs: st.Glyphs,
		})
	}
	if now.Before(ins.due) {
		return
	}
	ins.due = now.Add(inspectRefresh)
	ins.bump()
}

// bump makes the inspector render again, which reads the target again.
func (ins *inspector) bump() {
	ins.tick.Update(func(n int) int { return n + 1 })
}

// changed is the target's callback when its selection or its pick mode changes:
// the tree follows the selection, and the details are read again.
func (ins *inspector) changed() {
	ins.sel.Set(keyOf(ins.tg.Selected()))
	ins.bump()
}

// close ends the inspection: the target stops drawing its outlines and drops its
// live edits. It runs when the inspector's window closes, and once more, after
// the window it inspects has closed, when it does nothing.
func (ins *inspector) close() {
	if ins.target.insp == ins {
		ins.target.insp = nil
		ins.tg.Detach()
	}
}

// keyOf returns the tree key of an element: its ID as text.
func keyOf(id uint64) string { return strconv.FormatUint(id, 10) }

// idOf returns the element ID of a tree key, or 0.
func idOf(key string) uint64 {
	id, _ := strconv.ParseUint(key, 10, 64)
	return id
}
