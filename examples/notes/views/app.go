// Package views is the notes app's UI, built only from public jamb APIs.
package views

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

const fast = 120 * time.Millisecond

// keymapError is the first line of the last keymap problem, shown in the
// status bar. It is "" when keymap.json loaded cleanly.
var keymapError = ui.NewSignal("")

// ReportKeymap shows the first line of err in the status bar, or clears the
// message for nil. It is safe from any goroutine.
func ReportKeymap(err error) {
	msg := ""
	if err != nil {
		msg, _, _ = strings.Cut(err.Error(), "\n")
	}
	keymapError.Set(msg)
}

// keymapProblem is the part of a keymap error that says what is wrong. jamb's
// messages lead with the file, the section and the keys, which leave no room
// in the status bar; the tooltip still shows the whole line.
func keymapProblem(msg string) string {
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}

// App holds UI-only state: focus handles, the pending delete confirmation and
// the command palette.
type App struct {
	s       *store.Store
	search  *ui.FocusHandle
	title   *ui.FocusHandle
	body    *ui.FocusHandle
	confirm *ui.Signal[string] // ID of the note awaiting delete confirmation

	paletteOpen    *ui.Signal[bool]
	paletteQuery   *ui.Signal[string]
	paletteSel     *ui.Signal[int]
	paletteActions []*ui.Action    // the commands listed when the palette opened
	paletteFocus   *ui.FocusHandle // what had focus when the palette opened
	list           *ui.ListHandle  // the sidebar's note list, scrolled to the selection
}

// Root returns the window's render function.
func Root(s *store.Store) func(*ui.Context) ui.Element {
	a := &App{
		s:            s,
		search:       ui.NewFocusHandle(),
		title:        ui.NewFocusHandle(),
		body:         ui.NewFocusHandle(),
		confirm:      ui.NewSignal(""),
		paletteOpen:  ui.NewSignal(false),
		paletteQuery: ui.NewSignal(""),
		paletteSel:   ui.NewSignal(0),
		list:         ui.NewListHandle(),
	}
	return func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		return ui.Div().Row().Size(ui.Full, ui.Full).PaddingEdges(ui.Edges{Top: menuBarHeight}).
			Background(t.Background).TextColor(t.Text).
			OnAction(NewNote, a.newNote).
			OnAction(Search, func() { a.search.Focus() }).
			OnAction(PrevNote, func() { a.step(false) }).
			OnAction(NextNote, func() { a.step(true) }).
			OnAction(TogglePin, a.togglePin).
			OnAction(ToggleTheme, func() { toggleTheme(ctx) }).
			OnAction(Palette, func() { a.openPalette(ctx) }).
			OnAction(ZoomIn, func() { zoomStep(ctx, 1) }).
			OnAction(ZoomOut, func() { zoomStep(ctx, -1) }).
			OnAction(ZoomReset, func() { ctx.SetZoom(1) }).
			OnDrop(ui.Drop(a.importFiles)).
			Child(a.menuBar(ctx)).
			Child(a.sidebar()).
			Child(ui.Div().Width(ui.Px(1)).Height(ui.Full).Background(t.Border)).
			Child(a.editorPane()).
			Child(a.palette())
	}
}

func (a *App) newNote() {
	a.s.Create()
	a.title.Focus() // the title field is created by this change; focus lands once it exists
}

// maxImport is the largest file that can be imported, in bytes.
const maxImport = 1 << 20

// notice is the status line's message about the last import, drop or export.
// It is "" when there is none.
var notice = ui.NewSignal("")

// showNotice shows msg in the status line for a few seconds.
func showNotice(msg string) {
	notice.Set(msg)
	time.AfterFunc(6*time.Second, func() { notice.Set("") }) // Set is safe from any goroutine
}

