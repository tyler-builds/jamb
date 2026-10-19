package ui

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/tyler-builds/jamb/input"
)

// Actions are named commands. Key bindings, menus and the command palette all
// run them; an element says what an action does with OnAction:
//
//	var save = ui.NewAction("notes.save", "Save note")
//
//	ui.Div().OnAction(save, func() { ... })
//	rt.AddBindings(ui.Bind("ctrl+s", save))
//
// Create actions once, as package variables. Nothing is registered globally,
// so creating an action twice is harmless.

// Action is a named command that key bindings, menus and the command palette
// can run.
type Action struct {
	name, label string
	hidden      bool
	unbinds     *Action // for Unbind: the action whose bindings this one removes
}

// NoAction disables keys: BindIn("Editor", "ctrl+a", ui.NoAction) makes Ctrl+A
// do nothing in fields. Bindings ranked below it for the same keys are skipped,
// and the key goes on to OnKeyDown and text input.
var NoAction = NewAction("no-action", "No action").Hide()

// Unbind returns an action that removes a's bindings for the same keys ranked
// below it: BindIn("Editor", "ctrl+z", ui.Unbind(actions.Undo)). Other bindings for
// those keys still apply.
func Unbind(a *Action) *Action {
	if a == nil {
		panic("ui.Unbind(nil): give the action to unbind")
	}
	return &Action{name: "unbind:" + a.Name(), label: "Unbind " + a.Label(), hidden: true, unbinds: a}
}

// disables reports whether a turns keys off (NoAction, or an Unbind) rather
// than running something.
func (a *Action) disables() bool { return a == NoAction || a.unbinds != nil }

// NewAction creates an action. name identifies it (for example "notes.new"),
// label is the text a menu or command palette shows (for example "New note").
func NewAction(name, label string) *Action { return &Action{name: name, label: label} }

// Name returns the action's name.
func (a *Action) Name() string { return a.name }

// Label returns the action's display text.
func (a *Action) Label() string { return a.label }

// Hide marks a as hidden, for menus and command palettes to leave out, and
// returns a. A hidden action still runs, and AvailableActions still lists
// it; callers filter with Hidden.
func (a *Action) Hide() *Action {
	a.hidden = true
	return a
}

// Hidden reports whether Hide was called on a.
func (a *Action) Hidden() bool { return a.hidden }

// actionHandler is an element's OnAction registration.
type actionHandler struct {
	action *Action
	fn     func()
}

// Binding maps keys to an action. Create one with Bind or BindIn and pass it
// to Runtime.AddBindings or app.Window.Bindings.
type Binding struct {
	keys    string
	expr    string // the BindIn expression as written; "" for global bindings
	strokes []keystroke
	action  *Action
	pred    *contextPredicate // nil for global bindings
	layer   string            // "" for bindings from AddBindings; see SetBindings
}

// keystroke is one key press with the modifiers held.
type keystroke struct {
	key  input.Key
	mods input.Modifiers
}

// typesText reports whether the press can type text: a printable key, without
// Ctrl, Alt or Super. The platform sends text only for such keys, so a
// single-stroke binding that consumes any other key drops no text.
func (s keystroke) typesText() bool {
	if s.mods.Has(input.ModCtrl) || s.mods.Has(input.ModAlt) || s.mods.Has(input.ModSuper) {
		return false
	}
	switch {
	case s.key >= input.KeyA && s.key <= input.KeyZ, s.key >= input.Key0 && s.key <= input.Key9:
		return true
	}
	switch s.key {
	case input.KeySpace, input.KeyEqual, input.KeyMinus, input.KeySlash, input.KeyBackslash,
		input.KeyLeftBracket, input.KeyRightBracket, input.KeySemicolon, input.KeyApostrophe,
		input.KeyComma, input.KeyPeriod, input.KeyBacktick:
		return true
	}
	return false
}

