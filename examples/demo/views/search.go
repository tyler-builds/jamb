package views

import (
	"strings"

	"github.com/tyler-builds/jamb/ui"
)

// matchHighlight is how a search match looks: strong enough to find at a
// glance on both themes (AccentSoft was too faint on the dark one). It is the
// same as the notes app's.
func matchHighlight(t *ui.Theme) ui.Highlight {
	return ui.Highlight{Background: t.Accent.Alpha(0.4), Color: t.Text}
}

// searchOptions returns the suggestions of the header's search: the projects whose
// name or description contains the query. Each suggestion's value is the project's ID.
func searchOptions(s *Store) func(q string) []ui.Option[int] {
	return func(q string) []ui.Option[int] {
		q = strings.ToLower(strings.TrimSpace(q))
		var out []ui.Option[int]
		for _, p := range s.Projects.Get() {
			if q != "" && !strings.Contains(strings.ToLower(p.Name+" "+p.Description), q) {
				continue
			}
			out = append(out, ui.Option[int]{Value: p.ID, Label: p.Name})
		}
		return out
	}
}

// searchOption is the row of a search suggestion: the project's icon and its name,
// with the text of the query highlighted in the name.
func searchOption(s *Store, t *ui.Theme) func(o ui.Option[int], st ui.PartState) ui.Element {
	return func(o ui.Option[int], st ui.PartState) ui.Element {
		p, ok := s.Project(o.Value)
		if !ok {
			return ui.Text(o.Label).NoWrap()
		}
		return ui.Div().Row().Align(ui.Center).Gap(10).
			Child(IconTile(t, p.Icon, p.Color, 22)).
			Child(ui.Text(p.Name).NoWrap().HighlightMatches(s.Query.Peek(), matchHighlight(t)))
	}
}