// importFiles makes a note from each dropped .txt or .md file up to 1 MB, titled
// with the file name, and selects the last one. Other files are skipped. The
// status bar reports the counts for a few seconds.
func (a *App) importFiles(files ui.Files, _ *ui.DropEvent) {
	var last *store.Note
	imported, skipped := 0, 0
	for _, path := range files {
		if n := importFile(a.s, path); n != nil {
			last = n
			imported++
		} else {
			skipped++
		}
	}
	if last != nil {
		a.s.Selected.Set(last.ID)
	}
	if imported+skipped == 0 {
		return
	}
	msg := fmt.Sprintf("Imported %d %s", imported, plural(imported, "note"))
	if skipped > 0 {
		msg += fmt.Sprintf(", skipped %d %s", skipped, plural(skipped, "file"))
	}
	showNotice(msg)
}

// importFile reads one file as a note, or returns nil if it is not an
// importable file.
func importFile(s *store.Store, path string) *store.Note {
	ext := filepath.Ext(path)
	if !strings.EqualFold(ext, ".txt") && !strings.EqualFold(ext, ".md") {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxImport {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return s.Import(strings.TrimSuffix(filepath.Base(path), ext), string(data))
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// togglePin pins or unpins the selected note.
func (a *App) togglePin() {
	if n := a.s.Find(a.s.Selected.Peek()); n != nil {
		a.s.TogglePin(n)
	}
}

// toggleTheme switches between the dark and light themes.
func toggleTheme(ctx *ui.Context) {
	if ctx.Theme().Dark {
		ctx.SetTheme(ui.LightTheme)
	} else {
		ctx.SetTheme(ui.DarkTheme)
	}
}

// step selects the previous or next visible note.
func (a *App) step(down bool) {
	var visible []*store.Note
	q, tag := a.s.Query.Peek(), a.s.TagFilter.Peek()
	for _, n := range a.s.Notes.Peek() {
		if store.Visible(n, q, tag) {
			visible = append(visible, n)
		}
	}
	i := slices.IndexFunc(visible, func(n *store.Note) bool { return n.ID == a.s.Selected.Peek() })
	switch {
	case len(visible) == 0:
		return
	case i < 0:
		i = 0
	case down:
		i = min(i+1, len(visible)-1)
	default:
		i = max(i-1, 0)
	}
	a.s.Selected.Set(visible[i].ID)
	a.revealNote(visible[i].ID)
}

// revealNote scrolls the sidebar's list so that the note with the given ID is
// in view.
func (a *App) revealNote(id string) {
	entries := sidebarEntries(a.s.Notes.Peek(), a.s.Query.Peek(), a.s.TagFilter.Peek())
	for i, e := range entries {
		if e.note != nil && e.note.ID == id {
			a.list.ScrollToItem(i)
			return
		}
	}
}

func (a *App) sidebar() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		notes := a.s.Notes.Get()
		q := a.s.Query.Get()
		selected := a.s.Selected.Get()
		tagFilter := a.s.TagFilter.Get()

		// Only the rows near the viewport are built, so a long list costs
		// about as much as a short one.
		entries := sidebarEntries(notes, q, tagFilter)
		var list ui.Element
		if len(entries) == 0 {
			msg := "No notes yet"
			if q != "" || tagFilter != "" {
				msg = "No notes match"
			}
			list = ui.Div().Grow(1).Child(ui.Div().Padding(24).Align(ui.Center).
				Child(ui.Text(msg).Color(t.TextMuted)))
		} else {
			list = ui.ListOf(entries, sidebarKey, func(e sidebarEntry) ui.Element {
				if e.note == nil {
					return sectionLabel(t, e.label)
				}
				return ui.Div().Style(ui.Style{Padding: ui.Edges{Top: 1, Bottom: 1}}).
					Child(a.noteItem(e.note, e.note.ID == selected))
			}).Key("sidebar").Style(ui.Style{Grow: 1, Padding: ui.Edges{Top: 4, Bottom: 4, Left: 8, Right: 8}}).Handle(a.list)
		}

		search := ui.TextInput(a.s.Query).Placeholder("Search notes").Icon(icons.Search).
			FocusHandle(a.search).
			OnSubmit(func(string) { // Enter opens the first match
				for _, n := range a.s.Notes.Peek() {
					if store.Visible(n, a.s.Query.Peek(), a.s.TagFilter.Peek()) {
						a.s.Selected.Set(n.ID)
						a.body.Focus()
						return
					}
				}
			})

		return ui.Div().Width(ui.Px(300)).Height(ui.Full).Shrink(0).Background(t.Sidebar).
			Child(ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).PaddingX(16).Height(ui.Px(56)).Shrink(0).
				Child(ui.Div().Row().Align(ui.Center).Gap(10).
					Child(ui.Div().Size(ui.Px(26), ui.Px(26)).Radius(7).Background(t.Accent).Center().
						Child(ui.Icon(icons.Layers).Size(15).Color(ui.Hex("#ffffff")))).
					Child(ui.Text("Notes").Size(17).Bold())).
				Child(ui.IconButton(icons.Plus).Tooltip("New note (" + ctx.KeysFor(NewNote) + ")").OnClick(a.newNote))).
			Child(ui.Div().PaddingX(12).PaddingY(4).Shrink(0).
				Child(ui.Div().Row().Align(ui.Center).Gap(6).
					Child(search.Style(ui.Style{Grow: 1})).
					Child(ui.If(q != "", ui.IconButton(icons.X).Small().Tooltip("Clear search").OnClick(func() { a.s.Query.Set("") }))))).
			Child(a.tagChips()).
			Child(list).
			Child(a.status(len(notes)))
	})
}

