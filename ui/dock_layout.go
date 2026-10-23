package ui

import (
	"fmt"
	"math"
	"slices"
)

// A DockLayout is the arrangement of the panels of a Dock: a tree of splits and tab
// groups, and the groups that float in windows of their own. It is a value. Copy it,
// save it, and change it with the methods below, which return a new layout and never
// change their argument. It marshals to JSON with encoding/json, so an app saves it
// with its settings and loads it back as it was. A panel ID in a loaded layout that the
// Dock does not know is dropped, and a known panel that the layout does not hold can be
// shown again with DockLayout.Show.

// DockZone is where a panel lands when it is moved in a Dock. DockCenter adds it to a
// group as a tab. The four sides give it a group of its own, beside the group it is
// dropped on (or along an outer edge of the whole dock, when no group is named).
type DockZone uint8

const (
	DockCenter DockZone = iota // the panel joins the group as a tab
	DockLeft                   // the panel gets a group of its own, to the left
	DockRight                  // the panel gets a group of its own, to the right
	DockTop                    // the panel gets a group of its own, above
	DockBottom                 // the panel gets a group of its own, below
)

// vertical reports whether the zone stacks its group with the one it is beside.
func (z DockZone) vertical() bool { return z == DockTop || z == DockBottom }

// after reports whether the new group comes after the one it is beside: to the right,
// or below.
func (z DockZone) after() bool { return z == DockRight || z == DockBottom }

// DockLayout is the layout of a Dock: the root node of the tree of splits and groups,
// and the floating groups. A zero DockLayout is an empty dock.
type DockLayout struct {
	// Root is the root of the tree of splits and groups.
	Root DockNode `json:"root"`
	// Floating lists the groups that float, each in its own window.
	Floating []DockFloat `json:"floating,omitempty"`
}

// DockNode is a node of the tree of a DockLayout. A node with Children is a split: its
// children sit side by side, or one above the other when Vertical is set, and Sizes
// holds the share of the split's length that each child takes, one per child. A node
// without children is a tab group: Panels lists the panels it holds, in tab order, and
// Active is the one it shows. The sizes need not sum to 1; they are normalized when the
// layout is used.
type DockNode struct {
	// Vertical reports whether a split stacks its children from top to bottom.
	Vertical bool `json:"vertical,omitempty"`
	// Sizes are the share of the space that each child takes.
	Sizes []float64 `json:"sizes,omitempty"`
	// Children are the nodes of a split.
	Children []DockNode `json:"children,omitempty"`
	// Panels are the IDs of the panels in a tab group.
	Panels []string `json:"panels,omitempty"`
	// Active is the ID of the selected panel of a tab group.
	Active string `json:"active,omitempty"`
}

// DockFloat is a tab group that floats in a window of its own. ID names the window. X
// and Y are the window's position and Width and Height its size, in logical pixels: they
// are the position it opens at, and they are recorded when the window closes.
type DockFloat struct {
	// ID identifies the floating group.
	ID string `json:"id"`
	// Group is the tree of splits and tab groups of the floating window.
	Group DockNode `json:"group"`
	// X is the horizontal position of the window, in logical screen pixels.
	X int `json:"x"`
	// Y is the vertical position of the window, in logical screen pixels.
	Y int `json:"y"`
	// Width is the width of the window's content, in logical pixels.
	Width int `json:"width"`
	// Height is the height of the window's content, in logical pixels.
	Height int `json:"height"`
	// Home is where the group's first panel was docked before it floated, so that
	// docking it back returns it there. Nil docks it along the right edge.
	Home *DockHome `json:"home,omitempty"`
}

// DockHome is the place a panel was docked at, by a panel beside it: a tab of
// Panel's group, at Index, when Zone is DockCenter, or a group of its own on the
// Zone side of Panel's group.
type DockHome struct {
	Panel string   `json:"panel"`
	Zone  DockZone `json:"zone"`
	Index int      `json:"index,omitempty"`
}

