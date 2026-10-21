package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// openComponentsFiltered opens the Components page, filtered to query, and focuses
// the filter. The clock is the UI's own (see uitest), so the frames are the same
// on every run.
func openComponentsFiltered(t *testing.T, query string) (*uitest.UI, *Store) {
	t.Helper()
	s := NewStore()
	u := uitest.New(t, App(s), uitest.Size(1200, 760), uitest.Theme(DemoDarkTheme))
	s.Page.Set(PageComponents)
	u.Frame()
	s.SectionField.Focus()
	u.Advance(16 * time.Millisecond)
	u.Type(query)
	u.Advance(16 * time.Millisecond)
	return u, s
}

// idleFrames runs n frames and fails the test on the first one that rebuilds a
// component or builds the tree: an idle UI does neither.
func idleFrames(t *testing.T, u *uitest.UI, n int) {
	t.Helper()
	rt := u.Runtime()
	for i := range n {
		u.Advance(16 * time.Millisecond)
		st := rt.Stats()
		if st.Rendered != 0 || st.Built {
			t.Fatalf("frame %d of %d rebuilt the UI while idle: %d components rendered, built=%v", i+1, n, st.Rendered, st.Built)
		}
	}
}

// With the filter at "combo", the combobox is focused and its options list is open,
// the UI settles and an idle frame rebuilds nothing.
func TestComboboxIdleWhileOpen(t *testing.T) {
	u, _ := openComponentsFiltered(t, "combo")
	u.Press("tab") // the first field after the filter is the City combobox
	u.Type("ba")
	u.Press("alt+down")
	u.Settle()
	if n := countRole(u, access.RoleListItem); n == 0 {
		t.Fatal("the combobox list did not open")
	}
	idleFrames(t, u, 60)
}

// The background suggestions show a busy indicator while they load and go once the
// options arrive. After that the page is idle again, with the list still open.
func TestComboboxIdleAfterSuggestions(t *testing.T) {
	u, _ := openComponentsFiltered(t, "combo")
	for range 3 { // the background example is the third field after the filter
		u.Press("tab")
	}
	u.Type("a")
	u.WaitFor("the background suggestions", func() bool {
		return countRole(u, access.RoleListItem) > 0
	})
	u.Settle()
	idleFrames(t, u, 60)
}

// countRole returns the number of nodes in the access tree with role.
func countRole(u *uitest.UI, role access.Role) int {
	n := 0
	u.Tree().Walk(func(x *access.Node) bool {
		if x.Role == role {
			n++
		}
		return true
	})
	return n
}

// BenchmarkComponentsScroll scrolls the whole Components page one step per frame,
// with the pointer over the page, as a reader does. Each iteration is one frame:
// build, layout, paint and access tree included.
func BenchmarkComponentsScroll(b *testing.B) {
	s := NewStore()
	u := uitest.New(b, App(s), uitest.Size(1200, 760), uitest.Theme(DemoDarkTheme))
	s.Page.Set(PageComponents)
	u.Frame()
	rt := u.Runtime()
	rt.PointerMove(600, 400, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		rt.Scroll(600, 400, 0, 40, false, 0)
		u.Advance(16 * time.Millisecond)
	}
}