// Bind maps keys to a: the binding applies everywhere, after any contextual
// binding (BindIn) that matches, as the dispatch order in docs/input.md says.
// keys is strokes separated by spaces, each stroke modifiers and one key
// joined by "+", for example "ctrl+shift+p", "alt+up", "ctrl+k ctrl+t" or
// "f2".
// Modifiers are ctrl, shift, alt and super; keys are a-z, 0-9, f1-f12,
// enter, escape (or esc), backspace, delete, tab, space, left, right, up,
// down, home, end, pageup, pagedown, insert, and the punctuation keys = - / \ [ ] ; ' , . `.
// Bind panics on a malformed keys string.
//
// The punctuation keys name the physical key as the US layout prints it, so "ctrl+shift+["
// is the key that is [ on the US layout. With Shift held that key types {, and the binding
// still matches, since it is the same key position with Shift held.
func Bind(keys string, a *Action) Binding {
	b, err := ParseBinding("", keys, a)
	if err != nil {
		panic(fmt.Sprintf("ui.Bind(%q): %s", keys, err))
	}
	return b
}

// BindIn is Bind limited by a context expression, matched against the
// KeyContexts on the focus path. Expressions are GPUI's: a context name
// ("Editor"), key == value and key != value, ! (not), && and ||, > (a
// descendant: "Palette > Editor"), and parentheses. A binding whose
// expression matches at a deeper context runs before one that matches at a
// shallower context; global bindings run last. At the same depth, the
// binding added later runs first. So a binding for a field's own keys in a
// palette is BindIn("Palette > Editor", "up", ...), which beats the field's
// binding for Up. BindIn panics on a malformed expression.
func BindIn(expr, keys string, a *Action) Binding {
	if _, err := parseContextPredicate(expr); err != nil {
		panic(fmt.Sprintf("ui.BindIn(%q): %s", expr, err))
	}
	b, err := ParseBinding(expr, keys, a)
	if err != nil {
		panic(fmt.Sprintf("ui.BindIn(%q): %s", keys, err))
	}
	return b
}

// ParseBinding is Bind and BindIn returning an error instead of panicking,
// for keys and expressions that come from outside the program, such as a
// keymap file. expr is a BindIn expression, or "" for a global binding. The
// error says what is wrong, without the call that produced it.
func ParseBinding(expr, keys string, a *Action) (Binding, error) {
	var pred *contextPredicate
	if expr != "" {
		p, err := parseContextPredicate(expr)
		if err != nil {
			return Binding{}, err
		}
		pred = p
	}
	if a == nil {
		return Binding{}, errors.New("nil action; give the action to bind")
	}
	strokes, err := parseKeys(keys)
	if err != nil {
		return Binding{}, err
	}
	return Binding{keys: keys, expr: expr, strokes: strokes, action: a, pred: pred}, nil
}

var modNames = map[string]input.Modifiers{
	"ctrl": input.ModCtrl, "shift": input.ModShift, "alt": input.ModAlt, "super": input.ModSuper,
}

var keyNames = map[string]input.Key{
	"enter": input.KeyEnter, "escape": input.KeyEscape, "esc": input.KeyEscape,
	"backspace": input.KeyBackspace, "delete": input.KeyDelete, "tab": input.KeyTab,
	"space": input.KeySpace, "left": input.KeyLeft, "right": input.KeyRight,
	"up": input.KeyUp, "down": input.KeyDown, "home": input.KeyHome, "end": input.KeyEnd,
	"pageup": input.KeyPageUp, "pagedown": input.KeyPageDown, "insert": input.KeyInsert,
	"=": input.KeyEqual, "-": input.KeyMinus,
	"/": input.KeySlash, "\\": input.KeyBackslash, "[": input.KeyLeftBracket, "]": input.KeyRightBracket,
	";": input.KeySemicolon, "'": input.KeyApostrophe, ",": input.KeyComma, ".": input.KeyPeriod,
	"`": input.KeyBacktick,
}

// parseKeys turns a keys string into strokes. The error names the offending
// part.
func parseKeys(keys string) ([]keystroke, error) {
	words := strings.Fields(keys)
	if len(words) == 0 {
		return nil, errors.New("no keys; give a key such as \"ctrl+n\"")
	}
	strokes := make([]keystroke, 0, len(words))
	for _, w := range words {
		parts := strings.Split(strings.ToLower(w), "+")
		name := parts[len(parts)-1]
		var s keystroke
		for _, m := range parts[:len(parts)-1] {
			bit, ok := modNames[m]
			if !ok {
				return nil, fmt.Errorf("unknown modifier %q; use ctrl, shift, alt or super", m)
			}
			s.mods |= bit
		}
		if name == "" {
			return nil, fmt.Errorf("%q has no key; add one after the +", w)
		}
		k, ok := parseKeyName(name)
		if !ok {
			return nil, fmt.Errorf("unknown key %q; use a-z, 0-9, f1-f12, enter, escape, backspace, "+
				"delete, tab, space, left, right, up, down, home, end, pageup, pagedown, insert, "+
				"or one of = - / \\ [ ] ; ' , . `", name)
		}
		s.key = k
		strokes = append(strokes, s)
	}
	return strokes, nil
}

