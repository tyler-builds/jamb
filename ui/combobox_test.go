package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// comboFruits are the options of the combobox tests. Cherry is disabled.
var comboFruits = []Option[string]{
	{Value: "apple", Label: "Apple"},
	{Value: "apricot", Label: "Apricot"},
	{Value: "banana", Label: "Banana"},
	{Value: "blueberry", Label: "Blueberry"},
	{Value: "cherry", Label: "Cherry", Disabled: true},
}

// filterFruits returns the options whose label contains q, ignoring case.
func filterFruits(q string) []Option[string] {
	var out []Option[string]
	for _, o := range comboFruits {
		if strings.Contains(strings.ToLower(o.Label), strings.ToLower(q)) {
			out = append(out, o)
		}
	}
	return out
}

// comboRig is an access rig with a combobox keyed "fruit" and its text.
type comboRig struct {
	*accessRig
	text  *Signal[string]
	picks []string
	calls []string // the queries that suggest was asked for
	heard []string // the announcements the bridge delivered, see listen
}

// listen collects the announcements that the bridge delivers, as an assistive
// technology gets them.
func (r *comboRig) listen() {
	r.bridge.Listen(func(u access.Update) {
		for _, a := range u.Announcements {
			r.heard = append(r.heard, a.Text)
		}
	})
}

// announced returns the announcements heard since the last call, and forgets them. The
// bridge delivers them only with an update, so the queue is taken too.
func (r *comboRig) announced() []string {
	for _, a := range takeAnnouncements() {
		r.heard = append(r.heard, a.Text)
	}
	h := r.heard
	r.heard = nil
	return h
}

// newComboRig shows a combobox of fruits, configured by cfg.
func newComboRig(t *testing.T, cfg func(*ComboboxElement[string]) *ComboboxElement[string]) *comboRig {
	t.Helper()
	cr := &comboRig{text: NewSignal("")}
	build := func() *ComboboxElement[string] {
		c := Combobox(cr.text, func(q string) []Option[string] {
			cr.calls = append(cr.calls, q)
			return filterFruits(q)
		}).Key("fruit").Label("Fruit").Placeholder("Type a fruit")
		c.OnPick(func(v string) { cr.picks = append(cr.picks, v) })
		if cfg != nil {
			c = cfg(c)
		}
		return c
	}
	cr.accessRig = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(build())
	})
	cr.listen()
	return cr
}

// wait lets the frames run until cond holds, for the timers of the combobox.
func (r *comboRig) wait(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
		r.frame()
	}
}

// listed returns the labels of the rows of the open list, in reading order, as the
// accessibility tree gives them.
func (r *comboRig) listed() []string {
	var out []string
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleListItem {
			out = append(out, n.Name)
		}
		return true
	})
	return out
}

// highlighted returns the label of the highlighted row, or "".
func (r *comboRig) highlighted() string {
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.acc != nil && n.div.acc.role == access.RoleListItem && n.div.acc.highlighted {
			return n.div.acc.name
		}
	}
	return ""
}

// expanded reports whether the combobox's list is open, from its access state.
func (r *comboRig) expanded(t *testing.T) bool {
	t.Helper()
	return r.nodeByKey(t, "fruit").div.acc.expanded
}

// rowCenter returns a point inside the row labelled label.
func (r *comboRig) rowCenter(t *testing.T, label string) (float32, float32) {
	t.Helper()
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.acc != nil && n.div.acc.role == access.RoleListItem && n.div.acc.name == label {
			rc := r.rt.nodeRect(n)
			return rc.X + rc.W/2, rc.Y + rc.H/2
		}
	}
	t.Fatalf("no row %q", label)
	return 0, 0
}

// editorFocused reports whether the text has keyboard focus.
func (r *comboRig) editorFocused() bool { return r.focusedKey() == "fruit" }

// focusCombo gives the text keyboard focus, as a Tab would.
func (r *comboRig) focusCombo(t *testing.T) {
	t.Helper()
	r.focusKey(t, "fruit")
	r.rt.keyboardModality = true
}

// partNodeOrNil returns the first node of the part, or nil.
func partNodeOrNil(rt *Runtime, part string) *node {
	for _, n := range rt.byID {
		if n.div != nil && n.div.part == part {
			return n
		}
	}
	return nil
}

// nodeText returns the text under n, in its subtree.
func nodeText(n *node) string {
	if n.textEl != nil {
		return n.textEl.text
	}
	var b strings.Builder
	for _, c := range n.children {
		b.WriteString(nodeText(c))
	}
	return b.String()
}

