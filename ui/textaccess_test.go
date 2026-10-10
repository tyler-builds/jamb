package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/text"
)

// textOf returns the Text state of the field or text area named name in the
// rig's tree.
func textOf(t *testing.T, r *accessRig, role access.Role, name string) *access.TextInfo {
	t.Helper()
	n := accesstest.Find(t, r.tree(), role, name)
	if n.Text == nil {
		t.Fatalf("%q has no text state", name)
	}
	return n.Text
}

// editorOfNode returns the editor state of the access node id, from the rig's
// runtime, to compare the access text with the layout the editor paints.
func editorOfNode(t *testing.T, r *accessRig, id access.ID) *editState {
	t.Helper()
	n := r.rt.byID[uint64(id)&^(1<<63)]
	if n == nil || n.st == nil || n.st.edit == nil {
		t.Fatalf("node %d is not an editor", id)
	}
	return n.st.edit
}

// focusField gives keyboard focus to the field named name, through the access
// bridge, as an assistive technology would.
func focusField(t *testing.T, r *accessRig, role access.Role, name string) access.ID {
	t.Helper()
	id := accesstest.Find(t, r.tree(), role, name).ID
	r.bridge.Do(id, access.Request{Action: access.ActionFocus})
	r.frame()
	return id
}

func TestTextPointsCountGraphemeClusters(t *testing.T) {
	cases := []struct {
		name  string
		value string
		len   int // grapheme clusters
	}{
		{"ASCII", "hello", 5},
		{"combining accent", "éx", 2},
		{"surrogate pair", "a\U0001F600b", 3},
		{"CR LF is one cluster", "a\r\nb", 3},
		{"Arabic letters", "مرحبا", 5},
		{"empty", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newAccessRig(func(ctx *Context) Element {
				return Div().Column().Children(TextArea(NewSignal(tc.value)).Label("Body").Width(Px(300)))
			})
			ti := textOf(t, r, access.RoleTextArea, "Body")
			if got := ti.Content.Len(); got != tc.len {
				t.Errorf("grapheme clusters = %d, want %d (points %v)", got, tc.len, ti.Content.Points)
			}
			if ti.Content.Text != tc.value {
				t.Errorf("content text %q, want %q", ti.Content.Text, tc.value)
			}
		})
	}
}

func TestCaretAndSelectionAreGraphemePoints(t *testing.T) {
	// "e" with a combining accent (bytes 0 to 3), an emoji (3 to 7), then "x".
	// Points: 0 before the first cluster, 1 before the emoji, 2 before the x.
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(TextInput(NewSignal("é\U0001F600x")).Label("Search"))
	})
	field := accesstest.Find(t, r.tree(), access.RoleTextField, "Search")
	r.bridge.Do(field.ID, access.Request{Action: access.ActionSetTextSelection, Anchor: 3, Caret: 3})
	r.frame()
	ti := textOf(t, r, access.RoleTextField, "Search")
	if ti.Anchor != 1 || ti.Caret != 1 {
		t.Errorf("caret after the accent = %d, %d; want point 1", ti.Anchor, ti.Caret)
	}
	r.bridge.Do(field.ID, access.Request{Action: access.ActionSetTextSelection, Anchor: 3, Caret: 7})
	r.frame()
	ti = textOf(t, r, access.RoleTextField, "Search")
	if sel := ti.Selection(); sel.Start != 1 || sel.End != 2 {
		t.Errorf("selection = %d..%d, want points 1..2 (the emoji)", sel.Start, sel.End)
	}
	if got := ti.Selection().Text(); got != "\U0001F600" {
		t.Errorf("selected text = %q, want the emoji", got)
	}
}