func parseKeyName(name string) (input.Key, bool) {
	if k, ok := keyNames[name]; ok {
		return k, true
	}
	if len(name) == 1 {
		switch c := name[0]; {
		case c >= 'a' && c <= 'z':
			return input.KeyA + input.Key(c-'a'), true
		case c >= '0' && c <= '9':
			return input.Key0 + input.Key(c-'0'), true
		}
	}
	if rest, ok := strings.CutPrefix(name, "f"); ok {
		if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= 12 && rest == strconv.Itoa(n) {
			return input.KeyF1 + input.Key(n-1), true
		}
	}
	return 0, false
}

// keyLabel is the display text of a key, as KeysFor shows it.
func keyLabel(k input.Key) string {
	switch {
	case k >= input.KeyA && k <= input.KeyZ:
		return string(rune('A' + int(k-input.KeyA)))
	case k >= input.Key0 && k <= input.Key9:
		return string(rune('0' + int(k-input.Key0)))
	case k >= input.KeyF1 && k <= input.KeyF12:
		return "F" + strconv.Itoa(int(k-input.KeyF1)+1)
	}
	switch k {
	case input.KeyEnter:
		return "Enter"
	case input.KeyEscape:
		return "Escape"
	case input.KeyBackspace:
		return "Backspace"
	case input.KeyDelete:
		return "Delete"
	case input.KeyTab:
		return "Tab"
	case input.KeySpace:
		return "Space"
	case input.KeyLeft:
		return "Left"
	case input.KeyRight:
		return "Right"
	case input.KeyUp:
		return "Up"
	case input.KeyDown:
		return "Down"
	case input.KeyHome:
		return "Home"
	case input.KeyEnd:
		return "End"
	case input.KeyPageUp:
		return "PageUp"
	case input.KeyPageDown:
		return "PageDown"
	case input.KeyInsert:
		return "Insert"
	case input.KeyEqual:
		return "="
	case input.KeyMinus:
		return "-"
	case input.KeySlash:
		return "/"
	case input.KeyBackslash:
		return "\\"
	case input.KeyLeftBracket:
		return "["
	case input.KeyRightBracket:
		return "]"
	case input.KeySemicolon:
		return ";"
	case input.KeyApostrophe:
		return "'"
	case input.KeyComma:
		return ","
	case input.KeyPeriod:
		return "."
	case input.KeyBacktick:
		return "`"
	}
	return ""
}

// strokeLabel formats a stroke as "Ctrl+Shift+P".
func strokeLabel(s keystroke) string { return strings.Join(strokeKeys(s), "+") }

// strokeKeys returns the names of the keys of a stroke, the modifiers first, such
// as "Ctrl", "Shift" and "P". A Kbd draws each as a key cap.
func strokeKeys(s keystroke) []string {
	var parts []string
	for _, m := range []struct {
		bit  input.Modifiers
		name string
	}{{input.ModCtrl, "Ctrl"}, {input.ModAlt, "Alt"}, {input.ModShift, "Shift"}, {input.ModSuper, "Super"}} {
		if s.mods.Has(m.bit) {
			parts = append(parts, m.name)
		}
	}
	return append(parts, keyLabel(s.key))
}

// bindingsOn lists the bindings that apply on the focus path, in precedence
// order (GPUI's): a contextual binding's depth is the depth at which its
// expression matches the context stack, and a global binding's is 0. Deeper
// bindings come first, then later-added bindings.
func (rt *Runtime) bindingsOn(path []*node) []Binding {
	stack := contextStack(path)
	type ranked struct {
		b            Binding
		depth, order int
	}
	var out []ranked
	for i, b := range rt.keymap {
		depth := 0
		if b.pred != nil {
			d, ok := b.pred.depthOf(stack)
			if !ok {
				continue
			}
			depth = d
		}
		out = append(out, ranked{b, depth, i})
	}
	slices.SortStableFunc(out, func(x, y ranked) int {
		if c := cmp.Compare(y.depth, x.depth); c != 0 {
			return c
		}
		return cmp.Compare(y.order, x.order)
	})
	bs := make([]Binding, len(out))
	for i, r := range out {
		bs[i] = r.b
	}
	return bs
}