// sidebarEntry is a row of the sidebar's note list: a section label when note
// is nil, otherwise a note.
type sidebarEntry struct {
	label string
	note  *store.Note
}

// sidebarEntries lists the notes that match the query and tag: pinned notes
// first under a "Pinned" label, then the rest under "Notes" when there are
// pinned ones too.
func sidebarEntries(notes []*store.Note, q, tag string) []sidebarEntry {
	var pinned, others []*store.Note
	for _, n := range notes {
		if !store.Visible(n, q, tag) {
			continue
		}
		if n.Pinned.Peek() {
			pinned = append(pinned, n)
		} else {
			others = append(others, n)
		}
	}
	var es []sidebarEntry
	if len(pinned) > 0 {
		es = append(es, sidebarEntry{label: "Pinned"})
		for _, n := range pinned {
			es = append(es, sidebarEntry{note: n})
		}
	}
	if len(others) > 0 {
		if len(pinned) > 0 {
			es = append(es, sidebarEntry{label: "Notes"})
		}
		for _, n := range others {
			es = append(es, sidebarEntry{note: n})
		}
	}
	return es
}

// sidebarKey is the identity of a sidebar row: its note's ID, or its label.
func sidebarKey(e sidebarEntry) string {
	if e.note == nil {
		return "h:" + e.label
	}
	return e.note.ID
}

func sectionLabel(t *ui.Theme, s string) ui.Element {
	return ui.Text(s).Size(12).Medium().Color(t.TextSubtle).
		Style(ui.Style{Padding: ui.Edges{Top: 12, Left: 8, Bottom: 4}})
}

// titleOf is the note's title, or "Untitled" while it has none.
func titleOf(n *store.Note) string {
	if t := n.Title.Get(); t != "" {
		return t
	}
	return "Untitled"
}

