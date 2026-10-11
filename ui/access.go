package ui

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/tyler-builds/jamb/access"
)

// The accessibility tree is built from the runtime's nodes (see
// docs/accessibility.md). Nothing is built until an assistive technology asks
// for the tree, or development warnings are on. After that, a frame publishes
// a new tree only when the UI changed (a component re-rendered, a layout or
// paint moved something, or the focus moved), so frames cost nothing extra
// when no assistive technology is running.

// accessInfo holds what an element adds to the accessibility tree. It is set by
// the element constructors and the public methods below, and nil otherwise.
type accessInfo struct {
	role        access.Role
	name        string // an explicit label; wins over the text the element contains
	badge       string // the name of an attached badge, added to the name (see Badged)
	description string
	hidden      bool
	disabled    bool
	checked     bool
	checkedSet  bool // Checked was called, so a toggle or radio button has a state
	toggle      bool // the element is a toggle button (see ButtonElement.Checked)
	setPos      int  // 1-based position in a set, such as a row of a list
	setSize     int  // the number of items in the set
	level       int  // the depth of a menu, 1 at the top
	heading     int  // the level of a heading, 1 to 6
	expandable  bool
	expanded    bool
	// setExpanded runs an expand or collapse that is not a click, such as the
	// submenu of a menu item. Nil means a click toggles the element.
	setExpanded func(open bool)
	selectable  bool
	selected    bool
	container   bool // the element holds selection items
	labelledBy  []string
	describedBy []string
	// inert leaves the element out of the tree while its children stay in it.
	// A DropdownMenu's trigger is inert, since the menu's wrapper is the control.
	inert bool
	// focusInside is set on an element whose keyboard focus lands on an inert child
	// (see DropdownMenu): the element takes focus for it, and an action to focus
	// the element moves focus to that child.
	focusInside bool
	// highlighted marks the row a menu highlights (its keyboard position). The
	// menu's panel passes focus to that row, so the highlight is what a screen
	// reader hears (see focusTarget).
	highlighted bool
	// mixed is set on a check box in the indeterminate state (see CheckboxTri).
	mixed bool
	// invalid is set on a field whose text is not a valid value (see NumberField).
	invalid bool
	// readOnly is set on a control whose value is shown but not typed, such as a
	// Select.
	readOnly bool
	// value is the value of a combo box: the text it shows.
	value string
	// rng is the number of a slider or spin box, and setRange sets it, for the
	// RangeValue pattern (ActionSetRangeValue).
	rng      *access.Range
	setRange func(v float64)
	// gridRows and gridCols are the shape of a data grid; gridRow and gridCol the
	// 1-based position of a cell (see DataGrid). status is the status of a header.
	gridRows, gridCols int
	gridRow, gridCol   int
	status             string
	// multiSelect is set on a selection container that takes many items.
	multiSelect bool
	// selectWith chooses the item with the modifier Ctrl (true) or without it. A data
	// grid row sets it, so that AddToSelection and RemoveFromSelection keep the others.
	selectWith func(ctrl bool)
}

// access returns the element's accessibility settings, creating them on use.
func (d *DivElement) access() *accessInfo {
	if d.acc == nil {
		d.acc = &accessInfo{}
	}
	return d.acc
}

// Role gives the element an accessibility role, such as access.RoleCheckBox.
// Built-in widgets set their own roles; use Role for a custom control.
func (d *DivElement) Role(r access.Role) *DivElement { d.access().role = r; return d }

// Label names the element for assistive technology. It wins over the text the
// element contains, so it is also how an element with no visible text (an
// icon-only button, for example) gets a name.
func (d *DivElement) Label(s string) *DivElement { d.access().name = s; return d }

// Description adds help text, which assistive technology reads after the name.
func (d *DivElement) Description(s string) *DivElement { d.access().description = s; return d }

// Checked gives the element an on or off state: whether a check box, switch,
// radio button, toggle button or checkable menu item is on. Assistive technology
// reports the state after the name, and the element can be toggled. A check box,
// switch or radio button needs a state, so the development warnings ask for this
// call. A radio button is a choice in a group (see Selected), not a toggle.
func (d *DivElement) Checked(on bool) *DivElement {
	a := d.access()
	a.checked, a.checkedSet, a.toggle = on, true, true
	return d
}

// Expanded marks the element as one that opens and closes, such as a row with
// details or a dropdown trigger, and says whether it is open. Assistive
// technology reports the state and can open or close the element. Opening and
// closing run the element's click handler, which must toggle the state, when
// the element is not already in the requested state.
func (d *DivElement) Expanded(open bool) *DivElement {
	a := d.access()
	a.expandable, a.expanded = true, open
	return d
}

// Selected marks the element as one item of a selection, such as the selected
// note in a list, and says whether it is chosen. The item must be inside a
// SelectionContainer. Selecting an item that is not chosen runs its click
// handler, which must choose it.
func (d *DivElement) Selected(on bool) *DivElement {
	a := d.access()
	a.selectable, a.selected = true, on
	return d
}

