package markdown

import (
	"context"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// show renders the view that make returns, in a window of 900 by 700 with the
// development warnings of the access tree turned on, and checks that it gives none.
func show(t *testing.T, view func() *ViewElement) *uitest.UI {
	t.Helper()
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Size(ui.Full, ui.Full).ScrollY().Child(view().Key("doc"))
	}, uitest.Size(900, 700))
	u.Runtime().SetAccessWarnings(true)
	u.Frame()
	t.Cleanup(func() {
		if w := u.Warnings(); len(w) > 0 {
			t.Errorf("the view gave warnings: %v", w)
		}
	})
	return u
}

// nodes returns the access nodes of the UI's tree with the role r, in tree order.
func nodes(u *uitest.UI, r access.Role) []*access.Node {
	var out []*access.Node
	u.Tree().Walk(func(n *access.Node) bool {
		if n.Role == r {
			out = append(out, n)
		}
		return true
	})
	return out
}

// Headings are headings of their levels, named by their text, and paragraphs are
// text, with emphasis, code and strikethrough in the one text of their block.
func TestHeadingsAndParagraphs(t *testing.T) {
	u := show(t, func() *ViewElement {
		return View("# Title\n\nSome *soft* and **strong** text with `code` and ~~old~~ words.\n\n## Part two\n\n###### Small print\n")
	})
	var got []string
	for _, h := range nodes(u, access.RoleHeading) {
		got = append(got, h.Name+"/"+string(rune('0'+h.Heading)))
	}
	if want := "Title/1 Part two/2 Small print/6"; strings.Join(got, " ") != want {
		t.Errorf("headings %q, want %q", strings.Join(got, " "), want)
	}
	u.ByRole(access.RoleStaticText, "Some soft and strong text with code and old words.")
}

// Lists are lists of items, nested where the text nests them, and an ordered list
// numbers its items from its start.
func TestLists(t *testing.T) {
	u := show(t, func() *ViewElement {
		return View("- one\n- two\n  - two a\n  - two b\n\n3. three\n4. four\n")
	})
	lists := nodes(u, access.RoleList)
	if len(lists) != 3 {
		t.Fatalf("%d lists, want 3 (bullets, the nested list and the ordered list)", len(lists))
	}
	var items []string
	for _, it := range nodes(u, access.RoleListItem) {
		items = append(items, it.Name)
	}
	want := []string{"one", "two two a two b", "two a", "two b", "3. three", "4. four"}
	if strings.Join(items, "|") != strings.Join(want, "|") {
		t.Errorf("items %q, want %q", items, want)
	}
	if p := u.Tree().Node(nodes(u, access.RoleListItem)[2].Parent); p == nil || p.Role != access.RoleList {
		t.Errorf("the nested item's parent is %+v, want a list", p)
	}
}

// A table is a group named by its size, with its header and its cells in order.
func TestTable(t *testing.T) {
	u := show(t, func() *ViewElement {
		return View("| Name | Count |\n|:-----|------:|\n| apples | 12 |\n| pears |\n")
	})
	table := u.ByRole(access.RoleGroup, "Table, 2 columns, 3 rows")
	var cells []string
	for _, id := range table.Node().Children {
		cells = append(cells, u.Tree().Node(id).Name)
	}
	if want := "Name Count apples 12 pears"; strings.Join(cells, " ") != want {
		t.Errorf("cells %q, want %q", strings.Join(cells, " "), want)
	}
}

// Task list items have check boxes that say whether each task is done. Without
// OnTaskToggle they do not take focus; with it, a click asks for the other state.
func TestTasks(t *testing.T) {
	src := "- [x] Write the parser\n- [ ] Write the docs\n"
	u := show(t, func() *ViewElement { return View(src) })
	done, todo := u.ByRole(access.RoleCheckBox, "Write the parser"), u.ByRole(access.RoleCheckBox, "Write the docs")
	if !done.Checked() || todo.Checked() {
		t.Fatalf("checked %v and %v, want the first task done and the second not", done.Checked(), todo.Checked())
	}
	if todo.States().Has(access.StateFocusable) {
		t.Error("a read-only task's check box takes focus")
	}

	var toggled []int
	var states []bool
	u = show(t, func() *ViewElement {
		return View(src).OnTaskToggle(func(i int, on bool) { toggled, states = append(toggled, i), append(states, on) })
	})
	u.ByRole(access.RoleCheckBox, "Write the docs").Perform(access.ActionToggle)
	u.ByRole(access.RoleCheckBox, "Write the parser").Perform(access.ActionClick)
	if len(toggled) != 2 || toggled[0] != 1 || !states[0] || toggled[1] != 0 || states[1] {
		t.Fatalf("OnTaskToggle got %v %v, want task 1 on, then task 0 off", toggled, states)
	}
}

