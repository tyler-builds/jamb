package ui

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// dockEdgeBand is the width of the band along each outer edge of a dock that drops a
// panel onto the whole dock, in logical pixels. dockMinPane is the least width or height
// a pane of a split keeps, so that a split is never dragged shut.
const (
	dockEdgeBand = 24
	dockMinPane  = 48
)

// DockStyle is the look of each part of a dock. A part's style merges over the built-in
// defaults, then over the theme's Dock, then over the Styles of the instance; see
// docs/styling.md.
//
// Group is the box of each tab group, and Strip its tab strip. Tab is each tab, with
// TabHover while the pointer is over a tab that is not the active one, TabSelected on
// the active tab, and TabFocusVisible while it has keyboard focus. Indicator is the line
// under the active tab, Panel the box of each panel, and Preview the rectangle that shows
// where a dragged panel will land. The dividers between the groups are the Theme.Split.
type DockStyle struct {
	Group           Style // the box of each tab group
	Strip           Style // the tab strip of a group
	Tab             Style // each tab
	TabHover        Style // a tab the pointer is over, if it is not the active one
	TabSelected     Style // the active tab of a group
	TabFocusVisible Style // a tab that has keyboard focus
	Indicator       Style // the line under the active tab
	Panel           Style // the box of each panel
	Preview         Style // the rectangle that shows where a dragged panel lands
}

