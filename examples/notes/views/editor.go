package views

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// editorPane shows the selected note.
func (a *App) editorPane() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		n := a.s.Find(a.s.Selected.Get())
		pane := ui.Div().Grow(1).MinWidth(ui.Px(0)).Height(ui.Full)
		if n == nil {
			return pane.Center().Gap(12).
				Child(ui.Icon(icons.Layers).Size(40).Color(t.TextSubtle)).
				Child(ui.Text("No note selected").Size(16).Semibold()).
				Child(ui.Text("Create one with Ctrl+N.").Color(t.TextMuted)).
				Child(ui.Button("New note").Icon(icons.Plus).OnClick(a.newNote))
		}
		// Keyed by note: each note gets its own editor state (caret, undo).
		return pane.Child(a.toolbar(n)).Child(a.noteEditor(n))
	})
}

func (a *App) toolbar(n *store.Note) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		confirming := a.confirm.Get() == n.ID
		bar := ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).Gap(8).PaddingX(16).Height(ui.Px(56)).Shrink(0).
			Child(ui.Text("Edited " + relative(n.Updated.Get())).Size(13).Color(t.TextSubtle).Truncate())
		actions := ui.Div().Row().Align(ui.Center).Gap(4)
		if confirming {
			actions.
				Child(ui.Text("Delete this note?").Size(13).Color(t.TextMuted).NoWrap()).
				Child(ui.Button("Cancel").Key("confirm-cancel").Ghost().Small().OnClick(func() { a.confirm.Set("") })).
				Child(ui.Button("Delete").Key("confirm-delete").Danger().Small().Icon(icons.Trash).OnClick(func() {
					a.confirm.Set("")
					a.s.Delete(n)
					ui.Announce("Note deleted", access.Polite)
				}))
		} else {
			// The pin is a toggle button: its name stays "Pin note", and its state
			// says whether the note is pinned.
			pinned := n.Pinned.Get()
			pin := ui.IconButton(icons.Star).Label("Pin note").Checked(pinned).Tooltip("Pin").OnClick(func() { a.s.TogglePin(n) })
			if pinned {
				pin.Style(ui.Style{TextColor: t.Warning}).Tooltip("Unpin")
			}
			themeIcon := icons.Sun
			if !t.Dark {
				themeIcon = icons.Moon
			}
			actions.
				Child(pin).
				Child(ui.IconButton(icons.Trash).Label("Delete note").Tooltip("Delete note").OnClick(func() { a.confirm.Set(n.ID) })).
				Child(ui.Div().Width(ui.Px(1)).Height(ui.Px(20)).MarginX(4).Background(t.Border)).
				Child(ui.IconButton(themeIcon).Label("Toggle theme").Checked(t.Dark).Tooltip("Toggle theme (" + ctx.KeysFor(ToggleTheme) + ")").OnClick(func() { toggleTheme(ctx) }))
		}
		return bar.Child(actions)
	}).Key("toolbar-" + n.ID)
}

func (a *App) noteEditor(n *store.Note) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		changed := func(string) {
			a.s.Changed(n)
			if a.confirm.Peek() != "" {
				a.confirm.Set("")
			}
		}
		title := ui.TextInput(n.Title).Label("Title").Borderless().Placeholder("Untitled").
			FocusHandle(a.title).
			Style(ui.Style{FontSize: 28, FontWeight: ui.FontBold}).
			OnChange(changed).
			OnSubmit(func(string) { a.body.Focus() }) // Enter continues into the body
		body := ui.TextArea(n.Body).Label("Body").Borderless().MinRows(18).Placeholder("Start writing…").
			FocusHandle(a.body).
			Style(ui.Style{FontSize: 15, LineHeight: 1.6}).
			OnChange(changed)
		return ui.Div().Grow(1).ScrollY().
			Child(ui.Div().Width(ui.Full).MaxWidth(ui.Px(820)).PaddingX(48).PaddingY(24).Gap(10).
				Child(title).
				Child(a.meta(n)).
				Child(a.tagEditor(n)).
				Child(ui.Div().Height(ui.Px(1)).MarginY(6).Background(t.Border)).
				Child(body))
	}).Key("editor-" + n.ID)
}

// meta re-renders as the body changes; nothing else does.
func (a *App) meta(n *store.Note) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		body := n.Body.Get()
		words := len(strings.FieldsFunc(body, func(r rune) bool { return unicode.IsSpace(r) }))
		return ui.Div().Row().Gap(14).
			Child(ui.Text("Created " + n.Created.Format("Jan 2, 2006")).Size(12).Color(t.TextSubtle)).
			Child(ui.Textf("%s words", commas(words)).Size(12).Color(t.TextSubtle)).
			Child(ui.Textf("%s characters", commas(len([]rune(body)))).Size(12).Color(t.TextSubtle))
	})
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

func shortDate(t time.Time) string {
	now := time.Now()
	y1, m1, d1 := t.Date()
	y2, m2, d2 := now.Date()
	switch {
	case y1 == y2 && m1 == m2 && d1 == d2:
		return t.Format("15:04")
	case now.Sub(t) < 48*time.Hour && d2-d1 == 1:
		return "Yesterday"
	case now.Sub(t) < 6*24*time.Hour:
		return t.Format("Mon")
	case y1 == y2:
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}

func relative(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return shortDate(t)
}

func commas(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