// Links are links that Tab reaches; Enter and a click call OnLink with the
// destination as written.
func TestLinksCallOnLink(t *testing.T) {
	var got []string
	u := show(t, func() *ViewElement {
		return View("See [the docs](docs/README.md), <https://example.com> and www.example.org.").
			OnLink(func(url string) { got = append(got, url) })
	})
	for _, name := range []string{"the docs", "https://example.com", "www.example.org"} {
		u.Press("tab")
		if f := u.Focused().Node(); f.Role != access.RoleLink || f.Name != name {
			t.Fatalf("Tab: focus on the %s %q, want the link %q", f.Role, f.Name, name)
		}
		u.Press("enter")
	}
	u.ByRole(access.RoleLink, "the docs").Perform(access.ActionClick)
	want := "docs/README.md https://example.com http://www.example.org docs/README.md"
	if strings.Join(got, " ") != want {
		t.Fatalf("OnLink got %q, want %q", strings.Join(got, " "), want)
	}
}

// A link whose text mixes styles is one link: one Tab stop named by its whole text,
// and Enter follows it once.
func TestStyledLinkIsOneLink(t *testing.T) {
	var got []string
	u := show(t, func() *ViewElement {
		return View("Read [the **bold** guide](guide.md) now.").OnLink(func(url string) { got = append(got, url) })
	})
	if n := len(nodes(u, access.RoleLink)); n != 1 {
		t.Fatalf("%d links, want 1", n)
	}
	u.Press("tab")
	if f := u.Focused().Node(); f.Role != access.RoleLink || f.Name != "the bold guide" {
		t.Fatalf("Tab: focus on the %s %q, want the link \"the bold guide\"", f.Role, f.Name)
	}
	u.Press("enter")
	// The link is the only stop, so a second Tab comes back to it, not to a piece of it.
	u.Press("tab")
	if f := u.Focused().Node(); f.Role != access.RoleLink || f.Name != "the bold guide" {
		t.Errorf("a second Tab: focus on the %s %q, want the same link", f.Role, f.Name)
	}
	if strings.Join(got, " ") != "guide.md" {
		t.Errorf("OnLink got %q, want guide.md once", got)
	}
}

// Without OnLink, a link opens with the system through ui.Context.OpenURL, which
// opens http, https and mailto URLs and refuses the others; a "#name" link moves
// focus to the heading with that anchor.
func TestLinksOpenWithTheSystem(t *testing.T) {
	u := show(t, func() *ViewElement {
		return View("[site](https://example.com/a) [mail](mailto:team@example.com) [file](notes.txt) " +
			"[run](file:///C:/Windows/notepad.exe) [down](#getting-started)\n\n## Getting started\n")
	})
	var opened []string
	uihost.Of(u.Runtime()).(interface{ SetOpenURL(func(string) error) }).
		SetOpenURL(func(url string) error { opened = append(opened, url); return nil })
	for _, name := range []string{"site", "mail", "file", "run", "down"} {
		u.ByRole(access.RoleLink, name).Perform(access.ActionClick)
	}
	if want := "https://example.com/a mailto:team@example.com"; strings.Join(opened, " ") != want {
		t.Errorf("opened %q, want %q", strings.Join(opened, " "), want)
	}
	u.Frame()
	if f := u.Focused().Node(); f.Role != access.RoleHeading || f.Name != "Getting started" {
		t.Errorf("after the #getting-started link, focus is on the %s %q, want the heading", f.Role, f.Name)
	}
}

// HTML is shown as it is written, and a comment is left out.
func TestHTMLIsLiteral(t *testing.T) {
	u := show(t, func() *ViewElement {
		return View("<!-- hidden -->\n\nA <b>bold</b> word.\n\n<div align=\"center\">x</div>\n")
	})
	u.ByRole(access.RoleStaticText, "A <b>bold</b> word.")
	u.ByRole(access.RoleStaticText, "<div align=\"center\">x</div>")
	u.Tree().Walk(func(n *access.Node) bool {
		if strings.Contains(n.Name, "hidden") {
			t.Errorf("the comment is shown: %q", n.Name)
		}
		return true
	})
}

