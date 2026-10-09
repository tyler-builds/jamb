package ui

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// ComponentElement is a re-render boundary. Its function runs when the
// component first appears, when a signal it read changes, or when its parent
// re-renders. Otherwise its previous output — and the realized, laid-out
// nodes built from it — are kept as they are.
type ComponentElement struct {
	fn  func(ctx *Context) Element
	key string
}

// Component wraps a render function as a re-render boundary:
//
//	ui.Component(func(ctx *ui.Context) ui.Element {
//	    return ui.Textf("Count: %d", counter.Get())
//	})
//
// When counter changes only this function re-runs, only its subtree is
// rebuilt, and layout is recomputed only where sizes actually changed.
func Component(fn func(ctx *Context) Element) *ComponentElement {
	return &ComponentElement{fn: fn}
}

func (*ComponentElement) isElement()           {}
func (c *ComponentElement) elementKey() string { return c.key }

// Key gives the component a stable identity among its siblings.
func (c *ComponentElement) Key(k string) *ComponentElement { c.key = k; return c }

// componentInstance is the persistent side of a component or of a composite
// element (Button, TextInput, ...). It owns the node subtree realized from
// its last output.
type componentInstance struct {
	rt  *Runtime
	id  uint64
	el  Element // *ComponentElement or expander
	out Element

	dirty atomic.Bool
	depMu sync.Mutex // guards deps; signals may be read from other goroutines
	deps  []subscribable

	// Tree bookkeeping, UI thread only.
	gen        uint32 // frame in which the instance was last claimed by its parent
	depth      int
	parent     *componentInstance
	children   []*componentInstance
	root       *node     // realized output (nil when the output is nil)
	parentNode *node     // node root is attached to (nil for the runtime root)
	inh        textStyle // inherited text style root was built with
	disposed   bool

	// watchesFocus is set when the last render asked whether its element is
	// focused; a focus change then re-renders it.
	watchesFocus bool

	// locals holds signals created by Local, by name.
	locals map[string]any
}

func (inst *componentInstance) invalidate() {
	if inst.dirty.CompareAndSwap(false, true) {
		inst.rt.enqueueDirty(inst)
	}
}

func (inst *componentInstance) addDep(s subscribable) {
	inst.depMu.Lock()
	inst.deps = append(inst.deps, s)
	inst.depMu.Unlock()
}

// unsubscribe detaches the instance from every signal it read.
func (inst *componentInstance) unsubscribe() {
	inst.depMu.Lock()
	deps := inst.deps
	inst.deps = nil
	inst.depMu.Unlock()
	for _, d := range deps {
		d.unsubscribe(inst)
	}
}

// render runs the component function (or expands the composite element)
// with dependency tracking.
func (inst *componentInstance) render() {
	inst.unsubscribe()
	inst.dirty.Store(false)
	inst.watchesFocus = false
	ctx := &Context{rt: inst.rt, inst: inst, id: inst.id}
	prev := tracking.Swap(inst)
	defer tracking.Store(prev)
	switch e := inst.el.(type) {
	case *ComponentElement:
		inst.out = e.fn(ctx)
	case expander:
		inst.out = e.expand(ctx)
	}
	inst.rt.stats.Rendered++
}

// Context is passed to render functions. It gives access to the theme and
// window and lets components schedule their own re-render.
type Context struct {
	rt   *Runtime
	inst *componentInstance
	id   uint64
}

// Theme returns the active theme.
func (c *Context) Theme() *Theme { return c.rt.theme }

// SetTheme switches the theme and re-renders everything.
func (c *Context) SetTheme(t *Theme) { c.rt.SetTheme(t) }

// WindowSize returns the window size in logical pixels.
func (c *Context) WindowSize() (w, h float32) { return c.rt.viewW, c.rt.viewH }

// Window returns the window the component is shown in. Its Maximized and
// Focused methods are reactive: the component re-renders when they change.
// Without an app window (headless tests) the actions do nothing.
func (c *Context) Window() WindowControls { return windowRef{c.rt} }

// Post runs fn on the UI thread at the start of the next frame (see
// Runtime.Post). It is safe to call from a goroutine that holds the Context,
// so background work can hand its results back to the window.
func (c *Context) Post(fn func()) { c.rt.Post(fn) }

// Zoom returns the UI zoom factor (1 = 100%).
func (c *Context) Zoom() float32 { return c.rt.zoom }

// SetZoom scales the whole UI (see Runtime.SetZoom).
func (c *Context) SetZoom(z float32) { c.rt.SetZoom(z) }

// Invalidate re-renders the current component on the next frame.
func (c *Context) Invalidate() {
	if c.inst != nil {
		c.inst.invalidate()
	}
}

// Local returns a signal that belongs to the calling component and lives as
// long as the component does — state that no other part of the app needs,
// such as a draft being typed, an expanded/collapsed flag or a hover-card's
// visibility:
//
//	ui.Component(func(ctx *ui.Context) ui.Element {
//	    open := ui.Local(ctx, "open", false)
//	    return ui.Button("Details").OnClick(func() { open.Set(!open.Peek()) }) ...
//	})
//
// Locals are found by name, not by call order, so calling Local inside
// conditions or loops is fine. The first call creates the signal with
// initial; later calls return it (initial is ignored). Reading it with Get
// re-renders the component when it changes, like any signal. Using the same
// name with a different type panics with an explanation.
func Local[T any](ctx *Context, name string, initial T) *Signal[T] {
	inst := ctx.inst
	if inst == nil {
		panic("ui.Local(" + name + "): no component is rendering; call it inside a ui.Component render function")
	}
	if inst.locals == nil {
		inst.locals = map[string]any{}
	}
	if v, ok := inst.locals[name]; ok {
		s, ok := v.(*Signal[T])
		if !ok {
			panic(fmt.Sprintf("ui.Local(%q): this component already has a local named %q of type %T; "+
				"use a different name for a %T", name, name, v, initial))
		}
		return s
	}
	s := NewSignal(initial)
	inst.locals[name] = s
	return s
}

// state returns persistent per-element state for the element being expanded.
// It lives as long as the instance does.
func (c *Context) state() *nodeState { return c.rt.state(c.id) }

// isFocused reports whether the element produced by this expansion has focus.
// The instance re-renders when that changes.
func (c *Context) isFocused() bool {
	if c.inst != nil {
		c.inst.watchesFocus = true
	}
	return c.rt.focusID != 0 && c.rt.focusID == expandedID(c.id)
}

// KeyboardFocus reports whether the element produced by this render has focus
// and focus is keyboard-driven, the case in which FocusVisible styles show.
// Custom elements draw their focus ring with it. The component re-renders
// when it changes.
func (c *Context) KeyboardFocus() bool { return c.isFocused() && c.rt.keyboardModality }
