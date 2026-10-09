package views

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
)

// The file types the import and export dialogs offer.
var (
	importFilters = []dialog.Filter{{Name: "Notes", Patterns: []string{"*.md", "*.txt"}}}
	exportFilters = []dialog.Filter{
		{Name: "Markdown", Patterns: []string{"*.md"}},
		{Name: "Text", Patterns: []string{"*.txt"}},
	}
)

// importNotes asks for the Markdown and text files to import, and imports the
// ones chosen through importFiles, as a drop does. A canceled dialog changes
// nothing.
func (a *App) importNotes(ctx *ui.Context) {
	ctx.Window().OpenFiles(dialog.FileOptions{Title: "Import notes", Filters: importFilters, Multiple: true},
		func(r dialog.FileResult) {
			switch {
			case r.Err != nil:
				showNotice("Import failed: " + r.Err.Error())
			case !r.Canceled:
				a.importFiles(ui.Files(r.Paths), nil)
			}
		})
}

// exportNote asks where to save the selected note, and writes it there. The
// file type is the one the chosen name ends with: .txt is plain text, and
// anything else is Markdown.
func (a *App) exportNote(ctx *ui.Context) {
	n := a.s.Find(a.s.Selected.Peek())
	if n == nil {
		return
	}
	ctx.Window().SaveFile(dialog.FileOptions{
		Title:     "Export note",
		Name:      exportName(titleOf(n)) + ".md",
		Extension: "md",
		Filters:   exportFilters,
	}, func(r dialog.FileResult) {
		switch {
		case r.Err != nil:
			showNotice("Export failed: " + r.Err.Error())
		case !r.Canceled:
			showNotice(writeExport(n, r.Paths[0]))
		}
	})
}

// writeExport writes note n to path, and returns the status line's message. The
// message names the file, not its folder.
func writeExport(n *store.Note, path string) string {
	text := exportText(titleOf(n), n.Body.Peek(), path)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "Export failed: " + err.Error()
	}
	return "Exported " + filepath.Base(path)
}

// exportText is the content of an exported note: Markdown with the title as a
// level-one heading, or plain text with the title on its own first line.
func exportText(title, body, path string) string {
	if strings.EqualFold(filepath.Ext(path), ".txt") {
		return title + "\n\n" + body
	}
	return "# " + title + "\n\n" + body
}

// exportName turns a note's title into a file name. Characters that Windows
// does not allow in names become spaces, and runs of spaces are collapsed.
func exportName(title string) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, r) || r < 0x20 {
			return ' '
		}
		return r
	}, title)
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "Untitled"
	}
	return name
}