// A footnote reference is a link to the list of footnotes at the end, which follows a
// rule; following it moves focus to the note.
func TestFootnotes(t *testing.T) {
	u := show(t, func() *ViewElement { return View("Claim.[^1]\n\n[^1]: The source.\n") })
	notes := u.ByRole(access.RoleList, "Footnotes")
	if len(notes.Node().Children) != 1 {
		t.Fatalf("%d footnotes, want 1", len(notes.Node().Children))
	}
	u.ByRole(access.RoleLink, "[1]").Perform(access.ActionClick)
	u.Frame()
	if f := u.Focused().Node(); f.Role != access.RoleListItem || !strings.Contains(f.Name, "The source.") {
		t.Errorf("focus on the %s %q, want the footnote", f.Role, f.Name)
	}
}

// A code block is named by its language and shows its code; a highlighter that is
// not ready yet calls back, and the view asks again.
func TestCodeBlockAsksTheHighlighter(t *testing.T) {
	h := &fakeHighlighter{}
	u := show(t, func() *ViewElement { return View("```go\nfunc main() {}\n```\n").Code(h) })
	code := u.ByRole(access.RoleGroup, "Code, go")
	if len(code.Node().Children) != 1 || u.Tree().Node(code.Node().Children[0]).Name != "func main() {}" {
		t.Fatalf("the code block holds %v", code.Node().Children)
	}
	if len(h.asked) != 1 || h.asked[0] != "go|func main() {}" {
		t.Fatalf("the highlighter was asked %q", h.asked)
	}
	h.ready()
	u.WaitFor("the view to ask again", func() bool { return len(h.asked) >= 2 })
}

// fakeHighlighter is a CodeHighlighter that is never ready the first time.
type fakeHighlighter struct {
	asked []string
	ready func()
}

func (f *fakeHighlighter) HighlightCode(_ context.Context, lang, code string, ready func()) ([]CodeSpan, bool) {
	f.asked = append(f.asked, lang+"|"+code)
	if len(f.asked) == 1 {
		f.ready = ready
		return nil, false
	}
	return []CodeSpan{{Start: 0, End: 4, Capture: "keyword"}}, true
}

// recordingHighlighter keeps the contexts of its requests and is never ready.
type recordingHighlighter struct{ ctxs []context.Context }

func (r *recordingHighlighter) HighlightCode(ctx context.Context, _, _ string, _ func()) ([]CodeSpan, bool) {
	r.ctxs = append(r.ctxs, ctx)
	return nil, false
}

// The context of the requests to the highlighter lasts while the view shows the same
// text: a render of the same text keeps it, a new text ends it, and so does the
// view's removal.
func TestHighlightRequestsEndWithTheirText(t *testing.T) {
	h := &recordingHighlighter{}
	src, shown, tick := ui.NewSignal("```go\nx\n```\n"), ui.NewSignal(true), ui.NewSignal(0)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		tick.Get()
		return ui.Div().Child(ui.If(shown.Get(), View(src.Get()).Code(h)))
	})
	first := h.ctxs[len(h.ctxs)-1]
	tick.Set(1)
	u.Frame()
	if len(h.ctxs) < 2 || h.ctxs[len(h.ctxs)-1] != first || first.Err() != nil {
		t.Fatal("a render of the same text did not keep the context of its requests")
	}
	src.Set("```go\ny\n```\n")
	u.Frame()
	second := h.ctxs[len(h.ctxs)-1]
	if first.Err() == nil || second == first || second.Err() != nil {
		t.Fatalf("after a new text: old context %v, new context %v; want the old one done", first.Err(), second.Err())
	}
	shown.Set(false)
	u.Frame()
	if second.Err() == nil {
		t.Error("the view went away, and the context of its requests is not done")
	}
}

// The text is parsed once, and again only when it changes.
func TestTextIsParsedOnce(t *testing.T) {
	var c docCache
	a := c.get("# A")
	if c.get("# A") != a {
		t.Error("the same text was parsed again")
	}
	if c.get("# B") == a {
		t.Error("a new text was not parsed")
	}
}

// Anchors are made as GitHub makes them.
func TestSlugs(t *testing.T) {
	sl := slugger{}
	for _, c := range []struct{ in, want string }{
		{"Getting Started", "getting-started"},
		{"What's new in v0.2?", "whats-new-in-v02"},
		{"Getting Started", "getting-started-1"},
		{"Ünïcode_names", "ünïcode_names"},
	} {
		if got := sl.slug(c.in); got != c.want {
			t.Errorf("slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
