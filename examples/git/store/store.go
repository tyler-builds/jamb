// Package store holds the app's repositories as signals and loads their history
// off the UI thread.
package store

import (
	"slices"
	"strings"
	"sync"

	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/graph"
	"github.com/tyler-builds/jamb/examples/git/repo"
)

// PageSize is how many commits one read of the history loads.
const PageSize = 1500

// Entry is one row of the history: a commit, its slice of the graph and the
// refs that point at it.
type Entry struct {
	repo.Commit
	Row  graph.Row
	Refs []repo.Ref
}

// Repo is one repository the app shows. Its fields are signals, set from the
// loader's goroutine and read on the UI thread.
type Repo struct {
	// Path is the folder the user added, the repository's key.
	Path string

	Info    *ui.Signal[repo.Info]
	Entries *ui.Signal[[]Entry]
	Refs    *ui.Signal[[]repo.Ref]
	// Lanes is the widest row of the graph loaded so far.
	Lanes *ui.Signal[int]
	// Loading is true while a read of the history is running.
	Loading *ui.Signal[bool]
	// More is true while there may be older commits to load.
	More *ui.Signal[bool]
	// Err is the last failure to read the repository, or "".
	Err *ui.Signal[string]
	// Selected holds the hash of the selected commit, as the grid's selection.
	Selected *ui.Signal[[]string]

	mu      sync.Mutex // held by a load, so two loads never interleave
	builder graph.Builder
	gen     int // bumped by Reload, so a page from before it is dropped
}

func newRepo(path string) *Repo {
	return &Repo{
		Path:     path,
		Info:     ui.NewSignal(repo.Info{Name: baseName(path), Root: path}),
		Entries:  ui.NewSignal([]Entry(nil)),
		Refs:     ui.NewSignal([]repo.Ref(nil)),
		Lanes:    ui.NewSignal(1),
		Loading:  ui.NewSignal(false),
		More:     ui.NewSignal(false),
		Err:      ui.NewSignal(""),
		Selected: ui.NewSignal([]string(nil)),
	}
}

func baseName(path string) string {
	path = strings.TrimRight(path, `\/`)
	if i := strings.LastIndexAny(path, `\/`); i >= 0 {
		return path[i+1:]
	}
	return path
}

// Selection returns the selected entry, or false when none is.
func (r *Repo) Selection() (Entry, bool) {
	sel := r.Selected.Get()
	if len(sel) == 0 {
		return Entry{}, false
	}
	for _, e := range r.Entries.Get() {
		if e.Hash == sel[0] {
			return e, true
		}
	}
	return Entry{}, false
}

// Reload reads the repository again from the start: its description, its refs
// and the first page of its history. It returns at once.
func (r *Repo) Reload() {
	r.mu.Lock()
	r.gen++
	gen := r.gen
	r.mu.Unlock()
	r.Loading.Set(true)
	go func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if gen != r.gen {
			return
		}
		defer r.Loading.Set(false)
		info, err := repo.Open(r.Path)
		if err != nil {
			r.fail(err)
			return
		}
		refs, err := repo.Refs(info.Root)
		if err != nil {
			r.fail(err)
			return
		}
		commits, err := repo.Log(info.Root, 0, PageSize)
		if err != nil {
			r.fail(err)
			return
		}
		r.builder = graph.Builder{}
		entries := r.entries(commits, refs)
		r.Err.Set("")
		r.Info.Set(info)
		r.Refs.Set(refs)
		r.Entries.Set(entries)
		r.Lanes.Set(max(1, r.builder.Widest()))
		r.More.Set(len(commits) == PageSize)
		// Keep the selection when its commit is still there, else select HEAD.
		if sel := r.Selected.Peek(); len(sel) == 0 || !slices.ContainsFunc(entries, func(e Entry) bool { return e.Hash == sel[0] }) {
			if info.HeadHash != "" {
				r.Selected.Set([]string{info.HeadHash})
			}
		}
	}()
}

