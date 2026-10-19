package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// partDescriptionList is a list of three items, the last with a badge for its
// value, set up by opts and with the instance style st.
func partDescriptionList(st DescriptionListStyle, opts func(*DescriptionListElement)) Element {
	l := DescriptionList().ItemText("Owner", "Dana Lee").ItemText("Region", "West").
		Item("Status", BadgeText("Active")).Styles(st)
	if opts != nil {
		opts(l)
	}
	return l
}

// TestDescriptionListItemsAreGroups checks that each item is a group named by its
// term, with its value inside it.
func TestDescriptionListItemsAreGroups(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return partDescriptionList(DescriptionListStyle{}, nil) })
	tr := r.tree()
	owner := accesstest.Find(t, tr, access.RoleGroup, "Owner")
	if got := accesstest.Children(tr, owner); len(got) != 2 || got[1] != "Dana Lee" {
		t.Errorf("the Owner group holds %v, want the term and Dana Lee", got)
	}
	status := accesstest.Find(t, tr, access.RoleGroup, "Status")
	if got := accesstest.Children(tr, status); len(got) != 2 || got[1] != "Active" {
		t.Errorf("the Status group holds %v, want the term and the badge Active", got)
	}
}

// TestDescriptionListColumns checks that Columns sets the columns of the grid, and
// that the default and a value below 1 give one column. An n of 0 leaves Columns
// uncalled.
func TestDescriptionListColumns(t *testing.T) {
	for _, c := range []struct{ n, want int }{{0, 1}, {-2, 1}, {1, 1}, {3, 3}} {
		r := newAccessRig(func(ctx *Context) Element {
			return partDescriptionList(DescriptionListStyle{}, func(l *DescriptionListElement) {
				if c.n != 0 {
					l.Columns(c.n)
				}
			})
		})
		n := partNode(t, r.rt, "DescriptionList.DescriptionList", "")
		if got := len(n.style.GridColumns); got != c.want {
			t.Errorf("Columns(%d): %d grid columns, want %d", c.n, got, c.want)
		}
	}
}

// TestDescriptionListLayout checks that an item puts its term beside its value
// by default, and above it with Vertical.
func TestDescriptionListLayout(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		r := newAccessRig(func(ctx *Context) Element {
			l := DescriptionList().ItemText("Owner", "Dana Lee")
			if vertical {
				l.Vertical()
			}
			return l
		})
		term := r.rt.nodeRect(partNode(t, r.rt, "DescriptionList.Term", ""))
		value := r.rt.nodeRect(partNode(t, r.rt, "DescriptionList.Value", ""))
		switch {
		case !vertical && !(value.X > term.X && value.Y == term.Y):
			t.Errorf("beside: the value is at %v and the term at %v, want the value to the right", value, term)
		case vertical && !(value.Y > term.Y && value.X == term.X):
			t.Errorf("vertical: the value is at %v and the term at %v, want the value below", value, term)
		}
	}
}

// TestDescriptionListBordered checks that a bordered list is outlined, and that
// its items have a line above every row but the first and before every column but
// the first.
func TestDescriptionListBordered(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return DescriptionList().ItemText("A", "1").ItemText("B", "2").ItemText("C", "3").Columns(2).Bordered()
	})
	if n := partNode(t, r.rt, "DescriptionList.DescriptionList", ""); n.style.BorderWidth != 1 {
		t.Errorf("the list's border is %v, want 1", n.style.BorderWidth)
	}
	want := map[string]Edges{"A": {}, "B": {Left: 1}, "C": {Top: 1}}
	for _, n := range partNodes(r.rt, "DescriptionList.Item") {
		term := n.div.acc.name
		if n.style.BorderEdges != want[term] {
			t.Errorf("item %s: border edges %v, want %v", term, n.style.BorderEdges, want[term])
		}
	}
}

// TestDescriptionListUnderHighContrast checks that an app color does not reach a
// list under high contrast, and that the term is the text color there.
func TestDescriptionListUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.DescriptionList.Term = Style{TextColor: markFG}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return partDescriptionList(DescriptionListStyle{Term: Style{TextColor: markFG}}, nil)
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	for _, n := range partNodes(r.rt, "DescriptionList.Term") {
		if n.style.TextColor != th.Text {
			t.Errorf("high contrast: a term is %v, want the text color %v", n.style.TextColor, th.Text)
		}
	}
}

func testDescriptionListParts(t *testing.T, applied map[string]bool) {
	type listCase struct {
		part string
		set  func(*DescriptionListStyle, Style)
		text bool
	}
	cases := []listCase{
		{part: "DescriptionList.DescriptionList", set: func(s *DescriptionListStyle, m Style) { s.DescriptionList = m }},
		{part: "DescriptionList.Item", set: func(s *DescriptionListStyle, m Style) { s.Item = m }},
		{part: "DescriptionList.Term", set: func(s *DescriptionListStyle, m Style) { s.Term = m }, text: true},
		{part: "DescriptionList.Value", set: func(s *DescriptionListStyle, m Style) { s.Value = m }, text: true},
	}
	// Each part is checked in the default layout and in the vertical, bordered one.
	for _, c := range cases {
		for _, vb := range []bool{false, true} {
			applied[c.part] = true
			t.Run(c.part, func(t *testing.T) {
				var st DescriptionListStyle
				c.set(&st, Style{Background: markBG, TextColor: markFG})
				r := newAccessRig(func(ctx *Context) Element {
					return partDescriptionList(st, func(l *DescriptionListElement) {
						if vb {
							l.Vertical().Bordered().Columns(2)
						}
					})
				})
				for _, n := range partNodes(r.rt, c.part) {
					if c.text {
						wantTextColor(t, n, c.part, markFG)
						continue
					}
					wantBackground(t, n, c.part, markBG)
				}
				if len(partNodes(r.rt, c.part)) == 0 {
					t.Fatalf("%s is not drawn", c.part)
				}
			})
		}
	}
}
