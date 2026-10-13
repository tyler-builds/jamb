package views

import (
	"strings"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// newProjectModal is the New Project form: a name and a description. It is
// shown while s.NewOpen is set. Create submits the form: a blank name, or one
// longer than 80 characters, is reported in the summary and the field, and focus
// moves to the name. Enter in the name field submits it too.
func newProjectModal(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		if !s.NewOpen.Get() {
			return nil
		}
		form := s.NewForm.OnSubmit(func() error {
			id := s.CreateProject(strings.TrimSpace(s.NewName.Peek()), strings.TrimSpace(s.NewDesc.Peek()), s.NewDue.Peek())
			notify(ctx, s, ui.ToastSuccess, "Project created").Action("Undo", func() { s.DeleteProject(id) })
			s.CloseNewProject()
			return nil
		})
		create := ui.Button("Create project").Key("project-create").Icon(icons.Plus).OnClick(form.Submit)
		cancel := ui.Button("Cancel").Key("project-cancel").Secondary().OnClick(s.CloseNewProject)
		body := ui.Div().Column().Gap(12).
			Child(form.Summary()).
			Child(form.Field("name", "Name", ui.TextInput(s.NewName).Placeholder("Project name").Width(ui.Full).
				Validate(ui.Required[string](), ui.MaxLength(80))).Required()).
			Child(form.Field("description", "Description", ui.TextArea(s.NewDesc).Placeholder("What the project is for").
				MinRows(3).Width(ui.Full)).Description("Optional. Shown on the project's page.")).
			Child(form.Field("due", "Due date", ui.DatePicker(s.NewDue).Width(ui.Full)).
				Description("Optional. Shown in the Due column of the projects table."))
		return ui.Modal("New project", body).Footer(cancel, create).Default(create).
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
		body := labeledField(t, "Name", ui.TextInput(s.SampleName).Label("Name").LabelledBy(fieldLabel("Name")).Placeholder("A name for the sample").Width(ui.Full))
		return ui.Modal("Form in a modal", body).Footer(cancel, save).Default(save).
			OnClose(func() { s.SampleOpen.Set(false) }).CloseOnBackdrop()
	})
}

// labeledField is a field with its label above it.
func labeledField(t *ui.Theme, label string, field ui.Element) ui.Element {
	return ui.Div().Column().Gap(6).
		Child(ui.Text(label).FontSize(12).Medium().TextColor(t.TextMuted).Key(fieldLabel(label))).
		Child(field)
}

// sampleDrawer is the drawer of the Components page: display settings that
// slide in from the right, with the window's own controls in it.
func sampleDrawer(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		if !s.DrawerOpen.Get() {
			return nil
		}
		t := ctx.Theme()
		density := ui.Local(ctx, "density", "comfortable")
		closeFn := func() { s.DrawerOpen.Set(false) }
		body := ui.Div().Column().Gap(14).
			Child(ui.Text("Changes apply to this sample only.").FontSize(13).TextColor(t.TextMuted)).
			Child(ui.RadioGroup(density,
				ui.Option[string]{Value: "comfortable", Label: "Comfortable"},
				ui.Option[string]{Value: "compact", Label: "Compact"},
			).Label("Density"))
		done := ui.Button("Done").OnClick(closeFn)
		return ui.Drawer("Display settings", body).Side(ui.DrawerRight).
			Footer(done).Default(done).OnClose(closeFn)
	})
}
