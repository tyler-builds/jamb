package ui

import (
	"math"
	"strings"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/internal/scene"
	"github.com/tyler-builds/jamb/internal/text"
)

// SelectAcross makes the Selectable texts inside the element one selection, as
// the paragraphs of a page in a browser are: a drag that starts in one text
// selects across the others to the pointer, in the order of the tree, and
// Shift+click extends the selection into another text. Ctrl+A selects all of
// them, and Ctrl+C copies what is selected, with a blank line between texts, a
// line break between the items of a list and the rows of a table, and a tab
// between texts side by side. A double-click and a triple-click select within
// one text, as they do without the group.
//
// A markdown.View is one such group. A SelectAcross element around others
// joins them, and the texts beside them, into one: the outermost wins.
//
//	ui.Div().SelectAcross().
//		Child(ui.Text(title).Style(titleStyle).Selectable()).
//		Child(markdown.View(body))
func (d *DivElement) SelectAcross() *DivElement { d.selects = selectsAcross; return d }

// selPoint is a place in the text of a group: a byte offset in the text of the
// selectable text node id.
type selPoint struct {
	id  uint64
	off int
}

// groupSelection is the selection of a SelectAcross element (kept in its
// nodeState): from anchor to caret, in the order of its texts.
type groupSelection struct {
	anchor, caret selPoint
	// wordA and wordB are the word a finger's long press selected while its
	// drag selects by words (see wordDrag); zero otherwise.
	wordA, wordB selPoint
}

// memberPos is a place in a group's texts: the index of a text among the
// group's members, and a byte offset in it.
type memberPos struct{ i, off int }

func (p memberPos) before(q memberPos) bool { return p.i < q.i || p.i == q.i && p.off < q.off }

// selectGroup returns the group of the selectable text n: the outermost
// SelectAcross element around it, or nil.
func selectGroup(n *node) *node {
	var g *node
	for p := n.parent; p != nil; p = p.parent {
		if p.div != nil && p.div.selects == selectsAcross {
			g = p
		}
	}
	return g
}

// groupState returns the selection of group g.
func (rt *Runtime) groupState(g *node) *groupSelection {
	if g.st == nil {
		g.st = rt.state(g.id)
	}
	if g.st.group == nil {
		g.st.group = &groupSelection{}
	}
	return g.st.group
}

// groupMembers returns the selectable texts of group g, in the order of the
// tree, which is the order a selection runs in.
func (rt *Runtime) groupMembers(g *node) []*node {
	var out []*node
	var walk func(n *node)
	walk = func(n *node) {
		for _, c := range n.children {
			switch {
			case c.flags&flagNoDisplay != 0:
			case c.textEl != nil && c.st != nil && c.st.sel != nil:
				out = append(out, c)
			default:
				walk(c)
			}
		}
	}
	walk(g)
	return out
}

// memberIndex returns the index of the text id among members, or -1.
func memberIndex(members []*node, id uint64) int {
	for i, m := range members {
		if m.id == id {
			return i
		}
	}
	return -1
}

// resolve returns where p is among members, its offset kept within the text
// as it is now. ok is false when the text is no longer one of them.
func resolve(members []*node, p selPoint) (memberPos, bool) {
	i := memberIndex(members, p.id)
	if i < 0 {
		return memberPos{}, false
	}
	return memberPos{i, clampToRune(members[i].textEl.text, p.off)}, true
}

// groupSpan returns the start and end of the selection of g among members.
// ok is false when the selection is empty, or one of its ends has gone.
func (rt *Runtime) groupSpan(g *node, members []*node) (lo, hi memberPos, ok bool) {
	gs := rt.groupState(g)
	a, okA := resolve(members, gs.anchor)
	c, okC := resolve(members, gs.caret)
	if !okA || !okC || a == c {
		return memberPos{}, memberPos{}, false
	}
	if c.before(a) {
		a, c = c, a
	}
	return a, c, true
}

// memberRange returns the part of members[i]'s text that the selection from
// lo to hi covers, from a to b (a == b when none).
func memberRange(members []*node, i int, lo, hi memberPos) (a, b int) {
	if i < lo.i || i > hi.i {
		return 0, 0
	}
	a, b = 0, len(members[i].textEl.text)
	if i == lo.i {
		a = lo.off
	}
	if i == hi.i {
		b = hi.off
	}
	return a, max(a, b)
}

