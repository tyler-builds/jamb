package ui

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/layout"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
)

// The visual inspector reads and changes a window through the methods of
// runtimeHost below, which implement inspect.Target. Nothing here is public
// API: the app package reaches them through internal/uihost, and only while an
// inspector is attached. An inspector attached to a window makes it draw the
// outline of the selected element, and in pick mode the element under the
// pointer, on top of everything; both are painted, so layout and hit testing
// are the same with or without them. The element under the pointer in the
// inspector's tree is outlined too, more lightly (see Hover).

// inspectState is the inspector's state in one window, made by Attach.
type inspectState struct {
	edits    map[uint64]Style // the live edits, by element ID; each lasts until its element is built again
	picking  bool             // pick mode: the pointer outlines elements, and a press selects one
	hover    uint64           // the element under the pointer in pick mode, or in the inspector's tree; 0 for none
	selected uint64           // the element the inspector selected, 0 for none
	changed  func()           // called on the UI thread when the selection or the picking changes
}

// inspect.Target is implemented by the window's host side, see above.
var _ inspect.Target = (*runtimeHost)(nil)

// Attach turns the inspector's state on in the window. changed is called on the
// UI thread when the selection or the picking changes.
func (h *runtimeHost) Attach(changed func()) {
	rt := h.rt()
	if rt.insp == nil {
		rt.insp = &inspectState{edits: map[uint64]Style{}}
		// The elements built before the inspector was attached have no records of
		// their layers (see StyleLayer), so every component is rendered again.
		rt.hostInvalidateAll()
	}
	rt.insp.changed = changed
}

// Detach turns the inspector's state off: the outline goes, pick mode ends, and
// the live edits are dropped, so every element returns to its own style.
func (h *runtimeHost) Detach() {
	rt := h.rt()
	if rt.insp == nil {
		return
	}
	edits := rt.insp.edits
	rt.insp = nil
	for id := range edits {
		if n := rt.byID[id]; n != nil && n.div != nil {
			rt.restyleNode(n)
		}
	}
	rt.needPaint = true
	rt.requestFrame()
}

// picking reports whether the inspector is in pick mode.
func (rt *Runtime) picking() bool { return rt.insp != nil && rt.insp.picking }

// liveEdit returns the inspector's edit of element id, if it has one.
func (rt *Runtime) liveEdit(id uint64) (Style, bool) {
	if rt.insp == nil {
		return Style{}, false
	}
	s, ok := rt.insp.edits[id]
	return s, ok
}

func (rt *Runtime) hasLiveEdit(id uint64) bool {
	_, ok := rt.liveEdit(id)
	return ok
}

// Root returns the ID of the window's root element, or 0 when there is none.
func (h *runtimeHost) Root() uint64 {
	if rt := h.rt(); rt.tree != nil {
		return rt.tree.id
	}
	return 0
}

// Element returns the element with the given ID, as the inspector shows it.
func (h *runtimeHost) Element(id uint64) (inspect.Element, bool) {
	rt := h.rt()
	n := rt.byID[id]
	if n == nil {
		return inspect.Element{}, false
	}
	e := inspect.Element{ID: id, Kind: nodeKindName(n.kind), Key: n.key}
	if n.parent != nil {
		e.Parent = n.parent.id
	}
	for _, c := range n.children {
		e.Children = append(e.Children, c.id)
	}
	if n.div != nil {
		e.Part = n.div.part
		e.Hidden = n.div.noDisplay
	}
	if n.boundary != nil {
		e.Boundary = true
		e.Component = componentName(n.boundary)
	}
	if n.textEl != nil {
		e.Text = shorten(n.textEl.text, 40)
	}
	r := rt.nodeRect(n)
	e.Box = inspect.Box{X: r.X, Y: r.Y, W: r.W, H: r.H}
	s := &n.style
	e.Padding, e.Margin, e.Border = inspect.Edges(s.Padding), inspect.Edges(s.Margin), inspect.Edges(s.borderEdges())
	e.ContentW, e.ContentH = n.lay.ContentW, n.lay.ContentH
	e.OverflowX, e.OverflowY = overflowName(s.OverflowX), overflowName(s.OverflowY)
	return e, true
}

