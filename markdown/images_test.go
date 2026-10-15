package markdown

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// dump returns the element tree of u as text.
func dump(u *uitest.UI) string {
	var b bytes.Buffer
	u.Runtime().Dump(&b, 0)
	return b.String()
}

// imageShown reports whether the image named name is drawn at w by h pixels (the
// display's scale is 1), not as a placeholder.
func imageShown(u *uitest.UI, name string, w, h int) bool {
	n := u.Tree().Find(access.RoleImage, name)
	return n != nil && n.Bounds.W == float32(w) && n.Bounds.H == float32(h)
}

// The view asks its loader for each image by its source, once, and shows what it
// returns at the image's size.
func TestImagesComeFromTheLoader(t *testing.T) {
	var asked []string
	u := show(t, func() *ViewElement {
		return View("![Logo](img/logo.png)\n\nText.").Images(func(src string) (*ui.ImageSource, error) {
			asked = append(asked, src)
			return ui.ImageFromImage(image.NewRGBA(image.Rect(0, 0, 40, 30))), nil
		})
	})
	u.WaitFor("the image to load", func() bool { return imageShown(u, "Logo", 40, 30) })
	u.Frame()
	if strings.Join(asked, " ") != "img/logo.png" {
		t.Errorf("the loader was asked for %q, want img/logo.png once", asked)
	}
}

// An image that the loader cannot load is shown as a placeholder with its
// alternative text, and assistive technology reads it as an image of that name.
func TestImageThatFailsShowsItsAltText(t *testing.T) {
	u := show(t, func() *ViewElement {
		return View("![Build status](missing.png)").Images(func(string) (*ui.ImageSource, error) {
			return nil, errors.New("not found")
		})
	})
	u.WaitFor("the placeholder", func() bool { return strings.Contains(dump(u), "Image not shown: Build status") })
	u.ByRole(access.RoleImage, "Build status")
}

// The default loader reads local files relative to BaseDir, and refuses URLs, so an
// http image shows its alternative text and nothing is downloaded.
func TestDefaultLoaderReadsLocalFilesOnly(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 12, 8))); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "img", "dot one.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	u := show(t, func() *ViewElement {
		return View("![Dot](img/dot%20one.png?raw=true) ![Remote](https://example.com/x.png)").BaseDir(dir)
	})
	u.WaitFor("the images", func() bool {
		return imageShown(u, "Dot", 12, 8) && strings.Contains(dump(u), "Image not shown: Remote")
	})
}

// localPath refuses every source that is not a local path, network paths included,
// since Windows opens those by connecting to their server.
func TestLocalPathRefusesURLsAndNetworkPaths(t *testing.T) {
	for _, src := range []string{
		"https://example.com/a.png", "http://example.com/a.png", "HTTP://EXAMPLE.COM/a.png", "data:image/png;base64,AAAA",
		"file:///C:/a.png", "ftp://example.com/a.png", `\\server\share\a.png`, "//server/share/a.png",
		`\\?\C:\a.png`, `\\.\pipe\x`, "%5C%5Cserver%5Cshare%5Ca.png", "", "?x=1",
	} {
		if p, err := localPath(`C:\docs`, src); err == nil {
			t.Errorf("%q was accepted as %q", src, p)
		}
	}
	for src, want := range map[string]string{
		"a.png":               filepath.Join(`C:\docs`, "a.png"),
		"img/a%20b.png":       filepath.Join(`C:\docs`, "img", "a b.png"),
		"../up.png#fragment":  filepath.Join(`C:\`, "up.png"),
		`C:\pictures\abs.png`: `C:\pictures\abs.png`,
	} {
		if got, err := localPath(`C:\docs`, src); err != nil || got != want {
			t.Errorf("localPath(%q) = %q, %v; want %q", src, got, err, want)
		}
	}
}

// An image inside a link is a link named by the image's alternative text, which Tab
// reaches and which follows the link.
func TestImageInALink(t *testing.T) {
	var got []string
	u := show(t, func() *ViewElement {
		return View("[![Build](b.png)](https://ci.example.com)").
			Images(func(string) (*ui.ImageSource, error) { return nil, errors.New("offline") }).
			OnLink(func(url string) { got = append(got, url) })
	})
	u.Press("tab")
	if f := u.Focused().Node(); f.Role != access.RoleLink || f.Name != "Build" {
		t.Fatalf("Tab: focus on the %s %q, want the link Build", f.Role, f.Name)
	}
	u.Press("enter")
	if strings.Join(got, " ") != "https://ci.example.com" {
		t.Errorf("OnLink got %q", got)
	}
}