// applyGroup sets the selection of each text of g to its part of the group's
// selection, which is what paint draws, and asks for a paint.
func (rt *Runtime) applyGroup(g *node, members []*node) {
	rt.setGroupRanges(g, members)
	rt.needPaint = true
	rt.requestFrame()
}

// setGroupRanges sets the selection of each text of g to its part of the
// group's selection.
func (rt *Runtime) setGroupRanges(g *node, members []*node) {
	lo, hi, ok := rt.groupSpan(g, members)
	for i, m := range members {
		s := m.st.sel
		s.text, s.group = m.textEl.text, g.id
		s.anchor, s.caret = 0, 0
		if ok {
			s.anchor, s.caret = memberRange(members, i, lo, hi)
		}
	}
}

// groupMouseDown is the press of the left button at the position p of the
// selectable text n of group g, whose click s has counted.
func (rt *Runtime) groupMouseDown(g, n *node, s *textSelection, p int, e *MouseEvent) {
	gs := rt.groupState(g)
	members := rt.groupMembers(g)
	v := n.textEl.text
	at := func(off int) selPoint { return selPoint{n.id, off} }
	gs.wordA, gs.wordB = selPoint{}, selPoint{}
	switch {
	case rt.wordPress:
		// A finger's long press selects the word, and its drag more words.
		a, b := text.WordAt(v, p)
		gs.anchor, gs.caret = at(a), at(b)
		gs.wordA, gs.wordB = gs.anchor, gs.caret
		s.dragging = true
	case s.clicks == 2:
		a, b := text.WordAt(v, p)
		gs.anchor, gs.caret = at(a), at(b)
	case s.clicks >= 3:
		a, b := lineAt(v, p)
		gs.anchor, gs.caret = at(a), at(b)
	case e.Mods.Has(input.ModShift):
		if _, ok := resolve(members, gs.anchor); !ok {
			gs.anchor = at(0)
		}
		gs.caret = at(p)
	default:
		gs.anchor, gs.caret = at(p), at(p)
		s.dragging = true
	}
	rt.applyGroup(g, members)
}

// groupMouseMove is a move of the pointer while a drag selects in group g:
// the selection runs from where it began to the text under the pointer.
func (rt *Runtime) groupMouseMove(g *node, e *MouseEvent) {
	gs := rt.groupState(g)
	members := rt.groupMembers(g)
	p, ok := rt.groupHit(members, e.X, e.Y)
	if !ok {
		return
	}
	at := func(q memberPos) selPoint { return selPoint{members[q.i].id, q.off} }
	anchor, caret := gs.anchor, at(p)
	if gs.wordA != gs.wordB {
		wa, okA := resolve(members, gs.wordA)
		wb, okB := resolve(members, gs.wordB)
		if okA && okB {
			start, end := text.WordAt(members[p.i].textEl.text, p.off)
			switch {
			case p.before(wa):
				anchor, caret = gs.wordB, at(memberPos{p.i, start})
			case wb.before(p):
				anchor, caret = gs.wordA, at(memberPos{p.i, end})
			default:
				anchor, caret = gs.wordA, gs.wordB
			}
		}
	}
	if anchor == gs.anchor && caret == gs.caret {
		return
	}
	gs.anchor, gs.caret = anchor, caret
	rt.applyGroup(g, members)
	// The text at the caret comes into view, as a text area's caret does while
	// a drag selects in it, so that a drag past the edge of a scroll container
	// scrolls it and the selection grows.
	if c, ok := resolve(members, gs.caret); ok {
		m := members[c.i]
		if l := rt.memberLayout(m); l != nil && len(l.Lines) > 0 {
			r := rt.nodeRect(m)
			x, top := l.Caret(c.off)
			li := l.LineOf(c.off)
			rt.reveal = scene.Rect{X: r.X + m.style.Padding.Left + m.lineOffsetIn(l, li, r.W) + x, Y: r.Y + m.style.Padding.Top + top, W: 1, H: l.LineHeight}
			rt.revealFrom = m
		}
	}
}