// Fields returns the set fields of the element's resolved style, in declaration
// order. Each field has the layers that set it, lowest first (see Layers in
// inspect.Field); the last one is the winning value. A layer that a higher layer
// replaces is marked overridden, and a layer that sets the value of the layer
// below it is marked same.
func (h *runtimeHost) Fields(id uint64) []inspect.Field {
	rt := h.rt()
	n := rt.byID[id]
	if n == nil {
		return nil
	}
	var out []inspect.Field
	v := reflect.ValueOf(n.style)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		chain := n.style.prov[f.Name]
		if !f.IsExported() || v.Field(i).IsZero() && len(chain) == 0 {
			continue
		}
		cur := formatStyleValue(v.Field(i))
		layers := make([]inspect.Layer, 0, len(chain)+1)
		for _, e := range chain {
			layers = append(layers, inspect.Layer{Source: e.layer, Value: e.value})
		}
		if len(chain) == 0 || chain[len(chain)-1].value != cur {
			// The value was set after its layers, by a method of the element. A
			// sizing method is named for it when the records did not see it.
			src := inspect.SourceElement
			if bit, ok := sizingFields[f.Name]; ok && n.div != nil && n.div.sized&bit != 0 {
				src = inspect.SourceSizing
			}
			layers = append(layers, inspect.Layer{Source: src, Value: cur})
		}
		for j := range layers {
			layers[j].Overridden = j < len(layers)-1
			layers[j].Same = j > 0 && layers[j].Value == layers[j-1].Value
		}
		out = append(out, inspect.Field{Name: f.Name, Value: cur, Source: layers[len(layers)-1].Source, Layers: layers})
	}
	return out
}

// sizingFields are the fields that the sizing methods of a widget set, and the
// sizeSet bit of each.
var sizingFields = map[string]sizeSet{
	"Width": hasWidth, "Height": hasHeight, "MinWidth": hasMinWidth, "MinHeight": hasMinHeight,
	"MaxWidth": hasMaxWidth, "MaxHeight": hasMaxHeight, "Grow": hasGrow, "ColSpan": hasSpan, "RowSpan": hasSpan,
}

// Access returns the accessibility node of the element, with the warnings that
// JAMB_A11Y_WARN reports for it.
func (h *runtimeHost) Access(id uint64) inspect.Access {
	rt := h.rt()
	t, _, issues := rt.buildAccessIssues()
	an := t.Node(accID(id))
	if an == nil {
		return inspect.Access{}
	}
	a := inspect.Access{Exposed: true, Role: an.Role.String(), Name: an.Name, Description: an.Description, Value: an.Value}
	if s := an.States.String(); s != "" {
		a.States = []string{s}
	}
	for _, act := range an.Actions {
		a.Actions = append(a.Actions, act.String())
	}
	if w := accessWarning(an, t, issues[an.ID]); w != "" {
		a.Warnings = append(a.Warnings, w)
	}
	return a
}

// AccessNames returns the role and name of each element with an access node,
// keyed by element ID. The access node of an element is found by accID, as
// Access finds it.
func (h *runtimeHost) AccessNames() map[uint64]string {
	rt := h.rt()
	t, _ := rt.buildAccess()
	names := map[uint64]string{}
	for id := range rt.byID {
		if an := t.Node(accID(id)); an != nil {
			names[id] = an.Role.String() + " " + strconv.Quote(an.Name)
		}
	}
	return names
}

// EditableFields lists the style fields that Edit accepts, in declaration order.
func (h *runtimeHost) EditableFields() []string { return editableFields }

// editableFields are the fields of Style whose values Edit can parse.
var editableFields = func() []string {
	var names []string
	t := reflect.TypeOf(Style{})
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.IsExported() && editType(f.Type) {
			names = append(names, f.Name)
		}
	}
	return names
}()

// editType reports whether Edit can set a field of type t.
func editType(t reflect.Type) bool {
	switch t {
	case reflect.TypeOf(float32(0)), reflect.TypeOf(Color{}), reflect.TypeOf(Length{}),
		reflect.TypeOf(Edges{}), reflect.TypeOf(Corners{}), reflect.TypeOf(false),
		reflect.TypeOf(int(0)), reflect.TypeOf(""):
		return true
	}
	return false
}

