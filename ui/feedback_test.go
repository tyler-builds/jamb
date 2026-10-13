package ui

import (
	"strconv"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// feedbackRoot renders children in a column, with a Toaster last so that it is
// laid out over them.
func feedbackRoot(children ...Element) func(*Context) Element {
	return func(*Context) Element {
		return Div().Column().Gap(8).Padding(16).Children(append(children, Toaster().Key("toaster"))...)
	}
}

// progressNode returns the progress bar named name in the access tree.
func progressNode(t *testing.T, r *accessRig, name string) *access.Node {
	t.Helper()
	return accesstest.Find(t, r.tree(), access.RoleProgressBar, name)
}

// textNode returns the first drawn text node with the string s.
func textNode(r *accessRig, s string) (*node, bool) {
	for _, n := range r.rt.byID {
		if n.kind == kindText && n.textEl != nil && n.textEl.text == s {
			return n, true
		}
	}
	return nil, false
}

// nodeNamed returns the built element whose explicit label is name.
func nodeNamed(r *accessRig, name string) *node {
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.acc != nil && n.div.acc.name == name {
			return n
		}
	}
	return nil
}

func TestProgressBarDeterminateIsReadOnlyRange(t *testing.T) {
	r := newAccessRig(feedbackRoot(ProgressBar(0.45).Label("Upload")))
	r.frame()
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
	n := progressNode(t, r, "Upload")
	if n.Range == nil {
		t.Fatal("a determinate bar has no range")
	}
	if n.Range.Value != 0.45 || n.Range.Min != 0 || n.Range.Max != 1 {
		t.Errorf("range %+v, want value 0.45 in 0..1", *n.Range)
	}
	if !n.States.Has(access.StateReadOnly) {
		t.Error("a progress bar is not read-only")
	}
	for _, a := range n.Actions {
		if a == access.ActionSetRangeValue {
			t.Error("a progress bar offers SetValue; it must be read-only")
		}
	}
	if _, ok := textNode(r, "45%"); !ok {
		t.Error("the readout 45% is not drawn")
	}
}

func TestProgressBarClampsAndFormats(t *testing.T) {
	bar := ProgressBar(150).Min(0).Max(100).Label("Import").
		Format(func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + " items" })
	r := newAccessRig(feedbackRoot(bar))
	r.frame()
	n := progressNode(t, r, "Import")
	if n.Range.Value != 100 || n.Range.Max != 100 {
		t.Errorf("range %+v: a value above the range must clamp to its top", *n.Range)
	}
	if _, ok := textNode(r, "100 items"); !ok {
		t.Error("the custom readout 100 items is not drawn")
	}
}

func TestIndeterminateProgressBarHasNoRange(t *testing.T) {
	r := newAccessRig(feedbackRoot(ProgressBar(0).Indeterminate()))
	r.frame()
	n := progressNode(t, r, "Loading…")
	if n.Range != nil {
		t.Errorf("an indeterminate bar exposes range %+v; it must expose none", *n.Range)
	}
	if _, ok := textNode(r, "0%"); ok {
		t.Error("an indeterminate bar draws a readout")
	}
}

func TestProgressRingIsBusyIndicator(t *testing.T) {
	r := newAccessRig(feedbackRoot(ProgressRing().Size(RingLarge)))
	r.frame()
	n := progressNode(t, r, "Loading…")
	if n.Range != nil {
		t.Error("a busy ring exposes a range")
	}
	if n.Bounds.W != 40 || n.Bounds.H != 40 {
		t.Errorf("ring bounds %vx%v, want 40x40", n.Bounds.W, n.Bounds.H)
	}
	if len(r.warnings) != 0 {
		t.Errorf("warnings: %v", r.warnings)
	}
}

func TestProgressReducedMotionIsStatic(t *testing.T) {
	for _, c := range []struct {
		name string
		el   func() Element
	}{
		{"indeterminate bar", func() Element { return ProgressBar(0).Indeterminate() }},
		{"ring", func() Element { return ProgressRing() }},
	} {
		t.Run(c.name, func(t *testing.T) {
			moving := newAccessRig(feedbackRoot(c.el()))
			moving.frame()
			if !moving.rt.NeedsFrame() {
				t.Fatal("with motion, the indicator asks for frames")
			}
			still := newAccessRig(feedbackRoot(c.el()))
			still.rt.SetSystemSettings(SystemSettings{ReducedMotion: true})
			still.frame()
			if still.rt.NeedsFrame() {
				t.Error("with reduced motion, the indicator still asks for frames")
			}
		})
	}
}