func TestComboboxTypingOpensAndFilters(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	if !r.expanded(t) {
		t.Fatal("typing did not open the list")
	}
	if got := strings.Join(r.listed(), ","); got != "Apple,Apricot" {
		t.Fatalf("list = %q, want Apple,Apricot", got)
	}
	if r.highlighted() != "" {
		t.Fatalf("typing highlighted %q; nothing is highlighted until a key moves it", r.highlighted())
	}
	if !r.editorFocused() {
		t.Fatalf("focus left the text: %q", r.focusedKey())
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestComboboxHighlightMovesWithFocusInText(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyDown, 0)
	if r.highlighted() != "Apple" {
		t.Fatalf("Down highlighted %q, want Apple", r.highlighted())
	}
	r.press(input.KeyDown, 0)
	if r.highlighted() != "Apricot" {
		t.Fatalf("second Down highlighted %q, want Apricot", r.highlighted())
	}
	r.press(input.KeyDown, 0)
	if r.highlighted() != "Apricot" {
		t.Fatalf("Down past the last suggestion moved to %q", r.highlighted())
	}
	r.press(input.KeyUp, 0)
	r.press(input.KeyUp, 0)
	if r.highlighted() != "" {
		t.Fatalf("Up from the first suggestion highlighted %q, want none (back to the text)", r.highlighted())
	}
	if !r.editorFocused() {
		t.Fatalf("the keys moved focus to %q", r.focusedKey())
	}
	// The highlighted suggestion is the focused node in the accessibility tree: the
	// active descendant, with DOM focus still in the text.
	r.press(input.KeyDown, 0)
	tr := r.tree()
	apple := accesstest.Find(t, tr, access.RoleListItem, "Apple")
	if tr.Focus != apple.ID {
		t.Fatalf("the focused node is %d, want the Apple row %d", tr.Focus, apple.ID)
	}
}

func TestComboboxEnterChoosesHighlighted(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyDown, 0)
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0)
	if r.text.Peek() != "Apricot" {
		t.Fatalf("Enter set the text to %q, want Apricot", r.text.Peek())
	}
	if len(r.picks) != 1 || r.picks[0] != "apricot" {
		t.Fatalf("OnPick ran with %v, want [apricot]", r.picks)
	}
	if r.expanded(t) {
		t.Fatal("the list stayed open after a pick")
	}
	if !r.editorFocused() {
		t.Fatalf("focus left the text after a pick: %q", r.focusedKey())
	}
}

func TestComboboxEnterWithoutHighlightKeepsFreeText(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyEnter, 0) // no suggestion is highlighted
	if r.text.Peek() != "ap" || len(r.picks) != 0 {
		t.Fatalf("Enter without a highlight changed the text to %q, picks %v", r.text.Peek(), r.picks)
	}
	if r.expanded(t) {
		t.Fatal("Enter did not close the list")
	}
}

func TestComboboxEscapeClosesThenClears(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyEscape, 0)
	if r.expanded(t) {
		t.Fatal("the first Escape did not close the list")
	}
	if r.text.Peek() != "ap" || !r.editorFocused() {
		t.Fatalf("the first Escape changed the text to %q or left the field (%q)", r.text.Peek(), r.focusedKey())
	}
	r.press(input.KeyEscape, 0)
	if r.text.Peek() != "" {
		t.Fatalf("the second Escape left the text %q, want it cleared", r.text.Peek())
	}
	if !r.editorFocused() {
		t.Fatal("clearing the text took focus away")
	}
}

func TestComboboxEscapeWithEmptyTextLeavesField(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.press(input.KeyEscape, 0)
	if r.editorFocused() {
		t.Fatal("Escape with an empty text and a closed list should leave the field (the editor's Cancel)")
	}
}

func TestComboboxTabClosesWithoutChoosing(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyDown, 0)
	r.press(input.KeyTab, 0)
	if r.expanded(t) {
		t.Fatal("Tab did not close the list")
	}
	if r.text.Peek() != "ap" || len(r.picks) != 0 {
		t.Fatalf("Tab chose %q (picks %v); it must only close the list", r.text.Peek(), r.picks)
	}
}

func TestComboboxAltArrowsOpenAndClose(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyEscape, 0) // close
	r.press(input.KeyDown, input.ModAlt)
	if !r.expanded(t) {
		t.Fatal("Alt+Down did not open the list")
	}
	if r.highlighted() != "" {
		t.Fatalf("Alt+Down highlighted %q; it must not move the highlight", r.highlighted())
	}
	r.press(input.KeyUp, input.ModAlt)
	if r.expanded(t) {
		t.Fatal("Alt+Up did not close the list")
	}
	if !r.editorFocused() {
		t.Fatal("Alt+arrows moved focus")
	}
}