// clearFor names the Clear bit that a zero value of a field needs, since a zero
// value leaves the field unset: setting it to zero clears what the style had.
// Fields that are not here cannot take a zero value in an edit.
var clearFor = map[string]ClearSet{
	"Background": ClearBackground, "TextColor": ClearTextColor, "BorderWidth": ClearBorder,
	"Radius": ClearRadius, "Corners": ClearRadius, "Padding": ClearPadding,
	"Margin": ClearMargin, "Gap": ClearGap,
}

// Edit sets one style field of the element to value, live. It applies at once,
// and lasts until the element is built again. A zero value of a field that a
// Clear bit covers (such as Padding or Radius) clears the field; any other zero
// value is refused, because it would leave the field as it was.
func (h *runtimeHost) Edit(id uint64, field, value string) error {
	rt := h.rt()
	n := rt.byID[id]
	if n == nil || n.div == nil {
		return fmt.Errorf("ui: element %d is not a div; live edits change the style of divs", id)
	}
	if rt.insp == nil {
		return fmt.Errorf("ui: the inspector is not attached to this window")
	}
	var edit Style
	if err := setEditField(&edit, field, value); err != nil {
		return err
	}
	rt.insp.edits[id] = rt.insp.edits[id].Merge(tagStyle(edit, inspect.SourceEdit))
	rt.restyleNode(n)
	return nil
}

// setEditField sets the field named name of the style s to the value text.
func setEditField(s *Style, name, value string) error {
	f, ok := reflect.TypeOf(Style{}).FieldByName(name)
	if !ok || !f.IsExported() || !editType(f.Type) {
		return fmt.Errorf("ui: %q is not an editable style field", name)
	}
	fv := reflect.ValueOf(s).Elem().FieldByName(name)
	value = strings.TrimSpace(value)
	zero := false
	switch f.Type {
	case reflect.TypeOf(float32(0)):
		v, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return fmt.Errorf("ui: %s needs a number: %w", name, err)
		}
		fv.SetFloat(v)
		zero = v == 0
	case reflect.TypeOf(int(0)):
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("ui: %s needs a whole number: %w", name, err)
		}
		fv.SetInt(int64(v))
		zero = v == 0
	case reflect.TypeOf(false):
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("ui: %s needs true or false: %w", name, err)
		}
		fv.SetBool(v)
		zero = !v
	case reflect.TypeOf(""):
		fv.SetString(value)
		zero = value == ""
	case reflect.TypeOf(Color{}):
		c, err := ParseHex(value)
		if err != nil {
			return fmt.Errorf("ui: %s needs a hex color such as #18181b: %w", name, err)
		}
		fv.Set(reflect.ValueOf(c))
		zero = c.IsZero()
	case reflect.TypeOf(Length{}):
		l, err := parseLength(value)
		if err != nil {
			return fmt.Errorf("ui: %s: %w", name, err)
		}
		fv.Set(reflect.ValueOf(l))
		zero = l.IsAuto()
	case reflect.TypeOf(Edges{}):
		e, err := parseSides(value)
		if err != nil {
			return fmt.Errorf("ui: %s: %w", name, err)
		}
		fv.Set(reflect.ValueOf(Edges{e[0], e[1], e[2], e[3]}))
		zero = e == [4]float32{}
	case reflect.TypeOf(Corners{}):
		c, err := parseSides(value)
		if err != nil {
			return fmt.Errorf("ui: %s: %w", name, err)
		}
		fv.Set(reflect.ValueOf(Corners{c[0], c[1], c[2], c[3]}))
		zero = c == [4]float32{}
	}
	if !zero {
		return nil
	}
	bit, ok := clearFor[name]
	if !ok {
		return fmt.Errorf("ui: %s cannot be set to zero live; use a value above zero", name)
	}
	fv.Set(reflect.Zero(f.Type))
	s.Clear |= bit
	return nil
}

// parseLength reads a length: a number of pixels, "240px", "50%", "2fr" or "auto".
func parseLength(s string) (Length, error) {
	switch {
	case s == "auto":
		return Auto, nil
	case strings.HasSuffix(s, "%"):
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 32)
		return Percent(float32(v)), err
	case strings.HasSuffix(s, "fr"):
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "fr"), 32)
		return Fr(float32(v)), err
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 32)
	return Px(float32(v)), err
}

