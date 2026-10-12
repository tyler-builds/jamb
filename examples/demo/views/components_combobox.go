package views

import (
	"context"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// comboCities are the options of the static and async examples of the combobox.
var comboCities = []ui.Option[string]{
	{Value: "baton-rouge", Label: "Baton Rouge"},
	{Value: "covington", Label: "Covington"},
	{Value: "hammond", Label: "Hammond"},
	{Value: "lafayette", Label: "Lafayette"},
	{Value: "lake-charles", Label: "Lake Charles"},
	{Value: "mandeville", Label: "Mandeville"},
	{Value: "monroe", Label: "Monroe"},
	{Value: "new-orleans", Label: "New Orleans"},
	{Value: "shreveport", Label: "Shreveport"},
	{Value: "slidell", Label: "Slidell"},
}

// comboFruits are the options of the restricted example and the validated field.
var comboFruits = []ui.Option[string]{
	{Value: "apple", Label: "Apple"},
	{Value: "apricot", Label: "Apricot"},
	{Value: "banana", Label: "Banana"},
	{Value: "blueberry", Label: "Blueberry"},
	{Value: "cherry", Label: "Cherry"},
}

// containsOptions returns the options whose label contains the query, ignoring case.
// It is the suggest function of the examples that have their options in hand.
func containsOptions(opts []ui.Option[string]) func(q string) []ui.Option[string] {
	return func(q string) []ui.Option[string] {
		q = strings.ToLower(strings.TrimSpace(q))
		var out []ui.Option[string]
		for _, o := range opts {
			if strings.Contains(strings.ToLower(o.Label), q) {
				out = append(out, o)
			}
		}
		return out
	}
}

// comboboxSection shows the combobox: one with its suggestions highlighted, one that
// accepts only its suggestions, one whose suggestions come in the background after a
// delay, and a validated field in a form.
func comboboxSection(ctx *ui.Context, t *ui.Theme) ui.Element {
	city := ui.Local(ctx, "combo.city", "")
	fruit := ui.Local(ctx, "combo.fruit", "")
	chosen := ui.Local(ctx, "combo.chosen", "")
	slow := ui.Local(ctx, "combo.slow", "")
	field := ui.Local(ctx, "combo.field", "")
	return section(t, "Combobox",
		sample(t, "Free text, matches highlighted",
			ui.Combobox(city, containsOptions(comboCities)).Label("City").Placeholder("Type a city").
				EmptyText("No matching cities").Width(ui.Px(260))),
		sample(t, "Choose only from the suggestions",
			ui.Div().Column().Gap(8).
				Child(ui.Combobox(fruit, containsOptions(comboFruits)).Label("Fruit, from the list").
					Placeholder("Type a fruit").AllowFree(false).Width(ui.Px(260)).
					OnPick(func(v string) { chosen.Set(v) })).
				Child(ui.Text(choiceText(chosen.Get())).Color(t.TextMuted).Size(13))),
		sample(t, "Suggestions in the background",
			ui.Combobox[string](slow, nil).Label("City, found in the background").Placeholder("Type a city").
				Width(ui.Px(260)).MinChars(1).
				SuggestAsync(func(c context.Context, q string) ([]ui.Option[string], error) {
					select {
					case <-time.After(700 * time.Millisecond):
						return containsOptions(comboCities)(q), nil
					case <-c.Done():
						return nil, c.Err()
					}
				})),
		sample(t, "In a field, with a rule",
			ui.Field("Favorite fruit", ui.Combobox(field, containsOptions(comboFruits)).
				Placeholder("Type a fruit").Validate(ui.Required[string]())).
				Description("Type a fruit and choose it, or leave the text as it is.").
				Width(ui.Px(260))))
}

// choiceText says what the restricted example last chose.
func choiceText(v string) string {
	if v == "" {
		return "Nothing chosen yet."
	}
	return "Chosen: " + v + "."
}
