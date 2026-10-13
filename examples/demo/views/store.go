// Package views contains the demo application's UI.
package views

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// Page identifies a top-level screen.
type Page int

const (
	PageDashboard Page = iota
	PageProjects
	PageActivity
	PageComponents
	PageSettings
)

// Status is a project's state.
type Status int

const (
	StatusActive Status = iota
	StatusPaused
	StatusDone
)

func (s Status) String() string {
	return [...]string{"Active", "Paused", "Done"}[s]
}

// Project is one card on the projects page.
type Project struct {
	ID          int
	Name        string
	Description string
	Status      Status
	Progress    float32 // 0..1
	Tasks       int
	Updated     time.Time
	Due         time.Time // the due date, which the New Project form sets; zero when there is none
	Starred     bool
	Icon        *icons.Icon
	Color       ui.Color
}

// Store holds all application state as signals. Components that read a
// signal re-render when it changes.
type Store struct {
	Page     *ui.Signal[Page]
	Projects *ui.Signal[[]Project]
	Query    *ui.Signal[string]
	Filter   *ui.Signal[int] // -1 = all, otherwise a Status
	Selected *ui.Signal[int] // project ID, 0 = none
	Deploys  *ui.Signal[int]
	Notify   *ui.Signal[bool]
	Compact  *ui.Signal[bool]
	// DropMark is where a dragged project would land: the card it is over and
	// whether after it. The zero value shows no insertion line.
	DropMark *ui.Signal[dropMark]
	// EventQuery filters the Activity page.
	EventQuery *ui.Signal[string]
	// Expanded holds the build events whose details are open, by event ID.
	Expanded *ui.Signal[map[int]bool]
	// Notice is the status message that the Activity page announces, with a
	// sequence number so that each one is announced once (see announceQuiet).
	Notice *ui.Signal[notice]
	// Inbox holds the notifications the bell lists, newest first. Each toast
	// made by the app is also put here (see notify).
	Inbox *ui.Signal[[]inboxItem]
	// Typing is the project whose teammate is typing a reply, 0 for none.
	Typing *ui.Signal[int]
	quiet  *time.Timer // the pending announceQuiet, UI thread only
	quietN int         // the sequence number of the last announceQuiet
	// Threads holds each project's comments, by project ID.
	Threads *ui.Signal[map[int][]Message]
	// Windows holds the projects open in a window of their own, by project ID.
	Windows *ui.Signal[map[int]bool]
	// Search is the header's search field, which Ctrl+K focuses.
	Search *ui.FocusHandle
	// NewOpen shows the New Project form, whose fields are NewName and NewDesc.
	NewOpen *ui.Signal[bool]
	NewName *ui.Signal[string]
	NewDesc *ui.Signal[string]
	NewDue  *ui.Signal[time.Time] // the optional due date of the New Project form (see DatePicker)
	// NewForm checks the New Project form. It is reset when the form opens and closes.
	NewForm *ui.Form
	// SampleOpen shows the form sample of the Components page. SampleName is its
	// field, and SampleResult is what it last saved.
	SampleOpen   *ui.Signal[bool]
	SampleName   *ui.Signal[string]
	SampleResult *ui.Signal[string]

	nextID, nextMsg int
	// details are the project windows open, by project ID. Only the UI thread
	// uses them.
	details map[int]ui.Window
	// activity scrolls the Activity page's list.
	activity *ui.ListHandle
	// matched caches the events for the last Activity filter, so the list
	// gets the same slice while the filter holds.
	matchedQ   string
	matched    []event
	hasMatched bool
}

// dropMark is the insertion line shown while a project is dragged over a card.
type dropMark struct {
	ID    int
	After bool
}

var palette = []string{"#818cf8", "#34d399", "#f472b6", "#fbbf24", "#38bdf8", "#a78bfa", "#fb7185"}

