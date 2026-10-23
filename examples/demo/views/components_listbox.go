package views

import (
	"fmt"
	"strings"

	"github.com/tyler-builds/jamb/ui"
)

// listBoxFruit is an item of the list box examples.
type listBoxFruit struct {
	Key, Name, Color string
}

// listBoxFruits are the items of the list box examples, in order.
var listBoxFruits = []listBoxFruit{
	{"apple", "Apple", "#dc2626"},
	{"banana", "Banana", "#eab308"},
	{"blueberry", "Blueberry", "#4f46e5"},
	{"cherry", "Cherry", "#be123c"},
	{"lime", "Lime", "#16a34a"},
	{"mango", "Mango", "#f59e0b"},
}

// listBoxRow draws a fruit: a colored square and its name.
func listBoxRow(f listBoxFruit, _ ui.PartState) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(8).Children(
		ui.Div().Size(ui.Px(10), ui.Px(10)).Radius(2).Background(ui.Hex(f.Color)),
		ui.Text(f.Name),
	)
}

// listBoxSection shows the list box: a single selection with its selected item
// reported, a multiple selection with Ctrl+click and Shift+click and a context
// menu, and a list with no selection that only moves through its items.
func listBoxSection(ctx *ui.Context, t *ui.Theme) ui.Element {
	single := ui.Local(ctx, "listbox.single", []string(nil))
	multi := ui.Local(ctx, "listbox.multi", []string(nil))
	last := ui.Local(ctx, "listbox.last", "")
	key := func(f listBoxFruit) string { return f.Key }
	name := func(f listBoxFruit) string { return f.Name }
	return section(t, "List box",
		sample(t, "Single selection, activated with Enter",
			ui.Div().Column().Gap(6).
				Child(ui.ListBox(listBoxFruits, key, single, listBoxRow).
					Label("Fruit").ItemName(name).Height(ui.Px(180)).Width(ui.Px(260)).
					OnActivate(func(f listBoxFruit) { last.Set("Activated " + f.Name) })).
				Child(ui.Text(listBoxSelectedText(single.Get())).TextColor(t.TextMuted).FontSize(13))),
		sample(t, "Multiple selection, with a context menu",
			ui.Div().Column().Gap(6).
				Child(ui.ListBox(listBoxFruits, key, multi, listBoxRow).
					Label("Fruit, several").ItemName(name).Selection(ui.SelectionMultiple).
					Height(ui.Px(180)).Width(ui.Px(260)).
					ContextMenu(func(f listBoxFruit) []ui.MenuItem {
						return []ui.MenuItem{{Label: "Forget " + f.Name, OnClick: func() {
							last.Set("Forgot " + f.Name)
						}}}
					})).
				Child(ui.Text(listBoxSelectedText(multi.Get())).TextColor(t.TextMuted).FontSize(13)).
				Child(ui.Text(last.Get()).TextColor(t.TextMuted).FontSize(13))),
	)
}

// listBoxSelectedText describes the keys of a selection for the examples.
func listBoxSelectedText(keys []string) string {
	if len(keys) == 0 {
		return "Nothing selected"
	}
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		for _, f := range listBoxFruits {
			if f.Key == k {
				names = append(names, f.Name)
			}
		}
	}
	return fmt.Sprintf("Selected: %s", strings.Join(names, ", "))
}
