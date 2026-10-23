// Package views is the Git app's UI, built only from public jamb APIs.
package views

import (
	"slices"
	"strconv"
	"time"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/repo"
	"github.com/tyler-builds/jamb/examples/git/store"
)

// Page is what the middle of the window shows for a repository.
type Page string

const (
	PageHistory Page = "history"
	PageTags    Page = "tags"
)

// Layout is what the app keeps across runs, and the hooks it gives the window.
type Layout struct {
	// Sidebar and Detail hold the widths of the side panes in logical pixels.
	Sidebar, Detail *ui.Signal[float64]
	// OnChange runs after the user changes the panes, the repositories or the theme.
	OnChange func()
	// Now is the clock that dates are measured from; nil is time.Now.
	Now func() time.Time
}

// Default widths of the side panes, in logical pixels.
const (
	DefaultSidebarWidth = 248
	DefaultDetailWidth  = 380
)

// view is the state of the UI for one repository: what it shows and its search.
type view struct {
	page  *ui.Signal[Page]
	query *ui.Signal[string]
	grid  *ui.DataGridHandle
	diff  *ui.Signal[*diffTarget] // the file whose diff replaces the history, or nil
	// fileSel is the changed files list's selection: the path of the open diff.
	fileSel *ui.Signal[[]string]
	// The ref tree's selection and its open nodes.
	refSel  *ui.Signal[string]
	refOpen *ui.Signal[[]string]
	// The widths the user gave the history's columns, and the tags' sort.
	widths  *ui.Signal[map[string]float32]
	tagSort *ui.Signal[ui.SortState]
}

// App is the window's UI state.
type App struct {
	s       *store.Store
	layout  Layout
	views   map[string]*view
	search  *ui.FocusHandle
	palette *ui.Signal[bool]
	goTo    *ui.Signal[bool] // the Go to palette is open
	// themeChoice is "light", "dark" or "" to follow Windows.
	themeChoice *ui.Signal[string]
	// repoSel is the repository list's selection, the store's Current as a list
	// box holds it. syncRepos copies Current into it after each change.
	repoSel *ui.Signal[[]string]
}

// Root returns the window's render function.
func Root(s *store.Store, layout Layout, theme *ui.Signal[string]) func(*ui.Context) ui.Element {
	if layout.Sidebar == nil {
		layout.Sidebar = ui.NewSignal(float64(DefaultSidebarWidth))
	}
	if layout.Detail == nil {
		layout.Detail = ui.NewSignal(float64(DefaultDetailWidth))
	}
	if layout.Now == nil {
		layout.Now = time.Now
	}
	if theme == nil {
		theme = ui.NewSignal("")
	}
	a := &App{s: s, layout: layout, views: map[string]*view{}, search: ui.NewFocusHandle(),
		palette: ui.NewSignal(false), goTo: ui.NewSignal(false), themeChoice: theme, repoSel: ui.NewSignal([]string(nil))}
	a.syncRepos()
	sidebarSize := ui.NewSignal(ui.Px(float32(layout.Sidebar.Peek())))
	return func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		return ui.Div().Column().Size(ui.Full, ui.Full).Background(t.Background).TextColor(t.Text).
			FontSize(t.FontSize).
			OnAction(AddRepo, func() { a.addRepo(ctx) }).
			OnAction(Refresh, a.refresh).
			OnAction(Find, func() { a.search.Focus() }).
			OnAction(NextMatch, func() { a.step(1) }).
			OnAction(PrevMatch, func() { a.step(-1) }).
			OnAction(GoHead, a.goHead).
			OnAction(CopyHash, func() { a.copyHash(ctx) }).
			OnAction(ShowHistory, func() { a.show(PageHistory) }).
			OnAction(ShowTags, func() { a.show(PageTags) }).
			OnAction(CloseDiff, a.closeDiff).
			OnAction(PrevRepo, func() { a.cycle(-1) }).
			OnAction(NextRepo, func() { a.cycle(1) }).
			OnAction(ToggleTheme, func() { a.toggleTheme(ctx) }).
			OnAction(Palette, func() { a.palette.Set(true) }).
			OnAction(GoTo, func() { a.goTo.Set(true) }).
			Child(ui.Component(a.titleBar)).
			Child(ui.Div().Row().Grow(1).MinHeight(ui.Px(0)).
				Child(ui.Split(
					ui.Pane(a.sidebar()).Label("Repositories and refs").SizeSignal(sidebarSize).
						Min(ui.Px(200)).Max(ui.Px(420)),
					ui.Pane(a.main()).Label("History").Min(ui.Px(420)),
				).Key("outer").Label("Resize sidebar").OnChange(func() {
					a.layout.Sidebar.Set(float64(sidebarSize.Peek().Value))
					a.changed()
				}))).
			Child(a.statusBar()).
			Child(ui.CommandPalette(a.palette).Placeholder("Type a command or search…")).
			Child(ui.CommandPalette(a.goTo).Key("go-to").ItemsOnly().
				Placeholder("Go to a branch, tag or commit; > for commands").Items(a.goToItems)).
			Child(ui.Toaster().Key("toaster"))
	}
}

