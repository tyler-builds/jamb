package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// clickFound presses and releases the left button at the center of the element
// with key, as the runtime finds it. Unlike clickKey it also finds the elements of
// an open menu, whose keys are not on the trail.
func clickFound(t *testing.T, r *accessRig, key string) {
	t.Helper()
	rc, ok := r.rt.Find(key)
	if !ok {
		t.Fatalf("no element %q", key)
	}
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
}

// sampleCrumbs is a trail of four locations and a current one: Root and Two are
// links that log their label, Three is plain text, and Here is the current location,
// which has an OnClick that must not run.
func sampleCrumbs(log *[]string) []Crumb {
	return []Crumb{
		{Label: "Root", OnClick: func() { *log = append(*log, "Root") }},
		{Label: "Two", OnClick: func() { *log = append(*log, "Two") }},
		{Label: "Three"},
		{Label: "Here", OnClick: func() { *log = append(*log, "Here") }},
	}
}

func TestCrumbSlots(t *testing.T) {
	tests := []struct {
		n, limit int
		want     []int
	}{
		{0, 0, []int{}},
		{3, 0, []int{0, 1, 2}},
		{3, 3, []int{0, 1, 2}},
		{4, 3, []int{0, -1, 2, 3}},
		{5, 3, []int{0, -1, 3, 4}},
		{9, 4, []int{0, -1, 6, 7, 8}},
		{2, 1, []int{0, 1}},
		{5, 1, []int{0, -1, 4}},
	}
	for _, tt := range tests {
		if got := crumbSlots(tt.n, tt.limit); !slices.Equal(got, tt.want) {
			t.Errorf("crumbSlots(%d, %d) = %v, want %v", tt.n, tt.limit, got, tt.want)
		}
	}
}

// Enter and Space choose a link, as a click does. The current location is not a
// link and takes no focus, so a click on it does nothing.
func TestBreadcrumbsLinksChooseOnClickAndKeys(t *testing.T) {
	var log []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Breadcrumbs(sampleCrumbs(&log)...).Key("trail"))
	})
	r.clickKey(t, "trail/crumb:0")
	focusNode(r, keyNode(t, r.rt, "trail/crumb:1"))
	r.rt.KeyDown(input.KeyEnter, 0, false)
	r.rt.KeyUp(input.KeyEnter, 0)
	r.frame()
	focusNode(r, keyNode(t, r.rt, "trail/crumb:0"))
	r.rt.KeyDown(input.KeySpace, 0, false)
	r.rt.KeyUp(input.KeySpace, 0)
	r.frame()
	if want := []string{"Root", "Two", "Root"}; !slices.Equal(log, want) {
		t.Errorf("chosen %v, want %v", log, want)
	}

	cur := keyNode(t, r.rt, "trail/crumb:3")
	if cur.div.focusable {
		t.Error("the current location can take keyboard focus")
	}
	rc := r.rt.nodeRect(cur)
	r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
	if slices.Contains(log, "Here") {
		t.Errorf("a click on the current location ran its OnClick: %v", log)
	}
}

// The trail is a group named by its Label. Links are links, a location without
// OnClick is plain text, and the current location is plain text and selected.
func TestBreadcrumbsAccessRolesAndNames(t *testing.T) {
	var log []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Breadcrumbs(sampleCrumbs(&log)...).Key("trail").Label("Folder path"))
	})
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleGroup, "Folder path")
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleLink, "Root"), access.RoleLink, "Root", 0)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleLink, "Two"), access.RoleLink, "Two", 0)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleStaticText, "Three"), access.RoleStaticText, "Three", 0)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleStaticText, "Here"), access.RoleStaticText, "Here", access.StateSelected)
	accesstest.Absent(t, tr, access.RoleLink, "Here")
	if len(r.warnings) != 0 {
		t.Errorf("accessibility warnings: %v", r.warnings)
	}
}

// The default name of the trail is "Breadcrumb".
func TestBreadcrumbsDefaultLabel(t *testing.T) {
	var log []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Breadcrumbs(sampleCrumbs(&log)...))
	})
	accesstest.Find(t, r.tree(), access.RoleGroup, "Breadcrumb")
}

// With MaxItems(3) a five-location trail shows the first location, the "..." button,
// and the last two. The hidden locations are in the menu, a hidden location without
// OnClick is disabled there, and choosing one runs its OnClick and closes the menu.
func TestBreadcrumbsMaxItemsCollapsesIntoAMenu(t *testing.T) {
	var log []string
	chosen := func(label string) func() { return func() { log = append(log, label) } }
	crumbs := []Crumb{
		{Label: "Root", OnClick: chosen("Root")},
		{Label: "Two", OnClick: chosen("Two")},
		{Label: "Three"},
		{Label: "Four", OnClick: chosen("Four")},
		{Label: "Here"},
	}
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Breadcrumbs(crumbs...).Key("trail").MaxItems(3))
	})
	tr := r.tree()
	accesstest.Absent(t, tr, access.RoleLink, "Two")
	accesstest.Find(t, tr, access.RoleLink, "Root")
	accesstest.Find(t, tr, access.RoleLink, "Four")
	accesstest.Find(t, tr, access.RoleButton, "Show hidden locations")

	r.clickKey(t, "trail/overflow")
	tr = r.tree()
	accesstest.Find(t, tr, access.RoleMenuItem, "Two")
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleMenuItem, "Three"), access.RoleMenuItem, "Three", access.StateDisabled)
	clickFound(t, r, "jamb.menu.0.0")
	if !slices.Equal(log, []string{"Two"}) {
		t.Errorf("chosen %v, want [Two]", log)
	}
	if _, open := r.rt.Find("jamb.menu"); open {
		t.Error("the menu is still open after a location was chosen")
	}
}