// isSplit reports whether n is a split rather than a tab group.
func (n DockNode) isSplit() bool { return len(n.Children) > 0 }

// empty reports whether n is a tab group with no panels, which normalization removes.
func (n DockNode) empty() bool { return !n.isSplit() && len(n.Panels) == 0 }

// clone returns a copy of n that shares no slices with it.
func (n DockNode) clone() DockNode {
	n.Sizes = slices.Clone(n.Sizes)
	n.Panels = slices.Clone(n.Panels)
	if n.Children != nil {
		kids := make([]DockNode, len(n.Children))
		for i, c := range n.Children {
			kids[i] = c.clone()
		}
		n.Children = kids
	}
	return n
}

// clone returns a copy of l that shares no slices with it.
func (l DockLayout) clone() DockLayout {
	l.Root = l.Root.clone()
	if l.Floating != nil {
		fl := make([]DockFloat, len(l.Floating))
		for i, f := range l.Floating {
			f.Group = f.Group.clone()
			if f.Home != nil {
				home := *f.Home
				f.Home = &home
			}
			fl[i] = f
		}
		l.Floating = fl
	}
	return l
}

// Has reports whether the layout holds the panel id, docked or floating.
func (l DockLayout) Has(id string) bool { return l.group(id) != nil }

// group returns the tab group that holds the panel id, in the tree or in a floating
// group, or nil. The pointer is into l, so an edit changes the layout.
func (l *DockLayout) group(id string) *DockNode {
	if g := groupIn(&l.Root, id); g != nil {
		return g
	}
	for i := range l.Floating {
		if g := groupIn(&l.Floating[i].Group, id); g != nil {
			return g
		}
	}
	return nil
}

// groupIn returns the tab group under n that holds the panel id, or nil.
func groupIn(n *DockNode, id string) *DockNode {
	if !n.isSplit() {
		if slices.Contains(n.Panels, id) {
			return n
		}
		return nil
	}
	for i := range n.Children {
		if g := groupIn(&n.Children[i], id); g != nil {
			return g
		}
	}
	return nil
}

// floating reports whether a floating group holds the panel id.
func (l DockLayout) floating(id string) bool {
	for i := range l.Floating {
		if groupIn(&l.Floating[i].Group, id) != nil {
			return true
		}
	}
	return false
}

// remove takes the panel id out of the group, and moves the active panel to a
// neighbouring tab when the active one leaves. The group may be left empty; normalizing
// removes it.
func (n *DockNode) remove(id string) {
	i := slices.Index(n.Panels, id)
	if i < 0 {
		return
	}
	n.Panels = slices.Delete(n.Panels, i, i+1)
	if n.Active == id {
		n.Active = ""
		if len(n.Panels) > 0 {
			n.Active = n.Panels[min(i, len(n.Panels)-1)]
		}
	}
}

// newGroup returns a tab group that holds the single panel id, and shows it.
func newGroup(id string) DockNode { return DockNode{Panels: []string{id}, Active: id} }

// shares returns the sizes of a split of n children, normalized to sum to 1. Sizes that
// are missing or not positive give equal shares.
func shares(sizes []float64, n int) []float64 {
	out := make([]float64, n)
	valid := len(sizes) == n
	total := 0.0
	for _, s := range sizes {
		if !(s > 0) || math.IsInf(s, 1) {
			valid = false
		}
		total += s
	}
	for i := range out {
		if valid {
			out[i] = sizes[i] / total
		} else {
			out[i] = 1 / float64(n)
		}
	}
	return out
}