// parseSides reads one number, which applies to all four sides, or four numbers
// in the order top, right, bottom, left.
func parseSides(s string) ([4]float32, error) {
	var out [4]float32
	fields := strings.Fields(strings.ReplaceAll(s, ",", " "))
	if len(fields) != 1 && len(fields) != 4 {
		return out, fmt.Errorf("needs one number, or four numbers for top, right, bottom and left")
	}
	for i := range out {
		f := fields[min(i, len(fields)-1)]
		v, err := strconv.ParseFloat(f, 32)
		if err != nil {
			return out, err
		}
		out[i] = float32(v)
	}
	return out, nil
}

// ClearEdit drops the live edits of the element now.
func (h *runtimeHost) ClearEdit(id uint64) {
	rt := h.rt()
	if rt.insp == nil {
		return
	}
	if _, ok := rt.insp.edits[id]; !ok {
		return
	}
	delete(rt.insp.edits, id)
	if n := rt.byID[id]; n != nil && n.div != nil {
		rt.restyleNode(n)
	}
}

// Edited reports whether the element has live edits.
func (h *runtimeHost) Edited(id uint64) bool { return h.rt().hasLiveEdit(id) }

// GoLiteral returns the live edits of the element as a ui.Style literal, for
// the application's code. A field that the style cannot express in a literal
// (a gradient, for example) is not in the literal and is not in the edit either.
func (h *runtimeHost) GoLiteral(id uint64) string {
	edit, ok := h.rt().liveEdit(id)
	if !ok {
		return "ui.Style{}"
	}
	var b strings.Builder
	b.WriteString("ui.Style{\n")
	v := reflect.ValueOf(edit)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || v.Field(i).IsZero() {
			continue
		}
		if lit, ok := literalValue(v.Field(i)); ok {
			fmt.Fprintf(&b, "\t%s: %s,\n", f.Name, lit)
		}
	}
	b.WriteString("}")
	return b.String()
}

// literalValue returns v as Go source, for the field types an edit can hold.
func literalValue(v reflect.Value) (string, bool) {
	switch x := v.Interface().(type) {
	case float32:
		return num(x), true
	case int:
		return strconv.Itoa(x), true
	case bool:
		return strconv.FormatBool(x), true
	case string:
		return strconv.Quote(x), true
	case Color:
		if hex, ok := exactHex(x); ok {
			return fmt.Sprintf("ui.Hex(%q)", hex), true
		}
		return fmt.Sprintf("ui.Color{%s, %s, %s, %s}", num(x.R), num(x.G), num(x.B), num(x.A)), true
	case Length:
		switch x.Unit {
		case layout.UnitAuto:
			return "ui.Auto", true
		case layout.UnitPx:
			return fmt.Sprintf("ui.Px(%s)", num(x.Value)), true
		case layout.UnitPercent:
			return fmt.Sprintf("ui.Percent(%s)", num(x.Value)), true
		}
		return fmt.Sprintf("ui.Fr(%s)", num(x.Value)), true
	case Edges:
		if x.Top == x.Right && x.Right == x.Bottom && x.Bottom == x.Left {
			return fmt.Sprintf("ui.All(%s)", num(x.Top)), true
		}
		return fmt.Sprintf("ui.Edges{Top: %s, Right: %s, Bottom: %s, Left: %s}", num(x.Top), num(x.Right), num(x.Bottom), num(x.Left)), true
	case Corners:
		return fmt.Sprintf("ui.Corners{TopLeft: %s, TopRight: %s, BottomRight: %s, BottomLeft: %s}",
			num(x.TopLeft), num(x.TopRight), num(x.BottomRight), num(x.BottomLeft)), true
	case ClearSet:
		return clearText(x, "ui.", " | "), true
	}
	return "", false
}

