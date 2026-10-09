package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// eventKind says how an event's row is built.
type eventKind uint8

const (
	statusEvent  eventKind = iota // one line
	commentEvent                  // the comment's text wraps under the header
	buildEvent                    // a row that expands to show the build's details
)

type event struct {
	id              int
	kind            eventKind
	who, verb, what string
	minutes         int
	icon            *icons.Icon
	color           string
	body            string   // the text of a comment
	details         []string // the lines a build shows when it is expanded
}

// text is what the filter searches: the header and the comment.
func (e event) text() string {
	return e.who + " " + e.verb + " " + e.what + " " + e.body
}

var people = []string{"Alex Rivera", "Sam Chen", "Priya Patel", "Jordan Lee", "Maya Okafor"}

var statusVerbs = []struct {
	verb  string
	icon  *icons.Icon
	color string
}{
	{"completed a task in", icons.Check, "#34d399"},
	{"starred", icons.Star, "#fbbf24"},
	{"created", icons.Plus, "#f472b6"},
	{"scheduled a review for", icons.Calendar, "#a78bfa"},
	{"changed the status of", icons.Activity, "#fb7185"},
}

var targets = []string{"Project Alpha", "Project Beta", "Text Pipeline", "Windows Adapter", "Dev Runner"}

// sampleEvents makes n events in a repeating mix: two of three are status
// changes or comments, and every fourth is a build with details.
func sampleEvents(n int) []event {
	var out []event
	m := 2
	for i := 0; i < n; i++ {
		e := event{
			id:      i + 1,
			who:     people[(i*3)%len(people)],
			what:    targets[(i+i/3)%len(targets)],
			minutes: m,
		}
		switch i % 4 {
		case 1:
			e.kind = commentEvent
			e.verb, e.icon, e.color = "commented on", icons.Inbox, "#38bdf8"
			e.body = messageBody(i%len(targets)+1, i)
		case 3:
			e.kind = buildEvent
			e.verb, e.icon, e.color = "finished a build of", icons.GitBranch, "#818cf8"
			e.details = buildDetails(i)
		default:
			v := statusVerbs[(i*7+3)%len(statusVerbs)]
			e.verb, e.icon, e.color = v.verb, v.icon, v.color
		}
		out = append(out, e)
		m += 7 + (i*13)%45
	}
	return out
}

// buildDetails are the lines of a build's expandable section.
func buildDetails(i int) []string {
	env := []string{"staging", "test machine", "nightly"}
	return []string{
		fmt.Sprintf("Build #%d on main, compiled in %d.%d s", 400+i%600, 10+i%40, i%10),
		fmt.Sprintf("%s tests passed, %d skipped", thousands(900+(i*37)%400), i%3),
		fmt.Sprintf("Deployed to %s", env[i%len(env)]),
	}
}

var events = sampleEvents(10_000)

// thousands formats n with a comma between each group of three digits.
func thousands(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// matchingEvents returns the events whose text contains query, ignoring case.
// A blank query returns every event. The slice is kept while the query holds,
// so the list sees the same items on each render and does not rebuild its
// index.
func (s *Store) matchingEvents(query string) []event {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return events
	}
	if s.hasMatched && q == s.matchedQ {
		return s.matched
	}
	var out []event
	for _, e := range events {
		if strings.Contains(strings.ToLower(e.text()), q) {
			out = append(out, e)
		}
	}
	s.matchedQ, s.matched, s.hasMatched = q, out, true
	return out
}

// Activity is a long timeline with a filter. The header and toolbar stay put;
// the list builds only the rows on screen. Rows differ in height: a status
// change is one line, a comment's text wraps, and a build expands.
func Activity(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		t := ctx.Theme()
		compact := s.Compact.Get()
		query := s.EventQuery.Get()
		expanded := s.Expanded.Get()
		shown := s.matchingEvents(query)

		count := fmt.Sprintf("%s events", thousands(len(shown)))
		if len(shown) == 1 {
			count = "1 event"
		}
		if strings.TrimSpace(query) != "" {
			count = fmt.Sprintf("%s matching events", thousands(len(shown)))
			if len(shown) == 1 {
				count = "1 matching event"
			}
		}

		// Wrap keeps the toolbar on two lines in a narrow window.
		toolbar := ui.Div().Row().Align(ui.Center).Justify(ui.SpaceBetween).Gap(12).Wrap().Shrink(0).MaxWidth(ui.Px(900)).
			Child(ui.Div().Row().Align(ui.Center).Gap(12).
				Child(ui.TextInput(s.EventQuery).Placeholder("Filter events").Icon(icons.Search).
					Style(ui.Style{Width: ui.Px(260), MinWidth: ui.Px(140)})).
				Child(ui.Text(count).Size(12).Color(t.TextMuted).NoWrap())).
			Child(ui.Div().Row().Align(ui.Center).Gap(8).
				Child(ui.Button("Jump to top").Icon(icons.ChevronUp).Secondary().Small().Key("jump-top").
					OnClick(func() { s.activity.ScrollToItem(0) })).
				Child(ui.Button("Jump to latest").Icon(icons.ChevronDown).Secondary().Small().Key("jump-latest").
					OnClick(s.activity.ScrollToEnd)))

		var body ui.Element
		if len(shown) == 0 {
			body = noEvents(t, s)
		} else {
			body = ui.ListOf(shown, func(e event) string { return strconv.Itoa(e.id) }, func(e event) ui.Element {
				return eventRow(t, s, e, compact, expanded[e.id], query).Key(fmt.Sprintf("event-%d", e.id))
			}).Key("events").Style(ui.Style{Grow: 1}).Handle(s.activity)
		}
		return ui.Div().Grow(1).MinHeight(ui.Px(0)).PaddingX(32).PaddingY(28).Gap(20).
			Child(ui.Div().Gap(4).Shrink(0).
				Child(ui.Text("Activity").Size(26).Semibold()).
				Child(ui.Textf("%s events from %d people", thousands(len(events)), len(people)).Color(t.TextMuted))).
			Child(toolbar).
			Child(Card(t).MaxWidth(ui.Px(900)).Grow(1).MinHeight(ui.Px(0)).Gap(0).Padding(6).Child(body))
	})
}