// viewOf returns the UI state of r, making it on first use.
func (a *App) viewOf(r *store.Repo) *view {
	v := a.views[r.Path]
	if v == nil {
		v = &view{page: ui.NewSignal(PageHistory), query: ui.NewSignal(""), grid: ui.NewDataGridHandle(),
			diff: ui.NewSignal[*diffTarget](nil), fileSel: ui.NewSignal([]string(nil)),
			refSel: ui.NewSignal(""), refOpen: ui.NewSignal([]string{keyBranches, keyRemotes}),
			widths: ui.NewSignal(map[string]float32(nil)), tagSort: ui.NewSignal(ui.SortState{Key: "date", Desc: true})}
		a.views[r.Path] = v
	}
	return v
}

// closeDiff goes back from a file's diff to the history, and clears the changed
// files' selection with it.
func (v *view) closeDiff() {
	v.diff.Set(nil)
	v.fileSel.Set(nil)
}

// current returns the repository shown and its UI state, or nil. It does not
// track the signals, for event handlers.
func (a *App) current() (*store.Repo, *view) {
	r := a.s.Find(a.s.Current.Peek())
	if r == nil {
		return nil, nil
	}
	return r, a.viewOf(r)
}

// syncRepos makes the repository list select the store's current repository.
func (a *App) syncRepos() {
	var sel []string
	if cur := a.s.Current.Peek(); cur != "" {
		sel = []string{cur}
	}
	a.repoSel.Set(sel)
}

func (a *App) changed() {
	if a.layout.OnChange != nil {
		a.layout.OnChange()
	}
}

func (a *App) addRepo(ctx *ui.Context) {
	ctx.Window().PickFolder(dialog.FileOptions{Title: "Add a Git repository"}, func(res dialog.FileResult) {
		if res.Canceled || res.Err != nil || len(res.Paths) == 0 {
			return
		}
		a.openPath(ctx, res.Paths[0])
	})
}

// openPath adds the repository at path, or says why it cannot.
func (a *App) openPath(ctx *ui.Context, path string) {
	go func() {
		info, err := repo.Open(path)
		ctx.Post(func() {
			if err != nil {
				ctx.Toast(ui.ToastError, "Not a Git repository: "+err.Error())
				return
			}
			a.s.Add(info.Root)
			a.syncRepos()
			a.changed()
		})
	}()
}

func (a *App) refresh() {
	if r, _ := a.current(); r != nil {
		r.Reload()
	}
}

func (a *App) show(p Page) {
	if _, v := a.current(); v != nil {
		v.closeDiff()
		v.page.Set(p)
	}
}

func (a *App) closeDiff() {
	if _, v := a.current(); v != nil {
		v.closeDiff()
	}
}

// cycle shows the previous or next repository in the list, wrapping around.
func (a *App) cycle(by int) {
	repos := a.s.Repos.Peek()
	if len(repos) == 0 {
		return
	}
	i := slices.IndexFunc(repos, func(r *store.Repo) bool { return r.Path == a.s.Current.Peek() })
	a.s.Current.Set(repos[((i+by)%len(repos)+len(repos))%len(repos)].Path)
	a.syncRepos()
	a.changed()
}

// selectCommit selects the commit hash in the history and scrolls to it. It
// reports false when the commit is not among the commits loaded.
func (a *App) selectCommit(r *store.Repo, hash string) bool {
	v := a.viewOf(r)
	if !slices.ContainsFunc(r.Entries.Peek(), func(e store.Entry) bool { return e.Hash == hash }) {
		return false
	}
	v.closeDiff()
	v.page.Set(PageHistory)
	r.Selected.Set([]string{hash})
	v.grid.ScrollToRow(hash)
	return true
}

