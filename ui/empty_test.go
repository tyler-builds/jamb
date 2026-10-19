package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
)

// TestEmptyGroup checks that an empty state is a group named by its title and
// described by its description, with its action inside it.
func TestEmptyGroup(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Empty("No projects").Description("Create a project to get started.").
			Action(Button("New project").Key("new"))
	})
	tr := r.tree()
	n := accesstest.Find(t, tr, access.RoleGroup, "No projects")
	if n.Description != "Create a project to get started." {
		t.Errorf("description %q, want the empty state's description", n.Description)
	}
	b := accesstest.Find(t, tr, access.RoleButton, "New project")
	if b.Parent != n.ID {
		t.Error("the action is not inside the group")
	}
}

// TestEmptyIcon checks that an empty state shows an inbox icon by default, that
// Icon replaces it, and that Icon(nil) hides it.
func TestEmptyIcon(t *testing.T) {
	for _, c := range []struct {
		name  string
		build func() *EmptyElement
		icon  bool
	}{
		{"default", func() *EmptyElement { return Empty("Nothing here") }, true},
		{"replaced", func() *EmptyElement { return Empty("Nothing here").Icon(icons.Search) }, true},
		{"hidden", func() *EmptyElement { return Empty("Nothing here").Icon(nil) }, false},
	} {
		r := newAccessRig(func(ctx *Context) Element { return c.build() })
		if got := len(partNodes(r.rt, "Empty.Icon")) == 1; got != c.icon {
			t.Errorf("%s: icon drawn %v, want %v", c.name, got, c.icon)
		}
		if len(partNodes(r.rt, "Empty.Description")) != 0 || len(partNodes(r.rt, "Empty.Action")) != 0 {
			t.Errorf("%s: a description or an action is drawn without one", c.name)
		}
	}
}

// TestEmptyLook checks the default look: centered parts, a muted icon and
// description, and under high contrast the text color for both.
func TestEmptyLook(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return Empty("Nothing here").Description("Try again.") })
	th := r.rt.theme
	box := partNode(t, r.rt, "Empty.Empty", "")
	if box.style.Align != Center || box.style.Justify != Center {
		t.Errorf("box align %v and justify %v, want both centered", box.style.Align, box.style.Justify)
	}
	if c := partNode(t, r.rt, "Empty.Description", "").style.TextColor; c != th.TextMuted {
		t.Errorf("description %v, want the muted text color %v", c, th.TextMuted)
	}

	theme := *DarkTheme()
	theme.Empty.Icon = Style{TextColor: markFG}
	hc := newThemedRig(&theme, func(ctx *Context) Element {
		return Empty("Nothing here").Description("Try again.").Styles(EmptyStyle{Description: Style{TextColor: markFG}})
	})
	hc.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	hc.frame()
	th = hc.rt.theme
	for _, part := range []string{"Empty.Icon", "Empty.Description"} {
		if c := partNode(t, hc.rt, part, "").style.TextColor; c != th.Text {
			t.Errorf("high contrast: %s is %v, want the text color %v", part, c, th.Text)
		}
	}
}

// TestEmptyInADataGrid checks that an empty state fills a data grid's empty slot.
func TestEmptyInADataGrid(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return DataGrid([]string(nil), func(s string) string { return s },
			DataColumn[string]{Header: "Name", Key: "name", Text: func(s string) string { return s }}).
			Label("Files").Empty(Empty("No files").Description("Drop files here."))
	})
	accesstest.Find(t, r.tree(), access.RoleGroup, "No files")
}

func testEmptyParts(t *testing.T, applied map[string]bool) {
	type emptyCase struct {
		part string
		set  func(*EmptyStyle, Style)
		text bool
	}
	cases := []emptyCase{
		{part: "Empty.Empty", set: func(s *EmptyStyle, m Style) { s.Empty = m }},
		{part: "Empty.Icon", set: func(s *EmptyStyle, m Style) { s.Icon = m }, text: true},
		{part: "Empty.Title", set: func(s *EmptyStyle, m Style) { s.Title = m }, text: true},
		{part: "Empty.Description", set: func(s *EmptyStyle, m Style) { s.Description = m }, text: true},
		{part: "Empty.Action", set: func(s *EmptyStyle, m Style) { s.Action = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st EmptyStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element {
				return Empty("No projects").Description("Create one.").Action(Button("New project")).Styles(st)
			})
			n := partNode(t, r.rt, c.part, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}