// SelectionContainer marks the element as the group that holds selection items,
// such as the filter chips or the list of notes. Each item reports the group
// it belongs to, and the group reports its chosen items.
func (d *DivElement) SelectionContainer() *DivElement { d.access().container = true; return d }

// Heading makes the element a heading of the given level, 1 to 6. Its text is
// its name. Heading panics for any other level.
func (d *DivElement) Heading(level int) *DivElement {
	if level < 1 || level > 6 {
		panic(fmt.Sprintf("ui.Heading(%d): the level must be 1 to 6", level))
	}
	a := d.access()
	a.role, a.heading = access.RoleHeading, level
	return d
}

// LabelledBy names the element by the text of the elements with the given keys,
// such as a visible label above a field. The keys are the Key of those elements.
func (d *DivElement) LabelledBy(keys ...string) *DivElement {
	d.access().labelledBy = keys
	return d
}

// DescribedBy describes the element by the text of the elements with the given
// keys, read after its name.
func (d *DivElement) DescribedBy(keys ...string) *DivElement {
	d.access().describedBy = keys
	return d
}

// Hidden leaves the element and its children out of the accessibility tree.
// It does not change how the element is drawn.
func (d *DivElement) Hidden() *DivElement { d.access().hidden = true; return d }

// listItem marks the element as the pos-th of size items in a list, so that
// assistive technology can say "40 of 10,000" for a row of a virtualized list.
func (d *DivElement) listItem(pos, size int) *DivElement {
	a := d.access()
	a.role, a.setPos, a.setSize = access.RoleListItem, pos, size
	return d
}

// setListPosition marks the row r, an element of the list whose wrapper is w, as the
// pos-th of size items. A row that declares its own role, such as a list item
// with a selection, is the item and takes the position itself; otherwise the
// wrapper is the item.
func setListPosition(w *DivElement, r Element, pos, size int) {
	if d, ok := r.(*DivElement); ok && d.acc != nil && d.acc.role != access.RoleUnknown {
		a := d.access()
		a.setPos, a.setSize = pos, size
		return
	}
	w.listItem(pos, size)
}

// menuAccessRole maps a menu entry's role to the access role.
func menuAccessRole(r MenuRole) access.Role {
	switch r {
	case RoleMenuItemCheckbox:
		return access.RoleMenuItemCheck
	case RoleMenuItemRadio:
		return access.RoleMenuItemRadio
	case RoleSeparator:
		return access.RoleSeparator
	}
	return access.RoleMenuItem
}

// accessAnnouncements is the queue of Announce calls, drained by the next
// published tree. It is shared by the runtimes of the process, so it has a lock.
var accessAnnouncements struct {
	sync.Mutex
	queue []access.Announcement
}

// maxAnnouncements bounds the queue. Without an assistive technology nothing
// drains it, so the oldest messages are dropped.
const maxAnnouncements = 16

// Announce queues text for an assistive technology to speak, for a status
// message that has no focused element to carry it, such as "Saved". It is safe
// from any goroutine. Without an assistive technology it has no effect beyond
// the bounded queue.
func Announce(text string, p access.Politeness) {
	accessAnnouncements.Lock()
	defer accessAnnouncements.Unlock()
	q := append(accessAnnouncements.queue, access.Announcement{Text: text, Politeness: p})
	if len(q) > maxAnnouncements {
		q = q[len(q)-maxAnnouncements:]
	}
	accessAnnouncements.queue = q
}

// takeAnnouncements returns the queued announcements and empties the queue.
func takeAnnouncements() []access.Announcement {
	accessAnnouncements.Lock()
	defer accessAnnouncements.Unlock()
	q := accessAnnouncements.queue
	accessAnnouncements.queue = nil
	return q
}

// accessState is the runtime's side of the accessibility tree. It exists once
// the bridge is asked for or development warnings are turned on.
type accessState struct {
	bridge *access.Bridge
	warn   bool // development warnings are on: the tree is built without an assistive technology
	dirty  bool // the UI changed since the tree was last built
	tree   *access.Tree
	// targets maps each published node ID to what an action on it runs.
	targets map[access.ID]accessTarget
	// warned records the elements already warned about, so each is reported once.
	warned map[uint64]bool
}

// accessTarget is the element an action runs on: a node, and for a link the
// index of its segment in the node's rich text (-1 for the node itself).
type accessTarget struct {
	node *node
	seg  int
}

// AccessBridge returns the bridge that publishes the runtime's accessibility
// tree to the platform adapter. The host passes it to the adapter (see
// platform.AccessHandler). It is created on first use and is safe to call on
// the UI thread.
func (rt *Runtime) AccessBridge() *access.Bridge {
	return rt.accessState().bridge
}

// SetAccessWarnings turns the development warnings on or off. While they are
// on, the tree is built after each change even without an assistive
// technology, so that unlabeled elements are reported when they are built.
// Each element is reported once. Warnings go to the function set by
// SetWarnFunc.
func (rt *Runtime) SetAccessWarnings(on bool) {
	if rt.acc == nil && !on {
		return // nothing to switch off; keeps the runtime free of access state
	}
	rt.accessState().warn = on
}

