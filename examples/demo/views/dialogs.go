package views

import (
	"strings"
	"time"

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
			OnClose(func() { s.SampleOpen.Set(false) }).CloseOnBackdrop().CloseButton(true)
	})
}

// labeledField is a field with its label above it.
func labeledField(t *ui.Theme, label string, field ui.Element) ui.Element {
	return ui.Div().Column().Gap(6).
		Child(ui.Text(label).FontSize(12).Medium().TextColor(t.TextMuted).Key(fieldLabel(label))).
		Child(field)
}

// sampleAlertDialog is the alert dialog of the Components page: it asks before
// deleting a sample, and the delete takes a moment, during which its button is
// loading. The answer is written to SampleResult, which the page shows.
func sampleAlertDialog(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		deleting := ui.Local(ctx, "deleting", false)
		if !s.AlertOpen.Get() {
			return nil
		}
		return ui.AlertDialog("Delete the sample?", "The sample is removed from this page. This cannot be undone.").
			Intent(ui.AlertDialogDestructive).
			Cancel("", func() { s.AlertOpen.Set(false) }).
			Confirm("Delete", func() {
				deleting.Set(true)
				ctx.After(time.Second, func() {
					deleting.Set(false)
					s.SampleResult.Set("Deleted the sample")
					s.AlertOpen.Set(false)
				})
			}).
			ConfirmLoading(deleting.Get())
	})
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
			Footer(done).Default(done).OnClose(closeFn).CloseButton(true)
	})
}

// sampleFind is the find and replace sample of the Components page. It is a
// non-modal dialog, so the page stays usable while it is open, and F6 moves focus
// between it, the page and the toasts. Replace reports what it did in the dialog.
func sampleFind(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		status := ui.Local(ctx, "find-status", "")
		if !s.FindOpen.Get() {
			return nil
		}
		t := ctx.Theme()
		replace := ui.Button("Replace").Key("find-replace").Primary().OnClick(func() {
			find := strings.TrimSpace(s.FindQuery.Peek())
			if find == "" {
				status.Set("Type the text to find first.")
				return
			}
			status.Set("Replaced “" + find + "” with “" + strings.TrimSpace(s.FindReplace.Peek()) + "”.")
		})
		body := ui.Div().Column().Gap(12).
			Child(ui.Text("The page stays usable while this dialog is open. Press F6 to move between it, " +
				"the page and the toasts.").FontSize(13).TextColor(t.TextMuted)).
			Child(labeledField(t, "Find", ui.TextInput(s.FindQuery).Label("Find").LabelledBy(fieldLabel("Find")).
				Placeholder("Text to find").Width(ui.Full))).
			Child(labeledField(t, "Replace with", ui.TextInput(s.FindReplace).Label("Replace with").
				LabelledBy(fieldLabel("Replace with")).Placeholder("Replacement text").Width(ui.Full))).
			Child(ui.If(status.Get() != "", ui.Text(status.Get()).FontSize(13).TextColor(t.TextMuted)))
		return ui.Modal("Find and replace", body).Key("sample-find").NonModal().
			Footer(replace).Default(replace).OnClose(func() { s.FindOpen.Set(false) })
	})
}

// sampleSheet is the properties sample of the Components page: a non-modal drawer
// at the right edge, so the page stays usable while it is open, and it can be open
// together with the find sample. Its details are made up for the sample.
func sampleSheet(s *Store) ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		pinned := ui.Local(ctx, "sheet-pinned", false)
		if !s.SheetOpen.Get() {
			return nil
		}
		t := ctx.Theme()
		closeFn := func() { s.SheetOpen.Set(false) }
		body := ui.Div().Column().Gap(14).
			Child(ui.Text("A sample note, with made-up details.").FontSize(13).TextColor(t.TextMuted)).
			Child(ui.Checkbox("Pinned to the top", pinned))
		done := ui.Button("Done").Key("sheet-done").OnClick(closeFn)
		return ui.Drawer("Properties", body).Key("sample-sheet").Side(ui.DrawerRight).NonModal().
			Footer(done).Default(done).OnClose(closeFn)
	})
}