// eventRow is one timeline entry with its divider. The project name links to
// the project. A build row is clickable and focusable, and shows its details
// when open. Matches of the filter are highlighted in the header and the
// comment.
func eventRow(t *ui.Theme, s *Store, e event, compact, open bool, query string) *ui.DivElement {
	c := ui.Hex(e.color)
	pad := float32(12)
	if compact {
		pad = 6
	}
	when := fmt.Sprintf("%d min ago", e.minutes)
	if e.minutes >= 60 {
		when = fmt.Sprintf("%dh %02dm ago", e.minutes/60, e.minutes%60)
	}
	hl := matchHighlight(t)
	header := ui.Rich(
		ui.Seg(e.who).Style(ui.Highlight{Weight: ui.FontSemibold}),
		ui.Seg(" "+e.verb+" ").Color(t.TextMuted),
		ui.Link(e.what, openProject(s, e.what)).Style(ui.Highlight{Weight: ui.FontMedium}),
	).HighlightMatches(query, hl).Grow(1).Truncate()
	text := ui.Div().Grow(1).MinWidth(ui.Px(0)).Gap(4).Child(header)
	if e.body != "" {
		text.Child(ui.Text(e.body).Size(13).Color(t.TextMuted).MaxLines(6).HighlightMatches(query, hl))
	}
	if e.kind == buildEvent && open {
		details := ui.Div().Key(fmt.Sprintf("details-%d", e.id)).Gap(2).PaddingY(2)
		for _, line := range e.details {
			details.Child(ui.Text(line).Size(12).Mono().Color(t.TextMuted))
		}
		text.Child(details)
	}

	row := ui.Div().Row().Align(ui.Start).Gap(14).PaddingX(12).PaddingY(pad).Radius(8).
		Transition(fast).Hover(ui.Style{Background: t.SurfaceHover}).
		Child(IconTile(e.icon, c, 32)).
		Child(text)
	if e.kind == buildEvent {
		// Enter and Space expand it too, as a click does.
		chevron := icons.ChevronRight
		if open {
			chevron = icons.ChevronDown
		}
		row.Focusable().FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: t.FocusRing}}}).
			OnClick(func() { s.ToggleExpanded(e.id) }).
			Child(ui.Icon(chevron).Size(14).Color(t.TextSubtle).Style(ui.Style{Margin: ui.Edges{Top: 2}}))
	}
	row.Child(ui.Text(when).Size(12).Color(t.TextSubtle).NoWrap())
	return ui.Div().Child(row).Child(ui.Div().Height(ui.Px(1)).MarginX(12).Background(t.Border.Alpha(0.6)))
}

// noEvents is the empty state of the timeline, with a button to clear the filter.
func noEvents(t *ui.Theme, s *Store) ui.Element {
	return ui.Div().Key("no-events").Grow(1).Padding(48).Gap(10).Align(ui.Center).Justify(ui.Center).
		Child(ui.Text("No events match").Size(15).Semibold()).
		Child(ui.Text("Try a different filter.").Color(t.TextMuted)).
		Child(ui.Button("Clear filter").Secondary().Small().Style(ui.Style{Margin: ui.Edges{Top: 6}}).
			OnClick(func() { s.EventQuery.Set("") }))
}

// openProject returns the click handler of a project link: it shows the
// Projects page with the named project selected. The search and filter are
// cleared so the project is on screen.
func openProject(s *Store, name string) func() {
	return func() {
		for _, p := range s.Projects.Get() {
			if p.Name == name {
				s.Query.Set("")
				s.Filter.Set(-1)
				s.Selected.Set(p.ID)
				s.Page.Set(PageProjects)
				return
			}
		}
	}
}