// exactHex returns c as "#rrggbbaa" when its components are whole 8-bit values,
// so the literal ui.Hex(...) reproduces it exactly.
func exactHex(c Color) (string, bool) {
	ch := [4]uint8{}
	for i, v := range [4]float32{c.R, c.G, c.B, c.A} {
		x := math.Round(float64(v) * 255)
		if x < 0 || x > 255 || math.Abs(x/255-float64(v)) > 1e-6 {
			return "", false
		}
		ch[i] = uint8(x)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", ch[0], ch[1], ch[2], ch[3]), true
}

// formatStyleValue returns a field's value as the inspector shows it.
func formatStyleValue(v reflect.Value) string {
	switch x := v.Interface().(type) {
	case float32:
		return num(x)
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	case string:
		return strconv.Quote(x)
	case Color:
		return fmt.Sprintf("#%02x%02x%02x%02x", uint8(math.Round(float64(x.R)*255)), uint8(math.Round(float64(x.G)*255)),
			uint8(math.Round(float64(x.B)*255)), uint8(math.Round(float64(x.A)*255)))
	case Length:
		return lengthText(x)
	case Edges:
		return sidesText(x.Top, x.Right, x.Bottom, x.Left)
	case Corners:
		return sidesText(x.TopLeft, x.TopRight, x.BottomRight, x.BottomLeft)
	case Alignment:
		return alignmentNames[x]
	case Direction:
		if x == Row {
			return "row"
		}
		return "column"
	case Overflow:
		return overflowName(x)
	case ClearSet:
		return clearText(x, "", " | ")
	case time.Duration:
		return x.String()
	case FontWeight:
		return strconv.Itoa(int(x))
	case Gradient:
		return "gradient"
	case []Length:
		parts := make([]string, len(x))
		for i, l := range x {
			parts[i] = lengthText(l)
		}
		return "[" + strings.Join(parts, " ") + "]"
	case []Shadow:
		parts := make([]string, 0, len(x))
		for _, s := range x {
			parts = append(parts, fmt.Sprintf("%s %s %s %s %s", num(s.X), num(s.Y), num(s.Blur), num(s.Spread), formatStyleValue(reflect.ValueOf(s.Color))))
		}
		return strings.Join(parts, "; ")
	}
	return fmt.Sprint(v.Interface())
}

var alignmentNames = [...]string{0: "unset", Start: "start", Center: "center", End: "end", Stretch: "stretch",
	SpaceBetween: "space between", SpaceAround: "space around", SpaceEvenly: "space evenly"}

func lengthText(l Length) string {
	switch l.Unit {
	case layout.UnitAuto:
		return "auto"
	case layout.UnitPx:
		return num(l.Value) + "px"
	case layout.UnitPercent:
		return num(l.Value) + "%"
	}
	return num(l.Value) + "fr"
}

func sidesText(a, b, c, d float32) string {
	if a == b && b == c && c == d {
		return num(a)
	}
	return num(a) + " " + num(b) + " " + num(c) + " " + num(d)
}

func overflowName(o Overflow) string {
	switch o {
	case OverflowHidden:
		return "hidden"
	case OverflowScroll:
		return "scroll"
	}
	return "visible"
}

func num(v float32) string { return strconv.FormatFloat(float64(v), 'f', -1, 32) }

// clearNames are the names of the Clear bits, in bit order.
var clearNames = [...]string{"Background", "TextColor", "Border", "Radius", "Shadows", "Padding", "Margin", "Gap"}

// clearText lists the Clear bits set in c, each as a name with the prefix.
func clearText(c ClearSet, prefix, sep string) string {
	var parts []string
	for i, n := range clearNames {
		if c&(1<<i) != 0 {
			parts = append(parts, prefix+"Clear"+n)
		}
	}
	return strings.Join(parts, sep)
}

func nodeKindName(k nodeKind) string {
	switch k {
	case kindText:
		return "text"
	case kindIcon:
		return "icon"
	case kindImage:
		return "image"
	case kindEditor:
		return "editor"
	}
	return "div"
}

// componentName returns the name of the render function of a component boundary.
func componentName(inst *componentInstance) string {
	switch e := inst.el.(type) {
	case *ComponentElement:
		if e != nil && e.fn != nil {
			return runtime.FuncForPC(reflect.ValueOf(e.fn).Pointer()).Name()
		}
	case nil:
		return ""
	}
	return reflect.TypeOf(inst.el).String()
}

// shorten returns s cut to n characters, with an ellipsis when it was longer. It
// counts runes, so it never cuts a character in two.
func shorten(s string, n int) string {
	i := 0
	for j := range s {
		if i == n {
			return s[:j] + "…"
		}
		i++
	}
	return s
}

// Select makes the element with the given ID the selected one, which the window
// outlines. Zero clears the selection.
func (h *runtimeHost) Select(id uint64) {
	rt := h.rt()
	if rt.insp == nil {
		return
	}
	rt.insp.selected = id
	rt.setInspectPaint() // the window draws the new outline without waiting for its own input
	rt.changedInspect()
}

// Selected returns the selected element's ID, or 0.
func (h *runtimeHost) Selected() uint64 {
	if rt := h.rt(); rt.insp != nil {
		return rt.insp.selected
	}
	return 0
}

// Hover outlines the element with the given ID lightly, as the inspector's tree
// does for the row under the pointer. Zero clears it. In pick mode the pointer
// in the window moves it.
func (h *runtimeHost) Hover(id uint64) {
	rt := h.rt()
	if rt.insp == nil || rt.insp.hover == id {
		return
	}
	rt.insp.hover = id
	rt.setInspectPaint()
}

// Pick turns pick mode on or off.
func (h *runtimeHost) Pick(on bool) {
	rt := h.rt()
	if rt.insp == nil {
		return
	}
	if !on {
		rt.endPick()
		return
	}
	rt.insp.picking, rt.insp.hover = true, 0
	rt.setInspectPaint()
}

// Picking reports whether pick mode is on.
func (h *runtimeHost) Picking() bool { return h.rt().picking() }

// endPick ends pick mode without selecting anything.
func (rt *Runtime) endPick() {
	if rt.insp == nil {
		return
	}
	rt.insp.picking, rt.insp.hover = false, 0
	rt.setInspectPaint()
}

// pickMove outlines the element under the pointer, deepest first.
func (rt *Runtime) pickMove(x, y float32) {
	var hit uint64
	if path := rt.hitTest(x, y); len(path) > 0 {
		hit = path[0].id
	}
	if hit != rt.insp.hover {
		rt.insp.hover = hit
		rt.setInspectPaint()
	}
}

// pickPress selects the element under the pointer and ends pick mode. The press
// goes to the inspector, not to the element.
func (rt *Runtime) pickPress(x, y float32) {
	rt.pickMove(x, y)
	rt.insp.selected = rt.insp.hover
	rt.endPick()
	rt.changedInspect()
}

// changedInspect calls the inspector's changed function, on the UI thread.
func (rt *Runtime) changedInspect() {
	if rt.insp != nil && rt.insp.changed != nil {
		rt.insp.changed()
	}
}

// setInspectPaint asks for a frame that paints the outline again.
func (rt *Runtime) setInspectPaint() {
	rt.needPaint = true
	rt.requestFrame()
}

// The tints of the box model on the selected element, as the browser's
// developer tools draw them: translucent, so the element shows through on a
// light or a dark theme.
var (
	inspectMarginTint  = Hex("#f6b26b80") // the margin, outside the border box
	inspectBorderTint  = Hex("#ffe59980") // the border
	inspectPaddingTint = Hex("#93c47d70") // the padding, inside the border
	inspectContentTint = Hex("#6fa8dc60") // the content box
)

// paintInspect paints the inspector's marks: the selected element with its box
// model, and the element under the pointer (in pick mode, or in the inspector's
// tree) with a lighter outline. Each has a label that gives its kind, key and
// size. It is painted last, over every other layer.
func (rt *Runtime) paintInspect(view scene.Rect) {
	if rt.insp == nil {
		return
	}
	if rt.insp.selected != 0 {
		rt.paintInspected(rt.insp.selected, true, view)
	}
	if h := rt.insp.hover; h != 0 && h != rt.insp.selected {
		rt.paintInspected(h, false, view)
	}
}

// paintInspected marks the element id. The selected element gets its margin,
// border and padding as tinted bands, its content box filled, and an outline in
// the accent color 2 pixels wide; any other gets a light accent fill and a
// 1-pixel outline. An element with no width or no height has no inside to
// outline, so it gets a line 2 pixels thick along its edge (a dot when it has
// neither). The label above it stays inside view.
func (rt *Runtime) paintInspected(id uint64, selected bool, view scene.Rect) {
	n := rt.byID[id]
	if n == nil {
		return
	}
	accent := rt.theme.Accent
	r := rt.nodeRect(n)
	switch {
	case r.W < 1 || r.H < 1:
		rt.scene.AddQuad(scene.Quad{Bounds: rt.px(zeroMarker(r)), Background: accent.scene()})
	case selected:
		s := &n.style
		m, b, p := nonNegative(s.Margin), nonNegative(s.borderEdges()), nonNegative(s.Padding)
		rt.paintBand(outset(r, m), m, inspectMarginTint)
		rt.paintBand(r, b, inspectBorderTint)
		inner := outset(r, Edges{-b.Top, -b.Right, -b.Bottom, -b.Left})
		rt.paintBand(inner, p, inspectPaddingTint)
		content := outset(inner, Edges{-p.Top, -p.Right, -p.Bottom, -p.Left})
		if content.W > 0 && content.H > 0 {
			rt.scene.AddQuad(scene.Quad{Bounds: rt.px(content), Background: inspectContentTint.scene()})
		}
		rt.paintInspectFrame(r, 2, accent)
	default:
		rt.scene.AddQuad(scene.Quad{Bounds: rt.px(r), Background: accent.Alpha(0.12).scene()})
		rt.paintInspectFrame(r, 1, accent)
	}
	rt.paintInspectLabel(n, r, view)
}

// paintInspectFrame draws a border of width logical pixels just inside r, in c.
func (rt *Runtime) paintInspectFrame(r scene.Rect, width float32, c Color) {
	w := max(1, round(width*rt.scale))
	rt.scene.AddQuad(scene.Quad{
		Bounds:      rt.px(r),
		BorderSides: scene.Sides{Top: w, Right: w, Bottom: w, Left: w},
		BorderColor: c.scene(),
	})
}

// paintBand fills the band of the widths e just inside r, in c. A band with no
// width is not drawn.
func (rt *Runtime) paintBand(r scene.Rect, e Edges, c Color) {
	if e == (Edges{}) || r.W <= 0 || r.H <= 0 {
		return
	}
	s := rt.scale
	rt.scene.AddQuad(scene.Quad{
		Bounds:      rt.px(r),
		BorderSides: scene.Sides{Top: round(e.Top * s), Right: round(e.Right * s), Bottom: round(e.Bottom * s), Left: round(e.Left * s)},
		BorderColor: c.scene(),
	})
}

// paintInspectLabel draws the label of the element n, whose box is r: its kind,
// key and size on the accent color, above the box, or below it when there is no
// room above, and inside view.
func (rt *Runtime) paintInspectLabel(n *node, r, view scene.Rect) {
	label := nodeKindName(n.kind)
	if n.key != "" {
		label += " key=" + n.key
	}
	label += fmt.Sprintf("  %s × %s", num(float32(math.Round(float64(r.W)))), num(float32(math.Round(float64(r.H)))))
	l := rt.text.Layout(label, text.Style{Size: 11, NoWrap: true}, 1e6)
	box := scene.Rect{X: r.X, W: l.Width + 10, H: l.LineHeight + 4}
	box.Y = r.Y - box.H - 2
	if box.Y < 0 {
		box.Y = r.Y + max(r.H, 0) + 2
	}
	box.X = max(0, min(box.X, view.W-box.W))
	box.Y = max(0, min(box.Y, view.H-box.H))
	rt.scene.AddQuad(scene.Quad{Bounds: rt.px(box), Background: rt.theme.Accent.scene(), Radii: scene.Uniform(3 * rt.scale)})
	rt.text.Draw(&rt.scene, l, box.X+5, box.Y+2, l.Width, rt.scale, 0, rt.theme.AccentText.scene())
}

// outset returns r with e added on each side; a negative side shrinks it.
func outset(r scene.Rect, e Edges) scene.Rect {
	return scene.Rect{X: r.X - e.Left, Y: r.Y - e.Top, W: max(0, r.W+e.Left+e.Right), H: max(0, r.H+e.Top+e.Bottom)}
}

// nonNegative returns e with its negative sides set to zero.
func nonNegative(e Edges) Edges {
	return Edges{max(0, e.Top), max(0, e.Right), max(0, e.Bottom), max(0, e.Left)}
}

// zeroMarker returns the rectangle that marks an element with no width or no
// height: its edge widened to 2 pixels, centred on the element's line.
func zeroMarker(r scene.Rect) scene.Rect {
	if r.W < 1 {
		r.X, r.W = r.X+r.W/2-1, 2
	}
	if r.H < 1 {
		r.Y, r.H = r.Y+r.H/2-1, 2
	}
	return r
}
