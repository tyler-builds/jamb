//go:build js

package web

import (
	"errors"
	"image"
	"image/color"
	"math"
	"strings"
	"syscall/js"

	"github.com/tyler-builds/jamb/dialog"
)

// A page reads files only as the browser hands them to it: the ones the user
// chooses in the browser's file picker, and the ones of a paste. They have no
// paths, and their contents arrive later, as promises. The window reads them
// whole before it tells the app, which then has their bytes at once.

// then calls ok with the value of the promise p once it is fulfilled, or fail,
// when there is one, with its reason once it is rejected. They run on a
// goroutine of the browser's callback, not on the UI goroutine.
func then(p js.Value, ok, fail func(js.Value)) {
	var onOK, onFail js.Func
	arg := func(args []js.Value) js.Value {
		if len(args) == 0 {
			return js.Undefined()
		}
		return args[0]
	}
	onOK = js.FuncOf(func(_ js.Value, args []js.Value) any {
		onOK.Release()
		onFail.Release()
		ok(arg(args))
		return nil
	})
	onFail = js.FuncOf(func(_ js.Value, args []js.Value) any {
		onOK.Release()
		onFail.Release()
		if fail != nil {
			fail(arg(args))
		}
		return nil
	})
	p.Call("then", onOK, onFail)
}

// ShowFileDialog implements platform.Window. An Open dialog is the browser's
// file picker, which the browser shows only while the app handles a click or
// a key press. The files it returns are read whole, and are the result's Files:
// a page's files have no paths. A page has no file system to save to or to
// pick a folder of, so the other dialogs answer as canceled.
func (w *window) ShowFileDialog(kind dialog.FileKind, o dialog.FileOptions, done func(dialog.FileResult)) {
	if kind != dialog.Open || w.destroyed {
		Post(func() { done(dialog.FileResult{Canceled: true}) })
		return
	}
	doc := js.Global().Get("document")
	in := doc.Call("createElement", "input")
	in.Set("type", "file")
	in.Set("multiple", o.Multiple)
	if a := acceptOf(o.Filters); a != "" {
		in.Set("accept", a)
	}
	in.Get("style").Set("display", "none")
	// Some browsers report the choice only of an input that is in the page.
	doc.Get("body").Call("appendChild", in)

	var changed, canceled js.Func
	finish := func(r dialog.FileResult) {
		changed.Release()
		canceled.Release()
		in.Call("remove")
		Post(func() { done(r) })
	}
	changed = js.FuncOf(func(js.Value, []js.Value) any {
		list := in.Get("files")
		n := list.Get("length").Int()
		if n == 0 {
			finish(dialog.FileResult{Canceled: true})
			return nil
		}
		files := make([]dialog.File, 0, n)
		var read func(i int)
		read = func(i int) {
			if i == n {
				finish(dialog.FileResult{Files: files})
				return
			}
			f := list.Index(i)
			name := f.Get("name").String()
			readBlob(f, func(data []byte, err error) {
				if err != nil {
					finish(dialog.FileResult{Err: errors.New("platform/web: reading " + name + ": " + err.Error())})
					return
				}
				files = append(files, dialog.File{Name: name, Data: data})
				read(i + 1)
			})
		}
		read(0)
		return nil
	})
	canceled = js.FuncOf(func(js.Value, []js.Value) any {
		finish(dialog.FileResult{Canceled: true})
		return nil
	})
	in.Call("addEventListener", "change", changed)
	in.Call("addEventListener", "cancel", canceled)
	in.Call("click")
}

// acceptOf returns the file types of filters as a file input's accept
// attribute: the extensions of their patterns, such as ".png,.jpg". It is
// empty, for every file, when a pattern is not an extension's, as "*.*" is not.
func acceptOf(filters []dialog.Filter) string {
	var exts []string
	for _, f := range filters {
		for _, p := range f.Patterns {
			ext, ok := strings.CutPrefix(strings.TrimSpace(p), "*.")
			if !ok || ext == "" || strings.ContainsAny(ext, "*?,/ ") {
				return ""
			}
			exts = append(exts, "."+ext)
		}
	}
	return strings.Join(exts, ",")
}

