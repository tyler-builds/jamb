package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// TestPageSlots checks the pages shown for each current page: the first and the
// last page are always shown, the current page with its siblings, and 0 where pages
// are skipped. Up to seven pages are shown in full.
func TestPageSlots(t *testing.T) {
	tests := []struct {
		cur, pages, siblings int
		want                 []int
	}{
		{1, 1, 1, []int{1}},
		{4, 7, 1, []int{1, 2, 3, 4, 5, 6, 7}},
		{1, 8, 1, []int{1, 2, 0, 8}},
		{2, 8, 1, []int{1, 2, 3, 0, 8}},
		{1, 8, 0, []int{1, 0, 8}},
		{3, 8, 0, []int{1, 2, 3, 0, 8}}, // one hidden page is shown instead
		{1, 20, 1, []int{1, 2, 0, 20}},
		{2, 20, 1, []int{1, 2, 3, 0, 20}},
		{3, 20, 1, []int{1, 2, 3, 4, 0, 20}},
		{4, 20, 1, []int{1, 2, 3, 4, 5, 0, 20}},
		{17, 20, 1, []int{1, 0, 16, 17, 18, 19, 20}},
		{5, 20, 1, []int{1, 0, 4, 5, 6, 0, 20}},
		{10, 20, 1, []int{1, 0, 9, 10, 11, 0, 20}},
		{18, 20, 1, []int{1, 0, 17, 18, 19, 20}},
		{19, 20, 1, []int{1, 0, 18, 19, 20}},
		{20, 20, 1, []int{1, 0, 19, 20}},
		{5, 20, -1, []int{1, 0, 5, 0, 20}},
	}
	for _, tt := range tests {
		if got := pageSlots(tt.cur, tt.pages, tt.siblings); !slices.Equal(got, tt.want) {
			t.Errorf("pageSlots(%d, %d, %d) = %v, want %v", tt.cur, tt.pages, tt.siblings, got, tt.want)
		}
	}
}

// Pages are buttons: a click or Enter on one sets the page and runs OnChange. The
// current page is not a button, and Previous and Next are disabled at the ends.
func TestPaginationChoosesPagesAndEndsAreDisabled(t *testing.T) {
	page := NewSignal(4)
	var got []int
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Pagination(page, 20).Key("pager").OnChange(func(n int) { got = append(got, n) }))
	})
	r.clickKey(t, "pager/page:5")
	if page.Peek() != 5 || !slices.Equal(got, []int{5}) {
		t.Fatalf("after a click on page 5: page %d, OnChange %v; want 5 and [5]", page.Peek(), got)
	}
	r.clickKey(t, "pager/previous")
	r.clickKey(t, "pager/next")
	if page.Peek() != 5 || !slices.Equal(got, []int{5, 4, 5}) {
		t.Fatalf("after Previous and Next: page %d, OnChange %v; want 5 and [5 4 5]", page.Peek(), got)
	}
	// Page 5 is current now, so it cannot take focus; page 6 can.
	if keyNode(t, r.rt, "pager/page:5").div.focusable {
		t.Error("the current page can take keyboard focus")
	}
	if !keyNode(t, r.rt, "pager/page:6").div.focusable {
		t.Error("a page that is not current cannot take keyboard focus")
	}

	focusNode(r, keyNode(t, r.rt, "pager/page:6"))
	r.rt.KeyDown(input.KeyEnter, 0, false)
	r.rt.KeyUp(input.KeyEnter, 0)
	r.frame()
	if page.Peek() != 6 {
		t.Errorf("Enter on page 6: page %d, want 6", page.Peek())
	}

	page.Set(1)
	r.frame()
	tr := r.tree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Previous page"), access.RoleButton, "Previous page", access.StateDisabled)
	rc := r.rt.nodeRect(keyNode(t, r.rt, "pager/previous"))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if page.Peek() != 1 {
		t.Errorf("Previous at the first page: page %d, want 1", page.Peek())
	}

	page.Set(20)
	r.frame()
	tr = r.tree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Next page"), access.RoleButton, "Next page", access.StateDisabled)
	rc = r.rt.nodeRect(keyNode(t, r.rt, "pager/next"))
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if page.Peek() != 20 {
		t.Errorf("Next at the last page: page %d, want 20", page.Peek())
	}
}