func TestProgressHighContrastOutlinesTrack(t *testing.T) {
	r := newAccessRig(feedbackRoot(ProgressBar(0.5).Label("Upload")))
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	nd := nodeNamed(r, "Upload")
	if nd == nil || nd.div.style.BorderWidth == 0 {
		t.Fatal("under high contrast the track has no outline; the empty part would read as filled")
	}
	if nd.div.style.BorderColor != r.rt.theme.Text {
		t.Errorf("track outline is %v, want the text color %v", nd.div.style.BorderColor, r.rt.theme.Text)
	}
}

func TestBadgeFormatsCountAndCap(t *testing.T) {
	r := newAccessRig(feedbackRoot(Badge(6).Noun("items"), Badge(150).Noun("items")))
	r.frame()
	if _, ok := textNode(r, "6"); !ok {
		t.Error("badge 6 is not drawn")
	}
	if _, ok := textNode(r, "99+"); !ok {
		t.Error("badge 150 does not cap at 99+")
	}
	accesstest.Find(t, r.tree(), access.RoleGroup, "6 items")
	accesstest.Find(t, r.tree(), access.RoleGroup, "more than 99 items")
	if len(r.warnings) != 0 {
		t.Errorf("warnings: %v", r.warnings)
	}
}

func TestBadgeZeroDrawsNothing(t *testing.T) {
	r := newAccessRig(feedbackRoot(Badge(0)))
	r.frame()
	if _, ok := textNode(r, "0"); ok {
		t.Error("a count of zero draws a badge")
	}
}

func TestBadgeNamesItsOwner(t *testing.T) {
	r := newAccessRig(feedbackRoot(
		Div().Row().Label("Projects").Badged(Badge(6).Noun("items")),
	))
	r.frame()
	accesstest.Find(t, r.tree(), access.RoleGroup, "Projects, 6 items")
	// The attached badge is hidden: no element of its own is named with its count.
	if nd := nodeNamed(r, "6 items"); nd != nil {
		t.Error("the attached badge is a separate named element")
	}
}

func TestBadgeDotHasNoName(t *testing.T) {
	r := newAccessRig(feedbackRoot(BadgeDot()))
	r.frame()
	if len(r.warnings) != 0 {
		t.Errorf("warnings: %v", r.warnings)
	}
}

func TestBadgeHighContrastIsOutlined(t *testing.T) {
	r := newAccessRig(feedbackRoot(Badge(3).Noun("unread").Variant(BadgeDanger)))
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	nd := nodeNamed(r, "3 unread")
	if nd == nil || nd.div.style.BorderWidth == 0 {
		t.Error("a badge under high contrast is not outlined")
	}
}

// TestProgressBarColorIsTheFill checks that Color sets the fill, and that under
// high contrast the fill is the Highlight color whatever the app asked for.
func TestProgressBarColorIsTheFill(t *testing.T) {
	app := Hex("#34d399")
	r := newAccessRig(feedbackRoot(ProgressBar(0.5).Color(app).Label("Upload")))
	r.frame()
	fill := nodeNamed(r, "Upload").children[0]
	if got := fill.div.style.Background; got != app {
		t.Errorf("fill is %v, want the app color %v", got, app)
	}

	hc := newAccessRig(feedbackRoot(ProgressBar(0.5).Color(app).Label("Upload")))
	hc.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	hc.frame()
	hcFill := nodeNamed(hc, "Upload").children[0]
	if got := hcFill.div.style.Background; got != hc.rt.theme.Accent {
		t.Errorf("under high contrast the fill is %v, want Highlight %v", got, hc.rt.theme.Accent)
	}
}

// TestBadgeTextDotIsHidden checks that the dot of a badge is not read: the badge
// is named by its text alone, and draws without warnings.
func TestBadgeTextDotIsHidden(t *testing.T) {
	r := newAccessRig(feedbackRoot(BadgeText("Done").StatusDot().Variant(BadgeSuccess)))
	r.frame()
	accesstest.Find(t, r.tree(), access.RoleGroup, "Done")
	if len(r.warnings) != 0 {
		t.Errorf("warnings: %v", r.warnings)
	}
}
