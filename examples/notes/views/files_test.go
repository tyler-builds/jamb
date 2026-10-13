package views

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/ui"
)

// fakeWindow stands in for the app's window. It records each dialog the notes
// app opens, and keeps the callback, which a test answers.
type fakeWindow struct {
	kinds []string
	files []dialog.FileOptions
	done  func(dialog.FileResult)
}

func (f *fakeWindow) Minimize()                                             {}
func (f *fakeWindow) ToggleMaximize()                                       {}
func (f *fakeWindow) Maximized() bool                                       { return false }
func (f *fakeWindow) Close()                                                {}
func (f *fakeWindow) SetTitle(string)                                       {}
func (f *fakeWindow) Focused() bool                                         { return true }
func (f *fakeWindow) Position() (int, int)                                  { return 0, 0 }
func (f *fakeWindow) Size() (int, int)                                      { return 800, 600 }
func (f *fakeWindow) SetPosition(int, int)                                  {}
func (f *fakeWindow) SetSize(int, int)                                      {}
func (f *fakeWindow) SetFullscreen(bool)                                    {}
func (f *fakeWindow) Fullscreen() bool                                      { return false }
func (f *fakeWindow) Focus()                                                {}
func (f *fakeWindow) Closed() bool                                          { return false }
func (f *fakeWindow) MessageBox(dialog.MessageOptions, func(dialog.Button)) {}

func (f *fakeWindow) OpenFiles(o dialog.FileOptions, done func(dialog.FileResult)) {
	f.record("open", o, done)
}

func (f *fakeWindow) SaveFile(o dialog.FileOptions, done func(dialog.FileResult)) {
	f.record("save", o, done)
}

func (f *fakeWindow) PickFolder(o dialog.FileOptions, done func(dialog.FileResult)) {
	f.record("folder", o, done)
}

func (f *fakeWindow) record(kind string, o dialog.FileOptions, done func(dialog.FileResult)) {
	f.kinds = append(f.kinds, kind)
	f.files = append(f.files, o)
	f.done = done
}

// Import... opens the open dialog with the notes' file types, allowing several
// files. The files chosen are imported as a drop imports them: each becomes a
// note, the others are skipped, and the status line says so.
func TestImportMenuOpensDialogAndImports(t *testing.T) {
	r := newRig(t)
	fw := &fakeWindow{}
	r.rt.SetWindow(fw)
	dir := t.TempDir()
	md := filepath.Join(dir, "Alpha.md")
	txt := filepath.Join(dir, "beta.txt")
	pdf := filepath.Join(dir, "gamma.pdf")
	for path, body := range map[string]string{md: "# Alpha\n\nfirst", txt: "second", pdf: "%PDF"} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := len(r.s.Notes.Peek())

	r.clickKey("jamb.menubar.0")
	r.clickKey("jamb.menu.0.1")
	if len(fw.kinds) != 1 || fw.kinds[0] != "open" {
		t.Fatalf("Import... opened %v, want one open dialog", fw.kinds)
	}
	o := fw.files[0]
	if !o.Multiple || len(o.Filters) != 1 || o.Filters[0].Name != "Notes" ||
		len(o.Filters[0].Patterns) != 2 || o.Filters[0].Patterns[0] != "*.md" || o.Filters[0].Patterns[1] != "*.txt" {
		t.Errorf("open dialog options %+v; want several files of *.md and *.txt", o)
	}

	fw.done(dialog.FileResult{Paths: []string{md, txt, pdf}})
	r.frame()
	if got := len(r.s.Notes.Peek()); got != before+2 {
		t.Errorf("notes after the import: %d, want %d", got, before+2)
	}
	if want := "Imported 2 notes, skipped 1 file"; notice.Peek() != want {
		t.Errorf("status = %q; want %q", notice.Peek(), want)
	}
}