// AccessTree builds the accessibility tree now, from the current nodes, for
// tests and the tree dump. It does not publish it.
func (rt *Runtime) AccessTree() *access.Tree {
	t, _ := rt.buildAccess()
	return t
}

func (rt *Runtime) accessState() *accessState {
	if rt.acc == nil {
		a := &accessState{bridge: access.NewBridge(), dirty: true, warned: map[uint64]bool{}}
		a.bridge.SetHooks(rt.activateAccess, rt.performAccess)
		rt.acc = a
	}
	return rt.acc
}

// markAccess records that the UI changed, so the next frame publishes a new
// tree. It is cheap when no tree is wanted.
func (rt *Runtime) markAccess() {
	if rt.acc != nil {
		rt.acc.dirty = true
	}
}

// activateAccess runs on the UI thread when an assistive technology first asks
// for the tree. It publishes at once, so the first request already sees the
// tree, rather than waiting for the next frame.
func (rt *Runtime) activateAccess() {
	rt.acc.dirty = true
	rt.publishAccess()
}

// publishAccess rebuilds the tree if the UI changed and it is wanted, and
// publishes it with the changes since the previous tree.
func (rt *Runtime) publishAccess() {
	a := rt.acc
	if a == nil || !a.dirty || !(a.bridge.Active() || a.warn) {
		return
	}
	a.dirty = false
	prev := a.tree
	t, targets := rt.buildAccess()
	a.tree, a.targets = t, targets
	if a.bridge.Active() {
		u := access.Diff(prev, t)
		u.Announcements = takeAnnouncements()
		a.bridge.Publish(t, u)
	}
}

// performAccess runs a request made through the bridge. The bridge may call it
// from any goroutine, so it is posted to the UI thread.
func (rt *Runtime) performAccess(id access.ID, req access.Request) {
	rt.Post(func() { rt.runAccess(id, req) })
}

// runAccess runs a request on the UI thread. A request on a node that has gone
// since the last published tree does nothing.
func (rt *Runtime) runAccess(id access.ID, req access.Request) {
	a := rt.acc
	if a == nil {
		return
	}
	t, ok := a.targets[id]
	if !ok || rt.byID[t.node.id] != t.node {
		return
	}
	n := t.node
	role, _ := accessRole(n)
	switch req.Action {
	case access.ActionClick, access.ActionToggle:
		if t.seg >= 0 {
			if fn := n.rich.segs[t.seg].onClick; fn != nil {
				fn()
			}
		} else if n.div != nil && n.div.h.onClick != nil {
			n.div.h.onClick()
		}
	case access.ActionExpand, access.ActionCollapse:
		if n.div != nil && n.div.acc != nil && n.div.acc.expandable {
			rt.setExpandedFrom(n, req.Action == access.ActionExpand)
		}
	case access.ActionSelect:
		// A radio button is a selection item by its role, though it is set with Checked.
		if n.div != nil && n.div.acc != nil && (n.div.acc.selectable || radioRole(role)) && !isSelected(role, n.div.acc) {
			rt.clickNode(n)
		}
	case access.ActionAddToSelection, access.ActionRemoveFromSelection:
		if n.div == nil || n.div.acc == nil {
			break
		}
		a := n.div.acc
		selected := isSelected(role, a)
		switch {
		case a.selectWith != nil:
			if selected == (req.Action == access.ActionRemoveFromSelection) {
				a.selectWith(true)
			}
		case req.Action == access.ActionAddToSelection && a.selectable && !selected:
			rt.clickNode(n)
		}
	case access.ActionFocus:
		if n.div == nil || n.div.focusable || n.kind == kindEditor {
			rt.setFocus(n.id)
		} else if n.div.acc != nil && n.div.acc.focusInside {
			rt.focusChild(n)
		}
	case access.ActionSetValue:
		if n.kind == kindEditor {
			rt.editorSet(n.id, req.Value, len(req.Value), editOther)
		}
	case access.ActionSetTextSelection:
		if n.kind == kindEditor {
			rt.editorSelect(n.id, req.Anchor, req.Caret)
		}
	case access.ActionSetRangeValue:
		if n.div != nil && n.div.acc != nil && n.div.acc.setRange != nil {
			n.div.acc.setRange(req.X)
		}
	case access.ActionScrollIntoView:
		rt.scrollIntoView(n)
	case access.ActionScroll:
		rt.scrollBy(n, float32(req.X), float32(req.Y))
	case access.ActionSetScrollPercent:
		rt.scrollToPercent(n, req.X, req.Y)
	}
	rt.requestFrame()
}

// clickNode runs the click handler of an element, if it has one.
func (rt *Runtime) clickNode(n *node) {
	if n.div != nil && n.div.h.onClick != nil {
		n.div.h.onClick()
	}
}

// setExpandedFrom opens or closes an expandable element that is not already in
// the requested state. An element with setExpanded runs it; otherwise a click
// toggles the element, so its handler does the work.
func (rt *Runtime) setExpandedFrom(n *node, open bool) {
	a := n.div.acc
	if a.expanded == open {
		return
	}
	if a.setExpanded != nil {
		a.setExpanded(open)
		return
	}
	rt.clickNode(n)
}

