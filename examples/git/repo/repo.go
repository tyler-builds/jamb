// Package repo reads a Git repository by running the git command: its commits,
// branches, tags and the changes of a commit. It knows nothing of the UI, and
// every call blocks, so the app runs them off the UI thread.
package repo

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// emptyTree is the hash of Git's empty tree, the base a root commit is compared to.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Commit is one commit of the history.
type Commit struct {
	Hash    string
	Parents []string
	Author  string
	Email   string
	When    time.Time
	Subject string
}

// Short is the first seven characters of the hash, as git prints it.
func (c Commit) Short() string { return short(c.Hash) }

func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// RefKind is the kind of a ref.
type RefKind uint8

const (
	// Branch is a local branch, under refs/heads.
	Branch RefKind = iota
	// Remote is a remote-tracking branch, under refs/remotes.
	Remote
	// Tag is a tag, under refs/tags.
	Tag
)

// Ref is a branch, a remote-tracking branch or a tag.
type Ref struct {
	Kind RefKind
	// Name is the short name: "main", "origin/main" or "v1.0".
	Name string
	// Target is the commit the ref points at; for an annotated tag, the commit
	// the tag object points at.
	Target string
	// Annotated is true for a tag with a tag object of its own.
	Annotated bool
	// When is the date of the tag object for an annotated tag, and of the commit
	// otherwise.
	When time.Time
	// Subject is the first line of the tag's message, or of the commit's.
	Subject string
	// Upstream is the short name of a local branch's upstream, such as "origin/main".
	Upstream string
	// Ahead and Behind count the commits between a local branch and its upstream.
	Ahead, Behind int
}

// Info describes a repository as a whole.
type Info struct {
	// Root is the top folder of the working tree.
	Root string
	// Name is the base name of Root.
	Name string
	// Head is the current branch, or "" when HEAD is detached.
	Head string
	// HeadHash is the commit HEAD points at, or "" in a repository with no commits.
	HeadHash string
}

// Open checks that dir is inside a Git working tree and describes it.
func Open(dir string) (Info, error) {
	out, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Info{}, err
	}
	root := filepath.FromSlash(strings.TrimSpace(out))
	info := Info{Root: root, Name: filepath.Base(root)}
	if b, err := git(root, "symbolic-ref", "-q", "--short", "HEAD"); err == nil {
		info.Head = strings.TrimSpace(b)
	}
	if h, err := git(root, "rev-parse", "-q", "--verify", "HEAD"); err == nil {
		info.HeadHash = strings.TrimSpace(h)
	}
	return info, nil
}

// Log returns up to n commits of every branch, remote-tracking branch and tag
// (not the stash), newest first in topological order, after skipping the first
// skip of them. Pages read this way join up: page two starts where page one ended.
func Log(dir string, skip, n int) ([]Commit, error) {
	out, err := git(dir, "log", "--exclude=refs/stash", "--all", "--topo-order",
		"--format=%H%x1f%P%x1f%an%x1f%ae%x1f%at%x1f%s%x1e",
		"--skip="+strconv.Itoa(skip), "--max-count="+strconv.Itoa(n))
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for rec := range strings.SplitSeq(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\r\n")
		if rec == "" {
			continue
		}
		f := strings.Split(rec, "\x1f")
		if len(f) < 6 {
			return nil, fmt.Errorf("git log: unexpected record %q", rec)
		}
		commits = append(commits, Commit{
			Hash: f[0], Parents: strings.Fields(f[1]), Author: f[2], Email: f[3],
			When: unix(f[4]), Subject: f[5],
		})
	}
	return commits, nil
}

// Refs returns the branches, remote-tracking branches and tags, each kind in
// name order. A remote's HEAD (origin/HEAD) is left out.
func Refs(dir string) ([]Ref, error) {
	out, err := git(dir, "for-each-ref",
		"--format=%(refname)%1f%(objecttype)%1f%(objectname)%1f%(*objectname)%1f%(creatordate:unix)%1f%(subject)%1f%(upstream:short)%1f%(upstream:track,nobracket)",
		"refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, err
	}
	var refs []Ref
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x1f")
		if len(f) < 8 {
			return nil, fmt.Errorf("git for-each-ref: unexpected line %q", line)
		}
		r := Ref{Target: f[2], When: unix(f[4]), Subject: f[5], Upstream: f[6]}
		switch name := f[0]; {
		case strings.HasPrefix(name, "refs/heads/"):
			r.Kind, r.Name = Branch, strings.TrimPrefix(name, "refs/heads/")
			r.Ahead, r.Behind = track(f[7])
		case strings.HasPrefix(name, "refs/remotes/"):
			r.Kind, r.Name = Remote, strings.TrimPrefix(name, "refs/remotes/")
			if strings.HasSuffix(r.Name, "/HEAD") {
				continue
			}
		case strings.HasPrefix(name, "refs/tags/"):
			r.Kind, r.Name = Tag, strings.TrimPrefix(name, "refs/tags/")
			if f[1] == "tag" {
				r.Annotated = true
				r.Target = f[3]
			}
		default:
			continue
		}
		refs = append(refs, r)
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Kind != refs[j].Kind {
			return refs[i].Kind < refs[j].Kind
		}
		return refs[i].Name < refs[j].Name
	})
	return refs, nil
}

