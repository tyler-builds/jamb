package ui

import (
	"fmt"

	"github.com/tyler-builds/jamb/internal/uihost"
)

// Samples calls visit with each element of the tree, parents before their
// children, then with the elements drawn while their exit plays, and the held
// content drawn while the box that holds it shrinks (see reflowSpec). Elements
// hidden by Display(false) are left out, with what is inside them. It is for
// ui/uitest's motion recording, which calls it after each frame it records;
// nothing else calls it, so a frame costs nothing more for it. An element
// whose ID known reports true gets no Label, which the recording has already.
func (h *runtimeHost) Samples(known func(id uint64) bool, visit func(uihost.Sample)) {
	rt := h.rt()
	var walk func(n *node, parent uint64, index int, sx, sy float32, exiting, looping bool)
	walk = func(n *node, parent uint64, index int, sx, sy float32, exiting, looping bool) {
		if n.flags&(flagNoDisplay|flagReplaced) != 0 {
			return // hidden, or replaced by its match's flight (see Matched)
		}
		s := uihost.Sample{ID: n.id, Parent: parent, Key: n.key, Index: index, Exiting: exiting}
		if n.flags&flagMatched != 0 {
			s.Matched = n.matchedID()
		}
		if n.key == "" && !known(n.id) {
			s.Label = sampleLabel(n)
		}
		if n.st != nil && (n.style.OverflowX == OverflowScroll || n.style.OverflowY == OverflowScroll) {
			s.OwnScrollX, s.OwnScrollY = n.st.scrollX, n.st.scrollY
		}
		looping = looping || n.div != nil && n.div.play != nil && n.div.play.a.looping()
		if a, ok := n.owner.animate(); ok && a.anim.looping() {
			looping = true
		}
		if n.paintGen == rt.paintGen {
			d := rt.drawnStyle(n)
			s.Drawn, s.Style, s.Looping = true, d, looping
			s.ScrollX, s.ScrollY = sx, sy
			vis := n.abs.Intersect(n.clip)
			s.Visible = vis.W > 0 && vis.H > 0
			if n.kind == kindDiv {
				s.Blank = d.Background.A == 0 && d.BorderWidth.isZero() && len(n.style.Shadows) == 0
			}
		}
		visit(s)
		for i, c := range n.children {
			walk(c, n.id, i, sx+s.OwnScrollX, sy+s.OwnScrollY, exiting, looping)
		}
	}
	if rt.tree != nil {
		walk(rt.tree, 0, 0, 0, 0, false, false)
	}
	for _, g := range rt.ghosts {
		var parent uint64
		if g.parent != nil {
			parent = g.parent.id
		}
		walk(g, parent, 0, 0, 0, true, false)
	}
	for _, h := range rt.held {
		if h.gen == rt.paintGen && h.n.parent != nil {
			walk(h.n, h.n.parent.id, 0, 0, 0, true, false)
		}
	}
}

// sampleLabel names an element without a key for a motion finding.
func sampleLabel(n *node) string {
	switch n.kind {
	case kindText:
		t := []rune(n.textEl.text)
		if len(t) > 24 {
			t = append(t[:24], '…')
		}
		return fmt.Sprintf("text %q", string(t))
	case kindIcon:
		return "icon " + n.icon.icon.Name
	case kindImage:
		return "image"
	case kindEditor:
		return "editor"
	}
	return "div"
}

// animate returns the Animate that inst renders, if it is one.
func (inst *componentInstance) animate() (*AnimateElement, bool) {
	if inst == nil {
		return nil, false
	}
	a, ok := inst.el.(*AnimateElement)
	return a, ok
}