// scrollContainer returns the nearest scroll container above n, or nil.
func scrollContainer(n *node) *node {
	for p := n.parent; p != nil; p = p.parent {
		if isScroll(p) {
			return p
		}
	}
	return nil
}

// scrollIntoView scrolls the nearest scroll container the least amount that
// shows n completely. The offset of n is the sum of the layout offsets from n up
// to the container, which the layout gives relative to each parent.
func (rt *Runtime) scrollIntoView(n *node) {
	c := scrollContainer(n)
	if c == nil || c.st == nil {
		return
	}
	var top float32
	for p := n; p != nil && p != c; p = p.parent {
		top += p.lay.Y
	}
	revealRange(c, top, top+n.lay.H)
	rt.needPaint = true
	rt.scrolled = true
}

// scrollBy moves a scroll container by fractions of its view, positive down and
// right, clamped to its range.
func (rt *Runtime) scrollBy(n *node, fx, fy float32) {
	st := n.st
	if st == nil || !isScroll(n) {
		return
	}
	maxX := max(n.lay.ContentW-n.lay.W, 0)
	maxY := max(n.lay.ContentH-n.lay.H, 0)
	st.targetX = min(max(st.targetX+fx*n.lay.W, 0), maxX)
	st.targetY = min(max(st.targetY+fy*n.lay.H, 0), maxY)
	rt.needPaint = true
	rt.scrolled = true
}

// scrollToPercent moves a scroll container to a percent of its range on each
// axis. A negative percent leaves that axis where it is.
func (rt *Runtime) scrollToPercent(n *node, px, py float64) {
	st := n.st
	if st == nil || !isScroll(n) {
		return
	}
	maxX := max(n.lay.ContentW-n.lay.W, 0)
	maxY := max(n.lay.ContentH-n.lay.H, 0)
	if px >= 0 {
		st.targetX = min(max(float32(px/100)*maxX, 0), maxX)
	}
	if py >= 0 {
		st.targetY = min(max(float32(py/100)*maxY, 0), maxY)
	}
	rt.needPaint = true
	rt.scrolled = true
}

// accID converts a node's ID to an access ID. The top bit keeps element IDs
// clear of access.RootID.
func accID(id uint64) access.ID { return access.ID(id | 1<<63) }

// linkID is the access ID of the seg-th segment of a rich text node.
func linkID(id uint64, seg int) access.ID { return accID(mix(id, uint64(seg)+1)) }

// accessBuilder collects the nodes of one tree.
type accessBuilder struct {
	rt      *Runtime
	byID    map[access.ID]*access.Node
	targets map[access.ID]accessTarget
	focus   access.ID // the node that carries focus, found before the walk
	keys    map[string]access.ID
	rels    []accessRelation
	// issues holds the problem found with an element while it was built, one
	// message per element, reported with the other warnings.
	issues map[access.ID]string
}

// accessRelation is a LabelledBy or DescribedBy whose keys are resolved once
// every element is built.
type accessRelation struct {
	node        *access.Node
	labelledBy  []string
	describedBy []string
}

// buildAccess builds the tree from the runtime's nodes and the targets of its
// actions. It warns about elements that need a name, when warnings are on.
func (rt *Runtime) buildAccess() (*access.Tree, map[access.ID]accessTarget) {
	b := &accessBuilder{rt: rt, byID: map[access.ID]*access.Node{}, targets: map[access.ID]accessTarget{},
		keys: map[string]access.ID{}, issues: map[access.ID]string{}}
	b.focus = b.focusTarget()
	root := &access.Node{ID: access.RootID, Role: access.RoleWindow}
	root.Bounds = access.Rect{W: round(rt.viewW * rt.scale), H: round(rt.viewH * rt.scale)}
	b.byID[access.RootID] = root
	if rt.tree != nil {
		for _, id := range b.emit(rt.tree, false) {
			b.byID[id].Parent = access.RootID
			root.Children = append(root.Children, id)
		}
	}
	for _, r := range b.rels {
		r.node.LabelledBy = b.resolve(r.node, r.labelledBy, "LabelledBy")
		r.node.DescribedBy = b.resolve(r.node, r.describedBy, "DescribedBy")
	}
	nodes := make([]*access.Node, 0, len(b.byID))
	for _, n := range b.byID {
		nodes = append(nodes, n)
	}
	t := access.NewTree(b.focus, nodes)
	if rt.acc != nil && rt.acc.warn {
		t.Walk(func(n *access.Node) bool { rt.warnNode(n, t, b.issues[n.ID]); return true })
	}
	return t, b.targets
}

