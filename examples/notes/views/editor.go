package views

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/markdown"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// editorPane shows the selected note, with its Markdown preview beside the editor
// while the preview is on.
func (a *App) editorPane() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		n := a.s.Find(a.s.Selected.Get())
		pane := ui.Div().Grow(1).MinWidth(ui.Px(0)).Height(ui.Full)
		if n == nil {
			return pane.Center().Gap(12).
				Child(ui.Icon(icons.Layers).Size(40).Color(t.TextSubtle)).
				Child(ui.Text("No note selected").FontSize(16).Semibold()).
				Child(ui.Text("Create one with Ctrl+N.").TextColor(t.TextMuted)).
				Child(ui.Button("New note").Icon(icons.Plus).OnClick(a.newNote))
		}
		// Keyed by note: each note gets its own editor state (caret, undo).
		if !a.preview.Get() {
			return pane.Child(a.toolbar(n)).Child(a.noteEditor(n))
		}
		return pane.Child(a.toolbar(n)).
			Child(ui.Div().Row().Grow(1).MinHeight(ui.Px(0)).
				Child(ui.Div().Grow(1).Basis(ui.Px(0)).MinWidth(ui.Px(0)).Child(a.noteEditor(n))).
				Child(ui.Div().Width(ui.Px(1)).Background(t.Border)).
				Child(a.notePreview(n)))
	})
}

func (a *App) toolbar(n *store.Note) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		confirming := a.confirm.Get() == n.ID
		bar := ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).Gap(8).PaddingX(16).MinHeight(ui.Px(56)).Shrink(0).
			Child(ui.Text("Edited " + relative(n.Updated.Get())).FontSize(13).TextColor(t.TextSubtle).Truncate())
		actions := ui.Div().Row().Align(ui.Center).Gap(4)
		if confirming {
			actions.
				Child(ui.Text("Delete this note?").FontSize(13).TextColor(t.TextMuted).NoWrap()).
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
			pinIcon := icons.Star
			if pinned && t.HighContrast {
				pinIcon = icons.StarFilled // the pinned state shows in the shape under high contrast
			}
			pin := ui.IconButton(pinIcon).Label("Pin note").AccessChecked(pinned).Tooltip("Pin").OnClick(func() { a.s.TogglePin(n) })
			if pinned {
				pin.Style(ui.Style{TextColor: t.Warning}).Tooltip("Unpin")
			}
			themeIcon := icons.Sun
			if !t.Dark {
				themeIcon = icons.Moon
			}
			// The preview is a toggle button too: its state says whether the preview is shown.
			preview := a.preview.Get()
			actions.
				Child(ui.IconButton(icons.Eye).Key("preview-toggle").Label("Markdown preview").AccessChecked(preview).
					Tooltip("Markdown preview (" + ctx.KeysFor(TogglePreview) + ")").OnClick(a.togglePreview)).
				Child(pin).
				Child(ui.IconButton(icons.Trash).Label("Delete note").Tooltip("Delete note").OnClick(func() { a.confirm.Set(n.ID) })).
				Child(ui.Div().Width(ui.Px(1)).Height(ui.Px(20)).MarginX(4).Background(t.Border)).
				Child(ui.IconButton(themeIcon).Label("Toggle theme").AccessChecked(t.Dark).Tooltip("Toggle theme (" + ctx.KeysFor(ToggleTheme) + ")").OnClick(func() { toggleTheme(ctx) }))
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

// previewDelay is how long the preview waits after the last edit before it shows the
// new text. A long note costs several milliseconds to show again (see docs/markdown.md),
// so the preview follows a pause in typing, not each key.
const previewDelay = 150 * time.Millisecond

// notePreview shows the note's body as Markdown. It follows the edits after a pause
// of previewDelay; opening the preview or another note shows the text at once. The
// note is still stored as the text that is typed. Links open with the system, and
// images are read from the files they name.
func (a *App) notePreview(n *store.Note) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		// shown is the text the preview shows. Only the view's component reads it, and
		// only the watcher reads the body, so a key re-renders the watcher and nothing
		// else until the pause is over.
		shown := ui.Local(ctx, "shown", n.Body.Peek())
		shown.Set(n.Body.Peek())
		return ui.Div().Key("preview").Grow(1).Basis(ui.Px(0)).MinWidth(ui.Px(0)).ScrollY().
			Child(a.previewWatcher(n, shown)).
			Child(ui.Component(func(ctx *ui.Context) ui.Element {
				return ui.Div().Width(ui.Full).MaxWidth(ui.Px(820)).PaddingX(32).PaddingY(24).
					Child(markdown.View(shown.Get()).Label("Preview"))
			}))
	}).Key("preview-" + n.ID)
}

// previewWatcher shows nothing. It reads the note's body and copies it to shown once
// the body has not changed for previewDelay.
func (a *App) previewWatcher(n *store.Note, shown *ui.Signal[string]) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		body := n.Body.Get()
		pending := ui.Local(ctx, "pending", &previewTimer{cancel: func() {}}).Peek()
		if !pending.disposing {
			pending.disposing = true
			ctx.OnDispose(func() { pending.cancel() })
		}
		pending.cancel() // an edit during the pause starts it again
		pending.cancel = func() {}
		if body != shown.Peek() {
			pending.cancel = ctx.After(previewDelay, func() { shown.Set(n.Body.Peek()) })
		}
		return nil
	})
}

// previewTimer is the pause a preview waits for: the function that cancels it, and
// whether the watcher's OnDispose cancels it.
type previewTimer struct {
	cancel    func()
	disposing bool
}

// meta re-renders as the body changes; nothing else does.
func (a *App) meta(n *store.Note) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		body := n.Body.Get()
		words := len(strings.FieldsFunc(body, func(r rune) bool { return unicode.IsSpace(r) }))
		return ui.Div().Row().Gap(14).
			Child(ui.Text("Created " + n.Created.Format("Jan 2, 2006")).FontSize(12).TextColor(t.TextSubtle)).
			Child(ui.Textf("%s words", commas(words)).FontSize(12).TextColor(t.TextSubtle)).
			Child(ui.Textf("%s characters", commas(len([]rune(body)))).FontSize(12).TextColor(t.TextSubtle))
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
