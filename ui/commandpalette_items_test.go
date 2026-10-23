package ui

import (
	"slices"
	"testing"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// quickItems returns three items, as an app's branches, tags and commits might be. Each
// one's Run appends its key to ran.
func quickItems(ran *[]string) []PaletteItem {
	run := func(k string) func() { return func() { *ran = append(*ran, k) } }
	return []PaletteItem{
		{Key: "main", Label: "main", Detail: "branch", Run: run("main")},
		{Key: "v1", Label: "v1.0.0", Detail: "tag", Keywords: []string{"release"}, Run: run("v1")},
		{Key: "c1", Label: "Fix tab overflow", Detail: "a1b2c3d Alice", Icon: icons.Hash, Run: run("c1")},
	}
}

// quickRig is a palette with quickItems as its items, and the commands of the palette
// rig. queries records the query each time the items are asked for, and ran the items
// that ran.
func quickRig(t *testing.T, itemsOnly bool, queries *[]string, ran *[]string) *paletteRig {
	t.Helper()
	return newPaletteRig(t, func(p *CommandPaletteElement) *CommandPaletteElement {
		p.Items(func(q string) []PaletteItem {
			*queries = append(*queries, q)
			return quickItems(ran)
		})
		if itemsOnly {
			p.ItemsOnly()
		}
		return p
	})
}

func TestCommandPaletteItemsAreListedBeforeTheCommands(t *testing.T) {
	var queries []string
	var ran []string
	r := quickRig(t, false, &queries, &ran)
	r.openPalette(t)
	want := []string{"main", "v1.0.0", "Fix tab overflow", "Bold text", "Italic text", "New document"}
	if got := r.listed(); !slices.Equal(got, want) {
		t.Fatalf("listed %v, want %v", got, want)
	}
	if len(queries) == 0 || queries[len(queries)-1] != "" {
		t.Fatalf("the items were asked with queries %q, want the empty query", queries)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestCommandPaletteQueryMatchesItemsByLabelDetailAndKeywords(t *testing.T) {
	var queries []string
	var ran []string
	r := quickRig(t, false, &queries, &ran)
	r.openPalette(t)
	r.typeText("alice")
	if got := r.listed(); !slices.Equal(got, []string{"Fix tab overflow"}) {
		t.Fatalf("query alice listed %v, want the commit by its detail", got)
	}
	if queries[len(queries)-1] != "alice" {
		t.Fatalf("the items were last asked for %q, want the whole query alice", queries[len(queries)-1])
	}
	for range "alice" { // clear the query, then type a keyword
		r.press(input.KeyBackspace, 0)
	}
	r.typeText("release")
	if got := r.listed(); !slices.Equal(got, []string{"v1.0.0"}) {
		t.Fatalf("query release listed %v, want the tag by its keyword", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestCommandPaletteEnterRunsTheItemAndRestoresFocus(t *testing.T) {
	var queries []string
	var ran []string
	r := quickRig(t, false, &queries, &ran)
	r.openPalette(t)
	r.typeText("v1")
	r.press(input.KeyEnter, 0)
	if !slices.Equal(ran, []string{"v1"}) {
		t.Fatalf("ran %v, want [v1]", ran)
	}
	if r.open.Peek() {
		t.Fatalf("the palette is still open after running an item")
	}
	if got := r.focusedKey(); got != "doc" {
		t.Fatalf("focus on %q after the item ran, want the element that had it: doc", got)
	}
}

func TestCommandPaletteClickRunsAnItem(t *testing.T) {
	var queries []string
	var ran []string
	r := quickRig(t, false, &queries, &ran)
	r.openPalette(t)
	for _, n := range partNodes(r.rt, "CommandPalette.Item") {
		if n.div.acc.name == "Fix tab overflow" {
			rc := r.rt.nodeRect(n)
			r.clickAt(rc.X+rc.W/2, rc.Y+rc.H/2)
			break
		}
	}
	if !slices.Equal(ran, []string{"c1"}) {
		t.Fatalf("a click ran %v, want [c1]", ran)
	}
	if r.open.Peek() {
		t.Fatalf("the palette is still open after a click on an item")
	}
}

func TestCommandPaletteItemsOnlyListsItemsUntilTheQueryStartsWithGreater(t *testing.T) {
	var queries []string
	var ran []string
	r := quickRig(t, true, &queries, &ran)
	r.openPalette(t)
	if got := r.listed(); !slices.Equal(got, []string{"main", "v1.0.0", "Fix tab overflow"}) {
		t.Fatalf("items only listed %v, want the three items", got)
	}
	r.typeText(">")
	if got := r.listed(); !slices.Equal(got, []string{"Bold text", "Italic text", "New document"}) {
		t.Fatalf("\">\" listed %v, want the commands only", got)
	}
	r.typeText(" bold")
	if got := r.listed(); !slices.Equal(got, []string{"Bold text"}) {
		t.Fatalf("\"> bold\" listed %v, want Bold text", got)
	}
	r.press(input.KeyEnter, 0)
	if len(ran) != 0 {
		t.Fatalf("a command ran an item: %v", ran)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestCommandPaletteItemsOnlyWithNoMatchSaysNoResults(t *testing.T) {
	var queries []string
	var ran []string
	r := quickRig(t, true, &queries, &ran)
	r.openPalette(t)
	r.typeText("zzz")
	if got := r.listed(); len(got) != 0 {
		t.Fatalf("listed %v for a query that matches nothing", got)
	}
	if n := partNodes(r.rt, "CommandPalette.Empty"); len(n) != 1 {
		t.Fatalf("%d empty messages, want 1", len(n))
	}
}