// normalizeLayout puts l in order, in place, and returns it. Panels that keep rejects,
// and panels that appear more than once, are dropped. Empty groups go, and so do the
// floating groups that have none, and the floating groups with a repeated ID. A split
// with one child is replaced by that child, a split inside a split of the same direction
// is merged into it, and the sizes sum to 1. A nil keep keeps every panel.
func normalizeLayout(l DockLayout, keep func(id string) bool) DockLayout {
	seen := map[string]bool{}
	ok := func(id string) bool {
		if seen[id] || (keep != nil && !keep(id)) {
			return false
		}
		seen[id] = true
		return true
	}
	normalizeNode(&l.Root, ok)
	var floats []DockFloat
	for _, f := range l.Floating {
		normalizeNode(&f.Group, ok)
		if f.Group.empty() || slices.ContainsFunc(floats, func(o DockFloat) bool { return o.ID == f.ID }) {
			continue
		}
		floats = append(floats, f)
	}
	l.Floating = floats
	return l
}

// normalizeNode puts the tree under n in order, in place, as normalizeLayout describes.
// ok reports whether a panel is kept, and records it as seen.
func normalizeNode(n *DockNode, ok func(id string) bool) {
	if !n.isSplit() {
		kept := n.Panels[:0]
		for _, id := range n.Panels {
			if ok(id) {
				kept = append(kept, id)
			}
		}
		n.Panels = kept
		if len(kept) == 0 {
			*n = DockNode{}
			return
		}
		if !slices.Contains(kept, n.Active) {
			n.Active = kept[0]
		}
		return
	}
	sizes := shares(n.Sizes, len(n.Children))
	var kids []DockNode
	var kidSizes []float64
	for i := range n.Children {
		c := n.Children[i]
		normalizeNode(&c, ok)
		if c.empty() {
			continue
		}
		if c.isSplit() && c.Vertical == n.Vertical {
			for j := range c.Children {
				kids = append(kids, c.Children[j])
				kidSizes = append(kidSizes, sizes[i]*c.Sizes[j])
			}
			continue
		}
		kids = append(kids, c)
		kidSizes = append(kidSizes, sizes[i])
	}
	switch len(kids) {
	case 0:
		*n = DockNode{}
	case 1:
		*n = kids[0]
	default:
		n.Children, n.Sizes = kids, shares(kidSizes, len(kids))
	}
}

// wrapBeside returns a split of the group c and the new group g, in the order the zone
// gives, with equal shares.
func wrapBeside(c, g DockNode, zone DockZone) DockNode {
	kids := []DockNode{g, c}
	if zone.after() {
		kids = []DockNode{c, g}
	}
	return DockNode{Vertical: zone.vertical(), Sizes: []float64{0.5, 0.5}, Children: kids}
}

// placeBeside puts the new group g beside the group that holds target, on the side the
// zone gives, in the tree under n. The layout is normalized, so the sizes match the
// children. It reports whether it found the group.
func placeBeside(n *DockNode, target string, zone DockZone, g DockNode) bool {
	for i := range n.Children {
		c := &n.Children[i]
		if c.isSplit() {
			if placeBeside(c, target, zone, g) {
				return true
			}
			continue
		}
		if !slices.Contains(c.Panels, target) {
			continue
		}
		if n.Vertical != zone.vertical() {
			n.Children[i] = wrapBeside(*c, g, zone)
			return true
		}
		// The group takes half of its own share, and the new group the other half.
		half := n.Sizes[i] / 2
		n.Sizes[i] = half
		at := i
		if zone.after() {
			at = i + 1
		}
		n.Children = slices.Insert(n.Children, at, g)
		n.Sizes = slices.Insert(n.Sizes, at, half)
		return true
	}
	return false
}

// placeAtEdge puts the new group g along the outer edge of the whole dock that the zone
// names. A split of the same direction takes it as a child, with the others made
// smaller to fit.
func placeAtEdge(root *DockNode, zone DockZone, g DockNode) {
	switch {
	case root.empty():
		*root = g
	case root.isSplit() && root.Vertical == zone.vertical():
		n := len(root.Children)
		for i := range root.Sizes {
			root.Sizes[i] *= float64(n) / float64(n+1)
		}
		at := 0
		if zone.after() {
			at = n
		}
		root.Children = slices.Insert(root.Children, at, g)
		root.Sizes = slices.Insert(root.Sizes, at, 1/float64(n+1))
	default:
		*root = wrapBeside(*root, g, zone)
	}
}