// readBlob reads the contents of a Blob of the page, such as a file.
func readBlob(blob js.Value, done func([]byte, error)) {
	if blob.Get("arrayBuffer").Type() != js.TypeFunction {
		done(nil, errors.New("the browser does not read files"))
		return
	}
	then(blob.Call("arrayBuffer"), func(buf js.Value) {
		view := js.Global().Get("Uint8Array").New(buf)
		data := make([]byte, view.Get("length").Int())
		js.CopyBytesToGo(data, view)
		done(data, nil)
	}, func(reason js.Value) {
		done(nil, errors.New(errorText(reason)))
	})
}

// errorText is the message of a promise's rejection.
func errorText(reason js.Value) string {
	if reason.Type() == js.TypeObject {
		if m := reason.Get("message"); m.Type() == js.TypeString {
			return m.String()
		}
	}
	if reason.Type() == js.TypeString {
		return reason.String()
	}
	return "the browser could not read it"
}

// The most pixels of an image that the page decodes for the app, and the
// longest side: a larger one is scaled down to them. A decoded image is four
// bytes a pixel in the app's memory, which does not shrink again.
const (
	maxImagePixels = 32 << 20
	maxImageSide   = 8192
)

// imageSize returns the size an image of w by h pixels is decoded at.
func imageSize(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	scale := min(1, maxImageSide/float64(max(w, h)), math.Sqrt(maxImagePixels/(float64(w)*float64(h))))
	if scale == 1 {
		return w, h
	}
	return max(int(float64(w)*scale), 1), max(int(float64(h)*scale), 1)
}

// decodeImage decodes an image file of the page, a Blob, with the browser's
// decoders, which know more formats than the app links, and calls done with
// the image, or with nil when the browser cannot decode it.
func decodeImage(blob js.Value, done func(image.Image)) {
	g := js.Global()
	if g.Get("createImageBitmap").Type() != js.TypeFunction {
		done(nil)
		return
	}
	then(g.Call("createImageBitmap", blob), func(bmp js.Value) {
		w, h := imageSize(bmp.Get("width").Int(), bmp.Get("height").Int())
		ctx, ok := pixelCanvas(w, h)
		if !ok {
			done(nil)
			return
		}
		ctx.Call("drawImage", bmp, 0, 0, w, h)
		if bmp.Get("close").Type() == js.TypeFunction {
			bmp.Call("close")
		}
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		js.CopyBytesToGo(img.Pix, ctx.Call("getImageData", 0, 0, w, h).Get("data"))
		done(img)
	}, func(js.Value) { done(nil) })
}

// pixelCanvas returns the 2D context of a canvas of w by h pixels that is not
// in the page, to read an image's pixels from or to make an image file of.
func pixelCanvas(w, h int) (js.Value, bool) {
	if w <= 0 || h <= 0 {
		return js.Undefined(), false
	}
	c := js.Global().Get("document").Call("createElement", "canvas")
	c.Set("width", w)
	c.Set("height", h)
	ctx := c.Call("getContext", "2d")
	return ctx, ctx.Truthy()
}

// pastedImage returns the first image among the files of a paste's data.
func pastedImage(data js.Value) (js.Value, bool) {
	files := data.Get("files")
	if !files.Truthy() {
		return js.Undefined(), false
	}
	for i := range files.Get("length").Int() {
		if f := files.Index(i); strings.HasPrefix(f.Get("type").String(), "image/") {
			return f, true
		}
	}
	return js.Undefined(), false
}

