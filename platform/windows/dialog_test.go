//go:build windows

package windows

import (
	"testing"

	"github.com/tyler-builds/jamb/dialog"
)

// The message box flags combine the icon, the buttons and the default button,
// as MessageBoxW reads them. A default outside the set means the first button.
func TestMessageFlags(t *testing.T) {
	cases := []struct {
		name string
		o    dialog.MessageOptions
		want uint32
	}{
		{"OK", dialog.MessageOptions{}, mbOK},
		{"error with OK", dialog.MessageOptions{Icon: dialog.IconError}, mbIconError},
		{"warning, OK/Cancel, Cancel default", dialog.MessageOptions{Icon: dialog.IconWarning, Buttons: dialog.ButtonsOKCancel, Default: dialog.Cancel},
			mbIconWarning | mbOKCancel | mbDefButton2},
		{"question, Yes/No, default No", dialog.MessageOptions{Icon: dialog.IconQuestion, Buttons: dialog.ButtonsYesNo, Default: dialog.No},
			mbIconQuestion | mbYesNo | mbDefButton2},
		{"Yes/No with no default", dialog.MessageOptions{Buttons: dialog.ButtonsYesNo}, mbYesNo},
		{"Yes/No/Cancel, Cancel default", dialog.MessageOptions{Icon: dialog.IconInfo, Buttons: dialog.ButtonsYesNoCancel, Default: dialog.Cancel},
			mbIconInformation | mbYesNoCancel | mbDefButton3},
		{"default outside the set", dialog.MessageOptions{Buttons: dialog.ButtonsOKCancel, Default: dialog.Yes}, mbOKCancel},
	}
	for _, c := range cases {
		if got := messageFlags(c.o); got != c.want {
			t.Errorf("%s: messageFlags = %#x, want %#x", c.name, got, c.want)
		}
	}
}

// File dialogs ask for file system paths and leave the working directory
// alone. Open adds the existence checks, Save the overwrite prompt, and Folder
// picks folders.
func TestFileFlags(t *testing.T) {
	base := uintptr(fosNoChangeDir | fosForceFileSystem | fosPathMustExist)
	filters := []dialog.Filter{{Name: "Notes", Patterns: []string{"*.md"}}}
	cases := []struct {
		name string
		kind dialog.FileKind
		o    dialog.FileOptions
		want uintptr
	}{
		{"open one", dialog.Open, dialog.FileOptions{}, base | fosFileMustExist},
		{"open many", dialog.Open, dialog.FileOptions{Multiple: true}, base | fosFileMustExist | fosAllowMultiSelect},
		{"save", dialog.Save, dialog.FileOptions{}, base | fosOverwritePrompt},
		{"save with filters is strict", dialog.Save, dialog.FileOptions{Filters: filters}, base | fosOverwritePrompt | fosStrictFileTypes},
		{"folder", dialog.Folder, dialog.FileOptions{}, base | fosPickFolders},
		{"folder ignores multiple", dialog.Folder, dialog.FileOptions{Multiple: true}, base | fosPickFolders},
	}
	for _, c := range cases {
		if got := fileFlags(c.kind, c.o); got != c.want {
			t.Errorf("%s: fileFlags = %#x, want %#x", c.name, got, c.want)
		}
	}
}
