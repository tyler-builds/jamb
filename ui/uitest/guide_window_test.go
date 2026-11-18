package uitest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/dialog"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// The example of "The window: file dialogs and message boxes" in
// docs/testing.md, kept here so that the guide stays true.

func exportRoot(ctx *ui.Context) ui.Element {
	return ui.Button("Export").OnClick(func() {
		ctx.Window().SaveFile(dialog.FileOptions{Name: "query.sql"}, func(r dialog.FileResult) {
			if r.Err == nil && !r.Canceled {
				os.WriteFile(r.Paths[0], []byte("select 1;"), 0o644)
			}
		})
	})
}

type fakeWindow struct {
	ui.NopWindow
	saveName string                  // the name the last SaveFile offered
	save     func(dialog.FileResult) // its callback, which the test answers
}

func (f *fakeWindow) SaveFile(o dialog.FileOptions, done func(dialog.FileResult)) {
	f.saveName, f.save = o.Name, done
}

func TestExportSaves(t *testing.T) {
	u := uitest.New(t, exportRoot)
	fw := &fakeWindow{}
	u.Runtime().SetWindow(fw)

	u.ByRole(access.RoleButton, "Export").Click()
	if fw.save == nil || fw.saveName != "query.sql" {
		t.Fatalf("Export opened no save dialog for query.sql (name %q)", fw.saveName)
	}
	path := filepath.Join(t.TempDir(), "out.sql")
	fw.save(dialog.FileResult{Paths: []string{path}}) // the user chose path
	u.Frame()
	if got, _ := os.ReadFile(path); string(got) != "select 1;" {
		t.Errorf("saved %q", got)
	}
}

// Without SetWindow the runtime has no window: SaveFile does nothing and never
// calls back, as the guide says.
func TestExportWithoutWindow(t *testing.T) {
	u := uitest.New(t, exportRoot)
	u.ByRole(access.RoleButton, "Export").Click() // no panic, no callback
}