// requestPaste asks the browser for the system's clipboard, and reports
// whether it did. The browser may ask the user first, with a prompt or a Paste
// button of its own, and answers later: the clipboard's text, or else its
// image, then arrives as a paste does (see deliverPaste). When the browser
// refuses, what arrives is the page's own clipboard: what was last copied in
// the app. A page that is not served securely has no clipboard to ask for.
func (w *window) requestPaste() bool {
	clip := js.Global().Get("navigator").Get("clipboard")
	if !clip.Truthy() {
		return false
	}
	items := clip.Get("read").Type() == js.TypeFunction
	if !items && clip.Get("readText").Type() != js.TypeFunction {
		return false
	}
	w.clipPending = true
	finish := func(text string, img image.Image, ok bool) {
		Post(func() {
			w.clipPending = false
			if w.destroyed {
				return
			}
			if !ok {
				text, img = w.clipText, w.clipImage
			}
			w.deliverPaste(text, img)
		})
	}
	fail := func(js.Value) { finish("", nil, false) }
	if !items {
		then(clip.Call("readText"), func(s js.Value) { finish(s.String(), nil, true) }, fail)
		return true
	}
	then(clip.Call("read"), func(list js.Value) {
		// The text of the clipboard, or else its image, as a paste event gives them.
		var text, img js.Value
		var imgType string
		for i := range list.Get("length").Int() {
			item := list.Index(i)
			types := item.Get("types")
			for j := range types.Get("length").Int() {
				switch t := types.Index(j).String(); {
				case t == "text/plain" && !text.Truthy():
					text = item
				case strings.HasPrefix(t, "image/") && !img.Truthy():
					img, imgType = item, t
				}
			}
		}
		useImage := func() {
			if !img.Truthy() {
				finish("", nil, true)
				return
			}
			then(img.Call("getType", imgType), func(blob js.Value) {
				decodeImage(blob, func(m image.Image) { finish("", m, m != nil) })
			}, fail)
		}
		if !text.Truthy() {
			useImage()
			return
		}
		then(text.Call("getType", "text/plain"), func(blob js.Value) {
			then(blob.Call("text"), func(s js.Value) {
				if s.String() == "" {
					useImage()
					return
				}
				finish(s.String(), nil, true)
			}, fail)
		}, fail)
	}, fail)
	return true
}

// ClipboardImage implements platform.Window: the image of the paste being
// delivered (see paste), or the image last copied in the app.
func (w *window) ClipboardImage() (image.Image, bool) {
	if w.clipPending {
		return nil, false
	}
	return w.clipImage, w.clipImage != nil
}

// HasClipboardImage implements platform.Window.
func (w *window) HasClipboardImage() bool { return !w.clipPending && w.clipImage != nil }

// SetClipboardImage implements platform.Window. The image goes to the
// system's clipboard too, as a PNG file that the browser makes, where the
// browser lets a page write one, during a key press or a click (see runNow).
func (w *window) SetClipboardImage(img image.Image) {
	w.clipImage, w.clipText = img, ""
	if img == nil {
		return
	}
	g := js.Global()
	clip, item := g.Get("navigator").Get("clipboard"), g.Get("ClipboardItem")
	if !clip.Truthy() || clip.Get("write").Type() != js.TypeFunction || item.Type() != js.TypeFunction {
		return
	}
	b := img.Bounds()
	ctx, ok := pixelCanvas(b.Dx(), b.Dy())
	if !ok {
		return
	}
	data := ctx.Call("createImageData", b.Dx(), b.Dy())
	js.CopyBytesToJS(data.Get("data"), straightPixels(img))
	ctx.Call("putImageData", data, 0, 0)
	// The item takes a promise of the file, so that it is made while the
	// event lasts, as browsers ask.
	file := js.FuncOf(func(_ js.Value, args []js.Value) any {
		ctx.Get("canvas").Call("toBlob", args[0], "image/png")
		return nil
	})
	defer file.Release() // the promise calls it at once
	ignoreRejection(clip.Call("write", []any{item.New(map[string]any{"image/png": g.Get("Promise").New(file)})}))
}

// straightPixels returns the pixels of img as a canvas holds them: RGBA, with
// colors that are not multiplied by their alpha.
func straightPixels(img image.Image) []byte {
	b := img.Bounds()
	if n, ok := img.(*image.NRGBA); ok && n.Stride == b.Dx()*4 {
		return n.Pix[:b.Dx()*b.Dy()*4]
	}
	pix := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			pix = append(pix, c.R, c.G, c.B, c.A)
		}
	}
	return pix
}
