package store

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestMigrateLegacyDirMovesNotes(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, legacyDirName, "notes.json"))
	writeTestFile(t, filepath.Join(dir, legacyDirName, "settings.json"))
	if err := migrateLegacyDir(dir); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(dir, legacyDirName)) {
		t.Error("goui-notes still exists after migration")
	}
	for _, f := range []string{"notes.json", "settings.json"} {
		if !exists(filepath.Join(dir, dirName, f)) {
			t.Errorf("%s was not moved to jamb-notes", f)
		}
	}
}

func TestMigrateLegacyDirKeepsExistingTarget(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, legacyDirName, "notes.json"))
	writeTestFile(t, filepath.Join(dir, dirName, "notes.json"))
	if err := migrateLegacyDir(dir); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dir, legacyDirName, "notes.json")) {
		t.Error("goui-notes was moved over an existing jamb-notes")
	}
}

func TestMigrateLegacyDirWithoutLegacyDir(t *testing.T) {
	dir := t.TempDir()
	if err := migrateLegacyDir(dir); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("migration created %d entries in an empty config directory", len(entries))
	}
}

// DefaultPath runs the migration on the user config directory. The
// environment points that directory at a temporary one for each platform.
func TestDefaultPathMovesLegacyNotes(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("JAMB_NOTES_FILE", "")
	t.Setenv("APPDATA", cfg)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", cfg)
	writeTestFile(t, filepath.Join(cfg, legacyDirName, "notes.json"))

	got := DefaultPath()
	want := filepath.Join(cfg, dirName, "notes.json")
	if got != want {
		t.Fatalf("DefaultPath() = %s, want %s", got, want)
	}
	if !exists(want) {
		t.Error("notes.json was not carried over to jamb-notes")
	}
}