// handlerOn returns the deepest OnAction handler for a on the focus path, or
// nil if no element on it handles a.
func handlerOn(path []*node, a *Action) func() {
	for _, n := range path {
		if n.div == nil {
			continue
		}
		for _, h := range n.div.actions {
			if h.action == a {
				return h.fn
			}
		}
	}
	return nil
}

// AddBindings adds key bindings. Later bindings for the same keys take
// precedence over earlier ones at the same level.
func (rt *Runtime) AddBindings(bs ...Binding) {
	rt.keymap = append(rt.keymap, bs...)
}

// SetBindings replaces all bindings of one layer with bs. The new bindings
// take the place of the layer's first binding, so the layer keeps its
// precedence against bindings added before and after it; a layer with no
// bindings yet is added at the end. Empty bs removes the layer. layer must
// not be "": that is the layer AddBindings uses.
func (rt *Runtime) SetBindings(layer string, bs []Binding) {
	if layer == "" {
		panic(`ui.SetBindings(""): name the layer, such as "keymap:keymap.json"; AddBindings takes unnamed bindings`)
	}
	tagged := make([]Binding, len(bs))
	for i, b := range bs {
		b.layer = layer
		tagged[i] = b
	}
	out := make([]Binding, 0, len(rt.keymap)+len(bs))
	placed := false
	for _, b := range rt.keymap {
		if b.layer != layer {
			out = append(out, b)
			continue
		}
		if !placed {
			out = append(out, tagged...)
			placed = true
		}
	}
	if !placed {
		out = append(out, tagged...)
	}
	rt.keymap = out
}

// Dispatch runs a on the focus path (the focused element, then its ancestors,
// or the root when nothing is focused): the deepest element with an OnAction
// handler for a runs it. It reports whether a handler ran.
func (rt *Runtime) Dispatch(a *Action) bool {
	fn := handlerOn(rt.focusPath(), a)
	if fn == nil {
		return false
	}
	fn()
	return true
}

// KeysFor returns the display text of the most recently added binding for a,
// such as "Ctrl+Shift+P" or "Ctrl+K Ctrl+T", or "" if a is not bound. A
// binding that a later NoAction or Unbind turns off does not count.
func (rt *Runtime) KeysFor(a *Action) string {
	for i := len(rt.keymap) - 1; i >= 0; i-- {
		if b := rt.keymap[i]; b.action == a && !a.disables() && !rt.turnedOffLater(i) {
			labels := make([]string, len(b.strokes))
			for j, s := range b.strokes {
				labels[j] = strokeLabel(s)
			}
			return strings.Join(labels, " ")
		}
	}
	return ""
}

// turnedOffLater reports whether a binding added after keymap[i] turns it off:
// a NoAction, or an Unbind of its action, with the same keys in the same
// context expression.
func (rt *Runtime) turnedOffLater(i int) bool {
	b := rt.keymap[i]
	for _, u := range rt.keymap[i+1:] {
		if u.expr == b.expr && slices.Equal(u.strokes, b.strokes) && (u.action == NoAction || u.action.unbinds == b.action) {
			return true
		}
	}
	return false
}

// AvailableActions lists the actions handled on the focus path, deepest
// element first, without duplicates. Menus and the command palette use it.
func (rt *Runtime) AvailableActions() []*Action {
	var out []*Action
	for _, n := range rt.focusPath() {
		if n.div == nil {
			continue
		}
		for _, h := range n.div.actions {
			if !slices.Contains(out, h.action) {
				out = append(out, h.action)
			}
		}
	}
	return out
}

// Dispatch runs a along the focus path; see Runtime.Dispatch.
func (c *Context) Dispatch(a *Action) bool { return c.rt.Dispatch(a) }

// KeysFor returns the display text of a's most recent binding; see
// Runtime.KeysFor.
func (c *Context) KeysFor(a *Action) string { return c.rt.KeysFor(a) }

// AvailableActions lists the actions on the focus path; see
// Runtime.AvailableActions.
func (c *Context) AvailableActions() []*Action { return c.rt.AvailableActions() }