// merge returns s with the set fields of o applied on top, part by part.
func (s DockStyle) merge(o DockStyle) DockStyle {
	return DockStyle{
		Group:           s.Group.Merge(o.Group),
		Strip:           s.Strip.Merge(o.Strip),
		Tab:             s.Tab.Merge(o.Tab),
		TabHover:        s.TabHover.Merge(o.TabHover),
		TabSelected:     s.TabSelected.Merge(o.TabSelected),
		TabFocusVisible: s.TabFocusVisible.Merge(o.TabFocusVisible),
		Indicator:       s.Indicator.Merge(o.Indicator),
		Panel:           s.Panel.Merge(o.Panel),
		Preview:         s.Preview.Merge(o.Preview),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s DockStyle) plain() DockStyle {
	return DockStyle{
		Group:           plainStyle(s.Group),
		Strip:           plainStyle(s.Strip),
		Tab:             plainStyle(s.Tab),
		TabHover:        plainStyle(s.TabHover),
		TabSelected:     plainStyle(s.TabSelected),
		TabFocusVisible: plainStyle(s.TabFocusVisible),
		Indicator:       plainStyle(s.Indicator),
		Panel:           plainStyle(s.Panel),
		Preview:         plainStyle(s.Preview),
	}
}

// dockDefaults returns the built-in look of a dock, from the colors of th. The tabs are
// those of an underlined tab list, and the preview is the accent color, so that it shows
// under high contrast too.
func dockDefaults(th *Theme) DockStyle {
	return DockStyle{
		Strip: Style{Direction: Row, Gap: 4, BorderEdges: Edges{Bottom: 1}, BorderColor: th.Border},
		Tab: Style{Direction: Row, Align: Center, Gap: 6, MinHeight: Px(32), Padding: XY(12, 0),
			NoShrink: true, Radius: th.Radius / 2, TextColor: th.TextMuted},
		TabHover:        Style{Background: th.SurfaceHover},
		TabSelected:     Style{TextColor: th.Text},
		TabFocusVisible: Style{Shadows: []Shadow{{Spread: 2, Color: th.FocusRing}}},
		Indicator:       Style{Height: Px(2), Background: th.Accent},
		Preview:         Style{Background: th.AccentSoft, BorderWidth: 2, BorderColor: th.Accent, Radius: th.Radius},
	}
}

// resolveDock returns the look of a dock: the defaults, merged with the theme's Dock and
// then the instance's Styles. Under high contrast the two overrides are plain (see
// plainStyle).
func resolveDock(th *Theme, inst DockStyle) DockStyle {
	st := dockDefaults(th)
	for _, o := range []DockStyle{th.Dock, inst} {
		if th.HighContrast {
			o = o.plain()
		}
		st = st.merge(o)
	}
	return st
}

// DockPanel is one panel of a Dock: its title, which names its tab and its tab panel for
// assistive technology, its icon, and Render, which builds its content. The content keeps
// its state while the panel stays in its group, and while it is not the active tab. A
// panel that moves to another group, or to a floating window, is built again there, so
// keep the state the panel needs in signals the app owns. Closable adds a Close command to
// the panel's menu.
type DockPanel struct {
	Title    string
	Icon     *icons.Icon
	Render   func() Element
	Closable bool
}

// DockWindow describes the window of a floating group, for the opener that Dock.Floating
// takes. Root renders the group, Title names the window after its active panel, and X, Y,
// Width and Height place it. OnClose records the window's position and size in the layout
// as the window closes. OnClosed docks the group's panels back when the window closes,
// including when the user closes it, so a panel is never lost with its window. The
// window's own Root, Title and handlers are passed on to app.Window by app.OpenDockWindow.
type DockWindow struct {
	Title         string
	X, Y          int
	Width, Height int
	Root          func(ctx *Context) Element
	OnClose       func() bool
	OnClosed      func()
}

// DockElement is a dock: the panels of a DockLayout, in splits and tab groups that the
// user rearranges. See Dock.
type DockElement struct {
	layout   *Signal[DockLayout]
	panels   map[string]DockPanel
	key      string
	styles   DockStyle
	onChange func(DockLayout)
	open     func(DockWindow) Window
	size     sizing // the dock's outer box, from the sizing methods (see Sizing in docs/styling.md)
}

// Dock shows the panels of layout, which the app owns, so that the app can save the
// layout with its settings and load it back. Each panel is named by an ID that the layout
// holds, and is shown from panels; a panel ID in the layout that panels does not have is
// dropped when the dock is shown, and a panel of panels that the layout does not hold is
// hidden until DockShow shows it.
//
// The dock is split and grouped as its layout says. Each group is a tab strip over its
// panels, and each divider between groups is a Split. A user moves a panel by dragging its
// tab over a group, which shows the zone the panel will land in (the center adds it as a
// tab, a side makes a group beside that one), or over an outer edge of the dock. The tab's
// menu, opened with the context menu key or Shift+F10, moves a panel to the group on a
// side, splits a group, floats a panel, or closes it. Ctrl+Shift with an arrow key moves a
// panel to the group on that side. Ctrl+Tab and Ctrl+Shift+Tab select the next and
// previous panel of a group, and Ctrl+F6 moves focus to the next group.
//
// OnChange runs after each change the user makes, so the app can save the layout. A dock
// fills its container, as Split does; give it a size or a grow factor in a column with
// other children. See the Docking section of the README.
func Dock(layout *Signal[DockLayout], panels map[string]DockPanel) *DockElement {
	if layout == nil {
		panic("ui.Dock: the layout is nil; make one with ui.NewSignal(ui.DockLayout{...})")
	}
	return &DockElement{layout: layout, panels: panels}
}

func (*DockElement) isElement()           {}
func (d *DockElement) elementKey() string { return d.key }

// Key gives the dock a stable identity among its siblings. It also scopes the keys of its
// tabs and panels, so give two docks in one window different keys.
func (d *DockElement) Key(k string) *DockElement { d.key = k; return d }

// Styles sets the look of the parts of the dock, for this instance only. Set fields win
// over the theme's Dock and the defaults; see DockStyle and docs/styling.md. Calling
// Styles again merges with the earlier values.
func (d *DockElement) Styles(s DockStyle) *DockElement { d.styles = d.styles.merge(s); return d }

// OnChange runs fn with the new layout after the user changes it: a panel moved, a
// divider moved, a tab selected, a panel floated, docked back or closed, or a floating
// window closed. The signal is set first. Use it to save the layout.
func (d *DockElement) OnChange(fn func(DockLayout)) *DockElement { d.onChange = fn; return d }

// Floating lets the groups of the layout float in windows of their own. open opens the
// window of a floating group and returns it; app.OpenDockWindow does so with app.Open,
// which the ui package cannot import. The dock opens a window for each floating group it
// does not have one for, and closes the window of a group that docks back. Without
// Floating, the floating groups of the layout are not shown, and the panel's Float command
// is disabled.
//
// A floating window shows its group only. Its panels are not dragged across windows: a
// panel moves to another window with the Float and Dock back commands. Set QuitWhenClosed
// on the main window of the app, so that closing it closes the floating windows too; their
// positions are recorded when they close by the user, not when the main window closes.
func (d *DockElement) Floating(open func(DockWindow) Window) *DockElement { d.open = open; return d }

// The sizing methods give the dock's outer box a size. A dock fills its container by
// default; see Sizing in docs/styling.md.
func (d *DockElement) Width(l Length) *DockElement          { d.size.setWidth(l); return d }
func (d *DockElement) Height(l Length) *DockElement         { d.size.setHeight(l); return d }
func (d *DockElement) MinWidth(l Length) *DockElement       { d.size.setMinWidth(l); return d }
func (d *DockElement) MinHeight(l Length) *DockElement      { d.size.setMinHeight(l); return d }
func (d *DockElement) MaxWidth(l Length) *DockElement       { d.size.setMaxWidth(l); return d }
func (d *DockElement) MaxHeight(l Length) *DockElement      { d.size.setMaxHeight(l); return d }
func (d *DockElement) Grow(f float32) *DockElement          { d.size.setGrow(f); return d }
func (d *DockElement) GridSpan(cols, rows int) *DockElement { d.size.setSpan(cols, rows); return d }

// scope is the prefix of the keys of the dock: its Key, or the identity of its layout.
func (d *DockElement) scope() string {
	if d.key != "" {
		return d.key
	}
	return fmt.Sprintf("dock:%p", d.layout)
}

// keep reports whether the dock shows the panel id; the layout's other IDs are dropped.
func (d *DockElement) keep(id string) bool {
	_, ok := d.panels[id]
	return ok
}

// dockDrag is the payload of a dragged tab: the panel, and the dock it came from, so that
// only the dock accepts it.
type dockDrag struct {
	scope string
	panel string
}

// dockHover is the drag over a dock, and where the dragged panel would land: drag is set
// while a panel of the dock is dragged over it, group is the group the pointer is over,
// by its first panel, zone is the zone in that group, and edge is set when the pointer is
// over an edge strip, whose side is the zone. It is what the previews show.
type dockHover struct {
	drag  bool
	group string
	zone  DockZone
	edge  bool
}

// dockView is one render of a dock, or of a floating window's group. It holds what the
// parts of the render share.
type dockView struct {
	ctx     *Context
	d       *DockElement
	layout  *Signal[DockLayout]
	lay     DockLayout // the layout as this render shows it, normalized
	st      DockStyle
	th      *Theme
	strs    *Strings
	scope   string
	float   bool                         // the view is the group of a floating window
	panels  map[string]*ComponentElement // the component of each panel, kept across renders
	handles map[string]*FocusHandle      // the focus handle of each tab
	hover   *Signal[dockHover]
	hov     dockHover // the hover as this render shows it
	windows map[string]Window
}

// tabKey is the key of the tab of the panel id. The panel's tab panel is named by it.
func (v *dockView) tabKey(id string) string { return v.scope + "/tab:" + id }

// handle returns the focus handle of the tab of the panel id, made the first time.
func (v *dockView) handle(id string) *FocusHandle {
	h := v.handles[id]
	if h == nil {
		h = NewFocusHandle()
		v.handles[id] = h
	}
	return h
}

// setHover sets the drop preview, when it differs.
func (v *dockView) setHover(h dockHover) {
	if v.hover.Peek() != h {
		v.hover.Set(h)
	}
}

// record stores a new layout, the way the user changes it, and runs OnChange. The layout
// is normalized with the panels the dock shows, so a saved layout holds no unknown panel.
func (v *dockView) record(next DockLayout) {
	next = normalizeLayout(next, v.d.keep)
	v.layout.Set(next)
	if v.d.onChange != nil {
		v.d.onChange(next)
	}
}

// norm returns the layout as the dock shows it: normalized with its known panels.
func (v *dockView) norm(l DockLayout) DockLayout { return normalizeLayout(l.clone(), v.d.keep) }

// selectPanel makes the panel id the active tab of its group, and focuses its tab.
func (v *dockView) selectPanel(id string) {
	v.record(DockFocus(v.norm(v.layout.Peek()), id))
	v.handle(id).Focus()
}

// movePanel applies a layout operation to the panel id, and keeps focus on its tab.
func (v *dockView) movePanel(id string, next DockLayout) {
	v.record(next)
	v.handle(id).Focus()
}

// moves reports whether dropping the panel id on the zone of the group that holds target
// changes the layout. The preview of a drop that changes nothing is not shown.
func (v *dockView) moves(id, target string, zone DockZone) bool {
	cur := v.norm(v.layout.Peek())
	return !reflect.DeepEqual(DockMove(cur, id, target, zone), cur)
}

// zoneOf returns the zone of a drop on a group: the side or the center of the group under
// the pointer. A floating group has no sides, so its zone is always the center.
func (v *dockView) zoneOf(e *DropEvent) DockZone {
	if v.float {
		return DockCenter
	}
	return dockZoneAt(e.LocalX, e.LocalY, e.W, e.H)
}

// dockKeySide returns the side that Ctrl+Shift with the arrow key k moves a panel to.
func dockKeySide(k input.Key) (DockZone, bool) {
	switch k {
	case input.KeyLeft:
		return DockLeft, true
	case input.KeyRight:
		return DockRight, true
	case input.KeyUp:
		return DockTop, true
	case input.KeyDown:
		return DockBottom, true
	}
	return DockCenter, false
}

// dockPreview returns the preview of a zone, drawn over its group. It is a child of the
// group, so the group stays in the hit path, and it is not a drop target.
func dockPreview(st DockStyle, zone DockZone) *DivElement {
	p := Div().asPart("Dock.Preview").Absolute().Style(st.Preview)
	switch zone {
	case DockLeft:
		p.Left(0).Top(0).Bottom(0).Width(Percent(50))
	case DockRight:
		p.Right(0).Top(0).Bottom(0).Width(Percent(50))
	case DockTop:
		p.Top(0).Left(0).Right(0).Height(Percent(50))
	case DockBottom:
		p.Bottom(0).Left(0).Right(0).Height(Percent(50))
	default:
		p.Inset(0)
	}
	return p
}

// dockNodeAt returns the node at the index path under n, or nil when the path is not in
// the tree.
func dockNodeAt(n *DockNode, path []int) *DockNode {
	for _, i := range path {
		if i < 0 || i >= len(n.Children) {
			return nil
		}
		n = &n.Children[i]
	}
	return n
}

// dockGroups returns the tab groups under n, in reading order.
func dockGroups(n *DockNode, out []*DockNode) []*DockNode {
	if n.empty() {
		return out
	}
	if !n.isSplit() {
		return append(out, n)
	}
	for i := range n.Children {
		out = dockGroups(&n.Children[i], out)
	}
	return out
}

// floatByID returns the floating group with the ID id, or nil.
func floatByID(l DockLayout, id string) *DockFloat {
	for i := range l.Floating {
		if l.Floating[i].ID == id {
			return &l.Floating[i]
		}
	}
	return nil
}

// dockPath returns the path as the key of a split: its indices, separated by spaces.
func dockPath(path []int) string { return strings.Trim(fmt.Sprint(path), "[]") }

// appendPath returns a copy of path with the index k added, so the paths of the children
// do not share an array.
func appendPath(path []int, k int) []int { return append(slices.Clone(path), k) }

func (d *DockElement) expand(ctx *Context) Element {
	// panels holds the component of each panel, made by this dock. Rendering the dock again
	// reuses them, so a change to the layout does not build the panels again.
	panels := map[string]*ComponentElement{}
	return Component(func(ctx *Context) Element {
		v := &dockView{
			ctx: ctx, d: d, layout: d.layout, th: ctx.Theme(), strs: ctx.Strings(), scope: d.scope(),
			st: resolveDock(ctx.Theme(), d.styles), panels: panels,
			handles: Local(ctx, "handles", map[string]*FocusHandle{}).Peek(),
			hover:   Local(ctx, "hover", dockHover{}),
			windows: Local(ctx, "windows", map[string]Window{}).Peek(),
		}
		v.hov = v.hover.Get()
		v.lay = v.norm(d.layout.Get())
		root := Div().Key(v.scope).Column().Size(Full, Full)
		root.OnKeyDown(func(e *KeyEvent) {
			if e.Key != input.KeyF6 || e.Repeat || e.Mods != input.ModCtrl {
				return
			}
			e.StopPropagation()
			v.cycleGroup()
		})
		if !v.lay.Root.empty() {
			root.Child(v.node(v.lay.Root, nil))
		}
		if v.hov.drag && !v.float {
			for _, side := range [4]DockZone{DockLeft, DockRight, DockTop, DockBottom} {
				root.Child(v.band(side))
			}
		}
		if d.open != nil {
			ctx.Post(func() { d.syncWindows(v) })
		}
		return d.size.apply(root)
	})
}

// cycleGroup moves focus to the active tab of the group after the one that has focus.
func (v *dockView) cycleGroup() {
	groups := dockGroups(&v.lay.Root, nil)
	if len(groups) == 0 {
		return
	}
	cur := -1
	for i, g := range groups {
		for _, p := range g.Panels {
			if h := v.handles[p]; h != nil && h.Focused() {
				cur = i
			}
		}
	}
	v.handle(groups[(cur+1)%len(groups)].Active).Focus()
}

// node returns the element of a node of the layout: a group, or the splits of a split.
func (v *dockView) node(n DockNode, path []int) Element {
	if !n.isSplit() {
		return v.group(n)
	}
	return v.chain(n, path, 0)
}

// chain returns the split of the children of n from index k on. Each divider is a Split of
// one child and the children after it, so it moves the share of that child against the
// rest, and the children are nested as deep as they are many.
func (v *dockView) chain(n DockNode, path []int, k int) Element {
	if k == len(n.Children)-1 {
		return v.node(n.Children[k], appendPath(path, k))
	}
	sz := shares(n.Sizes, len(n.Children))
	rest := 0.0
	for _, s := range sz[k:] {
		rest += s
	}
	pos := NewSignal(sz[k] / rest)
	key := fmt.Sprintf("%s/split:%s:%d", v.scope, dockPath(path), k)
	sp := Split(v.node(n.Children[k], appendPath(path, k)), v.chain(n, path, k+1)).Key(key).Position(pos).
		MinFirst(Px(dockMinPane)).MinSecond(Px(dockMinPane)).
		OnChange(func(f float64) { v.resize(path, k, f) })
	if n.Vertical {
		sp.Orientation(SplitVertical)
	}
	return sp
}

// resize sets the share of child k of the split at path, so that the divider after it is
// at the fraction f of the split's length from the divider's start.
func (v *dockView) resize(path []int, k int, f float64) {
	out := v.norm(v.layout.Peek())
	n := dockNodeAt(&out.Root, path)
	if n == nil || !n.isSplit() || k+1 >= len(n.Children) {
		return
	}
	sz := shares(n.Sizes, len(n.Children))
	rest := 0.0
	for _, s := range sz[k+1:] {
		rest += s
	}
	f = min(max(f, 0.001), 0.999)
	sz[k] = f * rest / (1 - f)
	n.Sizes = sz
	v.record(out)
}

// group returns the element of a tab group: its tab strip, and each panel, which stays
// built while it is not the active tab. The group is a drop target for a dragged panel.
func (v *dockView) group(g DockNode) Element {
	anchor := g.Panels[0]
	gv := Div().asPart("Dock.Group").Key(v.scope + "/group:" + anchor).Column().Grow(1).
		MinWidth(Px(0)).MinHeight(Px(0)).Style(v.st.Group)
	strip := Div().asPart("Dock.Strip").Row().Role(access.RoleTabList).Label(v.strs.PanelGroup).Style(v.st.Strip)
	body := Div().Column().Grow(1).MinHeight(Px(0))
	for _, id := range g.Panels {
		strip.Child(v.tab(g, id))
		body.Child(v.panel(g, id))
	}
	gv.Child(strip).Child(body)
	if v.hov.group == anchor {
		gv.Child(dockPreview(v.st, v.hov.zone))
	}
	gv.OnKeyDown(func(e *KeyEvent) {
		if e.Key != input.KeyTab || e.Repeat || (e.Mods != input.ModCtrl && e.Mods != input.ModCtrl|input.ModShift) {
			return
		}
		dir := 1
		if e.Mods.Has(input.ModShift) {
			dir = -1
		}
		e.StopPropagation()
		i := slices.Index(g.Panels, g.Active)
		v.selectPanel(g.Panels[(i+dir+len(g.Panels))%len(g.Panels)])
	})
	return gv.OnDrop(Drop(func(p dockDrag, e *DropEvent) {
		v.setHover(dockHover{})
		v.movePanel(p.panel, DockMove(v.norm(v.layout.Peek()), p.panel, anchor, v.zoneOf(e)))
	}).Accept(func(p dockDrag) bool { return p.scope == v.scope }).
		Move(func(p dockDrag, e *DropEvent) {
			zone := v.zoneOf(e)
			if !v.moves(p.panel, anchor, zone) {
				v.setHover(dockHover{drag: true})
				return
			}
			v.setHover(dockHover{drag: true, group: anchor, zone: zone})
		}).
		Leave(func() { v.setHover(dockHover{drag: true}) }))
}

// band returns the strip along the outer edge of the dock on the side the zone gives. The
// strips are up while a panel is dragged over the dock, and a drop on one puts the panel
// along that edge. A strip is the preview of an edge drop; it is not drawn over the groups
// at other times, so it takes no pointer then.
func (v *dockView) band(side DockZone) *DivElement {
	var b *DivElement
	switch side {
	case DockLeft:
		b = Div().Absolute().Left(0).Top(0).Bottom(0).Width(Px(dockEdgeBand))
	case DockRight:
		b = Div().Absolute().Right(0).Top(0).Bottom(0).Width(Px(dockEdgeBand))
	case DockTop:
		b = Div().Absolute().Top(0).Left(0).Right(0).Height(Px(dockEdgeBand))
	default:
		b = Div().Absolute().Bottom(0).Left(0).Right(0).Height(Px(dockEdgeBand))
	}
	if v.hov.edge && v.hov.zone == side {
		b.asPart("Dock.Preview").Style(v.st.Preview)
	}
	return b.OnDrop(Drop(func(p dockDrag, e *DropEvent) {
		v.setHover(dockHover{})
		v.movePanel(p.panel, DockMove(v.norm(v.layout.Peek()), p.panel, "", side))
	}).Accept(func(p dockDrag) bool { return p.scope == v.scope }).
		Move(func(p dockDrag, e *DropEvent) {
			v.setHover(dockHover{drag: true, edge: true, zone: side})
		}).
		Leave(func() { v.setHover(dockHover{drag: true}) }))
}

// tab returns the tab of the panel id in the group g: its icon and title, its focus and
// selection, its menu, and the drag that moves it.
func (v *dockView) tab(g DockNode, id string) *DivElement {
	p := v.d.panels[id]
	on := g.Active == id
	style := v.st.Tab
	if on {
		style = style.Merge(v.st.TabSelected)
	}
	title := p.Title
	// The dragged tab is dimmed, except under high contrast, where it keeps its colors.
	source := Style{Opacity: 0.5}
	if v.th.HighContrast {
		source = Style{}
	}
	tab := Div().asPart("Dock.Tab").Key(v.tabKey(id)).Style(style).
		Role(access.RoleTab).Label(title).AccessSelected(on).Cursor(CursorPointer).
		Focusable().FocusVisible(v.st.TabFocusVisible).FocusHandle(v.handle(id)).
		Dragging(source).
		OnDrag(Drag(dockDrag{scope: v.scope, panel: id}).Preview(func() Element {
			return Div().Padding(6).Background(v.th.Surface).Border(1, v.th.Border).Child(Text(title))
		}).End(func(bool) { v.setHover(dockHover{}) })).
		OnClick(func() { v.selectPanel(id) }).
		OnKeyDown(v.tabKeys(g, id)).
		ContextMenu(v.menu(g, id))
	if !on {
		tab.Hover(v.st.TabHover).TabStop(false)
	}
	if p.Icon != nil {
		tab.Child(Icon(p.Icon).Size(16))
	}
	tab.Child(Text(title).NoWrap())
	ind := Div().asPart("Dock.Indicator").Absolute().Left(0).Right(0).Bottom(0).Style(v.st.Indicator)
	if !on {
		ind.Background(Transparent)
	}
	return tab.Child(ind)
}

// tabKeys returns the key handler of the tab of the panel id. Left and Right select the
// neighbouring tab, Home and End the first and the last, and Ctrl+Shift with an arrow moves
// the panel to the group on that side.
func (v *dockView) tabKeys(g DockNode, id string) func(*KeyEvent) {
	return func(e *KeyEvent) {
		if e.Repeat {
			return
		}
		if e.Mods == input.ModCtrl|input.ModShift {
			if side, ok := dockKeySide(e.Key); ok {
				e.StopPropagation()
				v.movePanel(id, DockMoveSide(v.norm(v.layout.Peek()), id, side))
			}
			return
		}
		if e.Mods != 0 {
			return
		}
		i := slices.Index(g.Panels, id)
		j := -1
		switch e.Key {
		case input.KeyLeft:
			j = i - 1
		case input.KeyRight:
			j = i + 1
		case input.KeyHome:
			j = 0
		case input.KeyEnd:
			j = len(g.Panels) - 1
		default:
			return
		}
		e.StopPropagation()
		if j >= 0 && j < len(g.Panels) && j != i {
			v.selectPanel(g.Panels[j])
		}
	}
}

// panel returns the box of the panel id in the group g. It is built once, and kept
// mounted while another tab is active, so its state survives the switch.
func (v *dockView) panel(g DockNode, id string) Element {
	p := v.d.panels[id]
	pc := v.panels[id]
	if pc == nil {
		render := p.Render
		pc = Component(func(*Context) Element {
			if render == nil {
				return Div()
			}
			return render()
		})
		v.panels[id] = pc
	}
	return Div().asPart("Dock.Panel").Key(v.scope + "/panel:" + id).Column().Grow(1).MinHeight(Px(0)).
		Role(access.RoleTabPanel).Label(p.Title).Display(g.Active == id).Style(v.st.Panel).
		LabelledBy(v.tabKey(id)).Child(pc)
}

// menu returns the context menu of the tab of the panel id, built when it opens. The items
// that the layout cannot do are disabled: a move with no group on that side, a split of a
// panel that is alone in its group, and any change to a floating group but Dock back.
func (v *dockView) menu(g DockNode, id string) func() []MenuItem {
	return func() []MenuItem {
		l := v.norm(v.layout.Peek())
		s := v.strs
		move := func(label string, side DockZone) MenuItem {
			return MenuItem{Label: label, Disabled: v.float || !dockHasSide(l, id, side),
				OnClick: func() { v.movePanel(id, DockMoveSide(v.norm(v.layout.Peek()), id, side)) }}
		}
		split := func(label string, side DockZone) MenuItem {
			return MenuItem{Label: label, Disabled: v.float || len(g.Panels) < 2,
				OnClick: func() { v.movePanel(id, DockMove(v.norm(v.layout.Peek()), id, id, side)) }}
		}
		items := []MenuItem{
			move(s.MovePanelLeft, DockLeft),
			move(s.MovePanelRight, DockRight),
			move(s.MovePanelUp, DockTop),
			move(s.MovePanelDown, DockBottom),
			MenuSeparator(),
			split(s.SplitPanelRight, DockRight),
			split(s.SplitPanelDown, DockBottom),
			MenuSeparator(),
		}
		if v.float || l.floating(id) {
			items = append(items, MenuItem{Label: s.DockPanelBack,
				OnClick: func() { v.movePanel(id, DockBack(v.norm(v.layout.Peek()), id)) }})
		} else {
			items = append(items, MenuItem{Label: s.FloatPanel, Disabled: v.d.open == nil,
				OnClick: func() { v.floatPanel(id) }})
		}
		if p := v.d.panels[id]; p.Closable {
			items = append(items, MenuSeparator(), MenuItem{Label: s.ClosePanel,
				OnClick: func() { v.movePanel(id, DockClose(v.norm(v.layout.Peek()), id)) }})
		}
		return items
	}
}

// floatPanel floats the panel id in a window placed a little below and to the right of
// the main window.
func (v *dockView) floatPanel(id string) {
	x, y := v.ctx.Window().Position()
	v.movePanel(id, DockFloatPanel(v.norm(v.layout.Peek()), id, x+48, y+48, 480, 360))
}

// syncWindows opens a window for each floating group that has none, and closes the window
// of each group the layout no longer floats. It runs after the render, on the UI thread.
func (d *DockElement) syncWindows(v *dockView) {
	cur := normalizeLayout(v.layout.Peek().clone(), d.keep)
	live := map[string]bool{}
	for _, f := range cur.Floating {
		live[f.ID] = true
	}
	for id, w := range v.windows {
		if !live[id] {
			delete(v.windows, id)
			w.Close()
		}
	}
	for _, f := range cur.Floating {
		if v.windows[f.ID] == nil {
			v.windows[f.ID] = d.open(d.floatWindow(v, f))
		}
	}
}

// floatWindow describes the window of the floating group f. Closing the window records
// its position and size, and docks its panels back: a window closed by the user, or by the
// dock when its panels leave, is then the same as a group that docks back.
func (d *DockElement) floatWindow(v *dockView, f DockFloat) DockWindow {
	id := f.ID
	title := ""
	if p, ok := d.panels[f.Group.Active]; ok {
		title = p.Title
	}
	return DockWindow{
		Title: title, X: f.X, Y: f.Y, Width: f.Width, Height: f.Height,
		Root: d.floatRoot(v.layout, id),
		OnClose: func() bool {
			if w := v.windows[id]; w != nil {
				x, y := w.Position()
				width, height := w.Size()
				v.record(dockSetFloatBounds(v.layout.Peek(), id, x, y, width, height))
			}
			return true
		},
		OnClosed: func() {
			delete(v.windows, id)
			l := v.layout.Peek()
			fl := floatByID(l, id)
			if fl == nil {
				return
			}
			panels := slices.Clone(fl.Group.Panels)
			next := l
			for i, p := range panels {
				if i == 0 {
					next = DockBack(next, p)
				} else {
					next = DockMove(next, p, panels[0], DockCenter)
				}
			}
			v.record(next)
		},
	}
}

// floatRoot returns the content of the window of the floating group id: the group, with
// its tabs and panels. The panels are built by the window's own runtime, so they keep their
// state only within it.
func (d *DockElement) floatRoot(layout *Signal[DockLayout], id string) func(*Context) Element {
	panels := map[string]*ComponentElement{}
	return func(ctx *Context) Element {
		v := &dockView{
			ctx: ctx, d: d, layout: layout, th: ctx.Theme(), strs: ctx.Strings(), scope: d.scope() + "/float:" + id,
			st: resolveDock(ctx.Theme(), d.styles), panels: panels, float: true,
			handles: Local(ctx, "handles", map[string]*FocusHandle{}).Peek(),
			hover:   Local(ctx, "hover", dockHover{}),
		}
		v.hov = v.hover.Get()
		v.lay = v.norm(layout.Get())
		f := floatByID(v.lay, id)
		if f == nil {
			return Div()
		}
		return Div().Column().Size(Full, Full).Child(v.group(f.Group))
	}
}