// NewStore returns a store seeded with sample data.
func NewStore() *Store {
	now := time.Now()
	seed := []Project{
		{Name: "Project Alpha", Description: "Native GPU renderer and the scene graph that feeds it.", Status: StatusActive, Progress: 0.72, Tasks: 18, Updated: now.Add(-2 * time.Minute), Due: now.AddDate(0, 0, 14), Starred: true, Icon: icons.Zap},
		{Name: "Project Beta", Description: "Flexbox-style layout engine with measure caching.", Status: StatusActive, Progress: 0.45, Tasks: 9, Updated: now.Add(-48 * time.Minute), Due: now.AddDate(0, 0, 3), Icon: icons.Layers},
		{Name: "Text Pipeline", Description: "Font loading, shaping, glyph atlas and subpixel positioning.", Status: StatusActive, Progress: 0.6, Tasks: 12, Updated: now.Add(-3 * time.Hour), Icon: icons.Box},
		{Name: "Windows Adapter", Description: "Win32 windowing, DPI awareness and input in pure Go.", Status: StatusDone, Progress: 1, Tasks: 0, Updated: now.Add(-26 * time.Hour), Starred: true, Icon: icons.Dashboard},
		{Name: "macOS Adapter", Description: "Cocoa windowing with a Metal backend behind render.Renderer.", Status: StatusPaused, Progress: 0.1, Tasks: 23, Updated: now.Add(-5 * 24 * time.Hour), Icon: icons.Moon},
		{Name: "Dev Runner", Description: "Watch, rebuild and restart with preserved window geometry.", Status: StatusPaused, Progress: 0.05, Tasks: 7, Updated: now.Add(-9 * 24 * time.Hour), Icon: icons.GitBranch},
	}
	s := &Store{
		Page:         ui.NewSignal(PageProjects),
		Query:        ui.NewSignal(""),
		Filter:       ui.NewSignal(-1),
		Selected:     ui.NewSignal(0),
		Deploys:      ui.NewSignal(12),
		Notify:       ui.NewSignal(true),
		Compact:      ui.NewSignal(false),
		DropMark:     ui.NewSignal(dropMark{}),
		EventQuery:   ui.NewSignal(""),
		Expanded:     ui.NewSignal(map[int]bool{}),
		Notice:       ui.NewSignal(notice{}),
		Inbox:        ui.NewSignal([]inboxItem(nil)),
		Typing:       ui.NewSignal(0),
		Windows:      ui.NewSignal(map[int]bool{}),
		Search:       ui.NewFocusHandle(),
		NewOpen:      ui.NewSignal(false),
		NewName:      ui.NewSignal(""),
		NewDesc:      ui.NewSignal(""),
		NewDue:       ui.NewSignal(time.Time{}),
		NewForm:      ui.NewForm(),
		SampleOpen:   ui.NewSignal(false),
		SampleName:   ui.NewSignal(""),
		SampleResult: ui.NewSignal(""),
		details:      map[int]ui.Window{},
		activity:     ui.NewListHandle(),
	}
	for i := range seed {
		s.nextID++
		seed[i].ID = s.nextID
		seed[i].Color = ui.Hex(palette[i%len(palette)])
	}
	s.Projects = ui.NewSignal(seed)
	threads, n := seedThreads(seed, now)
	s.Threads = ui.NewSignal(threads)
	s.nextMsg = n
	return s
}

// defaultDescription is a new project's description when its form leaves it empty.
const defaultDescription = "Freshly created. Add a description and some tasks."

// OpenNewProject shows the New Project form, empty.
func (s *Store) OpenNewProject() {
	s.NewName.Set("")
	s.NewDesc.Set("")
	s.NewDue.Set(time.Time{})
	s.NewForm.Reset()
	s.NewOpen.Set(true)
}

// CloseNewProject hides the New Project form, and resets its checks.
func (s *Store) CloseNewProject() {
	s.NewOpen.Set(false)
	s.NewForm.Reset()
}

// CreateProject adds a project with the name and description given, puts it at
// the top of the list and selects it, and shows the Projects page with every
// project listed. An empty description gets defaultDescription, and a zero due is no
// due date. It returns the new project's ID.
func (s *Store) CreateProject(name, desc string, due time.Time) int {
	if desc == "" {
		desc = defaultDescription
	}
	s.nextID++
	id := s.nextID
	ics := []*icons.Icon{icons.Star, icons.Folder, icons.Activity, icons.Box, icons.Layers}
	s.Projects.Update(func(ps []Project) []Project {
		p := Project{
			ID: id, Name: name, Description: desc, Due: due,
			Status: StatusActive, Progress: 0, Tasks: 0, Updated: time.Now(),
			Icon: ics[id%len(ics)], Color: ui.Hex(palette[id%len(palette)]),
		}
		return append([]Project{p}, ps...)
	})
	s.Selected.Set(id)
	s.Filter.Set(-1)
	s.Page.Set(PageProjects)
	return id
}

// DeleteProject removes a project that has no thread yet. It is the undo of
// CreateProject, so it clears the selection when the project was selected.
func (s *Store) DeleteProject(id int) {
	s.Projects.Update(func(ps []Project) []Project {
		return slices.DeleteFunc(slices.Clone(ps), func(p Project) bool { return p.ID == id })
	})
	if s.Selected.Peek() == id {
		s.Selected.Set(0)
	}
}

