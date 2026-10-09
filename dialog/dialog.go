// Package dialog describes the native dialogs an app opens from a window:
// file open, file save, folder pick and message boxes. Application code calls
// them through ui.WindowControls (ctx.Window()). The types are plain values,
// so the ui and platform packages share them without depending on each other.
//
// A dialog belongs to the window that opened it. The call returns at once,
// and the result goes to a callback on the UI thread when the dialog closes.
// That is the shape of a macOS sheet too, so the same calls can map onto one.
//
// On Windows, file dialogs use IFileOpenDialog and IFileSaveDialog, and
// message boxes use MessageBoxW. TaskDialog would show richer boxes, but it
// needs a common-controls version 6 manifest, which packaging will add. Then
// message boxes can move to TaskDialog without changing this package.
package dialog

// FileKind selects the file dialog to show.
type FileKind uint8

const (
	// Open chooses one file, or several when FileOptions.Multiple is set.
	Open FileKind = iota
	// Save chooses a path to write, which may name a file that exists.
	Save
	// Folder chooses a folder.
	Folder
)

// FileOptions describes a file or folder dialog. Each kind uses only the
// fields that apply to it: Multiple is for Open, Name and Extension for Save,
// and Filters for Open and Save.
type FileOptions struct {
	Title     string   // the dialog's title; empty keeps the system's
	Folder    string   // the folder the dialog starts in; empty keeps the system's choice
	Name      string   // Save: the file name the dialog starts with
	Extension string   // Save: added to a name that has no extension, without the dot ("md")
	Filters   []Filter // Open and Save: the file types offered; the first is selected
	Multiple  bool     // Open: the user may choose several files
}

// Filter is a named group of file types, such as "Notes" with the patterns
// "*.md" and "*.txt".
type Filter struct {
	Name     string
	Patterns []string
}

// FileResult is the outcome of a file dialog. Canceled is true when the user
// closed the dialog without choosing a file. That is a result, not an error.
// Err is set only when the dialog could not be shown.
type FileResult struct {
	Paths    []string // the chosen paths, as the dialog returns them
	Canceled bool
	Err      error
}

// MessageOptions describes a message box.
type MessageOptions struct {
	Title   string
	Text    string
	Icon    Icon
	Buttons ButtonSet
	Default Button // the button Enter chooses; zero means the first of Buttons
}

// Icon is the icon of a message box.
type Icon uint8

const (
	IconNone Icon = iota
	IconInfo
	IconWarning
	IconError
	IconQuestion
)

// ButtonSet is the set of buttons of a message box.
type ButtonSet uint8

const (
	ButtonsOK ButtonSet = iota
	ButtonsOKCancel
	ButtonsYesNo
	ButtonsYesNoCancel
)

// Button is a button of a message box, and the result when the user chooses
// one. A box closed without a choice returns the answer the system gives for
// it, which is Cancel for a set that has one.
type Button uint8

const (
	// NoButton means no button was chosen, because the box could not be shown.
	NoButton Button = iota
	OK
	Cancel
	Yes
	No
)
