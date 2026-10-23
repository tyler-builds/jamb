package repo_test

import . "github.com/tyler-builds/jamb/examples/git/repo"

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/examples/git/repo/repotest"
)

func TestOpenLogAndRefs(t *testing.T) {
	dir := repotest.Sample(t)
	info, err := Open(filepath.Join(dir))
	if err != nil {
		t.Fatal(err)
	}
	if info.Head != "main" || info.HeadHash == "" {
		t.Errorf("Open = %+v, want HEAD on main", info)
	}
	commits, err := Log(dir, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	var subjects []string
	for _, c := range commits {
		subjects = append(subjects, c.Subject)
	}
	if len(commits) != 6 || commits[len(commits)-1].Subject != "Initial commit" {
		t.Fatalf("Log subjects = %q", subjects)
	}
	for _, c := range commits {
		if c.Subject == "Merge branch 'feature'" && len(c.Parents) != 2 {
			t.Errorf("the merge has %d parents", len(c.Parents))
		}
		if c.Author != "Ada Lovelace" || c.When.IsZero() {
			t.Errorf("commit %s: author %q at %v", c.Short(), c.Author, c.When)
		}
	}
	page, err := Log(dir, 4, 100)
	if err != nil || len(page) != 2 || page[0].Hash != commits[4].Hash {
		t.Errorf("Log(skip 4) = %d commits, %v", len(page), err)
	}

	refs, err := Refs(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range refs {
		names = append(names, r.Name)
	}
	if want := []string{"feature", "main", "spike", "v0.1.0", "v0.2.0"}; !slices.Equal(names, want) {
		t.Errorf("Refs = %q, want %q", names, want)
	}
	for _, r := range refs {
		if r.Name == "v0.2.0" && (!r.Annotated || r.Subject != "Second release" || r.Target != info.HeadHash) {
			t.Errorf("annotated tag = %+v, want it on HEAD %s", r, info.HeadHash)
		}
		if r.Name == "v0.1.0" && (r.Annotated || r.Kind != Tag) {
			t.Errorf("lightweight tag = %+v", r)
		}
	}
}

func TestShowAndDiff(t *testing.T) {
	dir := repotest.Sample(t)
	commits, err := Log(dir, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	find := func(subject string) Commit {
		for _, c := range commits {
			if c.Subject == subject {
				return c
			}
		}
		t.Fatalf("no commit %q", subject)
		return Commit{}
	}
	d, err := Show(context.Background(), dir, find("Describe the sample").Hash)
	if err != nil {
		t.Fatal(err)
	}
	if d.Body != "The body of the message." || len(d.Files) != 1 {
		t.Fatalf("Show = body %q, files %+v", d.Body, d.Files)
	}
	if f := d.Files[0]; f.Status != Modified || f.Path != "readme.md" || f.Added != 2 || f.Deleted != 0 {
		t.Errorf("file = %+v, want readme.md modified with 2 lines added", f)
	}
	diff, err := Diff(context.Background(), dir, d.Commit, d.Files[0])
	if err != nil || !strings.Contains(diff, "+A sample repository.") {
		t.Errorf("Diff = %q, %v", diff, err)
	}

	r, err := Show(context.Background(), dir, find("Rename feature.go").Hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Files) != 1 || r.Files[0].Status != Renamed || r.Files[0].OldPath != "feature.go" || r.Files[0].Path != "sample.go" {
		t.Errorf("rename = %+v", r.Files)
	}

	root, err := Show(context.Background(), dir, find("Initial commit").Hash)
	if err != nil || len(root.Files) != 1 || root.Files[0].Status != Added {
		t.Errorf("root = %+v, %v", root.Files, err)
	}
}

func TestOpenOutsideARepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	if _, err := Open(t.TempDir()); err == nil {
		t.Error("Open of a plain folder succeeded")
	}
}