func TestSelectionRequestClampsToTheTextAndFocuses(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(TextInput(NewSignal("hello")).Label("Search"))
	})
	field := accesstest.Find(t, r.tree(), access.RoleTextField, "Search")
	r.bridge.Do(field.ID, access.Request{Action: access.ActionSetTextSelection, Anchor: 1, Caret: 99})
	r.frame()
	ti := textOf(t, r, access.RoleTextField, "Search")
	if sel := ti.Selection(); sel.Start != 1 || sel.End != 5 {
		t.Errorf("selection after a request past the end = %d..%d, want 1..5", sel.Start, sel.End)
	}
	if !accesstest.Find(t, r.tree(), access.RoleTextField, "Search").States.Has(access.StateFocused) {
		t.Errorf("the field did not take focus for its selection")
	}
}

func TestWrappedLinesAndRectsFollowTheLayout(t *testing.T) {
	const value = "one two three four five six seven eight"
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(TextArea(NewSignal(value)).Label("Notes").Width(Px(90)))
	})
	field := accesstest.Find(t, r.tree(), access.RoleTextArea, "Notes")
	ti := field.Text
	c := ti.Content
	layout := editorOfNode(t, r, field.ID).layout
	if layout == nil {
		t.Fatalf("the editor has no layout after a frame")
	}
	if len(c.Lines) < 3 || len(c.Lines) != len(layout.Lines) {
		t.Fatalf("got %d lines, the layout has %d; want wrapping into at least 3", len(c.Lines), len(layout.Lines))
	}
	// The line of each point is the editor's own: the expansion of a position
	// by line gives the text the layout puts on that line, including its break.
	for i := range layout.Lines {
		a, _ := layout.LineBounds(i)
		p := c.PointOf(a)
		got := access.TextRange{Info: ti, Start: p, End: p}.Expand(access.UnitLine)
		want := value[a:]
		if i+1 < len(layout.Lines) {
			next, _ := layout.LineBounds(i + 1)
			want = value[a:next]
		}
		if got.Text() != want {
			t.Errorf("line %d = %q, want the layout's %q", i, got.Text(), want)
		}
	}
	if got := ti.Whole().Expand(access.UnitLine).Text(); got != value {
		t.Errorf("the whole text expanded by line = %q, want the whole value", got)
	}
	rects := ti.Whole().Rects()
	if len(rects) != len(c.Lines) {
		t.Fatalf("got %d rects for %d lines", len(rects), len(c.Lines))
	}
	for i := 1; i < len(rects); i++ {
		if rects[i].Y <= rects[i-1].Y {
			t.Errorf("rect %d is not below rect %d: %v", i, i-1, rects)
		}
	}
	if rects[0].H <= 0 || rects[0].W <= 0 {
		t.Errorf("first rect is empty: %v", rects[0])
	}
}

func TestAccessMovesAreTheEditorsWordMovement(t *testing.T) {
	// The Move of a degenerate range by word is the editor's Ctrl+Right and
	// Ctrl+Left, which go to text.NextWord and text.PrevWord, from every point.
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme, nil)
	samples := []string{
		"foo   bar, baz",
		"naïve café 3rd-party_ok",
		"ét́ x\U0001F600y  z",
		"مرحبا بالعالم 42",
		"  leading and trailing  ",
		"one\ntwo\r\nthree",
	}
	for _, s := range samples {
		c := rt.buildTextContent(s, nil)
		info := &access.TextInfo{Content: c}
		for p := 0; p <= c.Len(); p++ {
			b := c.ByteOf(p)
			next, _ := access.TextRange{Info: info, Start: p, End: p}.Move(access.UnitWord, 1)
			if got, want := c.ByteOf(next.Start), c.ByteOf(c.PointOf(text.NextWordStart(s, b))); got != want {
				t.Errorf("%q: word forward from byte %d = byte %d, the editor says %d", s, b, got, want)
			}
			prev, _ := access.TextRange{Info: info, Start: p, End: p}.Move(access.UnitWord, -1)
			if got, want := c.ByteOf(prev.Start), c.ByteOf(c.PointOf(text.PrevWordStart(s, b))); got != want {
				t.Errorf("%q: word back from byte %d = byte %d, the editor says %d", s, b, got, want)
			}
		}
	}
}

