package views

import (
	"strings"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// newProjectModal is the New Project form: a name and a description. It is
// shown while s.NewOpen is set. Create is disabled until the name has text, and
// Enter runs it when the name field has focus.
func newProjectModal(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		if !s.NewOpen.Get() {
			return nil
		}
		t := ctx.Theme()
		create := ui.Button("Create project").Key("project-create").Icon(icons.Plus).
			Disabled(strings.TrimSpace(s.NewName.Get()) == "").
			OnClick(func() {
				s.CreateProject(strings.TrimSpace(s.NewName.Peek()), strings.TrimSpace(s.NewDesc.Peek()))
				s.CloseNewProject()
			})
		cancel := ui.Button("Cancel").Key("project-cancel").Secondary().OnClick(s.CloseNewProject)
		form := ui.Div().Column().Gap(12).
			Child(labeledField(t, "Name", ui.TextInput(s.NewName).Label("Name").Placeholder("Project name"))).
			Child(labeledField(t, "Description", ui.TextArea(s.NewDesc).Label("Description").Placeholder("What the project is for (optional)").MinRows(3)))
		return ui.Modal("New project", form).Footer(cancel, create).Default(create).
			OnClose(s.CloseNewProject).CloseOnBackdrop()
	})
}

// sampleModal is the form sample of the Components page. Its Save writes the
// name to SampleResult, which the page shows.
func sampleModal(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		if !s.SampleOpen.Get() {
			return nil
		}
		t := ctx.Theme()
		save := ui.Button("Save").Icon(icons.Check).
			OnClick(func() {
				s.SampleResult.Set("Saved “" + strings.TrimSpace(s.SampleName.Peek()) + "”")
				s.SampleOpen.Set(false)
			})
		cancel := ui.Button("Cancel").Secondary().OnClick(func() { s.SampleOpen.Set(false) })
		body := labeledField(t, "Name", ui.TextInput(s.SampleName).Label("Name").Placeholder("A name for the sample"))
		return ui.Modal("Form in a modal", body).Footer(cancel, save).Default(save).
			OnClose(func() { s.SampleOpen.Set(false) }).CloseOnBackdrop()
	})
}

// labeledField is a field with its label above it.
func labeledField(t *ui.Theme, label string, field ui.Element) ui.Element {
	return ui.Div().Column().Gap(6).
		Child(ui.Text(label).Size(12).Medium().Color(t.TextMuted)).
		Child(field)
}