func TestComboboxHomeAndEndStayInText(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyHome, 0)
	r.press(input.KeyEnd, 0)
	if !r.expanded(t) || !r.editorFocused() {
		t.Fatal("Home and End should move in the text and leave the list alone")
	}
}

func TestComboboxClickPicksAndKeepsFocus(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	x, y := r.rowCenter(t, "Apricot")
	r.clickAt(x, y)
	if r.text.Peek() != "Apricot" || len(r.picks) != 1 {
		t.Fatalf("clicking Apricot set %q with picks %v", r.text.Peek(), r.picks)
	}
	if r.expanded(t) {
		t.Fatal("the list stayed open after a click")
	}
	if !r.editorFocused() {
		t.Fatalf("the click moved focus to %q, want the text", r.focusedKey())
	}
}

func TestComboboxDisabledRowCannotBeChosen(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("e") // Apple, Blueberry and the disabled Cherry
	for range 6 {
		r.press(input.KeyDown, 0)
	}
	if r.highlighted() == "Cherry" {
		t.Fatal("the highlight reached a disabled suggestion")
	}
}

func TestComboboxMinCharsKeepsListClosed(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] { return c.MinChars(2) })
	r.focusCombo(t)
	r.typeText("a")
	if r.expanded(t) {
		t.Fatal("the list opened before MinChars")
	}
	r.typeText("p")
	if !r.expanded(t) {
		t.Fatal("the list did not open at MinChars")
	}
}

func TestComboboxEmptySlot(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] {
		return c.EmptyText("No matching fruit")
	})
	r.focusCombo(t)
	r.typeText("zzz")
	if len(r.listed()) != 0 {
		t.Fatalf("rows %v for a query with no match", r.listed())
	}
	if partNodeOrNil(r.rt, "Combobox.Empty") == nil {
		t.Fatal("no empty slot for a query with no match")
	}
	if !r.expanded(t) {
		t.Fatal("the list should stay open with its empty slot")
	}
}

func TestComboboxAnnouncesCount(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.announced()
	r.typeText("ap")
	r.wait(t, func() bool {
		for _, s := range r.announced() {
			if s == "2 suggestions" {
				return true
			}
		}
		return false
	})
}

func TestComboboxRestrictedRevertsOnBlur(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] { return c.AllowFree(false) })
	r.focusCombo(t)
	r.typeText("xyz")
	r.rt.setFocus(0) // focus leaves the field
	r.frame()
	if r.text.Peek() != "" {
		t.Fatalf("restricted field kept %q on blur", r.text.Peek())
	}
}

func TestComboboxRestrictedKeepsChosenOnBlur(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] { return c.AllowFree(false) })
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0)
	r.rt.setFocus(0)
	r.frame()
	if r.text.Peek() != "Apple" {
		t.Fatalf("the chosen text was reverted to %q", r.text.Peek())
	}
}

func TestComboboxRestrictedRevertsEditedChoice(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] { return c.AllowFree(false) })
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0)
	r.typeText("z") // the chosen text is edited, so it is no longer the choice
	r.rt.setFocus(0)
	r.frame()
	if r.text.Peek() != "Apple" {
		t.Fatalf("an edited choice was reverted to %q, want the chosen label Apple", r.text.Peek())
	}
}

func TestComboboxFreeKeepsText(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("kiwi")
	r.rt.setFocus(0)
	r.frame()
	if r.text.Peek() != "kiwi" {
		t.Fatalf("free text lost on blur: %q", r.text.Peek())
	}
}

func TestComboboxDebounceAsksOnce(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] {
		return c.Debounce(40 * time.Millisecond)
	})
	r.focusCombo(t)
	r.typeText("a")
	r.typeText("p")
	r.typeText("p")
	if len(r.calls) != 0 {
		t.Fatalf("suggest ran during the debounce: %v", r.calls)
	}
	r.wait(t, func() bool { return len(r.calls) > 0 })
	if len(r.calls) != 1 || r.calls[0] != "app" {
		t.Fatalf("suggest calls = %v, want one call for app", r.calls)
	}
}

// asyncGate holds the requests of an async combobox, by query, for the test to
// answer. The suggest function runs on its own goroutine, so the map is locked.
type asyncGate struct {
	mu sync.Mutex
	m  map[string]chan []Option[string]
}