// The control is a group named by its Label. Pages are named "Page n", the current
// page "Page n, current page" and selected, and the skipped pages are not announced.
func TestPaginationAccessNamesAndStates(t *testing.T) {
	page := NewSignal(4)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Pagination(page, 20).Key("pager").Label("Result pages"))
	})
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleGroup, "Result pages")
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Page 3"), access.RoleButton, "Page 3", 0)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleStaticText, "Page 4, current page"),
		access.RoleStaticText, "Page 4, current page", access.StateSelected)
	accesstest.Absent(t, tr, access.RoleButton, "Page 4")
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Previous page"), access.RoleButton, "Previous page", 0)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Next page"), access.RoleButton, "Next page", 0)
	accesstest.Absent(t, tr, access.RoleStaticText, "...")
	if len(r.warnings) != 0 {
		t.Errorf("accessibility warnings: %v", r.warnings)
	}
}

// Compact shows Previous, "Page 3 of 20" and Next. A page that is past the end is
// held to the last page when it is drawn.
func TestPaginationCompactAndClamping(t *testing.T) {
	page := NewSignal(3)
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Pagination(page, 20).Key("pager").Compact())
	})
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleStaticText, "Page 3 of 20")
	accesstest.Absent(t, tr, access.RoleButton, "Page 2")
	r.clickKey(t, "pager/next")
	accesstest.Find(t, r.tree(), access.RoleStaticText, "Page 4 of 20")

	page.Set(25)
	r.frame()
	tr = r.tree()
	accesstest.Find(t, tr, access.RoleStaticText, "Page 20 of 20")
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Next page"), access.RoleButton, "Next page", access.StateDisabled)
}

// The parts of a pagination control reach the nodes that draw them. The page is 4 of
// 20, so pages 3 and 5 are buttons, page 4 is current, and the ellipses are shown.
func testPaginationParts(t *testing.T, applied map[string]bool) {
	type pageCase struct {
		part  string
		set   func(*PaginationStyle, Style)
		drawn string
		key   string
		act   func(*testing.T, *accessRig)
	}
	cases := []pageCase{
		{part: "Pagination.Bar", set: func(s *PaginationStyle, m Style) { s.Bar = m }, drawn: "Pagination.Bar"},
		{part: "Pagination.Page", set: func(s *PaginationStyle, m Style) { s.Page = m }, drawn: "Pagination.Page", key: "pager/page:3"},
		{part: "Pagination.PageHover", set: func(s *PaginationStyle, m Style) { s.PageHover = m }, drawn: "Pagination.Page", key: "pager/page:3",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "pager/page:3")) }},
		{part: "Pagination.PageCurrent", set: func(s *PaginationStyle, m Style) { s.PageCurrent = m }, drawn: "Pagination.Page", key: "pager/page:4"},
		{part: "Pagination.Nav", set: func(s *PaginationStyle, m Style) { s.Nav = m }, drawn: "Pagination.Nav", key: "pager/next"},
		{part: "Pagination.Ellipsis", set: func(s *PaginationStyle, m Style) { s.Ellipsis = m }, drawn: "Pagination.Ellipsis"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st PaginationStyle
			c.set(&st, Style{Background: markBG})
			page := NewSignal(4)
			r := newAccessRig(func(ctx *Context) Element {
				return Div().Column().Child(Pagination(page, 20).Key("pager").Styles(st))
			})
			if c.act != nil {
				c.act(t, r)
			}
			var found []*node
			for _, n := range partNodes(r.rt, c.drawn) {
				if c.key == "" || n.key == c.key {
					found = append(found, n)
				}
			}
			if len(found) == 0 {
				t.Fatalf("%s: no node drawn for it", c.part)
			}
			for _, n := range found {
				wantBackground(t, n, c.part, markBG)
			}
		})
	}
}

// Under high contrast the current page is Highlight with HighlightText on it, and
// an app's color for the current page is not used.
func TestPaginationCurrentUsesHighlightUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Pagination = PaginationStyle{PageCurrent: Style{Background: markBG}}
	page := NewSignal(4)
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Div().Column().Child(Pagination(page, 20).Key("pager"))
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "Pagination.Page", "pager/page:4")
	if n.style.Background != th.Accent {
		t.Errorf("high contrast: the current page is %v, want Highlight %v", n.style.Background, th.Accent)
	}
	if n.style.TextColor != th.AccentText {
		t.Errorf("high contrast: the current page's text is %v, want HighlightText %v", n.style.TextColor, th.AccentText)
	}
}
