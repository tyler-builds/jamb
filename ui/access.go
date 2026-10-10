package ui

import (
	"fmt"
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
	description string
	hidden      bool
	disabled    bool
	checked     bool
	setPos      int // 1-based position in a set, such as a row of a list
	setSize     int // the number of items in the set
	level       int // the depth of a menu, 1 at the top
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

// Checked sets whether a check box, switch, radio button or checkable menu item
// is on. Assistive technology reads it after the name.
func (d *DivElement) Checked(on bool) *DivElement { d.access().checked = on; return d }

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

// performAccess runs an action requested through the bridge. The bridge may
// call it from any goroutine, so it is posted to the UI thread.
func (rt *Runtime) performAccess(id access.ID, act access.Action) {
	rt.Post(func() { rt.runAccess(id, act) })
}

// runAccess runs an action on the UI thread. An action on a node that has gone
// since the last published tree does nothing.
func (rt *Runtime) runAccess(id access.ID, act access.Action) {
	a := rt.acc
	if a == nil {
		return
	}
	t, ok := a.targets[id]
	if !ok || rt.byID[t.node.id] != t.node {
		return
	}
	n := t.node
	switch act {
	case access.ActionClick:
		if t.seg >= 0 {
			if fn := n.rich.segs[t.seg].onClick; fn != nil {
				fn()
			}
		} else if n.div != nil && n.div.h.onClick != nil {
			n.div.h.onClick()
		}
	case access.ActionFocus:
		if n.div == nil || n.div.focusable || n.kind == kindEditor {
			rt.setFocus(n.id)
		}
	}
	rt.requestFrame()
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
}

// buildAccess builds the tree from the runtime's nodes and the targets of its
// actions. It warns about elements that need a name, when warnings are on.
func (rt *Runtime) buildAccess() (*access.Tree, map[access.ID]accessTarget) {
	b := &accessBuilder{rt: rt, byID: map[access.ID]*access.Node{}, targets: map[access.ID]accessTarget{}}
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
	nodes := make([]*access.Node, 0, len(b.byID))
	for _, n := range b.byID {
		nodes = append(nodes, n)
	}
	t := access.NewTree(b.focus, nodes)
	if rt.acc != nil && rt.acc.warn {
		t.Walk(func(n *access.Node) bool { rt.warnNode(n); return true })
	}
	return t, b.targets
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
		if n.div != nil && n.div.acc != nil && n.div.acc.hidden {
			return 0
		}
		if n.kind == kindEditor || exposed(n) {
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
	if d == nil || (d.acc != nil && d.acc.hidden) {
		return nil
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
	an.States = b.states(n, id)
	an.Actions = b.actions(n)
	if d.acc != nil && d.acc.setSize > 0 {
		an.PosInSet, an.SetSize = d.acc.setPos, d.acc.setSize
	}
	if d.acc != nil {
		an.Level = d.acc.level
	}
	b.add(an, n, kids)
	return []access.ID{id}
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
	// Only what a user or an assistive technology can operate has a target.
	operable := an.Role.Interactive() && !an.States.Has(access.StateDisabled)
	if operable || n.kind == kindEditor || (n.div != nil && n.div.focusable) {
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
	if d.acc != nil && d.acc.role != access.RoleUnknown {
		return d.acc.role, true
	}
	if d.h.onClick != nil {
		return access.RoleButton, true
	}
	if d.acc != nil && d.acc.name != "" {
		return access.RoleGroup, true
	}
	if d.style.OverflowX == OverflowScroll || d.style.OverflowY == OverflowScroll {
		return access.RoleScrollArea, true
	}
	return access.RoleUnknown, false
}

// absorbsText reports whether a control's name is the text inside it.
func absorbsText(r access.Role) bool {
	switch r {
	case access.RoleButton, access.RoleCheckBox, access.RoleSwitch, access.RoleRadioButton,
		access.RoleLink, access.RoleMenuItem, access.RoleMenuItemCheck, access.RoleMenuItemRadio,
		access.RoleListItem, access.RoleTab, access.RoleTooltip:
		return true
	}
	return false
}

// name returns the accessible name of an exposed div. The explicit label wins,
// then the text the element contains (for a control whose name is its text),
// then its tooltip.
func (b *accessBuilder) name(n *node, absorbs bool) string {
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
		if c.div != nil && c.div.acc != nil && c.div.acc.hidden {
			continue
		}
		collectText(c, parts)
	}
}

// states returns the states of an exposed div.
func (b *accessBuilder) states(n *node, id access.ID) access.State {
	var s access.State
	d := n.div
	if d.focusable {
		s |= access.StateFocusable
	}
	if d.acc != nil && d.acc.disabled {
		s |= access.StateDisabled
	}
	if d.acc != nil && d.acc.checked {
		s |= access.StateChecked
	}
	if id == b.focus {
		s |= access.StateFocused
	}
	s |= b.offscreen(n)
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

// actions returns the actions a div node supports.
func (b *accessBuilder) actions(n *node) []access.Action {
	d := n.div
	if d.acc != nil && d.acc.disabled {
		return nil
	}
	var out []access.Action
	if d.h.onClick != nil {
		out = append(out, access.ActionClick)
	}
	if d.focusable {
		out = append(out, access.ActionFocus)
	}
	return out
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
	an := &access.Node{ID: id, Role: access.RoleStaticText, Name: s, Bounds: b.bounds(n)}
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

// emitField returns a text field node for an editor.
func (b *accessBuilder) emitField(n *node) []access.ID {
	id := accID(n.id)
	role := access.RoleTextField
	if n.editor.multiline {
		role = access.RoleTextArea
	}
	an := &access.Node{ID: id, Role: role, Name: n.editor.name, Key: n.key, Bounds: b.bounds(n),
		Actions: []access.Action{access.ActionFocus}}
	an.States = access.StateFocusable | b.offscreen(n)
	if id == b.focus {
		an.States |= access.StateFocused
	}
	if n.editor.multiline {
		an.States |= access.StateMultiLine
	}
	b.add(an, n, nil)
	return []access.ID{id}
}

// warnNode reports an element that a user operates without a name, or an
// image without alternative text. Each element is reported once.
func (rt *Runtime) warnNode(an *access.Node) {
	a := rt.acc
	if a == nil || a.warned[uint64(an.ID)] {
		return
	}
	var msg string
	switch {
	case an.Role == access.RoleImage && an.Name == "":
		msg = "image has no alternative text; set Alt on the Image"
	case an.Role.Interactive() && an.Name == "":
		msg = fmt.Sprintf("%s has no accessible name; set Label on the element or give it visible text", an.Role)
	default:
		return
	}
	a.warned[uint64(an.ID)] = true
	if an.Key != "" {
		msg += fmt.Sprintf(" (key %q)", an.Key)
	}
	if rt.warn != nil {
		rt.warn("accessibility: " + msg)
	}
}