// request returns the channel that the request for q waits on.
func (g *asyncGate) request(q string) chan []Option[string] {
	g.mu.Lock()
	defer g.mu.Unlock()
	ch := make(chan []Option[string], 1)
	g.m[q] = ch
	return ch
}

// pending reports whether a request for q has started.
func (g *asyncGate) pending(q string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.m[q] != nil
}

// answer completes the request for q with v.
func (g *asyncGate) answer(q string, v []Option[string]) {
	g.mu.Lock()
	ch := g.m[q]
	g.mu.Unlock()
	ch <- v
}

// newAsyncRig is a combobox whose suggestions come from a gate the test answers.
func newAsyncRig(t *testing.T) (*comboRig, *asyncGate) {
	t.Helper()
	gate := &asyncGate{m: map[string]chan []Option[string]{}}
	cr := &comboRig{text: NewSignal("")}
	cr.accessRig = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Combobox[string](cr.text, nil).Key("fruit").Label("Fruit").
			SuggestAsync(func(c context.Context, q string) ([]Option[string], error) {
				ch := gate.request(q)
				select {
				case v := <-ch:
					return v, nil
				case <-c.Done():
					return nil, c.Err()
				}
			}))
	})
	cr.listen()
	return cr, gate
}

func TestComboboxAsyncStaleResultsAreDropped(t *testing.T) {
	r, gate := newAsyncRig(t)
	r.focusCombo(t)
	r.typeText("a")
	r.wait(t, func() bool { return gate.pending("a") })
	r.typeText("p")
	r.wait(t, func() bool { return gate.pending("ap") })
	gate.answer("ap", []Option[string]{{Value: "apple", Label: "Apple"}})
	r.wait(t, func() bool { return len(r.listed()) == 1 })
	// The request for "a" is stale: its answer must not replace the one for "ap".
	gate.answer("a", []Option[string]{{Value: "banana", Label: "Banana"}, {Value: "apricot", Label: "Apricot"}})
	time.Sleep(30 * time.Millisecond)
	r.frame()
	if got := strings.Join(r.listed(), ","); got != "Apple" {
		t.Fatalf("a stale result replaced the current one: %q", got)
	}
}

func TestComboboxAsyncShowsSpinnerWhileLoading(t *testing.T) {
	r, gate := newAsyncRig(t)
	r.focusCombo(t)
	r.typeText("a")
	r.wait(t, func() bool { return gate.pending("a") })
	if partNodeOrNil(r.rt, "Combobox.Loading") == nil {
		t.Fatal("no loading row while suggestions are being found")
	}
	gate.answer("a", nil)
	r.wait(t, func() bool { return partNodeOrNil(r.rt, "Combobox.Loading") == nil })
}

func TestComboboxValidateInField(t *testing.T) {
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Field("Fruit", Combobox(text, filterFruits).Key("fruit").
			Validate(Required[string]())))
	})
	r.focusKey(t, "fruit")
	r.typeText("a")
	r.press(input.KeyBackspace, 0)
	r.rt.setFocus(0) // the field is left after a change, so its error is shown
	r.frame()
	n := partNodeOrNil(r.rt, "FormField.Error")
	if n == nil || n.div.noDisplay {
		t.Fatal("the required error is not shown after the text was emptied and left")
	}
	if !strings.Contains(nodeText(n), "This field is required.") {
		t.Fatalf("error text %q", nodeText(n))
	}
}

func TestComboboxDisabledIsNotEditable(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Combobox(NewSignal("Apple"), filterFruits).Key("fruit").Label("Fruit").Disabled(true))
	})
	for _, n := range r.rt.byID {
		if n.key == "fruit" {
			if f := firstFocusable(n); f != nil {
				t.Fatal("a disabled combobox has a focusable element")
			}
		}
	}
	if !r.nodeByKey(t, "fruit").div.acc.disabled {
		t.Fatal("a disabled combobox is not reported disabled")
	}
}

func TestComboboxAccessTree(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	tr := r.tree()
	box := accesstest.Find(t, tr, access.RoleComboBox, "Fruit")
	if !box.States.Has(access.StateExpanded) {
		t.Fatalf("the combobox does not report expanded: [%s]", box.States)
	}
	accesstest.Find(t, tr, access.RoleTextField, "Fruit")
	apple := accesstest.Find(t, tr, access.RoleListItem, "Apple")
	if !apple.States.Has(access.StateSelectable) {
		t.Fatal("a suggestion is not selectable (SelectionItem)")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestComboboxRenderOptionKeepsLabel(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] {
		return c.RenderOption(func(o Option[string], s PartState) Element {
			return Text("* " + o.Label)
		})
	})
	r.focusCombo(t)
	r.typeText("ap")
	accesstest.Find(t, r.tree(), access.RoleListItem, "Apple")
}

