// Package repotest builds sample Git repositories for tests.
package repotest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Sample builds a small repository in a temporary folder: a main branch with a
// merged feature branch, a renamed file, an annotated and a lightweight tag, and
// an unmerged branch. Authors and dates are fixed, so hashes are the same every run.
func Sample(t testing.TB) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	run := func(date string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Ada Lovelace", "GIT_AUTHOR_EMAIL=ada@example.com",
			"GIT_COMMITTER_NAME=Ada Lovelace", "GIT_COMMITTER_EMAIL=ada@example.com",
			"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date, "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("", "init", "-q", "-b", "main")
	run("", "config", "commit.gpgsign", "false")
	write("readme.md", "# Sample\n")
	run("2026-01-01T10:00:00Z", "add", ".")
	run("2026-01-01T10:00:00Z", "commit", "-q", "-m", "Initial commit")
	run("2026-01-01T10:00:00Z", "tag", "v0.1.0")
	run("", "checkout", "-q", "-b", "feature")
	write("feature.go", "package sample\n\nfunc Feature() {}\n")
	run("2026-01-02T10:00:00Z", "add", ".")
	run("2026-01-02T10:00:00Z", "commit", "-q", "-m", "Add the feature")
	run("", "checkout", "-q", "main")
	write("readme.md", "# Sample\n\nA sample repository.\n")
	run("2026-01-03T10:00:00Z", "commit", "-q", "-am", "Describe the sample", "-m", "The body of the message.")
	run("2026-01-04T10:00:00Z", "merge", "-q", "--no-ff", "-m", "Merge branch 'feature'", "feature")
	run("2026-01-05T10:00:00Z", "mv", "feature.go", "sample.go")
	run("2026-01-05T10:00:00Z", "commit", "-q", "-m", "Rename feature.go")
	run("2026-01-05T10:00:00Z", "tag", "-a", "v0.2.0", "-m", "Second release")
	run("", "checkout", "-q", "-b", "spike", "HEAD~1")
	write("spike.txt", "try this\n")
	run("2026-01-06T10:00:00Z", "add", ".")
	run("2026-01-06T10:00:00Z", "commit", "-q", "-m", "Try a spike")
	run("", "checkout", "-q", "main")
	return dir
}
