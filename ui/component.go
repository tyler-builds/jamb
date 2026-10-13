package ui

import (
	"fmt"
	"image"
	neturl "net/url"
	"strings"
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
	tr    tracker // records this instance's reads while it renders

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
	// watchesPointer is set when the last render asked whether its element is
	// hovered or pressed; a change then re-renders it (see hovered).
	watchesPointer bool

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
	inst.watchesPointer = false
	ctx := &Context{rt: inst.rt, inst: inst, id: inst.id}
	inst.tr.o = inst
	prev := tracking.Swap(&inst.tr)
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

// HighContrast reports whether Windows high contrast is on, and the theme is
// then the high contrast one (see HighContrastTheme). Use it to pick solid
// colors for content that does not take its colors from the theme.
func (c *Context) HighContrast() bool { return c.rt.sys.HighContrast }

// ReducedMotion reports whether the user asked for less motion. Animations
// that the app runs itself should then jump to their end (see Animate).
func (c *Context) ReducedMotion() bool { return c.rt.sys.ReducedMotion }

// OpenURL opens url with the system: an http or https URL in the default
// browser, a mailto URL in the mail app. Other schemes, file paths included,
// are refused with an error, because opening them could run a program; a URL
// built from text the app does not control stays safe. A headless runtime
// opens nothing and returns nil.
func (c *Context) OpenURL(url string) error {
	if err := checkOpenURL(url); err != nil {
		return err
	}
	if open := c.rt.services.OpenURL; open != nil {
		return open(url)
	}
	return nil
}

// SystemDark reports whether the system's app mode is dark. A window that
// follows the system (see app.Window) already uses its dark theme then; use it
// for content that does not take its colors from the theme.
func (c *Context) SystemDark() bool { return c.rt.sys.Dark }

// TextScale returns the Text size setting in percent, from 100 to 225. Text
// sizes are scaled by it already; use it for sizes the app computes itself.
func (c *Context) TextScale() int { return c.rt.sys.TextScale }

// SetTheme switches the theme and re-renders everything.
func (c *Context) SetTheme(t *Theme) { c.rt.SetTheme(t) }

// WindowSize returns the window size in logical pixels.
func (c *Context) WindowSize() (w, h float32) { return c.rt.viewW, c.rt.viewH }

// Window returns the window the component is shown in. Its Maximized and
// Focused methods are reactive: the component re-renders when they change.
// Without an app window (headless tests) the actions do nothing.
func (c *Context) Window() Window { return windowRef{c.rt} }

// Post runs fn on the UI thread at the start of the next frame (see
// Runtime.Post). It is safe to call from a goroutine that holds the Context,
// so background work can hand its results back to the window.
func (c *Context) Post(fn func()) { c.rt.Post(fn) }

// Zoom returns the UI zoom factor (1 = 100%).
func (c *Context) Zoom() float32 { return c.rt.zoom }

// SetZoom scales the whole UI (see Runtime.SetZoom).
func (c *Context) SetZoom(z float32) { c.rt.SetZoom(z) }

// Clipboard returns the clipboard's plain text, with LF line endings, and false
// when it holds none. Text fields use the same clipboard. Plain text and images
// are supported (see ClipboardImage); HTML is not yet. Headless runtimes keep a
// clipboard of their own.
func (c *Context) Clipboard() (string, bool) { return c.rt.clipboard() }

// SetClipboard replaces the clipboard's contents with plain text (see
// Clipboard).
func (c *Context) SetClipboard(s string) { c.rt.setClipboard(s) }

// ClipboardImage returns the image on the clipboard, such as a screenshot or
// an image copied from a browser, and false when it holds none. Show it with
// ImageFromImage, or save it with image/png. Copied images keep their
// transparency when the program that copied them kept it.
func (c *Context) ClipboardImage() (image.Image, bool) { return c.rt.clipboardImage() }

// SetClipboardImage replaces the clipboard's contents with img, which other
// programs can paste, with its transparency where they support it.
func (c *Context) SetClipboardImage(img image.Image) { c.rt.setClipboardImage(img) }

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

// hovered reports whether the pointer is over the element produced by this
// render. The component re-renders when that changes. Widgets use it for
// render slots, whose content is built by the slot's function (see PartState).
func (c *Context) hovered() bool {
	if c.inst != nil {
		c.inst.watchesPointer = true
	}
	return c.rt.isHovered(expandedID(c.id))
}

// pressed reports whether the element produced by this render is being
// pressed. The component re-renders when that changes; see hovered.
func (c *Context) pressed() bool {
	if c.inst != nil {
		c.inst.watchesPointer = true
	}
	return c.rt.isPressed(expandedID(c.id))
}

// AppTheme returns the theme the app set with SetTheme, which under high
// contrast is not the active theme (see Theme).
func (c *Context) AppTheme() *Theme { return c.rt.appTheme }

// checkOpenURL accepts the URLs OpenURL opens: http, https and mailto, with
// something after the scheme.
func checkOpenURL(raw string) error {
	u, err := neturl.Parse(raw)
	if err != nil {
		return fmt.Errorf("ui.OpenURL: %q is not a URL: %w", raw, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return fmt.Errorf("ui.OpenURL: %q has no host", raw)
		}
	case "mailto":
		if u.Opaque == "" && u.Path == "" {
			return fmt.Errorf("ui.OpenURL: %q has no address", raw)
		}
	default:
		return fmt.Errorf("ui.OpenURL: %q is not an http, https or mailto URL", raw)
	}
	return nil
}