// ToggleStar flips a project's star.
func (s *Store) ToggleStar(id int) {
	s.Projects.Update(func(ps []Project) []Project {
		out := append([]Project(nil), ps...)
		for i := range out {
			if out[i].ID == id {
				out[i].Starred = !out[i].Starred
			}
		}
		return out
	})
}

// RenameProject changes a project's name, and the title of its window if it
// has one.
func (s *Store) RenameProject(id int, name string) {
	s.Projects.Update(func(ps []Project) []Project {
		out := slices.Clone(ps)
		for i := range out {
			if out[i].ID == id {
				out[i].Name = name
			}
		}
		return out
	})
	if h := s.details[id]; h != nil {
		h.SetTitle(name)
	}
}

// MoveProject moves project id to just before, or just after, project
// targetID in the whole list. A status filter does not change where it lands.
func (s *Store) MoveProject(id, targetID int, after bool) {
	if id == targetID {
		return
	}
	s.Projects.Update(func(ps []Project) []Project {
		from := slices.IndexFunc(ps, func(p Project) bool { return p.ID == id })
		out := slices.Clone(ps)
		if from < 0 {
			return ps
		}
		moved := out[from]
		out = slices.Delete(out, from, from+1)
		to := slices.IndexFunc(out, func(p Project) bool { return p.ID == targetID })
		if to < 0 {
			return ps
		}
		if after {
			to++
		}
		return slices.Insert(out, to, moved)
	})
}

// SetProjectStatus sets the status of project id.
func (s *Store) SetProjectStatus(id int, status Status) {
	s.Projects.Update(func(ps []Project) []Project {
		out := slices.Clone(ps)
		for i := range out {
			if out[i].ID == id {
				out[i].Status = status
			}
		}
		return out
	})
}

// ToggleExpanded opens or closes the details of build event id.
func (s *Store) ToggleExpanded(id int) {
	s.Expanded.Update(func(m map[int]bool) map[int]bool {
		out := maps.Clone(m)
		if out[id] {
			delete(out, id)
		} else {
			out[id] = true
		}
		return out
	})
}

// setWindowOpen records whether project id is open in a window of its own.
func (s *Store) setWindowOpen(id int, open bool) {
	s.Windows.Update(func(m map[int]bool) map[int]bool {
		out := maps.Clone(m)
		if open {
			out[id] = true
		} else {
			delete(out, id)
		}
		return out
	})
}

// Comment appends a message to the thread of project id and returns it.
func (s *Store) Comment(id int, author, body string) Message {
	s.nextMsg++
	m := Message{ID: s.nextMsg, Author: author, Body: body, At: time.Now()}
	s.Threads.Update(func(ts map[int][]Message) map[int][]Message {
		out := maps.Clone(ts)
		// Clip, so the append copies: the old thread keeps its length.
		out[id] = append(slices.Clip(ts[id]), m)
		return out
	})
	return m
}

// Visible returns the projects matching the search query and status filter.
func (s *Store) Visible() []Project {
	q := strings.ToLower(strings.TrimSpace(s.Query.Get()))
	f := s.Filter.Get()
	var out []Project
	for _, p := range s.Projects.Get() {
		if f >= 0 && int(p.Status) != f {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Description), q) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour")
	}
	return plural(int(d.Hours()/24), "day")
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s ago", unit)
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}

// Project returns the project with the given ID.
func (s *Store) Project(id int) (Project, bool) {
	for _, p := range s.Projects.Get() {
		if p.ID == id {
			return p, true
		}
	}
	return Project{}, false
}

// notice is a status message for assistive technology, numbered.
type notice struct {
	Text string
	Seq  int
}

// quietFor is how long the user must stop before a debounced announcement is
// made.
const quietFor = 600 * time.Millisecond

// announceQuiet announces text once the user has been quiet for quietFor. Each
// call restarts the wait, so a burst of keystrokes is announced once. The timer
// runs on its own goroutine, so it sets the Notice signal, and the Activity page
// makes the announcement when it renders (ui.Announce is queued for the next
// published tree, which a frame makes).
func (s *Store) announceQuiet(text string) {
	if s.quiet != nil {
		s.quiet.Stop()
	}
	s.quietN++
	n := notice{Text: text, Seq: s.quietN}
	s.quiet = time.AfterFunc(quietFor, func() { s.Notice.Set(n) })
}