// memberLayout returns the layout of the text of n as the last paint drew it,
// or for a text the last paint left out (being out of view), as it would draw it.
func (rt *Runtime) memberLayout(n *node) *text.Layout {
	s := n.st.sel
	if s.layout != nil && n.paintGen == rt.paintGen {
		return s.layout
	}
	if n.textPaint == nil {
		n.textPaint = rt.newPaintedText()
	}
	contentW := max(n.lay.W-n.style.Padding.Left-n.style.Padding.Right, 0)
	s.layout = n.textPaint.layout(rt, n, contentW+0.01)
	return s.layout
}

// groupHit returns the place in a group's texts for the pointer at x, y: in
// the text under it, or the nearest text beside it on its line, or else, in
// the space between texts, the end of the text above it, or the start of the
// first text when there is none above. ok is false for a group with no texts.
func (rt *Runtime) groupHit(members []*node, x, y float32) (memberPos, bool) {
	if len(members) == 0 {
		return memberPos{}, false
	}
	beside, besideDX := -1, float32(math.Inf(1))
	above, aboveY := -1, float32(math.Inf(-1))
	for i, m := range members {
		r := rt.nodeRect(m)
		if y >= r.Y && y < r.Bottom() {
			dx := max(r.X-x, x-r.Right(), 0)
			if dx < besideDX {
				beside, besideDX = i, dx
			}
		} else if r.Bottom() <= y && r.Bottom() >= aboveY {
			above, aboveY = i, r.Bottom()
		}
	}
	switch {
	case beside >= 0:
		m := members[beside]
		return memberPos{beside, hitText(m, rt.memberLayout(m), rt.nodeRect(m), x, y)}, true
	case above >= 0:
		return memberPos{above, len(members[above].textEl.text)}, true
	}
	return memberPos{}, true
}

// groupText returns the text selected in group g, as Copy puts it on the
// clipboard: the selected part of each text, with selectionBreak between them.
func (rt *Runtime) groupText(g *node, members []*node) string {
	lo, hi, ok := rt.groupSpan(g, members)
	if !ok {
		return ""
	}
	var sb strings.Builder
	var prev *node
	var prevR scene.Rect
	for i := lo.i; i <= hi.i; i++ {
		a, b := memberRange(members, i, lo, hi)
		if a == b {
			continue
		}
		m := members[i]
		r := rt.nodeRect(m)
		if prev != nil {
			sb.WriteString(selectionBreak(g, prev, m, prevR, r))
		}
		sb.WriteString(m.textEl.text[a:b])
		prev, prevR = m, r
	}
	return sb.String()
}

// selectionBreak returns what separates the texts a and b of group g, laid
// out at ra and rb, in copied text: a tab between texts side by side, such as
// the cells of a table's row; a line break between the items of a list and
// the rows of a table; and a blank line between other blocks, such as
// paragraphs.
func selectionBreak(g, a, b *node, ra, rb scene.Rect) string {
	if rb.X >= ra.Right()-0.5 && rb.Y < ra.Bottom() && ra.Y < rb.Bottom() {
		return "\t"
	}
	for p := commonAncestor(a, b); p != nil; p = p.parent {
		if p.div != nil && p.div.acc != nil {
			switch p.div.acc.role {
			case access.RoleList, access.RoleListItem, access.RoleDataGrid:
				return "\n"
			}
		}
		if p == g {
			break
		}
	}
	return "\n\n"
}

// commonAncestor returns the innermost node around both a and b.
func commonAncestor(a, b *node) *node {
	for p := a.parent; p != nil; p = p.parent {
		for q := b.parent; q != nil; q = q.parent {
			if p == q {
				return p
			}
		}
	}
	return nil
}

// groupSelectAll selects all the texts of group g.
func (rt *Runtime) groupSelectAll(g *node) {
	members := rt.groupMembers(g)
	if len(members) == 0 {
		return
	}
	gs := rt.groupState(g)
	last := members[len(members)-1]
	gs.anchor, gs.caret = selPoint{members[0].id, 0}, selPoint{last.id, len(last.textEl.text)}
	gs.wordA, gs.wordB = selPoint{}, selPoint{}
	rt.applyGroup(g, members)
}

