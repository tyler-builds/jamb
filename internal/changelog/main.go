// Command changelog keeps the unreleased entries of CHANGELOG.md in fragment
// files, one per change, so that pull requests made at the same time never
// edit the same lines. Run it from the root of the repository:
//
//	go run ./internal/changelog check            # checks every fragment; CI runs it
//	go run ./internal/changelog preview          # prints the Unreleased section the fragments make
//	go run ./internal/changelog release 0.6.4    # moves them into a 0.6.4 section dated today
//
// A fragment is changelog/unreleased/YYYY-MM-DD-slug.md. It holds one or more
// Keep a Changelog sections, each a "### " heading followed by a bullet list:
//
//	### Fixed
//
//	- A text field keeps its caret in view after a paste.
//
// Text before the first heading is an introduction: release puts it under the
// version's heading, before the sections, as the release notes for cmd/jamb are.
//
// release merges the fragments in file name order, so the date in the name
// keeps the entries in the order they were written. It writes the sections in
// the order Keep a Changelog lists them, adds the version's comparison link at
// the end of the file, and deletes the fragments. -date sets the date.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	changelogFile = "CHANGELOG.md"
	fragmentDir   = "changelog/unreleased"
	unreleased    = "## [Unreleased]"
)

// sections are the section names Keep a Changelog defines, in its order.
var sections = []string{"Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"}

