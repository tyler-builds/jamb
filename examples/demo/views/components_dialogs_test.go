package views

import (
	"errors"
	"testing"

	"github.com/tyler-builds/jamb/dialog"
)

// The Dialogs section names files by their file name only: the folders they
// are in never reach the page, which may be screenshotted.
func TestDialogResultsShowFileNamesOnly(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"two files", describeFiles(dialog.FileResult{Paths: []string{`C:\Users\someone\Notes\a.md`, `D:\other\b.txt`}}), "Selected 2 files: a.md, b.txt"},
		{"one file", describeFiles(dialog.FileResult{Paths: []string{`C:\Users\someone\Notes\a.md`}}), "Selected 1 file: a.md"},
		{"canceled", describeFiles(dialog.FileResult{Canceled: true}), "Canceled."},
		{"save", describeSave(dialog.FileResult{Paths: []string{`C:\Users\someone\Notes\Untitled.md`}}), "Saved as Untitled.md"},
		{"folder", describeFolder(dialog.FileResult{Paths: []string{`C:\Users\someone\Notes`}}), "Folder: Notes"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

// A dialog that could not open says so, with the reason.
func TestDialogErrorsAreReported(t *testing.T) {
	if got := describeFiles(dialog.FileResult{Err: errors.New("boom")}); got != "The dialog could not open: boom" {
		t.Errorf("error result: %q", got)
	}
}

func TestMessageAnswers(t *testing.T) {
	want := map[dialog.Button]string{dialog.OK: "OK", dialog.Cancel: "Canceled", dialog.Yes: "Yes", dialog.No: "No"}
	for b, w := range want {
		if got := describeButton(b); got != w {
			t.Errorf("describeButton(%v) = %q, want %q", b, got, w)
		}
	}
	if got := describeButton(dialog.NoButton); got != "the box could not open" {
		t.Errorf("describeButton(NoButton) = %q", got)
	}
}