// A canceled open dialog changes nothing, and an error shows in the status line.
func TestImportCanceledAndFailed(t *testing.T) {
	r := newRig(t)
	fw := &fakeWindow{}
	r.rt.SetWindow(fw)
	before := len(r.s.Notes.Peek())
	notice.Set("")

	r.clickKey("jamb.menubar.0")
	r.clickKey("jamb.menu.0.1")
	fw.done(dialog.FileResult{Canceled: true})
	if len(r.s.Notes.Peek()) != before || notice.Peek() != "" {
		t.Errorf("a canceled import changed notes (%d, was %d) or status %q", len(r.s.Notes.Peek()), before, notice.Peek())
	}

	r.clickKey("jamb.menubar.0")
	r.clickKey("jamb.menu.0.1")
	fw.done(dialog.FileResult{Err: os.ErrPermission})
	if notice.Peek() != "Import failed: "+os.ErrPermission.Error() {
		t.Errorf("status after a failed dialog = %q", notice.Peek())
	}
}

// Export writes the selected note: Markdown for a .md name, plain text for a
// .txt name. The name offered is the note's title.
func TestExportWritesChosenFormat(t *testing.T) {
	r := newRig(t)
	fw := &fakeWindow{}
	r.rt.SetWindow(fw)
	n := r.s.Create()
	n.Title.Set("Plan: Q3")
	n.Body.Set("Line one\nLine two")
	r.s.Selected.Set(n.ID)
	r.frame()

	r.clickKey("jamb.menubar.0")
	r.clickKey("jamb.menu.0.2")
	if len(fw.kinds) != 1 || fw.kinds[0] != "save" {
		t.Fatalf("Export note... opened %v, want one save dialog", fw.kinds)
	}
	if o := fw.files[0]; o.Name != "Plan Q3.md" || o.Extension != "md" || len(o.Filters) != 2 {
		t.Errorf("save dialog options %+v; want the title as the name, with .md and two filters", o)
	}

	dir := t.TempDir()
	md := filepath.Join(dir, "out.md")
	fw.done(dialog.FileResult{Paths: []string{md}})
	if got, _ := os.ReadFile(md); string(got) != "# Plan: Q3\n\nLine one\nLine two" {
		t.Errorf("exported Markdown = %q", got)
	}

	r.clickKey("jamb.menubar.0")
	r.clickKey("jamb.menu.0.2")
	txt := filepath.Join(dir, "out.txt")
	fw.done(dialog.FileResult{Paths: []string{txt}})
	if got, _ := os.ReadFile(txt); string(got) != "Plan: Q3\n\nLine one\nLine two" {
		t.Errorf("exported text = %q", got)
	}
	if notice.Peek() != "Exported out.txt" {
		t.Errorf("status after export = %q; want the file name only", notice.Peek())
	}
}

// A canceled save writes no file.
func TestExportCanceledWritesNothing(t *testing.T) {
	r := newRig(t)
	fw := &fakeWindow{}
	r.rt.SetWindow(fw)
	r.s.Selected.Set(r.s.Notes.Peek()[0].ID)
	notice.Set("") // the status line is shared by the tests of this package
	r.frame()
	dir := t.TempDir()
	r.clickKey("jamb.menubar.0")
	r.clickKey("jamb.menu.0.2")
	fw.done(dialog.FileResult{Canceled: true})
	if entries, _ := os.ReadDir(dir); len(entries) != 0 || notice.Peek() != "" {
		t.Errorf("a canceled export wrote %d files, status %q", len(entries), notice.Peek())
	}
}

// Titles become file names with the characters Windows forbids replaced.
func TestExportNameMakesAFileName(t *testing.T) {
	cases := map[string]string{
		"Plan: Q3":         "Plan Q3",
		`a/b\c*?"<>|  d`:   "a b c d",
		"   ":              "Untitled",
		"Untitled":         "Untitled",
		"Notes.v2 (draft)": "Notes.v2 (draft)",
	}
	for title, want := range cases {
		if got := exportName(title); got != want {
			t.Errorf("exportName(%q) = %q, want %q", title, got, want)
		}
	}
}

var _ ui.Window = (*fakeWindow)(nil)