func TestComboboxHighlightsMatches(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	found := false
	for _, n := range r.rt.byID {
		if n.textEl != nil && n.textEl.text == "Apple" && len(n.textEl.highlights) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("the match in Apple is not highlighted")
	}
}

// The list is at least as wide as the field, whatever its width.
func TestComboboxListMatchesFieldWidth(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] {
		return c.Width(Px(300))
	})
	r.focusCombo(t)
	r.typeText("ap")
	popup := partNodeOrNil(r.rt, "Combobox.Popup")
	if popup == nil || !popup.div.matchAnchor {
		t.Fatal("the popup does not match the width of the field")
	}
}

// partCombobox is a combobox of the fruits with the instance style st.
func partCombobox(text *Signal[string], st ComboboxStyle) Element {
	return Div().Column().Child(Combobox(text, filterFruits).Key("fruit").Label("Fruit").
		Placeholder("Type a fruit").Styles(st))
}

// testComboboxParts sets a distinctive style on each part of the combobox, puts the
// state of the part in effect, and asserts that the style reaches the node that draws
// it (see TestEveryPartIsApplied).
func testComboboxParts(t *testing.T, applied map[string]bool) {
	type comboCase struct {
		part    string
		set     func(*ComboboxStyle, Style)
		text    string // the text typed, to open the list
		down    bool   // move the highlight to the first suggestion
		row     string // for an option part: the row that draws it
		colored bool   // the part is colored by its text color
		drawn   string // the node that draws the part, when not the part's own
	}
	cases := []comboCase{
		{part: "Combobox.Field", set: func(s *ComboboxStyle, m Style) { s.Field.Frame = m }, drawn: "Combobox.Frame"},
		{part: "Combobox.Popup", set: func(s *ComboboxStyle, m Style) { s.Popup = m }, text: "ap"},
		{part: "Combobox.Option", set: func(s *ComboboxStyle, m Style) { s.Option = m }, text: "ap", row: "Apricot"},
		{part: "Combobox.OptionHighlighted", set: func(s *ComboboxStyle, m Style) { s.OptionHighlighted = m },
			text: "ap", down: true, row: "Apple"},
		{part: "Combobox.OptionDisabled", set: func(s *ComboboxStyle, m Style) { s.OptionDisabled = m }, text: "e", row: "Cherry"},
		{part: "Combobox.Empty", set: func(s *ComboboxStyle, m Style) { s.Empty = m }, text: "zz", colored: true, drawn: "Combobox.Empty"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ComboboxStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			text := NewSignal("")
			cr := &comboRig{text: text}
			cr.accessRig = newAccessRig(func(ctx *Context) Element { return partCombobox(text, st) })
			cr.focusCombo(t)
			if c.text != "" {
				cr.typeText(c.text)
			}
			if c.down {
				cr.press(input.KeyDown, 0)
			}
			switch {
			case c.row != "":
				wantBackground(t, accessNode(t, cr.accessRig, access.RoleListItem, c.row), c.part, markBG)
			case c.colored:
				wantTextColor(t, partNode(t, cr.rt, c.drawn, ""), c.part, markFG)
			default:
				drawn := c.drawn
				if drawn == "" {
					drawn = strings.TrimPrefix(c.part, "Combobox.")
					drawn = "Combobox." + drawn
				}
				wantBackground(t, partNode(t, cr.rt, drawn, ""), c.part, markBG)
			}
		})
	}
	// The loading row, while an async request is pending.
	applied["Combobox.Loading"] = true
	t.Run("Combobox.Loading", func(t *testing.T) {
		var st ComboboxStyle
		st.Loading = Style{Background: markBG}
		text := NewSignal("")
		cr := &comboRig{text: text}
		block := make(chan struct{})
		t.Cleanup(func() { close(block) })
		cr.accessRig = newAccessRig(func(ctx *Context) Element {
			return Div().Column().Child(Combobox[string](text, nil).Key("fruit").Label("Fruit").Styles(st).
				SuggestAsync(func(c context.Context, q string) ([]Option[string], error) {
					<-block
					return nil, nil
				}))
		})
		cr.focusCombo(t)
		cr.typeText("a")
		wantBackground(t, partNode(t, cr.rt, "Combobox.Loading", ""), "Combobox.Loading", markBG)
	})
	// The error row, when an async request fails.
	applied["Combobox.Error"] = true
	t.Run("Combobox.Error", func(t *testing.T) {
		var st ComboboxStyle
		st.Error = Style{Background: markBG}
		text := NewSignal("")
		cr := &comboRig{text: text}
		cr.accessRig = newAccessRig(func(ctx *Context) Element {
			return Div().Column().Child(Combobox[string](text, nil).Key("fruit").Label("Fruit").Styles(st).
				SuggestAsync(func(c context.Context, q string) ([]Option[string], error) {
					return nil, errors.New("offline")
				}))
		})
		cr.focusCombo(t)
		cr.typeText("a")
		cr.wait(t, func() bool { return partNodeOrNil(cr.rt, "Combobox.Error") != nil })
		wantBackground(t, partNode(t, cr.rt, "Combobox.Error", ""), "Combobox.Error", markBG)
	})
	// The match is drawn in the labels: the highlight of the match in Apple has the
	// Match background.
	applied["Combobox.Match"] = true
	t.Run("Combobox.Match", func(t *testing.T) {
		var st ComboboxStyle
		st.Match = Style{Background: markBG}
		text := NewSignal("")
		cr := &comboRig{text: text}
		cr.accessRig = newAccessRig(func(ctx *Context) Element { return partCombobox(text, st) })
		cr.focusCombo(t)
		cr.typeText("ap")
		for _, n := range cr.rt.byID {
			if n.textEl != nil && n.textEl.text == "Apple" {
				if len(n.textEl.highlights) == 0 || n.textEl.highlights[0].hl.Background != markBG {
					t.Fatalf("the match in Apple is not drawn with the Match background")
				}
				return
			}
		}
		t.Fatal("no Apple label")
	})
}