// noteItem is its own component: typing in a note re-renders only its item.
func (a *App) noteItem(n *store.Note, selected bool) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		q := a.s.Query.Get()
		// Matches are highlighted in the note's own title and snippet, not in
		// the placeholders.
		title := ui.Text(titleOf(n))
		if n.Title.Get() != "" {
			title = ui.Text(n.Title.Get()).HighlightMatches(q, matchHighlight(t))
		}
		snippet := ui.Text("No additional text")
		if line := firstLine(n.Body.Get()); line != "" {
			snippet = ui.Text(line).HighlightMatches(q, matchHighlight(t))
		}
		// Tab reaches the row; Enter opens the note, as a click does.
		return ui.Div().Gap(3).PaddingX(10).PaddingY(9).Radius(8).Transition(fast).
			Hover(ui.Style{Background: t.SurfaceHover}).
			When(selected, func(d *ui.DivElement) { d.Background(t.SurfaceActive) }).
			Focusable().FocusVisible(focusRing(t)).
			OnClick(func() { a.s.Selected.Set(n.ID) }).
			ContextMenu(func() []ui.MenuItem { return a.noteMenu(n) }).
			OnDrag(ui.Drag(n).Preview(func() ui.Element {
				return ui.Div().PaddingX(10).PaddingY(5).Radius(999).Background(t.Surface).
					Border(1, t.Accent).Shadow(ui.ShadowMd).
					Child(ui.Text(titleOf(n)).Size(13).Semibold().NoWrap()) // reads the title, so it follows edits
			})).
			Dragging(ui.Style{Opacity: 0.5}).
			Child(ui.Div().Row().Align(ui.Center).Gap(6).
				Child(ui.If(n.Pinned.Get(), ui.Icon(icons.Star).Size(12).Color(t.Warning))).
				Child(title.Semibold().Truncate().Grow(1)).
				Child(ui.Text(shortDate(n.Updated.Get())).Size(12).Color(t.TextSubtle).NoWrap())).
			Child(snippet.Size(13).Color(t.TextMuted).Truncate())
	}).Key(n.ID)
}

// status shows whether everything is saved. The saver goroutine updates the
// signal; the UI picks it up on the next frame.
func (a *App) status(count int) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		st := a.s.Save.Get()
		label, color := "Saved", t.TextSubtle
		switch {
		case st.Err != "":
			label, color = "Save failed: "+st.Err, t.Danger
		case st.Saving:
			label = "Saving…"
		case st.At.IsZero():
			label = "All changes saved"
		}
		msg := keymapError.Get()
		if line := notice.Get(); line != "" && st.Err == "" {
			label, color = line, t.TextMuted
		}
		bar := ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).Gap(8).PaddingX(16).Height(ui.Px(40)).Shrink(0).
			Child(ui.Div().Row().Align(ui.Center).Gap(6).MinWidth(ui.Px(0)).
				Child(ui.Icon(icons.Check).Size(13).Color(color)).
				Child(ui.Text(label).Size(12).Color(color).Truncate())).
			Child(ui.Div().Row().Align(ui.Center).Gap(8).
				Child(ui.If(ctx.Zoom() != 1, // shown only when zoomed; click to reset
					ui.Div().Key("zoom").PaddingX(6).PaddingY(2).Radius(5).Transition(fast).
						Hover(ui.Style{Background: t.SurfaceHover}).
						OnClick(func() { ctx.SetZoom(1) }).
						Child(ui.Textf("%.0f%%", ctx.Zoom()*100).Size(12).Color(t.TextMuted)))).
				Child(ui.Textf("%d notes", count).Size(12).Color(t.TextSubtle).NoWrap()))
		if msg == "" {
			return bar
		}
		// A keymap problem gets its own line: the sidebar is too narrow to
		// share one with the save status. The tooltip has the whole message.
		return ui.Div().Shrink(0).
			Child(ui.Div().Key("keymap-error").Row().Align(ui.Center).Gap(6).PaddingEdges(ui.Edges{Top: 8, Left: 16, Right: 16}).Tooltip(msg).
				Child(ui.Icon(icons.Alert).Size(13).Color(t.Danger).Style(ui.Style{AlignSelf: ui.Start, Margin: ui.Edges{Top: 2}})).
				Child(ui.Text("keymap: " + keymapProblem(msg)).Size(12).Color(t.Danger).MaxLines(2).Grow(1))).
			Child(bar)
	})
}