// firstGroup returns the first tab group of the tree under n, in reading order, or nil.
func firstGroup(n *DockNode) *DockNode {
	if !n.isSplit() {
		if n.empty() {
			return nil
		}
		return n
	}
	for i := range n.Children {
		if g := firstGroup(&n.Children[i]); g != nil {
			return g
		}
	}
	return nil
}

// entryPanel returns the panel that a group entered from the side zone shows: the
// active panel of the first group, in the direction of the move, of the tree under n.
func entryPanel(n *DockNode, side DockZone) string {
	for n.isSplit() {
		k := 0
		if !side.after() {
			k = len(n.Children) - 1
		}
		n = &n.Children[k]
	}
	return n.Active
}

// neighbourPanel returns a panel of the group that lies beside the one that holds id,
// on the side the zone gives. The neighbour is in the nearest split of the right
// direction that has a child on that side, so it is the closest group that way.
func neighbourPanel(n *DockNode, id string, side DockZone) (string, bool) {
	if !n.isSplit() {
		return "", false
	}
	for i := range n.Children {
		if groupIn(&n.Children[i], id) == nil {
			continue
		}
		if p, ok := neighbourPanel(&n.Children[i], id, side); ok {
			return p, true
		}
		if n.Vertical != side.vertical() {
			return "", false
		}
		j := i - 1
		if side.after() {
			j = i + 1
		}
		if j < 0 || j >= len(n.Children) {
			return "", false
		}
		return entryPanel(&n.Children[j], side), true
	}
	return "", false
}

// nextFloatID returns an ID that no floating group of l has, of the form float-1,
// float-2 and so on, the lowest free number first.
func (l DockLayout) nextFloatID() string {
	for k := 1; ; k++ {
		id := fmt.Sprintf("float-%d", k)
		if !slices.ContainsFunc(l.Floating, func(f DockFloat) bool { return f.ID == id }) {
			return id
		}
	}
}

// Clone returns a copy of l, the layout an app starts from. An app's Reset layout
// command sets the layout to it.
func (l DockLayout) Clone() DockLayout { return normalizeLayout(l.clone(), nil) }

// Focus returns l with the panel id shown in its group, so that it is the active
// tab. A panel that l does not hold leaves the layout as it is.
func (l DockLayout) Focus(id string) DockLayout {
	out := normalizeLayout(l.clone(), nil)
	if g := out.group(id); g != nil {
		g.Active = id
	}
	return out
}

// Move returns l with the panel id moved to the zone of the group that holds the
// panel target. The panel leaves its group, and a group left empty goes. A target of ""
// names the whole dock: a side zone then puts the panel along that outer edge, and
// DockCenter does nothing. A target that is the panel itself names the group the panel
// is in, so a side zone splits that group, with the other panels of it.
//
// A floating group holds one group and has no sides, so a side zone onto a panel that
// floats does nothing, and a panel dropped at the center of a floating group joins it.
// A move that cannot happen, such as a panel alone in its group dropped beside it, or a
// target the layout does not hold, leaves the layout as it is.
func (l DockLayout) Move(id, target string, zone DockZone) DockLayout {
	out := normalizeLayout(l.clone(), nil)
	if out.group(id) == nil {
		return out
	}
	if target == "" {
		if zone == DockCenter {
			return out
		}
		out.removePanel(id)
		placeAtEdge(&out.Root, zone, newGroup(id))
		return normalizeLayout(out, nil)
	}
	if target == id {
		other, ok := otherPanel(out.group(id), id)
		if !ok {
			// The panel is alone in its group: it is already where it can be.
			if zone == DockCenter {
				return out.Focus(id)
			}
			return out
		}
		target = other
	}
	if out.group(target) == nil {
		return out
	}
	if zone != DockCenter && groupIn(&out.Root, target) == nil {
		return out
	}
	out.removePanel(id)
	if zone == DockCenter {
		g := out.group(target)
		g.Panels = append(g.Panels, id)
		g.Active = id
		return normalizeLayout(out, nil)
	}
	g := newGroup(id)
	if !out.Root.isSplit() {
		out.Root = wrapBeside(out.Root, g, zone)
	} else {
		placeBeside(&out.Root, target, zone, g)
	}
	return normalizeLayout(out, nil)
}