var (
	fragmentName = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-[a-z0-9]+(-[a-z0-9]+)*\.md$`)
	versionRe    = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	unreleasedRe = regexp.MustCompile(`(?m)^\[Unreleased\]: (\S+)/compare/v(\S+)\.\.\.HEAD$`)
)

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: go run ./internal/changelog check | preview | release [-date YYYY-MM-DD] VERSION")
}

func main() {
	if err := run(".", os.Args[1:], os.Stdout, time.Now()); err != nil {
		fmt.Fprintln(os.Stderr, "changelog:", err)
		os.Exit(1)
	}
}

func run(root string, args []string, stdout io.Writer, now time.Time) error {
	if len(args) == 0 {
		usage(os.Stderr)
		return errors.New("no command")
	}
	switch args[0] {
	case "check":
		if len(args) != 1 {
			return errors.New("check takes no arguments")
		}
		frags, err := loadFragments(root)
		if err != nil {
			return err
		}
		if err := checkUnreleasedEmpty(root); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "changelog: %d fragment(s) checked, CHANGELOG.md has no hand-written unreleased entries\n", len(frags))
		return nil
	case "preview":
		if len(args) != 1 {
			return errors.New("preview takes no arguments")
		}
		frags, err := loadFragments(root)
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, render(merge(frags)))
		return nil
	case "release":
		fs := flag.NewFlagSet("release", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		date := fs.String("date", now.Format(time.DateOnly), "the release date")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return errors.New("release takes one version, such as 0.6.4")
		}
		if _, err := time.Parse(time.DateOnly, *date); err != nil {
			return fmt.Errorf("-date %q is not YYYY-MM-DD", *date)
		}
		return release(root, strings.TrimPrefix(fs.Arg(0), "v"), *date)
	default:
		usage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// fragment is one parsed fragment file.
type fragment struct {
	name     string
	intro    string
	sections map[string]string // section name to its bullet list
}

// merged is the content of all the fragments, section by section.
type merged struct {
	intro    []string
	sections map[string][]string
}

func loadFragments(root string) ([]fragment, error) {
	dir := filepath.Join(root, filepath.FromSlash(fragmentDir))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var frags []fragment
	var errs []error
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := fragmentDir + "/" + e.Name()
		if !fragmentName.MatchString(e.Name()) {
			errs = append(errs, fmt.Errorf("%s: name it YYYY-MM-DD-slug.md, with a slug of lowercase letters, digits and hyphens", path))
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		f, err := parseFragment(e.Name(), string(data))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		frags = append(frags, f)
	}
	sort.Slice(frags, func(i, j int) bool { return frags[i].name < frags[j].name })
	return frags, errors.Join(errs...)
}

func parseFragment(name, text string) (fragment, error) {
	f := fragment{name: name, sections: map[string]string{}}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var cur string // the section being read; "" is the introduction
	var body []string
	flush := func() error {
		b := strings.Trim(strings.Join(body, "\n"), "\n")
		body = body[:0]
		if cur == "" {
			f.intro = b
			return nil
		}
		if b == "" {
			return fmt.Errorf("section %q has no entries", cur)
		}
		if !strings.HasPrefix(b, "- ") {
			return fmt.Errorf("section %q must be a bullet list, each entry starting with \"- \"", cur)
		}
		f.sections[cur] = b
		return nil
	}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimRight(line, " \t")
		if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "### ") {
			return f, fmt.Errorf("heading %q: use only \"### \" section headings", line)
		}
		if h, ok := strings.CutPrefix(line, "### "); ok {
			if err := flush(); err != nil {
				return f, err
			}
			h = strings.TrimSpace(h)
			if !slices.Contains(sections, h) {
				return f, fmt.Errorf("section %q is not one of %s", h, strings.Join(sections, ", "))
			}
			if _, dup := f.sections[h]; dup {
				return f, fmt.Errorf("section %q appears twice", h)
			}
			cur = h
			continue
		}
		body = append(body, line)
	}
	if err := flush(); err != nil {
		return f, err
	}
	if f.intro == "" && len(f.sections) == 0 {
		return f, errors.New("the fragment is empty")
	}
	return f, nil
}

func merge(frags []fragment) merged {
	m := merged{sections: map[string][]string{}}
	for _, f := range frags {
		if f.intro != "" {
			m.intro = append(m.intro, f.intro)
		}
		for _, s := range sections {
			if b, ok := f.sections[s]; ok {
				m.sections[s] = append(m.sections[s], b)
			}
		}
	}
	return m
}

// render writes the body of a version's section: the introduction, then each
// section with entries, each part followed by a blank line.
func render(m merged) string {
	var b strings.Builder
	for _, p := range m.intro {
		b.WriteString(p + "\n\n")
	}
	for _, s := range sections {
		if len(m.sections[s]) == 0 {
			continue
		}
		b.WriteString("### " + s + "\n\n")
		b.WriteString(strings.Join(m.sections[s], "\n") + "\n\n")
	}
	return b.String()
}

// unreleasedBody returns the text between the Unreleased heading and the next
// version's heading, and where it starts and ends in text.
func unreleasedBody(text string) (body string, start, end int, err error) {
	i := strings.Index(text, "\n"+unreleased+"\n")
	if i < 0 {
		return "", 0, 0, fmt.Errorf("%s has no %q heading", changelogFile, unreleased)
	}
	start = i + len(unreleased) + 2
	end = len(text)
	if j := strings.Index(text[start:], "\n## "); j >= 0 {
		end = start + j + 1
	}
	return text[start:end], start, end, nil
}

func checkUnreleasedEmpty(root string) error {
	data, err := os.ReadFile(filepath.Join(root, changelogFile))
	if err != nil {
		return err
	}
	body, _, _, err := unreleasedBody(strings.ReplaceAll(string(data), "\r\n", "\n"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) != "" {
		return fmt.Errorf("%s has entries under %s; move them to a file in %s (see CONTRIBUTING.md, \"Changelog\")", changelogFile, unreleased, fragmentDir)
	}
	return nil
}

func release(root, version, date string) error {
	if !versionRe.MatchString(version) {
		return fmt.Errorf("version %q is not MAJOR.MINOR.PATCH", version)
	}
	frags, err := loadFragments(root)
	if err != nil {
		return err
	}
	if len(frags) == 0 {
		return fmt.Errorf("%s has no fragments to release", fragmentDir)
	}
	path := filepath.Join(root, changelogFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.Contains(text, "\n## ["+version+"]") {
		return fmt.Errorf("%s already has a %s section", changelogFile, version)
	}
	body, start, end, err := unreleasedBody(text)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) != "" {
		return fmt.Errorf("%s has entries under %s; move them to a file in %s first", changelogFile, unreleased, fragmentDir)
	}
	link := unreleasedRe.FindStringSubmatchIndex(text)
	if link == nil {
		return fmt.Errorf("%s has no \"[Unreleased]: <repo>/compare/vX.Y.Z...HEAD\" link", changelogFile)
	}
	repo, prev := text[link[2]:link[3]], text[link[4]:link[5]]

	section := "\n## [" + version + "] - " + date + "\n\n" + render(merge(frags))
	links := "[Unreleased]: " + repo + "/compare/v" + version + "...HEAD\n" +
		"[" + version + "]: " + repo + "/compare/v" + prev + "...v" + version
	// The link comes after the Unreleased section, so replace it first.
	text = text[:link[0]] + links + text[link[1]:]
	text = text[:start] + section + text[end:]
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	for _, f := range frags {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(fragmentDir), f.name)); err != nil {
			return err
		}
	}
	return nil
}
