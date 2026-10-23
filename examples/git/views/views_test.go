package views

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui/uitest"

	"github.com/tyler-builds/jamb/examples/git/repo"
	"github.com/tyler-builds/jamb/examples/git/repo/repotest"
	"github.com/tyler-builds/jamb/examples/git/store"
)

// open renders the app with the sample repository and waits for its history.
func open(t *testing.T) (*uitest.UI, *store.Store, *store.Repo) {
	t.Helper()
	dir := repotest.Sample(t)
	s := store.New([]string{dir}, dir)
	now := time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC)
	u := uitest.New(t, Root(s, Layout{Now: func() time.Time { return now }}, nil),
		uitest.Size(1360, 840), uitest.Theme(DarkTheme), uitest.Bindings(Bindings()...))
	r := s.Find(dir)
	u.WaitFor("the history to load", func() bool { return len(r.Entries.Peek()) > 0 && !r.Loading.Peek() })
	u.Frame()
	return u, s, r
}

// entry returns the loaded commit with the subject.
func entry(t *testing.T, r *store.Repo, subject string) store.Entry {
	t.Helper()
	for _, e := range r.Entries.Peek() {
		if e.Subject == subject {
			return e
		}
	}
	t.Fatalf("no commit %q", subject)
	return store.Entry{}
}

func TestHistoryShowsEveryCommitAndSelectsHead(t *testing.T) {
	u, _, r := open(t)
	if n := len(r.Entries.Peek()); n != 6 {
		t.Fatalf("%d commits loaded, want 6", n)
	}
	head := r.Info.Peek().HeadHash
	if sel := r.Selected.Peek(); len(sel) != 1 || sel[0] != head {
		t.Fatalf("selection = %v, want HEAD %s", sel, head)
	}
	grid := u.ByRole(access.RoleDataGrid, "Commits")
	if !strings.Contains(u.Get("history").Text(), "Rename feature.go") {
		t.Errorf("the history does not show the HEAD commit: %q", grid.Name())
	}
	// HEAD's row names its branch and its tag ahead of the subject.
	u.ByName("main (HEAD), v0.2.0 (tag), Rename feature.go")
	u.WaitFor("the details of HEAD", func() bool { return u.Has("detail-files") })
	if got := u.Get("detail-files").Text(); !strings.Contains(got, "1 file changed") || !strings.Contains(got, "sample.go") {
		t.Errorf("detail files = %q", got)
	}
	if w := u.Warnings(); len(w) > 0 {
		t.Errorf("warnings: %q", w)
	}
}

func TestGraphPutsTheMergedBranchInItsOwnLane(t *testing.T) {
	_, _, r := open(t)
	merge := entry(t, r, "Merge branch 'feature'")
	feature := entry(t, r, "Add the feature")
	main := entry(t, r, "Describe the sample")
	if !merge.Row.Merge {
		t.Error("the merge is not drawn as a merge")
	}
	if feature.Row.Lane == main.Row.Lane {
		t.Errorf("the feature commit shares lane %d with main", main.Row.Lane)
	}
	if merge.Row.Lane != main.Row.Lane {
		t.Errorf("the merge is in lane %d, main in %d", merge.Row.Lane, main.Row.Lane)
	}
}

func TestSelectingAFileOpensItsDiffAndEscapeCloses(t *testing.T) {
	u, _, r := open(t)
	e := entry(t, r, "Describe the sample")
	r.Selected.Set([]string{e.Hash})
	u.WaitFor("the details", func() bool {
		return u.Has("detail-files") && strings.Contains(u.Get("detail-files").Text(), "readme.md")
	})
	u.ByRole(access.RoleListItem, "Modified readme.md").Click()
	u.WaitFor("the diff", func() bool { return u.Has("diff-view") && !u.Has("diff-loading") })
	if !strings.Contains(u.Get("diff-view").Text(), "readme.md") {
		t.Errorf("diff header = %q", u.Get("diff-view").Text())
	}
	u.Get("diff-back").Focus()
	u.Press("escape")
	if u.Has("diff-view") {
		t.Error("Escape left the diff open")
	}
}

func TestSearchCountsAndStepsThroughMatches(t *testing.T) {
	u, _, r := open(t)
	u.Press("ctrl+f")
	u.Type("feature")
	if got := u.Get("search").Text(); !strings.Contains(got, "3 found") {
		t.Errorf("search = %q, want 3 found", got)
	}
	u.Press("f3")
	first := r.Selected.Peek()
	u.Press("f3")
	second := r.Selected.Peek()
	if len(first) == 0 || len(second) == 0 || first[0] == second[0] {
		t.Errorf("F3 did not move between matches: %v then %v", first, second)
	}
	for _, sel := range [][]string{first, second} {
		for _, e := range r.Entries.Peek() {
			if e.Hash == sel[0] && !strings.Contains(strings.ToLower(e.Subject), "feature") {
				t.Errorf("F3 selected %q", e.Subject)
			}
		}
	}
}