// otherPanel returns a panel of the group g other than id, the first in tab order.
func otherPanel(g *DockNode, id string) (string, bool) {
	for _, p := range g.Panels {
		if p != id {
			return p, true
		}
	}
	return "", false
}

// removePanel takes the panel id out of its group, wherever it is.
func (l *DockLayout) removePanel(id string) {
	if g := l.group(id); g != nil {
		g.remove(id)
	}
}

// Close returns l without the panel id. A closed panel is shown again with DockLayout.Show.
func (l DockLayout) Close(id string) DockLayout {
	out := normalizeLayout(l.clone(), nil)
	out.removePanel(id)
	return normalizeLayout(out, nil)
}

// Show returns l with the panel id shown. A panel that the layout already holds is
// made the active tab of its group. Otherwise it joins the first group of the dock, in
// reading order, as its active tab, or it fills an empty dock.
func (l DockLayout) Show(id string) DockLayout {
	out := normalizeLayout(l.clone(), nil)
	if out.group(id) != nil {
		return out.Focus(id)
	}
	if out.Root.empty() {
		out.Root = newGroup(id)
		return normalizeLayout(out, nil)
	}
	if g := firstGroup(&out.Root); g != nil {
		g.Panels = append(g.Panels, id)
		g.Active = id
	}
	return normalizeLayout(out, nil)
}

// Float returns l with the panel id moved out of its group into a floating group
// of its own, in a window at x, y of w by h logical pixels. The window's ID is the lowest
// free one of the form float-1. A panel that already floats alone only has its window's
// position and size set. A panel that l does not hold leaves the layout as it is.
func (l DockLayout) Float(id string, x, y, w, h int) DockLayout {
	out := normalizeLayout(l.clone(), nil)
	if out.group(id) == nil {
		return out
	}
	for i := range out.Floating {
		if f := &out.Floating[i]; len(f.Group.Panels) == 1 && f.Group.Panels[0] == id {
			f.X, f.Y, f.Width, f.Height = x, y, w, h
			return out
		}
	}
	home := homeOf(&out.Root, id)
	out.removePanel(id)
	out.Floating = append(out.Floating, DockFloat{ID: out.nextFloatID(), Group: newGroup(id), X: x, Y: y,
		Width: w, Height: h, Home: home})
	return normalizeLayout(out, nil)
}

// Dock returns l with the panel id taken out of its floating group and docked back
// into the dock where it was before it floated: as a tab of the group it left, or as
// a group of its own beside the group it was next to (see DockFloat.Home). When that
// place is gone, because the panel it was next to has gone too, or the panel was not
// the one that floated the group, it docks as a group along the right edge. A panel
// that is docked already, or that does not float, leaves the layout as it is.
func (l DockLayout) Dock(id string) DockLayout {
	out := normalizeLayout(l.clone(), nil)
	if groupIn(&out.Root, id) != nil || !out.floating(id) {
		return out
	}
	var home *DockHome
	for _, f := range out.Floating {
		if len(f.Group.Panels) > 0 && f.Group.Panels[0] == id {
			home = f.Home
		}
	}
	out.removePanel(id)
	if home == nil || !placeHome(&out.Root, id, *home) {
		placeAtEdge(&out.Root, DockRight, newGroup(id))
	}
	return normalizeLayout(out, nil)
}

