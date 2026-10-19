package views

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/ui"
)

// sampleFilters are the file types the open and save samples offer.
var sampleFilters = []dialog.Filter{{Name: "Notes", Patterns: []string{"*.md", "*.txt"}}}

// The message boxes the Dialogs section shows, one per style.
var (
	infoBox = dialog.MessageOptions{Title: "Information", Text: "The notes were saved.", Icon: dialog.IconInfo}
	warnBox = dialog.MessageOptions{Title: "Warning", Text: "This replaces the file on disk.", Icon: dialog.IconWarning,
		Buttons: dialog.ButtonsOKCancel, Default: dialog.Cancel}
	errorBox = dialog.MessageOptions{Title: "Error", Text: "The file could not be read.", Icon: dialog.IconError}
	questBox = dialog.MessageOptions{Title: "Delete note", Text: "Delete this note? This cannot be undone.", Icon: dialog.IconQuestion,
		Buttons: dialog.ButtonsYesNo, Default: dialog.No}
	closeBox = dialog.MessageOptions{Title: "Unsaved changes", Text: "Save the changes before closing?", Icon: dialog.IconQuestion,
		Buttons: dialog.ButtonsYesNoCancel, Default: dialog.Cancel}
)

// dialogsSection shows the native dialogs, a form in a jamb modal, and the
// clipboard. The line under the buttons says what the last one returned. Files
// are named by their file name only, so no folder shows on the page.
func dialogsSection(ctx *ui.Context, t *ui.Theme, s *Store) ui.Element {
	result := ui.Local(ctx, "dialog-result", "No dialog has been opened yet.")
	openFiles := func(title string, multiple bool) func() {
		return func() {
			ctx.Window().OpenFiles(dialog.FileOptions{Title: title, Filters: sampleFilters, Multiple: multiple},
				func(r dialog.FileResult) { result.Set(describeFiles(r)) })
		}
	}
	message := func(o dialog.MessageOptions) func() {
		return func() {
			ctx.Window().MessageBox(o, func(b dialog.Button) { result.Set("Answer: " + describeButton(b)) })
		}
	}
	return section(t, "Dialogs",
		sample(t, "Open file", ui.Button("Open file").Secondary().OnClick(openFiles("Open a note", false))),
		sample(t, "Open several", ui.Button("Open several").Secondary().OnClick(openFiles("Open notes", true))),
		sample(t, "Save", ui.Button("Save as…").Secondary().OnClick(func() {
			ctx.Window().SaveFile(dialog.FileOptions{Title: "Save a note", Name: "Untitled.md", Extension: "md", Filters: sampleFilters},
				func(r dialog.FileResult) { result.Set(describeSave(r)) })
		})),
		sample(t, "Folder", ui.Button("Pick folder").Secondary().OnClick(func() {
			ctx.Window().PickFolder(dialog.FileOptions{Title: "Choose a folder"},
				func(r dialog.FileResult) { result.Set(describeFolder(r)) })
		})),
		sample(t, "Information", ui.Button("Information").Secondary().OnClick(message(infoBox))),
		sample(t, "Warning", ui.Button("Warning").Secondary().OnClick(message(warnBox))),
		sample(t, "Error", ui.Button("Error").Secondary().OnClick(message(errorBox))),
		sample(t, "Question", ui.Button("Question").Secondary().OnClick(message(questBox))),
		sample(t, "Yes, No or Cancel", ui.Button("Yes / No / Cancel").Secondary().OnClick(message(closeBox))),
		sample(t, "Form in a modal", ui.Button("Form in a modal").Secondary().OnClick(func() {
			s.SampleName.Set("")
			s.SampleOpen.Set(true)
		})),
		sample(t, "Drawer", ui.Button("Open drawer").Secondary().OnClick(func() { s.DrawerOpen.Set(true) })),
		sample(t, "Command palette", ui.Button("Open command palette").Secondary().
			OnClick(func() { s.PaletteOpen.Set(true) })),
		sample(t, "Clipboard", ui.Div().Row().Gap(8).
			Child(ui.Button("Copy text").Secondary().OnClick(func() {
				ctx.SetClipboard("Copied from the Components page")
				result.Set("Copied the sample text to the clipboard.")
				notify(ctx, s, ui.ToastSuccess, "Copied to the clipboard")
			})).
			Child(ui.Button("Paste").Secondary().OnClick(func() {
				text, ok := ctx.Clipboard()
				if !ok {
					result.Set("The clipboard holds no text.")
					return
				}
				result.Set(fmt.Sprintf("The clipboard holds %d characters of text.", utf8.RuneCountInString(text)))
			}))),
		sample(t, "Result", ui.Div().MaxWidth(ui.Px(560)).Child(ui.Text(result.Get()).FontSize(13).TextColor(t.TextMuted))),
		ui.If(s.SampleResult.Get() != "", sample(t, "Modal form result", ui.Text(s.SampleResult.Get()).FontSize(13).TextColor(t.TextMuted))),
	)
}

// describeFiles says what an open dialog returned, by file name.
func describeFiles(r dialog.FileResult) string {
	switch {
	case r.Err != nil:
		return "The dialog could not open: " + r.Err.Error()
	case r.Canceled:
		return "Canceled."
	}
	names := make([]string, len(r.Paths))
	for i, p := range r.Paths {
		names[i] = filepath.Base(p)
	}
	noun := "files"
	if len(names) == 1 {
		noun = "file"
	}
	return fmt.Sprintf("Selected %d %s: %s", len(names), noun, strings.Join(names, ", "))
}

// describeSave says what a save dialog returned, by file name.
func describeSave(r dialog.FileResult) string {
	switch {
	case r.Err != nil:
		return "The dialog could not open: " + r.Err.Error()
	case r.Canceled:
		return "Canceled."
	}
	return "Saved as " + filepath.Base(r.Paths[0])
}

// describeFolder says what the folder dialog returned, by the folder's name.
func describeFolder(r dialog.FileResult) string {
	switch {
	case r.Err != nil:
		return "The dialog could not open: " + r.Err.Error()
	case r.Canceled:
		return "Canceled."
	}
	return "Folder: " + filepath.Base(r.Paths[0])
}

// describeButton names the button a message box returned.
func describeButton(b dialog.Button) string {
	switch b {
	case dialog.OK:
		return "OK"
	case dialog.Cancel:
		return "Canceled"
	case dialog.Yes:
		return "Yes"
	case dialog.No:
		return "No"
	}
	return "the box could not open"
}