// The field is 240 px wide at the Text size of 100 percent by default, and the sizing
// methods change it.
func TestComboboxSizing(t *testing.T) {
	r := newComboRig(t, nil)
	if w := r.nodeByKey(t, "fruit").style.Width; w != Px(textFieldWidth) {
		t.Fatalf("default width %v, want %d px", w, textFieldWidth)
	}
	r = newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] { return c.Width(Px(300)) })
	if w := r.nodeByKey(t, "fruit").style.Width; w != Px(300) {
		t.Fatalf("width %v, want 300 px", w)
	}
	r = newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] { return c.Width(Full) })
	if w := r.nodeByKey(t, "fruit").style.Width; w != Full {
		t.Fatalf("width %v, want Full", w)
	}
}

// Under high contrast the highlighted suggestion is drawn in the system Highlight
// colors, and the match is underlined rather than colored (see ComboboxStyle).
func TestComboboxHighContrastHighlight(t *testing.T) {
	th := *DarkTheme
	th.HighContrast = true
	text := NewSignal("")
	cr := &comboRig{text: text}
	cr.accessRig = &accessRig{now: time.Unix(1_000_000, 0)}
	cr.rt = NewRuntime(func(ctx *Context) Element { return partCombobox(text, ComboboxStyle{}) }, &th, nil)
	cr.rt.clock = func() time.Time { return cr.now }
	cr.rt.SetWarnFunc(func(s string) { cr.warnings = append(cr.warnings, s) })
	cr.rt.SetAccessWarnings(true)
	cr.rt.SetViewport(500, 400, 1)
	cr.rt.Frame(cr.now)
	cr.bridge = cr.rt.AccessBridge()
	cr.bridge.Activate()
	cr.focusCombo(t)
	cr.typeText("ap")
	cr.press(input.KeyDown, 0)
	for _, n := range cr.rt.byID {
		if n.div != nil && n.div.part == "Combobox.Option" && n.div.acc != nil && n.div.acc.highlighted {
			if n.style.Background != th.Accent || n.style.TextColor != th.AccentText {
				t.Fatalf("the highlighted suggestion is %v on %v, want AccentText on Accent", n.style.TextColor, n.style.Background)
			}
			if n.style.FontWeight != 0 {
				t.Fatalf("the highlight changes the font weight to %v; it must not", n.style.FontWeight)
			}
			return
		}
	}
	t.Fatal("no highlighted suggestion")
}
func TestComboboxMaxVisibleScrollsAllRows(t *testing.T) {
	var many []Option[string]
	for i := range 50 {
		many = append(many, Option[string]{Value: fmt.Sprint(i), Label: fmt.Sprintf("Item %02d", i)})
	}
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Combobox(text, func(q string) []Option[string] { return many }).
			Key("fruit").Label("Fruit").MaxVisible(8))
	})
	r.focusKey(t, "fruit")
	r.typeText("i")
	popup := partNodeOrNil(r.rt, "Combobox.Popup")
	if popup == nil {
		t.Fatal("no popup")
	}
	if got := popup.style.MaxHeight; got != Px(8*comboboxRowHeight+8) {
		t.Fatalf("popup max height %v, want room for 8 rows", got)
	}
	rows := 0
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleListItem {
			rows++
		}
		return true
	})
	if rows != 50 {
		t.Fatalf("%d rows in the tree, want all 50", rows)
	}
	for range 20 {
		r.press(input.KeyDown, 0)
	}
	row := accessNode(t, r, access.RoleListItem, "Item 19")
	for range 60 { // the scroll eases to the row
		r.frame()
	}
	rr := r.rt.nodeRect(row)
	pr := r.rt.nodeRect(popup)
	if rr.Y < pr.Y || rr.Y+rr.H > pr.Y+pr.H {
		t.Fatalf("the highlighted row %v is outside the list %v", rr, pr)
	}
}