// track reads "ahead 2, behind 1" from %(upstream:track,nobracket).
func track(s string) (ahead, behind int) {
	for part := range strings.SplitSeq(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), " ")
		if !ok {
			continue
		}
		n, _ := strconv.Atoi(v)
		switch k {
		case "ahead":
			ahead = n
		case "behind":
			behind = n
		}
	}
	return ahead, behind
}

// Details is everything the detail pane shows about one commit.
type Details struct {
	Commit
	Body           string // the message after the subject, trimmed
	Committer      string
	CommitterEmail string
	Committed      time.Time
	Files          []FileChange
}

// FileStatus is how a commit changed a file.
type FileStatus byte

// The statuses git diff --name-status reports.
const (
	Added    FileStatus = 'A'
	Modified FileStatus = 'M'
	Deleted  FileStatus = 'D'
	Renamed  FileStatus = 'R'
	Copied   FileStatus = 'C'
	TypeChg  FileStatus = 'T'
)

// FileChange is one file a commit changed. A merge is compared to its first parent.
type FileChange struct {
	Status FileStatus
	Path   string
	// OldPath is the path before a rename or copy, else "".
	OldPath string
	// Added and Deleted count lines; both are -1 for a binary file.
	Added, Deleted int
}

// Base is what a commit's changes are measured against: its first parent, or the
// empty tree for a root commit.
func Base(c Commit) string {
	if len(c.Parents) > 0 {
		return c.Parents[0]
	}
	return emptyTree
}

// Show returns the details of the commit hash.
func Show(dir, hash string) (Details, error) {
	out, err := git(dir, "show", "-s", "--format=%H%x1f%P%x1f%an%x1f%ae%x1f%at%x1f%cn%x1f%ce%x1f%ct%x1f%s%x1f%b", hash)
	if err != nil {
		return Details{}, err
	}
	f := strings.SplitN(strings.TrimRight(out, "\r\n"), "\x1f", 10)
	if len(f) < 10 {
		return Details{}, fmt.Errorf("git show: unexpected output for %s", hash)
	}
	d := Details{
		Commit: Commit{Hash: f[0], Parents: strings.Fields(f[1]), Author: f[2], Email: f[3],
			When: unix(f[4]), Subject: f[8]},
		Committer: f[5], CommitterEmail: f[6], Committed: unix(f[7]),
		Body: strings.TrimSpace(f[9]),
	}
	d.Files, err = changes(dir, Base(d.Commit), d.Hash)
	return d, err
}

// changes lists the files that differ between base and hash, with line counts.
func changes(dir, base, hash string) ([]FileChange, error) {
	names, err := git(dir, "diff", "--name-status", "-z", "-M", base, hash)
	if err != nil {
		return nil, err
	}
	var files []FileChange
	f := strings.Split(names, "\x00")
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "" {
			continue
		}
		fc := FileChange{Status: FileStatus(f[i][0])}
		if (fc.Status == Renamed || fc.Status == Copied) && i+2 < len(f) {
			fc.OldPath, fc.Path = f[i+1], f[i+2]
			i += 2
		} else {
			fc.Path = f[i+1]
			i++
		}
		files = append(files, fc)
	}
	nums, err := git(dir, "diff", "--numstat", "-z", "-M", base, hash)
	if err != nil {
		return nil, err
	}
	counts := map[string][2]int{}
	n := strings.Split(nums, "\x00")
	for i := 0; i < len(n); i++ {
		fields := strings.SplitN(n[i], "\t", 3)
		if len(fields) < 3 {
			continue
		}
		path := fields[2]
		if path == "" && i+2 < len(n) { // a rename: the old and new paths follow
			path = n[i+2]
			i += 2
		}
		counts[path] = [2]int{count(fields[0]), count(fields[1])}
	}
	for i := range files {
		c := counts[files[i].Path]
		files[i].Added, files[i].Deleted = c[0], c[1]
	}
	return files, nil
}

// count reads a numstat count, which is "-" for a binary file.
func count(s string) int {
	if s == "-" {
		return -1
	}
	n, _ := strconv.Atoi(s)
	return n
}

// Diff returns the unified diff of one file of a commit, against the commit's base.
func Diff(dir string, c Commit, f FileChange) (string, error) {
	args := []string{"diff", "-M", "--no-color", Base(c), c.Hash, "--", f.Path}
	if f.OldPath != "" {
		args = append(args, f.OldPath)
	}
	return git(dir, args...)
}

func unix(s string) time.Time {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(n, 0)
}

// ErrNoGit is returned when the git command cannot be found.
var ErrNoGit = errors.New("git was not found on PATH")

// git runs git in dir and returns its standard output. A failure returns the
// first line git wrote to its standard error.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.quotepath=off", "-c", "log.showSignature=false"}, args...)...)
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrNoGit
		}
		msg, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], strings.TrimPrefix(msg, "fatal: "))
	}
	return stdout.String(), nil
}
