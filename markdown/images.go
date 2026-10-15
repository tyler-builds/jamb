package markdown

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// maxImageBytes is the largest file that the default loader reads.
const maxImageBytes = 32 << 20

// imageCache keeps the images of a view between its renders, by loader and source,
// so each image is loaded once. It is used on the UI thread only: a load runs on a
// goroutine, and its result is stored by a function posted to the UI thread.
type imageCache struct {
	m map[string]*imageEntry
}

// imageEntry is one image: loading while done is false, then loaded or failed.
type imageEntry struct {
	done bool
	src  *ui.ImageSource
	err  error
}

// image returns the element of an image: the image once it has loaded, else a
// placeholder with its alternative text, inside a link when link is set.
func (b *builder) image(n *ast.Image, link func()) ui.Element {
	dest := string(n.Destination)
	alt := plainText(n, b.doc.src)
	name := alt
	if name == "" {
		name = b.s.MarkdownImage
	}
	var el ui.Element
	switch e := b.loadImage(dest); {
	case e.done && e.err == nil && e.src != nil:
		w, h := e.src.Size()
		el = ui.Image(e.src).Alt(name).Size(ui.Px(float32(w)), ui.Px(float32(h))).Fit(ui.FitContain).
			Style(ui.Style{MaxWidth: ui.Full, AlignSelf: ui.Start})
	default:
		msg := alt
		if e.done {
			msg = b.s.MarkdownImageNotShown(alt)
		}
		box := ui.Div().Role(access.RoleImage).Label(name).Style(b.st.ImagePlaceholder)
		if msg != "" {
			// The placeholder is named by the alternative text, so its text is not read again.
			box.Child(ui.Div().AccessHidden().Child(ui.Text(msg)))
		}
		el = box
	}
	if link == nil {
		return el
	}
	return ui.Div().Role(access.RoleLink).Label(name).Focusable().Cursor(ui.CursorPointer).AlignSelf(ui.Start).
		FocusVisible(ui.Style{Shadows: []ui.Shadow{{Spread: 2, Color: b.t.FocusRing}}}).
		OnClick(link).Child(el)
}

// loadImage returns the entry of the image with the source dest, and starts loading it
// on a goroutine if it is not known yet. The view renders again when it has loaded.
func (b *builder) loadImage(dest string) *imageEntry {
	load := b.v.images
	key := "\x00" + dest
	if load == nil {
		load = fileLoader(b.v.baseDir)
		key = b.v.baseDir + "\x00" + dest
	}
	c := b.images
	if c.m == nil {
		c.m = map[string]*imageEntry{}
	}
	if e := c.m[key]; e != nil {
		return e
	}
	e := &imageEntry{}
	c.m[key] = e
	go func() {
		src, err := load(dest)
		if err == nil && src == nil {
			err = errors.New("markdown: the image loader returned no image")
		}
		b.update(func() { e.done, e.src, e.err = true, src, err })
	}()
	return e
}

// fileLoader returns the default image loader: it reads the local file that src names,
// relative to base (or to the working directory when base is empty), and refuses
// everything that is not a local path (see localPath).
func fileLoader(base string) func(src string) (*ui.ImageSource, error) {
	return func(src string) (*ui.ImageSource, error) {
		path, err := localPath(base, src)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		// A device, such as CON, would block the read, and a directory has nothing to show.
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("markdown: %s is not a regular file", path)
		}
		if info.Size() > maxImageBytes {
			return nil, fmt.Errorf("markdown: %s is larger than %d bytes", path, maxImageBytes)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return ui.ImageFromBytes(data)
	}
}

// localPath returns the file that an image's source names, relative to base. A
// source with a URL scheme (http, https, file, data...) is refused, as are network
// paths (\\server\share and //server/share) and Windows device paths (\\?\ and \\.\),
// so that loading an image never reaches the network: Windows opens a network path
// by connecting to its server. The source's query and fragment are dropped, and its
// percent escapes decoded, as a browser does for a relative URL.
func localPath(base, src string) (string, error) {
	s := src
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	if d, err := url.PathUnescape(s); err == nil {
		s = d
	}
	switch {
	case strings.TrimSpace(s) == "":
		return "", errors.New("markdown: the image has no source")
	case hasScheme(s):
		return "", fmt.Errorf("markdown: %q is a URL; the default image loader reads local files only (see Images)", src)
	case strings.HasPrefix(s, `\\`) || strings.HasPrefix(s, "//") || strings.HasPrefix(s, `\/`) || strings.HasPrefix(s, `/\`):
		return "", fmt.Errorf("markdown: %q is a network path; the default image loader reads local files only", src)
	}
	p := filepath.FromSlash(s)
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	return p, nil
}

// hasScheme reports whether s starts with a URL scheme: two or more letters, digits,
// "+", "-" or ".", starting with a letter, then a colon. One letter and a colon is a
// Windows drive.
func hasScheme(s string) bool {
	i := strings.IndexByte(s, ':')
	if i < 2 {
		return false
	}
	for j, r := range s[:i] {
		letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !letter && (j == 0 || !(r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.')) {
			return false
		}
	}
	return true
}