func TestComboboxClickOpensAndNeverCloses(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyEscape, 0)
	if r.expanded(t) {
		t.Fatal("Escape did not close the list")
	}
	editor := partNodeOrNil(r.rt, "Combobox.Text")
	rc := r.rt.nodeRect(editor)
	x, y := rc.X+rc.W/2, rc.Y+rc.H/2
	r.clickAt(x, y)
	if !r.expanded(t) {
		t.Fatal("a click in the text did not open the list")
	}
	r.clickAt(x, y)
	if !r.expanded(t) {
		t.Fatal("a click in the text closed the list; a press places the caret and must not close it")
	}
}

func TestComboboxExpandCollapseThroughPattern(t *testing.T) {
	r := newComboRig(t, nil)
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyEscape, 0)
	box := accesstest.Find(t, r.tree(), access.RoleComboBox, "Fruit")
	if n := r.nodeByKey(t, "fruit"); n.div.acc.setExpanded == nil {
		t.Fatal("the combobox has no expand handler of its own; Expand would run the click")
	}
	r.bridge.Perform(box.ID, access.ActionExpand)
	r.frame()
	if !r.expanded(t) {
		t.Fatal("Expand from assistive technology did not open the list")
	}
	r.bridge.Perform(box.ID, access.ActionCollapse)
	r.frame()
	if r.expanded(t) {
		t.Fatal("Collapse from assistive technology did not close the list")
	}
	if !r.editorFocused() {
		t.Fatalf("expand and collapse moved focus to %q", r.focusedKey())
	}
}

// The async error row replaces the empty slot, is announced, and leaves the text usable
// for free text.
func TestComboboxAsyncErrorShowsErrorRow(t *testing.T) {
	var mu sync.Mutex
	pending := map[string]chan error{}
	text := NewSignal("")
	cr := &comboRig{text: text}
	cr.accessRig = newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Combobox[string](text, nil).Key("fruit").Label("Fruit").ErrorText("Offline").
			SuggestAsync(func(c context.Context, q string) ([]Option[string], error) {
				ch := make(chan error, 1)
				mu.Lock()
				pending[q] = ch
				mu.Unlock()
				return nil, <-ch
			}))
	})
	cr.listen()
	cr.focusCombo(t)
	cr.announced()
	cr.typeText("k")
	cr.wait(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return pending["k"] != nil
	})
	mu.Lock()
	pending["k"] <- errors.New("offline")
	mu.Unlock()
	cr.wait(t, func() bool { return partNodeOrNil(cr.rt, "Combobox.Error") != nil })
	if n := partNodeOrNil(cr.rt, "Combobox.Error"); nodeText(n) != "Offline" {
		t.Fatalf("error row says %q, want the ErrorText", nodeText(n))
	}
	if partNodeOrNil(cr.rt, "Combobox.Empty") != nil {
		t.Fatal("the empty slot shows on an error; it must say the suggestions failed")
	}
	found := false
	for _, s := range cr.announced() {
		found = found || s == "Offline"
	}
	if !found {
		t.Fatal("the error was not announced")
	}
	cr.typeText("iwi")
	cr.press(input.KeyEnter, 0)
	if cr.text.Peek() != "kiwi" {
		t.Fatalf("after the error the free text is %q, want it kept as typed", cr.text.Peek())
	}
}