// groupCollapse clears the selection of group g, leaving its caret where it
// was, from which a Shift+click extends.
func (rt *Runtime) groupCollapse(g *node) {
	gs := rt.groupState(g)
	if gs.anchor == gs.caret && gs.wordA == gs.wordB {
		return
	}
	gs.anchor = gs.caret
	gs.wordA, gs.wordB = selPoint{}, selPoint{}
	rt.applyGroup(g, rt.groupMembers(g))
}

// groupBlur is a text of group g losing focus: the selection clears unless
// focus moves to another text of the group, whose press then decides it.
func (rt *Runtime) groupBlur(g *node) {
	if to := rt.byID[rt.blurTo]; to != nil && to.textEl != nil && to.st != nil && to.st.sel != nil && selectGroup(to) == g {
		return
	}
	rt.groupCollapse(g)
}

// groupMenu is the context menu of a text of group g: Copy, and Select all.
func (rt *Runtime) groupMenu(g *node) []MenuItem {
	members := rt.groupMembers(g)
	_, _, ok := rt.groupSpan(g, members)
	empty := true
	for _, m := range members {
		empty = empty && m.textEl.text == ""
	}
	return []MenuItem{
		{Action: textCopy, Disabled: !ok},
		{Action: textSelectAll, Disabled: empty},
	}
}

// groupContextPress is selectContextPress for the text n of group g: a press
// outside the group's selection clears it, and one inside keeps it.
func (rt *Runtime) groupContextPress(g, n *node, s *textSelection, e *MouseEvent) {
	members := rt.groupMembers(g)
	i := memberIndex(members, n.id)
	if i < 0 {
		return
	}
	p := memberPos{i, rt.selectHit(n, s, e.X, e.Y)}
	if lo, hi, ok := rt.groupSpan(g, members); ok && !p.before(lo) && !hi.before(p) {
		return
	}
	gs := rt.groupState(g)
	gs.anchor, gs.caret = selPoint{n.id, p.off}, selPoint{n.id, p.off}
	rt.applyGroup(g, members)
}

// shownSelectGroup returns the group whose selection paint shows: that of the
// selectable text with the focus, or with the context menu open, or 0.
func (rt *Runtime) shownSelectGroup() uint64 {
	for _, id := range [2]uint64{rt.focusID, rt.menuHeld} {
		if n := rt.byID[id]; n != nil && n.textEl != nil && n.st != nil && n.st.sel != nil {
			if g := selectGroup(n); g != nil {
				return g.id
			}
		}
	}
	return 0
}

// groupReport returns the selection of group g for OnTextSelect: its text,
// the box around all of it and its last line, in logical window coordinates,
// as its texts are laid out. ok is false when there is none, or no part of
// it is drawn in view.
func (rt *Runtime) groupReport(g *node) (txt string, bounds, last scene.Rect, ok bool) {
	members := rt.groupMembers(g)
	lo, hi, ok := rt.groupSpan(g, members)
	if !ok {
		return "", scene.Rect{}, scene.Rect{}, false
	}
	shown, have := false, false
	for i := lo.i; i <= hi.i; i++ {
		a, b := memberRange(members, i, lo, hi)
		if a == b {
			continue
		}
		m := members[i]
		s := m.st.sel
		if m.paintGen == rt.paintGen && s.drawnGen == rt.paintGen && !s.drawn.Intersect(m.clip).Empty() {
			shown = true
		}
		l := rt.memberLayout(m)
		if l == nil {
			continue
		}
		r := rt.nodeRect(m)
		x, y := r.X+m.style.Padding.Left, r.Y+m.style.Padding.Top
		rt.selRects = l.SelectionRects(a, b, rt.selRects[:0])
		for _, sr := range rt.selRects {
			li := 0
			if l.LineHeight > 0 {
				li = min(max(int(sr.Y/l.LineHeight+0.5), 0), len(l.Lines)-1)
			}
			lr := scene.Rect{X: x + m.lineOffsetIn(l, li, r.W) + sr.X, Y: y + sr.Y, W: sr.W, H: sr.H}
			if have {
				bounds = union(bounds, lr)
			} else {
				bounds, have = lr, true
			}
			last = lr
		}
	}
	if !shown || !have {
		return "", scene.Rect{}, scene.Rect{}, false
	}
	return rt.groupText(g, members), bounds, last, true
}