func TestTagsPageAndRefTree(t *testing.T) {
	u, _, r := open(t)
	u.Press("ctrl+2")
	grid := u.ByRole(access.RoleDataGrid, "Tags")
	if got := u.Get("tags").Text(); !strings.Contains(got, "v0.1.0") || !strings.Contains(got, "Second release") {
		t.Errorf("tags = %q (%s)", got, grid.Name())
	}
	u.Press("ctrl+1")
	// The ref tree: open Tags and choose v0.1.0, which selects the first commit.
	u.ByRole(access.RoleTreeItem, "Tags").Perform(access.ActionExpand)
	u.ByRole(access.RoleTreeItem, "v0.1.0").Click()
	root := entry(t, r, "Initial commit")
	if sel := r.Selected.Peek(); len(sel) != 1 || sel[0] != root.Hash {
		t.Errorf("selection after choosing v0.1.0 = %v, want %s", sel, root.Hash)
	}
}

func TestRemovingTheLastRepositoryShowsTheWelcome(t *testing.T) {
	u, s, r := open(t)
	s.Remove(r.Path)
	u.Frame()
	u.ByName("No repositories yet")
}

func TestLabelsFoldRemotesIntoTheirBranch(t *testing.T) {
	ls := labels([]repo.Ref{
		{Kind: repo.Tag, Name: "v1"},
		{Kind: repo.Remote, Name: "origin/main"},
		{Kind: repo.Branch, Name: "main"},
		{Kind: repo.Remote, Name: "origin/other"},
		{Kind: repo.Branch, Name: "dev"},
	}, "main")
	var got []string
	for _, l := range ls {
		got = append(got, l.text())
	}
	want := []string{"main (HEAD, origin)", "dev", "origin/other", "v1 (tag)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("labels = %q, want %q", got, want)
	}
}

// The graph draws each row's slice edge to edge, so its lines join only while
// the rows keep the grid's height and the canvas fills the cell.
func TestRowsKeepTheGridHeight(t *testing.T) {
	u, _, _ := open(t)
	b := u.ByRole(access.RoleDataItem, "Describe the sample").Bounds()
	if b.H != 36 {
		t.Errorf("a history row is %v px high, want 36", b.H)
	}
}

// The search sits in the middle of the window, whatever is beside it.
func TestSearchIsCentered(t *testing.T) {
	u, _, _ := open(t)
	b := u.Get("search").Bounds()
	if mid := b.X + b.W/2; mid < 679 || mid > 681 {
		t.Errorf("the search's middle is at %v, want the window's, 680", mid)
	}
}

// The window buttons fill the title bar above its 1 px border, so Close's red reaches the
// window's corner.
func TestWindowButtonsFillTheTitleBar(t *testing.T) {
	u, _, _ := open(t)
	for _, name := range []string{"Minimize", "Maximize", "Close"} {
		if b := u.ByRole(access.RoleButton, name).Bounds(); b.Y != 0 || b.H != 43 {
			t.Errorf("%s is %v high at y %v, want 43 at y 0", name, b.H, b.Y)
		}
	}
}

// Ctrl+P opens Go to, which finds a commit by its subject and selects it.
func TestGoToSelectsACommit(t *testing.T) {
	u, _, r := open(t)
	u.Press("ctrl+p")
	u.Type("spike")
	u.Press("enter")
	if sel := r.Selected.Peek(); len(sel) != 1 || sel[0] != entry(t, r, "Try a spike").Hash {
		t.Errorf("selection after Go to = %v, want the spike commit", sel)
	}
}

// The details bar's copy button copies the whole hash, without a toast.
func TestCopyButtonCopiesTheHash(t *testing.T) {
	u, _, r := open(t)
	u.Get("copy-hash").Click()
	if got, want := u.Clipboard(), r.Info.Peek().HeadHash; got != want {
		t.Errorf("clipboard = %q, want HEAD's hash %q", got, want)
	}
}

// Choosing a file in the changed files list box opens its diff. (The sample's
// commits each change one file, so the arrow keys are left to ui.ListBox's tests.)
func TestChoosingAChangedFileOpensItsDiff(t *testing.T) {
	u, _, r := open(t)
	r.Selected.Set([]string{entry(t, r, "Initial commit").Hash})
	u.WaitFor("the details", func() bool { return u.Has("detail-files") })
	u.ByRole(access.RoleListItem, "Added readme.md").Click()
	u.WaitFor("the diff", func() bool { return u.Has("diff-view") })
	if !strings.Contains(u.Get("diff-view").Text(), "readme.md") {
		t.Errorf("diff header = %q", u.Get("diff-view").Text())
	}
}

// Go to a tag whose commit is not loaded yet loads the history until it is, then
// selects it.
func TestGoToLoadsTheHistoryUntilTheTarget(t *testing.T) {
	defer func(n int) { store.PageSize = n }(store.PageSize)
	store.PageSize = 2
	u, _, r := open(t)
	if n := len(r.Entries.Peek()); n != 2 {
		t.Fatalf("%d commits loaded, want the first page of 2", n)
	}
	u.Press("ctrl+p")
	u.Type("v0.1.0")
	u.Press("enter")
	root := ""
	u.WaitFor("the tag's commit to load and be selected", func() bool {
		for _, e := range r.Entries.Peek() {
			if e.Subject == "Initial commit" {
				root = e.Hash
			}
		}
		sel := r.Selected.Peek()
		return root != "" && len(sel) == 1 && sel[0] == root
	})
}
