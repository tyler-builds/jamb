package markdown

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// OnLinkMenu gives each link a context menu built for its destination, which
// Shift+F10 opens while the link has focus; a reference to a footnote has none.
func TestOnLinkMenu(t *testing.T) {
	var got []string
	u := show(t, func() *ViewElement {
		return View("See [the **bold** docs](docs/README.md) and <https://example.com>.[^1]\n\n[^1]: A note.").
			OnLinkMenu(func(url string) []ui.MenuItem {
				return []ui.MenuItem{{Label: "Copy link address", OnClick: func() { got = append(got, url) }}}
			})
	})
	for range 2 {
		u.Press("tab")
		u.Press("shift+f10")
		u.ByRole(access.RoleMenuItem, "Copy link address").Click()
	}
	if want := "docs/README.md https://example.com"; strings.Join(got, " ") != want {
		t.Fatalf("the menus got %q, want %q", strings.Join(got, " "), want)
	}
	u.Press("tab") // the reference to the footnote
	if f := u.Focused().Node(); f.Role != access.RoleLink || f.Name != "[1]" {
		t.Fatalf("Tab: focus on the %s %q, want the footnote reference", f.Role, f.Name)
	}
	u.Press("shift+f10")
	if u.HasRole(access.RoleMenuItem, "Copy link address") {
		t.Error("the reference to a footnote has a link menu")
	}
}
