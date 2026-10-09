package views

import (
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// tagChips is the sidebar's row of every tag in use. Clicking a chip filters
// the list to notes with that tag; clicking the active chip clears it. Dropping
// a note on a chip adds that tag to the note.
func (a *App) tagChips() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		tags := a.s.AllTags()
		active := a.s.TagFilter.Get()
		if len(tags) == 0 {
			return nil
		}
		chips := ui.Div().Row().Wrap().Gap(6).PaddingX(12).PaddingY(6).Shrink(0)
		for _, tag := range tags {
			on := tag == active
			chips.Child(ui.Div().Row().Align(ui.Center).Gap(3).PaddingX(9).PaddingY(3).Radius(999).
				Border(1, t.Border).Cursor(ui.CursorPointer).Transition(fast).
				Hover(ui.Style{Background: t.SurfaceHover}).
				When(on, func(d *ui.DivElement) {
					d.Background(t.AccentSoft).Border(1, t.Accent).TextColor(t.Accent)
				}).
				Focusable().FocusVisible(focusRing(t)). // Tab reaches it; Enter toggles it
				OnClick(func() { a.s.ToggleTagFilter(tag) }).
				OnDrop(ui.Drop(func(n *store.Note, e *ui.DropEvent) { a.s.AddTag(n, tag) }).
					Over(ui.Style{Background: t.AccentSoft, BorderColor: t.Accent})).
				Key("chip-" + tag).
				Child(ui.Icon(icons.Hash).Size(11)).
				Child(ui.Text(tag).Size(12).Medium().NoWrap()))
		}
		return chips
	})
}

// focusRing is the keyboard focus ring for rows and chips: a spread shadow in
// the theme's focus color, so it adds no layout shift.
func focusRing(t *ui.Theme) ui.Style {
	return ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}}
}

// tagEditor shows a note's tags as removable pills plus an "Add tag" field.
func (a *App) tagEditor(n *store.Note) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		draft := ui.Local(ctx, "draft", "") // the half-typed tag
		row := ui.Div().Row().Wrap().Align(ui.Center).Gap(6)
		for _, tag := range n.Tags.Get() {
			row.Child(ui.Div().Row().Align(ui.Center).Gap(4).Radius(999).
				PaddingEdges(ui.Edges{Top: 2, Bottom: 2, Left: 9, Right: 3}).
				Background(t.AccentSoft).TextColor(t.Accent).
				Key("pill-" + tag).
				Child(ui.Text(tag).Size(12).Medium().NoWrap()).
				Child(ui.Div().Size(ui.Px(16), ui.Px(16)).Radius(8).Center().
					Cursor(ui.CursorPointer).Transition(fast).
					Hover(ui.Style{Background: t.SurfaceHover}).
					OnClick(func() { a.s.RemoveTag(n, tag) }).
					Key("remove-" + tag).
					Child(ui.Icon(icons.X).Size(10))))
		}
		row.Child(ui.TextInput(draft).Small().Placeholder("Add tag…").Icon(icons.Tag).Width(ui.Px(140)).
			Key("add-tag").
			OnSubmit(func(v string) {
				a.s.AddTag(n, v)
				draft.Set("")
			}))
		return row
	})
}
