package store

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A write that fails keeps the changes pending: the next Flush writes them,
// rather than reporting that there is nothing to save.
func TestFlushAfterAFailedWrite(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("needs a directory that the user cannot write")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	n := s.Create()
	n.Title.Set("Unsaved title")
	s.Changed(n)

	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if err := s.Flush(); err == nil {
		t.Fatal("Flush wrote into a read-only directory")
	}
	if err := s.Flush(); err == nil {
		t.Fatal("a second Flush reported success with the changes still unwritten")
	}
	os.Chmod(dir, 0o755)
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), "Unsaved title") {
		t.Fatalf("the notes file lacks the change: %v", err)
	}
}