func TestTextChangesAreReportedInTheSnapshotDiff(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(TextInput(NewSignal("")).Label("Search"))
	})
	field := focusField(t, r, access.RoleTextField, "Search")
	before := r.rt.AccessTree()
	r.rt.TextInput("h")
	r.rt.TextInput("i")
	r.frame()
	u := access.Diff(before, r.rt.AccessTree())
	if len(u.TextChanged) != 1 || u.TextChanged[0] != field {
		t.Errorf("TextChanged = %v, want the field %d", u.TextChanged, field)
	}
	if len(u.TextSelection) != 1 || u.TextSelection[0] != field {
		t.Errorf("TextSelection = %v, want the field", u.TextSelection)
	}
	// Moving the caret without changing the text is a selection change only.
	before = r.rt.AccessTree()
	r.bridge.Do(field, access.Request{Action: access.ActionSetTextSelection, Anchor: 0, Caret: 0})
	r.frame()
	u = access.Diff(before, r.rt.AccessTree())
	if len(u.TextChanged) != 0 || len(u.TextSelection) != 1 {
		t.Errorf("caret move reported TextChanged %v, TextSelection %v", u.TextChanged, u.TextSelection)
	}
}

func TestTextContentIsSharedUntilTheTextOrLayoutChanges(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(TextInput(NewSignal("abc")).Label("Search"))
	})
	field := focusField(t, r, access.RoleTextField, "Search")
	first := textOf(t, r, access.RoleTextField, "Search").Content
	r.rt.TextInput("!")
	r.frame()
	second := textOf(t, r, access.RoleTextField, "Search").Content
	if first == second {
		t.Fatalf("the content was not rebuilt after the text changed")
	}
	// A selection change does not rebuild the content.
	r.bridge.Do(field, access.Request{Action: access.ActionSetTextSelection, Anchor: 1, Caret: 1})
	r.frame()
	if got := textOf(t, r, access.RoleTextField, "Search").Content; got != second {
		t.Errorf("a selection change rebuilt the content")
	}
}

// BenchmarkBuildTextContent measures the content of a long, wrapped text, which is
// built again on each change to the text.
func BenchmarkBuildTextContent(b *testing.B) {
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme, nil)
	s := strings.Repeat("the quick brown fox jumps over the lazy dog ", 500)
	l := rt.text.Layout(s, text.Style{Size: 14}, 400)
	b.ReportMetric(float64(len(l.Lines)), "lines")
	b.ReportMetric(float64(len(s)), "bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.buildTextContent(s, l)
	}
}

// TestWordExpandIsTheUnitAtThePosition checks that ExpandToEnclosingUnit(Word) of a
// degenerate range gives the word unit that holds the position, and that moving by
// a word and then expanding gives the next unit, as Notepad does.
func TestWordExpandIsTheUnitAtThePosition(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme, nil)
	s := "A small note-taking app. Next line  \rSecond line, ok."
	c := rt.buildTextContent(s, nil)
	info := &access.TextInfo{Content: c}
	starts := text.WordStarts(s)
	unitAt := func(b int) (int, int) {
		a, end := 0, len(s)
		for i, st := range starts {
			if st <= b {
				a = st
				end = len(s)
				if i+1 < len(starts) {
					end = starts[i+1]
				}
			}
		}
		return a, end
	}
	for p := 0; p < c.Len(); p++ {
		b := c.ByteOf(p)
		a, end := unitAt(b)
		got := access.TextRange{Info: info, Start: p, End: p}.Expand(access.UnitWord)
		if got.Text() != s[a:end] {
			t.Errorf("Expand(Word) at byte %d = %q, want the unit %q", b, got.Text(), s[a:end])
		}
	}
	moved, n := access.TextRange{Info: info}.Move(access.UnitWord, 1)
	if n != 1 || moved.Expand(access.UnitWord).Text() != "small " {
		t.Errorf("Move(Word, 1) from the start then Expand = %q, want \"small \"", moved.Expand(access.UnitWord).Text())
	}
}
