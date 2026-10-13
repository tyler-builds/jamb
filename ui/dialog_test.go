package ui

import (
	"image"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/dialog"
)

// The dialog methods pass their options to the window, and hand the window's
// result to the callback unchanged. A canceled file dialog is a result, not an
// error.
func TestDialogsReachTheWindow(t *testing.T) {
	win := &fakeWindow{}
	var got []dialog.FileResult
	var answer dialog.Button
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme(), nil)
	rt.SetWindow(win)
	rt.SetViewport(400, 300, 1)
	rt.Frame(time.Now())

	ctx := &Context{rt: rt}
	ctx.Window().OpenFiles(dialog.FileOptions{Title: "Import", Multiple: true, Filters: []dialog.Filter{{Name: "Notes", Patterns: []string{"*.md", "*.txt"}}}},
		func(r dialog.FileResult) { got = append(got, r) })
	if len(win.fileOpts) != 1 || win.fileOpts[0].Title != "Import" || !win.fileOpts[0].Multiple ||
		len(win.fileOpts[0].Filters) != 1 || win.fileOpts[0].Filters[0].Patterns[1] != "*.txt" {
		t.Fatalf("window got options %+v; want the import options unchanged", win.fileOpts)
	}
	win.fileDone(dialog.FileResult{Paths: []string{"one.md", "two.txt"}})
	win.fileDone(dialog.FileResult{Canceled: true})
	if len(got) != 2 || len(got[0].Paths) != 2 || got[0].Canceled || got[0].Err != nil || !got[1].Canceled || got[1].Err != nil {
		t.Errorf("callbacks got %+v; want two paths, then a canceled result with no error", got)
	}

	ctx.Window().MessageBox(dialog.MessageOptions{Buttons: dialog.ButtonsYesNo, Default: dialog.No}, func(b dialog.Button) { answer = b })
	win.msgDone(dialog.Yes)
	if answer != dialog.Yes {
		t.Errorf("message box answer %v, want Yes", answer)
	}
	if win.calls[len(win.calls)-1] != "message" || win.msgOpts[len(win.msgOpts)-1].Default != dialog.No {
		t.Errorf("message box calls %v, options %+v; want one message box with No as default", win.calls, win.msgOpts)
	}
}

// Without a window, as in headless tests, the dialogs do nothing: no callback
// runs and nothing panics.
func TestHeadlessDialogsDoNothing(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme(), nil)
	ctx := &Context{rt: rt}
	called := false
	ctx.Window().OpenFiles(dialog.FileOptions{}, func(dialog.FileResult) { called = true })
	ctx.Window().SaveFile(dialog.FileOptions{}, func(dialog.FileResult) { called = true })
	ctx.Window().PickFolder(dialog.FileOptions{}, func(dialog.FileResult) { called = true })
	ctx.Window().MessageBox(dialog.MessageOptions{}, func(dialog.Button) { called = true })
	if called {
		t.Error("a headless runtime called a dialog callback")
	}
}

// The clipboard is the platform's when one is wired in, and the runtime's own
// otherwise. Context reads and writes plain text through the same path as text
// fields.
func TestContextClipboardUsesPlatform(t *testing.T) {
	text, has := "from the platform", true
	var written []string
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme(), nil)
	rt.hostSetServices(platformServices{
		Clipboard:    func() (string, bool) { return text, has },
		SetClipboard: func(s string) { written = append(written, s) },
	})
	ctx := &Context{rt: rt}
	if got, ok := ctx.Clipboard(); got != "from the platform" || !ok {
		t.Errorf("Clipboard() = %q, %v; want the platform's text", got, ok)
	}
	has = false
	if _, ok := ctx.Clipboard(); ok {
		t.Error("Clipboard() reported text when the platform has none")
	}
	ctx.SetClipboard("copied")
	if len(written) != 1 || written[0] != "copied" {
		t.Errorf("platform received %q; want [copied]", written)
	}
}

func TestHeadlessClipboardRoundTrips(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme(), nil)
	ctx := &Context{rt: rt}
	if _, ok := ctx.Clipboard(); ok {
		t.Fatal("a new clipboard holds text")
	}
	ctx.SetClipboard("line one\nline two")
	if got, ok := ctx.Clipboard(); got != "line one\nline two" || !ok {
		t.Errorf("Clipboard() = %q, %v after SetClipboard", got, ok)
	}
}

// A headless clipboard holds text or an image, as the system's does: copying
// one replaces the other.
func TestHeadlessClipboardImage(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme(), nil)
	ctx := &Context{rt: rt}
	if _, ok := ctx.ClipboardImage(); ok {
		t.Fatal("a new clipboard holds an image")
	}
	ctx.SetClipboard("text")
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	ctx.SetClipboardImage(img)
	if got, ok := ctx.ClipboardImage(); !ok || got != image.Image(img) {
		t.Fatalf("ClipboardImage() = %v, %v after SetClipboardImage", got, ok)
	}
	if _, ok := ctx.Clipboard(); ok {
		t.Error("copying an image left the text on the clipboard")
	}
	ctx.SetClipboard("again")
	if _, ok := ctx.ClipboardImage(); ok {
		t.Error("copying text left the image on the clipboard")
	}
}

// The image clipboard is the platform's when one is wired in, and stays wired
// in when the other services are set after it.
func TestContextClipboardImageUsesPlatform(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element { return Div() }, DarkTheme(), nil)
	var written []image.Image
	platformImg := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	h := (*runtimeHost)(rt)
	h.SetImageClipboard(func() (image.Image, bool) { return platformImg, true },
		func(img image.Image) { written = append(written, img) })
	h.SetServices(func() (string, bool) { return "", false }, func(string) {}, nil)
	ctx := &Context{rt: rt}
	if got, ok := ctx.ClipboardImage(); !ok || got != image.Image(platformImg) {
		t.Errorf("ClipboardImage() = %v, %v; want the platform's image", got, ok)
	}
	ctx.SetClipboardImage(platformImg)
	if len(written) != 1 {
		t.Errorf("platform received %d images; want 1", len(written))
	}
}