// homeOf returns the place of the docked panel id in the tree under root: beside a
// tab of its group, or, alone in its group, beside the nearest group of the split it
// is in. A panel alone in the whole dock, or not in it, has none.
func homeOf(root *DockNode, id string) *DockHome {
	g := groupIn(root, id)
	if g == nil {
		return nil
	}
	if len(g.Panels) > 1 {
		i := slices.Index(g.Panels, id)
		other := g.Panels[1]
		if i > 0 {
			other = g.Panels[i-1]
		}
		return &DockHome{Panel: other, Zone: DockCenter, Index: i}
	}
	parent, i := parentOf(root, g)
	if parent == nil {
		return nil
	}
	// Into the sibling after it, if any, or the one before. The panel lies on the
	// other side of the sibling from the way in.
	j, in, zone := i+1, DockRight, DockLeft
	if parent.Vertical {
		in, zone = DockBottom, DockTop
	}
	if j >= len(parent.Children) {
		j, in, zone = i-1, opposite(in), opposite(zone)
	}
	return &DockHome{Panel: entryPanel(&parent.Children[j], in), Zone: zone}
}

// parentOf returns the split under n that has g as a child, and g's place in it.
func parentOf(n, g *DockNode) (*DockNode, int) {
	for i := range n.Children {
		c := &n.Children[i]
		if c == g {
			return n, i
		}
		if p, j := parentOf(c, g); p != nil {
			return p, j
		}
	}
	return nil, 0
}

// opposite returns the side across from z.
func opposite(z DockZone) DockZone {
	switch z {
	case DockLeft:
		return DockRight
	case DockRight:
		return DockLeft
	case DockTop:
		return DockBottom
	case DockBottom:
		return DockTop
	}
	return z
}

// placeHome docks the panel id at home in the tree under root, and reports whether
// home's panel is still there to place it by.
func placeHome(root *DockNode, id string, home DockHome) bool {
	g := groupIn(root, home.Panel)
	if g == nil {
		return false
	}
	if home.Zone == DockCenter {
		g.Panels = slices.Insert(g.Panels, min(max(home.Index, 0), len(g.Panels)), id)
		g.Active = id
		return true
	}
	if !root.isSplit() {
		*root = wrapBeside(*root, newGroup(id), home.Zone)
		return true
	}
	return placeBeside(root, home.Panel, home.Zone, newGroup(id))
}

// MoveSide returns l with the panel id moved into the group beside its own, on the
// side given (DockLeft, DockRight, DockTop or DockBottom), as one of its tabs. When no
// group lies that way, the layout is unchanged. It is the keyboard move of a panel.
func (l DockLayout) MoveSide(id string, side DockZone) DockLayout {
	out := normalizeLayout(l.clone(), nil)
	other, ok := neighbourPanel(&out.Root, id, side)
	if !ok {
		return out
	}
	return out.Move(id, other, DockCenter)
}

// dockHasSide reports whether DockLayout.MoveSide would move the panel id on the side given.
func dockHasSide(l DockLayout, id string, side DockZone) bool {
	n := normalizeLayout(l.clone(), nil)
	_, ok := neighbourPanel(&n.Root, id, side)
	return ok
}

// dockSetFloatBounds returns l with the window of the floating group id at x, y, of
// w by h logical pixels. An unknown ID leaves the layout as it is.
func dockSetFloatBounds(l DockLayout, id string, x, y, w, h int) DockLayout {
	out := l.clone()
	for i := range out.Floating {
		if f := &out.Floating[i]; f.ID == id {
			f.X, f.Y, f.Width, f.Height = x, y, w, h
		}
	}
	return out
}

// dockZoneAt returns the zone of a point (x, y) in a group of w by h logical pixels. The
// nearest side wins when the point is within a quarter of the group from it; elsewhere
// the zone is DockCenter.
func dockZoneAt(x, y, w, h float32) DockZone {
	if w <= 0 || h <= 0 {
		return DockCenter
	}
	best, zone := float32(0.25), DockCenter
	for _, c := range [4]struct {
		d float32
		z DockZone
	}{{x / w, DockLeft}, {1 - x/w, DockRight}, {y / h, DockTop}, {1 - y/h, DockBottom}} {
		if c.d < best {
			best, zone = c.d, c.z
		}
	}
	return zone
}