func (a *App) goHead() {
	if r, _ := a.current(); r != nil {
		a.selectCommit(r, r.Info.Peek().HeadHash)
	}
}

func (a *App) copyHash(ctx *ui.Context) {
	r, _ := a.current()
	if r == nil {
		return
	}
	if sel := r.Selected.Peek(); len(sel) > 0 {
		ctx.SetClipboard(sel[0])
		ctx.Toast(ui.ToastSuccess, "Copied "+sel[0][:min(7, len(sel[0]))])
	}
}

// step selects the next (by 1) or previous (by -1) commit that matches the
// search, wrapping around.
func (a *App) step(by int) {
	r, v := a.current()
	if r == nil || v.query.Peek() == "" {
		return
	}
	entries := r.Entries.Peek()
	at := -1
	if sel := r.Selected.Peek(); len(sel) > 0 {
		at = slices.IndexFunc(entries, func(e store.Entry) bool { return e.Hash == sel[0] })
	}
	q := v.query.Peek()
	for n := 1; n <= len(entries); n++ {
		i := ((at+by*n)%len(entries) + len(entries)) % len(entries)
		if e := entries[i]; matches(q, e.Hash, e.Subject, e.Author) {
			a.selectCommit(r, e.Hash)
			return
		}
	}
}

func (a *App) toggleTheme(ctx *ui.Context) {
	if ctx.Theme().Dark {
		a.themeChoice.Set("light")
		ctx.SetTheme(LightTheme)
	} else {
		a.themeChoice.Set("dark")
		ctx.SetTheme(DarkTheme)
	}
	a.changed()
}

// statusBar shows the repository's folder, its HEAD and how much history is loaded.
func (a *App) statusBar() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		r := a.s.CurrentRepo()
		if r == nil {
			return ui.StatusBar().Start(ui.Text("No repository"))
		}
		info := r.Info.Get()
		n := len(r.Entries.Get())
		count := strconv.Itoa(n) + " commits"
		if r.More.Get() {
			count += " loaded"
		}
		head := info.Head
		if head == "" && info.HeadHash != "" {
			head = "detached at " + info.HeadHash[:7]
		}
		end := []ui.Element{ui.Text(count)}
		if r.Loading.Get() {
			end = append([]ui.Element{ui.Div().Row().Align(ui.Center).Gap(6).
				Child(ui.ProgressRing().Size(ui.RingSmall).Label("Loading history")).Child(ui.Text("Loading"))}, end...)
		}
		return ui.StatusBar().Label("Repository status").
			Start(ui.Div().Row().Align(ui.Center).Gap(6).
				Child(ui.Icon(icons.GitBranch).Size(13).Color(t.TextMuted)).Child(ui.Text(head)),
				ui.Text(info.Root).Truncate()).
			End(end...)
	})
}

// goToMax is the most items the Go to palette lists, since each one is a row.
const goToMax = 200

// goToItems lists the shown repository's branches, tags and loaded commits that
// match the query, refs first. Choosing one selects its commit in the history.
func (a *App) goToItems(query string) []ui.PaletteItem {
	r, _ := a.current()
	if r == nil {
		return nil
	}
	var items []ui.PaletteItem
	// add keeps it when it matches, and reports whether there is room for more.
	add := func(it ui.PaletteItem, hash, subject, author string) bool {
		if matches(query, hash, subject, author) {
			items = append(items, it)
		}
		return len(items) < goToMax
	}
	for _, ref := range r.Refs.Peek() {
		ic, kind := icons.GitBranch, "branch"
		switch ref.Kind {
		case repo.Remote:
			kind = "remote branch"
		case repo.Tag:
			ic, kind = icons.Tag, "tag"
		}
		target := ref.Target
		if !add(ui.PaletteItem{Key: "ref:" + refKey(ref), Label: ref.Name, Detail: kind, Icon: ic,
			Run: func() { a.selectCommit(r, target) }}, "", ref.Name, "") {
			return items
		}
	}
	for _, e := range r.Entries.Peek() {
		hash := e.Hash
		if !add(ui.PaletteItem{Key: "commit:" + hash, Label: e.Subject, Detail: e.Short() + " · " + e.Author,
			Icon: icons.GitCommitHorizontal, Run: func() { a.selectCommit(r, hash) }}, hash, e.Subject, e.Author) {
			return items
		}
	}
	return items
}