// The parts of a breadcrumb trail reach the nodes that draw them. The trail collapses
// to two links, the current location and three separators, so each part has a node.
func testBreadcrumbsParts(t *testing.T, applied map[string]bool) {
	type crumbCase struct {
		part  string
		set   func(*BreadcrumbsStyle, Style)
		drawn string // the part whose node draws this part (a state is drawn by its base part)
		key   string // the element whose node draws the part, "" for every node of the part
		act   func(*testing.T, *accessRig)
	}
	cases := []crumbCase{
		{part: "Breadcrumbs.Bar", set: func(s *BreadcrumbsStyle, m Style) { s.Bar = m }, drawn: "Breadcrumbs.Bar"},
		{part: "Breadcrumbs.Item", set: func(s *BreadcrumbsStyle, m Style) { s.Item = m }, drawn: "Breadcrumbs.Item", key: "trail/crumb:0"},
		{part: "Breadcrumbs.ItemHover", set: func(s *BreadcrumbsStyle, m Style) { s.ItemHover = m }, drawn: "Breadcrumbs.Item", key: "trail/crumb:0",
			act: func(t *testing.T, r *accessRig) { hoverNode(r, keyNode(t, r.rt, "trail/crumb:0")) }},
		{part: "Breadcrumbs.Current", set: func(s *BreadcrumbsStyle, m Style) { s.Current = m }, drawn: "Breadcrumbs.Current", key: "trail/crumb:4"},
		{part: "Breadcrumbs.Separator", set: func(s *BreadcrumbsStyle, m Style) { s.Separator = m }, drawn: "Breadcrumbs.Separator"},
		{part: "Breadcrumbs.Overflow", set: func(s *BreadcrumbsStyle, m Style) { s.Overflow = m }, drawn: "Breadcrumbs.Overflow"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st BreadcrumbsStyle
			c.set(&st, Style{Background: markBG})
			var log []string
			r := newAccessRig(func(ctx *Context) Element {
				return Div().Column().Child(Breadcrumbs(
					Crumb{Label: "Root", OnClick: func() { log = append(log, "Root") }},
					Crumb{Label: "Two", OnClick: func() { log = append(log, "Two") }},
					Crumb{Label: "Three", OnClick: func() { log = append(log, "Three") }},
					Crumb{Label: "Four", OnClick: func() { log = append(log, "Four") }},
					Crumb{Label: "Here"},
				).Key("trail").MaxItems(3).Styles(st))
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

// TestBreadcrumbsRoundTripsThroughTheme checks that the theme's Breadcrumbs reaches
// a trail, and that the instance's Styles win over it.
func TestBreadcrumbsRoundTripsThroughTheme(t *testing.T) {
	theme := *DarkTheme()
	theme.Breadcrumbs = BreadcrumbsStyle{Bar: Style{Background: markBG}}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Div().Column().Child(Breadcrumbs(Crumb{Label: "Root"}, Crumb{Label: "Here"}).Key("trail").
			Styles(BreadcrumbsStyle{Bar: Style{TextColor: markFG}}))
	})
	n := partNode(t, r.rt, "Breadcrumbs.Bar", "")
	wantBackground(t, n, "Breadcrumbs.Bar", markBG)
	wantTextColor(t, n, "Breadcrumbs.Bar", markFG)
}

// TestBreadcrumbsAndPaginationFollowTextScale checks that the text of a trail and of
// a pagination control grows with the Text size setting.
func TestBreadcrumbsAndPaginationFollowTextScale(t *testing.T) {
	heights := func(scale int) map[string]float32 {
		r := newAccessRig(func(ctx *Context) Element {
			return Div().Column().Gap(12).
				Child(Breadcrumbs(Crumb{Label: "Root", OnClick: func() {}}, Crumb{Label: "Here"})).
				Child(Pagination(NewSignal(3), 20))
		})
		if scale != 100 {
			r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
			r.frame()
		}
		out := map[string]float32{}
		for _, n := range r.rt.byID {
			if n.kind == kindText && n.textEl != nil {
				switch n.textEl.text {
				case "Here", "Root", "3":
					out[n.textEl.text] = n.lay.H
				}
			}
		}
		return out
	}
	base, big := heights(100), heights(225)
	for _, label := range []string{"Here", "Root", "3"} {
		if big[label] <= base[label] {
			t.Errorf("%q is %v px tall at 225%%, %v px at 100%%: it does not grow", label, big[label], base[label])
		}
	}
}