// The default error text is the one the brief names.
func TestComboboxDefaultErrorText(t *testing.T) {
	c := Combobox[string](NewSignal(""), nil)
	if c.errorText != "Couldn't load suggestions" {
		t.Fatalf("default error text %q", c.errorText)
	}
}

// A restricted field says what it reverted or cleared, aloud and on the field.
func TestComboboxRestrictedRevertIsAnnounced(t *testing.T) {
	r := newComboRig(t, func(c *ComboboxElement[string]) *ComboboxElement[string] { return c.AllowFree(false) })
	r.focusCombo(t)
	r.announced()
	r.typeText("xyz")
	r.rt.setFocus(0)
	r.frame()
	got := r.announced()
	if len(got) != 1 || got[0] != "Not in the list; cleared" {
		t.Fatalf("announcements %v, want the cleared notice", got)
	}
	if !nodeTextContains(r.rt, "Not in the list; cleared") {
		t.Fatal("the notice is not shown on the field")
	}
	if d := r.tree(); accesstest.Find(t, d, access.RoleComboBox, "Fruit").Description != "Not in the list; cleared" {
		t.Fatalf("the field's description is %q", accesstest.Find(t, d, access.RoleComboBox, "Fruit").Description)
	}
	// A choice that is edited is reverted to the choice, and the notice says so.
	r.focusCombo(t)
	r.typeText("ap")
	r.press(input.KeyDown, 0)
	r.press(input.KeyEnter, 0)
	r.announced()
	r.typeText("q")
	r.rt.setFocus(0)
	r.frame()
	got = r.announced()
	if len(got) != 1 || got[0] != "Not in the list; reverted to Apple" {
		t.Fatalf("announcements %v, want the reverted notice", got)
	}
	if r.text.Peek() != "Apple" {
		t.Fatalf("the text is %q, want Apple", r.text.Peek())
	}
}

// nodeTextContains reports whether a text node under the runtime has the text.
func nodeTextContains(rt *Runtime, s string) bool {
	for _, n := range rt.byID {
		if n.textEl != nil && n.textEl.text == s {
			return true
		}
	}
	return false
}

// With reduced motion the list opens and closes without frames of motion: it is drawn at
// once and gone at once. Without it, the same close keeps the list for its exit.
func TestComboboxReducedMotion(t *testing.T) {
	for _, reduced := range []bool{true, false} {
		r := newComboRig(t, nil)
		r.rt.sys.ReducedMotion = reduced
		r.focusCombo(t)
		r.typeText("ap")
		if reduced && (r.rt.animating || len(r.rt.animInsts) != 0) {
			t.Fatal("opening the list with reduced motion requested an animation")
		}
		if partNodeOrNil(r.rt, "Combobox.Popup") == nil {
			t.Fatal("no popup after typing")
		}
		r.press(input.KeyEscape, 0)
		// A list that is closing is drawn as a ghost while its exit motion runs.
		closing := len(r.rt.ghosts) > 0
		if reduced && closing {
			t.Fatal("with reduced motion the closed list is still drawn for its exit")
		}
		if !reduced && !closing {
			t.Fatal("without reduced motion the closing list has no exit motion")
		}
	}
}

// Clear removes the defaults of the combobox's parts, as it does for the other widgets.
func TestComboboxClearRemovesDefaults(t *testing.T) {
	text := NewSignal("")
	st := ComboboxStyle{
		Popup:             Style{Clear: ClearShadows},
		OptionHighlighted: Style{Clear: ClearBackground},
		Field:             FieldStyle{Frame: Style{Clear: ClearBorder}},
	}
	r := newAccessRig(func(ctx *Context) Element { return partCombobox(text, st) })
	r.focusKey(t, "fruit")
	r.typeText("app")
	r.press(input.KeyDown, 0) // highlights Apple, whose background is the highlight's default
	if n := partNode(t, r.rt, "Combobox.Popup", ""); len(n.style.Shadows) != 0 {
		t.Fatalf("the popup keeps %d shadows under ClearShadows", len(n.style.Shadows))
	}
	if n := accessNode(t, r, access.RoleListItem, "Apple"); !n.style.Background.IsZero() {
		t.Fatal("the option keeps its background under ClearBackground")
	}
	if n := partNode(t, r.rt, "Combobox.Frame", ""); n.style.BorderWidth != 0 {
		t.Fatal("the frame keeps its border under ClearBorder")
	}
}
