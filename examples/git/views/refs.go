package views

import (
	"cmp"
	"slices"
	"strings"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/repo"
)

// label is a ref as a pill shows it. A local branch and the remote branches at
// the same commit with the same name are one label, "main" with "origin".
type label struct {
	Name    string
	Kind    repo.RefKind
	Remotes []string // the remotes whose branch of this name is here too
	Head    bool     // the branch HEAD is on
}

// labels groups the refs of one commit for display: HEAD's branch first, then
// the other local branches, the remote branches no local branch matches, and
// the tags.
func labels(refs []repo.Ref, head string) []label {
	var out []label
	local := map[string]int{}
	for _, r := range refs {
		if r.Kind == repo.Branch {
			local[r.Name] = len(out)
			out = append(out, label{Name: r.Name, Kind: repo.Branch, Head: r.Name == head})
		}
	}
	for _, r := range refs {
		switch r.Kind {
		case repo.Remote:
			remote, name, _ := strings.Cut(r.Name, "/")
			if i, ok := local[name]; ok {
				out[i].Remotes = append(out[i].Remotes, remote)
				continue
			}
			out = append(out, label{Name: r.Name, Kind: repo.Remote})
		case repo.Tag:
			out = append(out, label{Name: r.Name, Kind: repo.Tag})
		}
	}
	slices.SortStableFunc(out, func(a, b label) int {
		if a.Head != b.Head {
			if a.Head {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Kind, b.Kind)
	})
	return out
}

// text is the label's accessible text, as "main (HEAD, origin)".
func (l label) text() string {
	var extra []string
	if l.Head {
		extra = append(extra, "HEAD")
	}
	extra = append(extra, l.Remotes...)
	if l.Kind == repo.Tag {
		extra = append(extra, "tag")
	}
	if len(extra) == 0 {
		return l.Name
	}
	return l.Name + " (" + strings.Join(extra, ", ") + ")"
}

// pill draws one label in the color of its commit's lane: a tinted fill, the
// lane's color on the icon, and the name in the text color, which reads on the
// tint in both themes. HEAD's branch is filled solid.
func pill(t *ui.Theme, l label, lane ui.Color) ui.Element {
	ic := icons.GitBranch
	if l.Kind == repo.Tag {
		ic = icons.Tag
	}
	fill, border, ink, iconInk := lane.Alpha(0.14), lane.Alpha(0.32), t.Text, lane
	if l.Head {
		fill, border, ink, iconInk = lane, lane, ui.Hex("#ffffff"), ui.Hex("#ffffff")
	}
	if t.HighContrast {
		fill, border, ink, iconInk = t.Background, t.Text, t.Text, t.Text
	}
	d := ui.Div().Row().Align(ui.Center).Gap(4).Height(ui.Px(20)).PaddingX(6).Shrink(0).MaxWidth(ui.Px(200)).
		Radius(5).Background(fill).Border(1, border).
		Child(ui.Icon(ic).Size(11).Color(iconInk)).
		Child(ui.Text(l.Name).FontSize(11.5).Weight(ui.FontMedium).TextColor(ink).Truncate())
	if len(l.Remotes) > 0 {
		d.Child(ui.Text(strings.Join(l.Remotes, " ")).FontSize(10.5).TextColor(ink.Alpha(0.7)).Truncate())
	}
	return d.Tooltip(l.text())
}

// pills draws up to two labels and a count of the rest.
func pills(t *ui.Theme, ls []label, lane ui.Color) ui.Element {
	row := ui.Div().Row().Align(ui.Center).Gap(4).Shrink(0)
	for i, l := range ls {
		if i == 2 {
			var rest []string
			for _, r := range ls[2:] {
				rest = append(rest, r.text())
			}
			row.Child(ui.Div().Row().Align(ui.Center).Height(ui.Px(20)).PaddingX(6).Radius(5).
				Background(t.SurfaceActive).
				Child(ui.Text("+" + itoa(len(ls)-2)).FontSize(11).Weight(ui.FontMedium).TextColor(t.TextMuted)).
				Tooltip(strings.Join(rest, "\n")))
			break
		}
		row.Child(pill(t, l, lane))
	}
	return row
}