// LoadMore reads the next page of the history and appends it. It does nothing
// while a read is running or when the history is complete.
func (r *Repo) LoadMore() {
	if r.Loading.Peek() || !r.More.Peek() {
		return
	}
	r.Loading.Set(true)
	r.mu.Lock()
	gen := r.gen
	r.mu.Unlock()
	go func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		defer r.Loading.Set(false)
		if gen != r.gen {
			return
		}
		have := r.Entries.Peek()
		commits, err := repo.Log(r.Info.Peek().Root, len(have), PageSize)
		if err != nil {
			r.fail(err)
			return
		}
		more := r.entries(commits, r.Refs.Peek())
		// A new slice, so the grid sees the change.
		r.Entries.Set(append(slices.Clip(have), more...))
		r.Lanes.Set(max(1, r.builder.Widest()))
		r.More.Set(len(commits) == PageSize)
	}()
}

func (r *Repo) fail(err error) {
	r.Err.Set(err.Error())
	r.More.Set(false)
}

// entries lays out commits on the graph and attaches the refs.
func (r *Repo) entries(commits []repo.Commit, refs []repo.Ref) []Entry {
	at := map[string][]repo.Ref{}
	for _, ref := range refs {
		at[ref.Target] = append(at[ref.Target], ref)
	}
	gc := make([]graph.Commit, len(commits))
	for i, c := range commits {
		gc[i] = graph.Commit{Hash: c.Hash, Parents: c.Parents}
	}
	rows := r.builder.Add(gc)
	out := make([]Entry, len(commits))
	for i, c := range commits {
		out[i] = Entry{Commit: c, Row: rows[i], Refs: at[c.Hash]}
	}
	return out
}

// Store is the list of repositories and which one is shown.
type Store struct {
	Repos *ui.Signal[[]*Repo]
	// Current is the Path of the repository shown, or "".
	Current *ui.Signal[string]
}

// New returns a store with the repositories at paths, each loading.
func New(paths []string, current string) *Store {
	s := &Store{Repos: ui.NewSignal([]*Repo(nil)), Current: ui.NewSignal("")}
	for _, p := range paths {
		s.Add(p)
	}
	if current != "" && s.Find(current) != nil {
		s.Current.Set(current)
	}
	return s
}

// Find returns the repository with the path, or nil.
func (s *Store) Find(path string) *Repo {
	for _, r := range s.Repos.Peek() {
		if samePath(r.Path, path) {
			return r
		}
	}
	return nil
}

// CurrentRepo returns the repository shown, or nil when there is none. It is a
// tracked read of both signals.
func (s *Store) CurrentRepo() *Repo {
	cur := s.Current.Get()
	for _, r := range s.Repos.Get() {
		if r.Path == cur {
			return r
		}
	}
	return nil
}

// Add adds the repository at path, starts loading it and shows it. A path
// that is already in the list is shown.
func (s *Store) Add(path string) *Repo {
	if r := s.Find(path); r != nil {
		s.Current.Set(r.Path)
		return r
	}
	r := newRepo(path)
	s.Repos.Set(append(slices.Clip(s.Repos.Peek()), r))
	s.Current.Set(path)
	r.Reload()
	return r
}

// Remove takes the repository at path out of the list. If it was shown, the one
// next to it is shown.
func (s *Store) Remove(path string) {
	repos := s.Repos.Peek()
	i := slices.IndexFunc(repos, func(r *Repo) bool { return r.Path == path })
	if i < 0 {
		return
	}
	next := slices.Delete(slices.Clone(repos), i, i+1)
	s.Repos.Set(next)
	if s.Current.Peek() == path {
		cur := ""
		if len(next) > 0 {
			cur = next[min(i, len(next)-1)].Path
		}
		s.Current.Set(cur)
	}
}

// Paths returns the paths of the repositories in order, for saving.
func (s *Store) Paths() []string {
	var out []string
	for _, r := range s.Repos.Peek() {
		out = append(out, r.Path)
	}
	return out
}

// samePath compares folder paths as Windows does: without regard to case or
// the kind of slash.
func samePath(a, b string) bool {
	norm := func(p string) string { return strings.TrimRight(strings.ReplaceAll(p, `/`, `\`), `\`) }
	return strings.EqualFold(norm(a), norm(b))
}
