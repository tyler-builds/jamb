//go:build jambdev

package views

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/scene"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// Every scene opens, renders without warnings and shows what it is for.
func TestScenes(t *testing.T) {
	names := scene.Names()
	if len(names) == 0 {
		t.Fatal("no scenes are registered")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			root, err := scene.Make(name)
			if err != nil {
				t.Fatal(err)
			}
			u := uitest.New(t, root, uitest.Size(1100, 720))
			if w := u.Warnings(); len(w) > 0 {
				t.Errorf("warnings: %v", w)
			}
		})
	}
	root, _ := scene.Make("Notes/Many")
	u := uitest.New(t, root, uitest.Size(1100, 720))
	found := false
	u.Tree().Walk(func(n *access.Node) bool {
		found = found || strings.Contains(n.Name, "Synthetic note")
		return !found
	})
	if !found {
		t.Error("Notes/Many shows none of its synthetic notes")
	}
}