// resolve returns the IDs of the elements with the keys. A key that no exposed
// element has is an issue of the element that names it.
func (b *accessBuilder) resolve(owner *access.Node, keys []string, rel string) []access.ID {
	var ids []access.ID
	for _, k := range keys {
		id, ok := b.keys[k]
		if !ok {
			b.issue(owner.ID, fmt.Sprintf("%s key %q matches no exposed element", rel, k))
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// issue records a problem with the element id, unless it has one already.
func (b *accessBuilder) issue(id access.ID, msg string) {
	if _, ok := b.issues[id]; !ok {
		b.issues[id] = msg
	}
}

// focusTarget returns the access ID of the node that should carry focus: the
// focused element if it is exposed, otherwise its nearest exposed ancestor, or
// the window when there is none. Focus inside a hidden subtree, or with nothing
// focused, is zero.
func (b *accessBuilder) focusTarget() access.ID {
	n := b.rt.byID[b.rt.focusID]
	if n == nil {
		return 0
	}
	for ; n != nil; n = n.parent {
		if n.flags&flagNoDisplay != 0 || (n.div != nil && n.div.acc != nil && n.div.acc.hidden) {
			return 0
		}
		if n.kind == kindEditor || exposed(n) {
			// A menu that has focus passes it to the row it highlights, so that
			// moving the highlight with the keys moves the focus the screen reader
			// announces.
			if row := highlightedRow(n); row != nil {
				return accID(row.id)
			}
			return accID(n.id)
		}
	}
	return access.RootID
}

// emit returns the access IDs that the node contributes to its parent's
// children: none for an element left out, its own ID for one exposed, or the
// IDs of its children when it is a layout container without semantics, which
// is flattened into its parent. absorbed is set inside a control whose name
// already holds the text under it, so that text is not reported twice.
func (b *accessBuilder) emit(n *node, absorbed bool) []access.ID {
	switch n.kind {
	case kindText:
		return b.emitText(n, absorbed)
	case kindIcon:
		// An icon is decorative. Inside a button it is covered by the button's
		// name, and it has no meaning on its own.
		return nil
	case kindImage:
		return b.emitImage(n)
	case kindEditor:
		return b.emitField(n)
	}
	d := n.div
	if d == nil || d.noDisplay || (d.acc != nil && d.acc.hidden) {
		return nil
	}
	if d.acc != nil && d.acc.inert {
		var kids []access.ID
		for _, c := range n.children {
			kids = append(kids, b.emit(c, true)...)
		}
		return kids
	}
	role, ok := accessRole(n)
	absorbs := ok && absorbsText(role)
	var kids []access.ID
	for _, c := range n.children {
		kids = append(kids, b.emit(c, absorbed || absorbs)...)
	}
	if !ok {
		return kids
	}
	id := accID(n.id)
	an := &access.Node{ID: id, Role: role, Key: n.key, Children: kids}
	an.Name = b.name(n, absorbs)
	an.Description = d.acc.descriptionOr(d.tooltip, an.Name)
	an.Bounds = b.bounds(n)
	an.States = b.states(n, id, role)
	if role == access.RoleMenu && b.hasRadio(kids) {
		an.States |= access.StateSelectionContainer
	}
	an.Actions = b.actions(n, role, an.States)
	an.Scroll = b.scroll(n)
	if d.acc != nil && d.acc.setSize > 0 {
		an.PosInSet, an.SetSize = d.acc.setPos, d.acc.setSize
	}
	if d.acc != nil {
		an.Level = d.acc.level
		an.Heading = d.acc.heading
		an.Value = d.acc.value
		an.Range = d.acc.rng
		an.GridRows, an.GridColumns = d.acc.gridRows, d.acc.gridCols
		an.GridRow, an.GridColumn = d.acc.gridRow, d.acc.gridCol
		an.ItemStatus = d.acc.status
	}
	b.checkState(an, d.acc, role)
	b.relate(an, d.acc)
	b.add(an, n, kids)
	return []access.ID{id}
}

// hasRadio reports whether one of the nodes is a radio item of a menu.
func (b *accessBuilder) hasRadio(ids []access.ID) bool {
	for _, id := range ids {
		if n := b.byID[id]; n != nil && n.Role == access.RoleMenuItemRadio {
			return true
		}
	}
	return false
}

// checkState records an issue for a check box, switch or radio button whose
// state was never set, since it would be reported as off without a reason.
func (b *accessBuilder) checkState(an *access.Node, a *accessInfo, role access.Role) {
	if !toggleRole(role) && !radioRole(role) {
		return
	}
	if a != nil && a.checkedSet {
		return
	}
	b.issue(an.ID, fmt.Sprintf("%s has no checked state; call Checked on the element", role))
}

// relate records the LabelledBy and DescribedBy keys of an element, to resolve
// once every element is built.
func (b *accessBuilder) relate(an *access.Node, a *accessInfo) {
	if a != nil && (len(a.labelledBy) > 0 || len(a.describedBy) > 0) {
		b.rels = append(b.rels, accessRelation{node: an, labelledBy: a.labelledBy, describedBy: a.describedBy})
	}
}

// descriptionOr returns the element's description, or the tooltip when that is
// not the name already.
func (a *accessInfo) descriptionOr(tooltip, name string) string {
	if a != nil && a.description != "" {
		return a.description
	}
	if tooltip != "" && tooltip != name {
		return tooltip
	}
	return ""
}

// add records an exposed node and its targets, and makes it the parent of its
// children.
func (b *accessBuilder) add(an *access.Node, n *node, kids []access.ID) {
	for _, k := range kids {
		b.byID[k].Parent = an.ID
	}
	b.byID[an.ID] = an
	if an.Key != "" {
		if _, ok := b.keys[an.Key]; !ok {
			b.keys[an.Key] = an.ID
		}
	}
	// Only what a user or an assistive technology can operate has a target.
	operable := an.Role.Interactive() && !an.States.Has(access.StateDisabled)
	if len(an.Actions) > 0 || operable || n.kind == kindEditor || (n.div != nil && n.div.focusable) {
		b.targets[an.ID] = accessTarget{node: n, seg: -1}
	}
}

// exposed reports whether a div node is in the access tree.
func exposed(n *node) bool {
	_, ok := accessRole(n)
	return ok
}

// accessRole returns the role of a div node and whether it is exposed. A node
// is exposed when it has a role, a name, or is a scroll container; a clickable
// element is a button unless it says otherwise. Focusable elements that have no
// role or name are flattened, so focus on them is reported on their nearest
// exposed ancestor.
func accessRole(n *node) (access.Role, bool) {
	d := n.div
	if d == nil {
		return access.RoleUnknown, false
	}
	if d.acc != nil && d.acc.inert {
		return access.RoleUnknown, false
	}
	if d.acc != nil && d.acc.role != access.RoleUnknown {
		return d.acc.role, true
	}
	if d.h.onClick != nil {
		return access.RoleButton, true
	}
	if d.acc != nil && d.acc.name != "" {
		return access.RoleGroup, true
	}
	if isScroll(n) {
		return access.RoleScrollArea, true
	}
	return access.RoleUnknown, false
}

// isScroll reports whether a node is a scroll container: its style scrolls on
// at least one axis.
func isScroll(n *node) bool {
	return n.div != nil && (n.div.style.OverflowX == OverflowScroll || n.div.style.OverflowY == OverflowScroll)
}

// toggleRole reports whether a role has an on or off state of its own.
func toggleRole(r access.Role) bool {
	return r == access.RoleCheckBox || r == access.RoleSwitch || r == access.RoleMenuItemCheck
}

// radioRole reports whether a role is one choice of a group, with a selected state.
func radioRole(r access.Role) bool {
	return r == access.RoleRadioButton || r == access.RoleMenuItemRadio
}

// isSelected reports whether a selection item is chosen. A radio button or radio
// menu item is chosen when it is checked.
func isSelected(role access.Role, a *accessInfo) bool {
	return a.selected || (radioRole(role) && a.checked)
}

// absorbsText reports whether a control's name is the text inside it.
func absorbsText(r access.Role) bool {
	switch r {
	case access.RoleButton, access.RoleCheckBox, access.RoleSwitch, access.RoleRadioButton,
		access.RoleLink, access.RoleMenuItem, access.RoleMenuItemCheck, access.RoleMenuItemRadio,
		access.RoleListItem, access.RoleTab, access.RoleTreeItem, access.RoleTooltip, access.RoleHeading,
		access.RoleDataItem, access.RoleHeaderItem:
		return true
	}
	return false
}

// name returns the accessible name of an exposed div. The explicit label wins,
// then the text the element contains (for a control whose name is its text),
// then its tooltip.
func (b *accessBuilder) name(n *node, absorbs bool) string {
	s := b.ownName(n, absorbs)
	if n.div.acc == nil || n.div.acc.badge == "" {
		return s
	}
	// An attached badge adds to the name, as in "Projects, 6 items".
	if s == "" {
		return n.div.acc.badge
	}
	return s + ", " + n.div.acc.badge
}

// ownName is the name of an exposed div before any attached badge is added.
func (b *accessBuilder) ownName(n *node, absorbs bool) string {
	if n.div.acc != nil && n.div.acc.name != "" {
		return n.div.acc.name
	}
	if absorbs {
		var parts []string
		collectText(n, &parts)
		if s := strings.Join(parts, " "); s != "" {
			return s
		}
	}
	return n.div.tooltip
}

// collectText appends the text under n, in order. Overlays are left out: a
// tooltip or a menu that hangs from a control is not part of its name.
func collectText(n *node, parts *[]string) {
	for _, c := range n.children {
		if c.flags&flagOverlay != 0 {
			continue
		}
		if c.kind == kindText {
			if s := strings.TrimSpace(c.textEl.text); s != "" {
				*parts = append(*parts, s)
			}
			continue
		}
		if c.flags&flagNoDisplay != 0 || (c.div != nil && c.div.acc != nil && c.div.acc.hidden) {
			continue
		}
		collectText(c, parts)
	}
}

// states returns the states of an exposed div.
func (b *accessBuilder) states(n *node, id access.ID, role access.Role) access.State {
	var s access.State
	d := n.div
	a := d.acc
	if d.focusable || (a != nil && a.focusInside) {
		s |= access.StateFocusable
	}
	if a != nil && a.disabled {
		s |= access.StateDisabled
	}
	if a != nil && a.checked {
		s |= access.StateChecked
	}
	if a != nil && a.mixed {
		s |= access.StateMixed
	}
	if a != nil && a.invalid {
		s |= access.StateInvalid
	}
	if a != nil && a.readOnly {
		s |= access.StateReadOnly
	}
	if id == b.focus {
		s |= access.StateFocused
	}
	s |= b.offscreen(n)
	if toggleRole(role) || (a != nil && a.toggle && !radioRole(role)) {
		s |= access.StateToggle
	}
	if a != nil && a.expandable {
		s |= access.StateExpandable
		if a.expanded {
			s |= access.StateExpanded
		}
	}
	if radioRole(role) || (a != nil && a.selectable) {
		s |= access.StateSelectable
		if a != nil && isSelected(role, a) {
			s |= access.StateSelected
		}
	}
	if role == access.RoleTabList || (a != nil && a.container) {
		s |= access.StateSelectionContainer
	}
	if a != nil && a.multiSelect {
		s |= access.StateMultiSelectable
	}
	return s
}

// offscreen returns StateOffscreen when a node was not drawn in the last paint:
// it is clipped out of view by a scroll container, or outside the window.
func (b *accessBuilder) offscreen(n *node) access.State {
	if n.paintGen != b.rt.paintGen {
		return access.StateOffscreen
	}
	return 0
}

// actions returns the actions a div node supports. A disabled node supports
// none. A node inside a scroll container can be scrolled into view when it is a
// list row, a control, or can take focus.
func (b *accessBuilder) actions(n *node, role access.Role, st access.State) []access.Action {
	d := n.div
	if d.acc != nil && d.acc.disabled {
		return nil
	}
	var out []access.Action
	// A tab is chosen with Select, not invoked, so it has no Invoke pattern
	// (Microsoft's Tab control rules; see the Tabs widget).
	if d.h.onClick != nil && role != access.RoleTab {
		out = append(out, access.ActionClick)
	}
	if st.Has(access.StateFocusable) {
		out = append(out, access.ActionFocus)
	}
	if st.Has(access.StateToggle) {
		out = append(out, access.ActionToggle)
	}
	if st.Has(access.StateExpandable) {
		out = append(out, access.ActionExpand, access.ActionCollapse)
	}
	if st.Has(access.StateSelectable) {
		out = append(out, access.ActionSelect)
	}
	if d.acc != nil && d.acc.setRange != nil {
		out = append(out, access.ActionSetRangeValue)
	}
	if isScroll(n) {
		out = append(out, access.ActionScroll, access.ActionSetScrollPercent)
	}
	if scrollContainer(n) != nil && (role == access.RoleListItem || d.focusable || role.Interactive()) {
		out = append(out, access.ActionScrollIntoView)
	}
	return out
}

// scroll returns the scroll state of a scroll container, and the zero value for
// any other node. The position is the one the container is moving to, so that
// a scroll in progress reads as its target.
func (b *accessBuilder) scroll(n *node) access.Scroll {
	if !isScroll(n) {
		return access.Scroll{}
	}
	maxX := max(n.lay.ContentW-n.lay.W, 0)
	maxY := max(n.lay.ContentH-n.lay.H, 0)
	s := access.Scroll{
		Container: true,
		CanX:      n.div.style.OverflowX == OverflowScroll && maxX > 0,
		CanY:      n.div.style.OverflowY == OverflowScroll && maxY > 0,
		ViewX:     viewPercent(n.lay.W, n.lay.ContentW),
		ViewY:     viewPercent(n.lay.H, n.lay.ContentH),
	}
	if st := n.st; st != nil {
		s.X = percentOf(st.targetX, maxX)
		s.Y = percentOf(st.targetY, maxY)
	}
	return s
}

// percentOf returns part as a percent of whole, from 0 to 100. A whole that is
// empty gives 0.
func percentOf(part, whole float32) float64 {
	if whole <= 0 {
		return 0
	}
	return math.Min(math.Max(float64(part)/float64(whole)*100, 0), 100)
}

// viewPercent returns the visible part of the content, in percent. Content that
// is not measured yet counts as all visible.
func viewPercent(view, content float32) float64 {
	if content <= 0 {
		return 100
	}
	return percentOf(view, content)
}

// bounds returns a node's rectangle in physical client pixels. A node that was
// never drawn has no meaningful place, so it reports an empty rectangle.
func (b *accessBuilder) bounds(n *node) access.Rect {
	if n.paintGen == 0 {
		return access.Rect{}
	}
	r := b.rt.px(n.abs)
	return access.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

// emitText returns a static text node for a text element that is not inside a
// control. Links in rich text become child nodes of it.
func (b *accessBuilder) emitText(n *node, absorbed bool) []access.ID {
	s := strings.TrimSpace(n.textEl.text)
	if absorbed || s == "" {
		return nil
	}
	id := accID(n.id)
	an := &access.Node{ID: id, Role: access.RoleStaticText, Name: s, Key: n.key, Bounds: b.bounds(n)}
	an.States = b.offscreen(n)
	if n.rich != nil {
		for i, seg := range n.rich.segs {
			if seg.onClick == nil {
				continue
			}
			lid := linkID(n.id, i)
			link := &access.Node{ID: lid, Role: access.RoleLink, Name: seg.text, Bounds: an.Bounds, Parent: id,
				Actions: []access.Action{access.ActionClick}}
			link.States = an.States
			an.Children = append(an.Children, lid)
			b.byID[lid] = link
			b.targets[lid] = accessTarget{node: n, seg: i}
		}
	}
	b.add(an, n, nil)
	return []access.ID{id}
}

// emitImage returns an image node, named by its alternative text. An image
// without alternative text is still reported, so that the developer sees it.
func (b *accessBuilder) emitImage(n *node) []access.ID {
	id := accID(n.id)
	an := &access.Node{ID: id, Role: access.RoleImage, Name: n.img.alt, Key: n.key, Bounds: b.bounds(n)}
	an.States = b.offscreen(n)
	b.add(an, n, nil)
	return []access.ID{id}
}

// emitField returns a text field node for an editor. Its value is the text it
// holds, and it can be set by an assistive technology.
func (b *accessBuilder) emitField(n *node) []access.ID {
	id := accID(n.id)
	role := access.RoleTextField
	if n.editor.multiline {
		role = access.RoleTextArea
	}
	v := n.editor.value.Peek()
	an := &access.Node{ID: id, Role: role, Name: n.editor.name, Key: n.key, Bounds: b.bounds(n),
		Value:   v,
		Actions: []access.Action{access.ActionFocus, access.ActionSetValue, access.ActionSetTextSelection}}
	if n.st != nil && n.st.edit != nil {
		n.st.edit.sync(v)
		an.Text = b.textInfo(n, n.st.edit, v)
	}
	an.States = access.StateFocusable | b.offscreen(n)
	if id == b.focus {
		an.States |= access.StateFocused
	}
	if n.editor.multiline {
		an.States |= access.StateMultiLine
	}
	if an.Text != nil {
		an.Text.ReadOnly = an.States.Has(access.StateReadOnly)
	}
	b.relate(an, n.div.acc)
	b.add(an, n, nil)
	return []access.ID{id}
}

// warnNode reports an element that a user operates without a name, an image
// without alternative text, a selection item outside a selection container, or
// a problem recorded while the element was built (issue, when not empty). Each
// element is reported once.
func (rt *Runtime) warnNode(an *access.Node, t *access.Tree, issue string) {
	var msg string
	switch {
	case issue != "":
		msg = issue
	case an.Role == access.RoleImage && an.Name == "":
		msg = "image has no alternative text; set Alt on the Image"
	case an.Role.Interactive() && an.Name == "":
		msg = fmt.Sprintf("%s has no accessible name; set Label on the element or give it visible text", an.Role)
	case an.States.Has(access.StateSelectable) && !inSelectionContainer(t, an):
		msg = fmt.Sprintf("%s is a selection item outside a selection container; call SelectionContainer on the group that holds it", an.Role)
	default:
		return
	}
	if an.Key != "" {
		msg += fmt.Sprintf(" (key %q)", an.Key)
	}
	rt.warnOnce(an.ID, msg)
}

// warnOnce reports msg as the accessibility warning for the element id, unless
// that element was reported already.
func (rt *Runtime) warnOnce(id access.ID, msg string) {
	a := rt.acc
	if a == nil || a.warned[uint64(id)] {
		return
	}
	a.warned[uint64(id)] = true
	if rt.warn != nil {
		rt.warn("accessibility: " + msg)
	}
}

// inSelectionContainer reports whether a selection item has a selection
// container above it in the tree.
func inSelectionContainer(t *access.Tree, n *access.Node) bool {
	for id := n.Parent; id != 0; {
		p := t.Node(id)
		if p == nil {
			return false
		}
		if p.States.Has(access.StateSelectionContainer) {
			return true
		}
		id = p.Parent
	}
	return false
}

// focusChild moves keyboard focus to the first focusable child of n, for an
// element that takes focus for its child (see accessInfo.focusInside).
func (rt *Runtime) focusChild(n *node) {
	for _, c := range n.children {
		if c.flags&flagNoDisplay == 0 && c.div != nil && c.div.focusable {
			rt.setFocus(c.id)
			return
		}
	}
}

// highlightedRow returns the row that a menu, a list or a tree highlights, when n
// is one: the deepest highlighted row in its subtree, so that a submenu's row wins
// over the row that opened it. It returns nil for anything else.
func highlightedRow(n *node) *node {
	if n.div == nil || n.div.acc == nil {
		return nil
	}
	switch n.div.acc.role {
	case access.RoleMenu, access.RoleList, access.RoleTree, access.RoleDataGrid:
	default:
		return nil
	}
	var found *node
	var walk func(*node)
	walk = func(c *node) {
		for _, k := range c.children {
			if k.div != nil && k.div.acc != nil && k.div.acc.highlighted {
				found = k
			}
			walk(k)
		}
	}
	walk(n)
	return found
}
