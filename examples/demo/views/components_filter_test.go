package views

import (
	"slices"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// sectionTitles returns the titles of secs, in order.
func sectionTitles(secs []componentSection) []string {
	var out []string
	for _, s := range secs {
		out = append(out, s.title)
	}
	return out
}

// The filter matches the words of the query in the titles and keywords of the
// sections, ignoring case; every word must match.
func TestMatchingSections(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{
		{"stepper", []string{"More widgets"}},
		{"kbd", []string{"Feedback"}},
		{"keyboard shortcut", []string{"Feedback"}},
		{"Shortcut  KEYBOARD", []string{"Feedback"}},
		{"rating", []string{"Form controls"}},
		{"stars", []string{"Form controls"}},
		{"otp", []string{"Form controls"}},
		{"one-time code", []string{"Form controls"}},
		{"toggle", []string{"Form controls"}},
		{"password", []string{"Form controls"}},
		{"chart", []string{"Charts"}},
		{"spinner", []string{"Feedback"}},
		{"sheet", []string{"Dialogs"}},
		{"command palette", []string{"Dialogs"}},
		{"motion", []string{"Motion"}},
		{"zzz", nil},
		{"stepper zzz", nil},
	}
	for _, c := range cases {
		if got := sectionTitles(matchingSections(c.query)); !slices.Equal(got, c.want) {
			t.Errorf("matchingSections(%q) = %q, want %q", c.query, got, c.want)
		}
	}
	for _, q := range []string{"", "   "} {
		if got := matchingSections(q); len(got) != len(componentSections) {
			t.Errorf("matchingSections(%q) has %d sections, want all %d", q, len(got), len(componentSections))
		}
	}
}

// Each section is in the table once, with keywords, and shows its title as a
// heading, so a match names the section the user sees.
func TestComponentSectionsTable(t *testing.T) {
	seen := map[string]bool{}
	for _, sec := range componentSections {
		if seen[sec.title] {
			t.Errorf("the section %q is in the table twice", sec.title)
		}
		seen[sec.title] = true
		if sec.keywords == "" {
			t.Errorf("the section %q has no keywords", sec.title)
		}
	}
	r := newDemoRig(t)
	showComponents(r)
	tr := r.tree()
	for _, sec := range componentSections {
		accesstest.Find(t, tr, access.RoleHeading, sec.title)
	}
}

// A query shows only the matching sections, with the count beside the field;
// one that matches nothing says so.
func TestComponentsFilterShowsMatches(t *testing.T) {
	r := newDemoRig(t)
	showComponents(r)
	r.s.SectionQuery.Set("stepper")
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleHeading, "More widgets")
	accesstest.Absent(t, tr, access.RoleHeading, "Feedback")
	if !hasText(r, "1 of 26 sections") {
		t.Error("the bar does not show the count")
	}
	r.s.SectionQuery.Set("zzz")
	r.frame()
	if !hasText(r, "No sections match “zzz”.") {
		t.Error("a query that matches nothing does not say so")
	}
	r.s.SectionQuery.Set("")
	tr = r.tree()
	accesstest.Find(t, tr, access.RoleHeading, "Notification area and taskbar")
	if hasText(r, "26 sections") {
		t.Error("the count shows while nothing is filtered")
	}
}

// The page announces its own counts, and not another page's.
func TestComponentsFilterAnnouncesCount(t *testing.T) {
	r := newDemoRig(t)
	showComponents(r)
	announced := func(text string) bool {
		for _, u := range r.updates {
			for _, a := range u.Announcements {
				if a.Text == text && a.Politeness == access.Polite {
					return true
				}
			}
		}
		return false
	}
	r.s.Notice.Set(notice{Text: "12 matching events", Seq: 100, Page: PageActivity})
	r.frame()
	r.s.Notice.Set(notice{Text: "1 of 26 sections", Seq: 101, Page: PageComponents})
	r.frame()
	r.frame()
	if !announced("1 of 26 sections") {
		t.Error("the page did not announce its count")
	}
	if announced("12 matching events") {
		t.Error("the page announced the Activity page's count")
	}
}

// Ctrl+F focuses the filter on the Components page, from anywhere on the page.
func TestCtrlFFocusesComponentsFilter(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageComponents)
	rt := shortcutRig(s)
	rt.Press(input.KeyF, input.ModCtrl)
	rt.Frame(time.Now())
	if !s.SectionField.Focused() {
		t.Error("Ctrl+F did not focus the filter of the Components page")
	}
}

// Ctrl+F in the code editor opens the editor's find bar, and Ctrl+F in that bar
// stays there: neither focuses the page's filter.
func TestCtrlFInCodeEditorFinds(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageComponents)
	// The filter brings the editor to the top of the page, where a click reaches it.
	s.SectionQuery.Set("code editor")
	rt := shortcutRig(s)
	if err := rt.ClickKey("code-editor"); err != nil {
		t.Fatal(err)
	}
	accesstest.Focused(t, rt.AccessTree(), access.RoleTextArea, "Sample Go file")
	rt.Press(input.KeyF, input.ModCtrl)
	rt.Frame(time.Now())
	rt.Frame(time.Now())
	if s.SectionField.Focused() {
		t.Fatal("Ctrl+F in the code editor focused the page's filter")
	}
	accesstest.Focused(t, rt.AccessTree(), access.RoleTextField, "Find")
	rt.Press(input.KeyF, input.ModCtrl)
	rt.Frame(time.Now())
	if s.SectionField.Focused() {
		t.Fatal("Ctrl+F in the code editor's find bar focused the page's filter")
	}
}
