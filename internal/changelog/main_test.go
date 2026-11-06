package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testChangelog = `# Changelog

Intro.

## [Unreleased]

## [0.6.3] - 2026-11-06

### Fixed

- Old fix.

[Unreleased]: https://github.com/tyler-builds/jamb/compare/v0.6.3...HEAD
[0.6.3]: https://github.com/tyler-builds/jamb/compare/v0.6.2...v0.6.3
`

// repo makes a repository root holding testChangelog (or changelog) and the
// fragments, each a file name and its content.
func repo(t *testing.T, changelog string, fragments ...string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, changelogFile), []byte(changelog), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, filepath.FromSlash(fragmentDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(fragments); i += 2 {
		if err := os.WriteFile(filepath.Join(dir, fragments[i]), []byte(fragments[i+1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runIn(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(root, args, &out, time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC))
	return out.String(), err
}

func TestReleaseMergesFragmentsInNameOrder(t *testing.T) {
	root := repo(t, testChangelog,
		"2026-11-07-second.md", "### Fixed\n\n- Second fix,\n  wrapped.\n",
		"2026-11-06-first.md", "The `jamb` command is released with it.\n\n### Fixed\n\n- First fix.\n\n### Added\n\n- A feature.\n",
	)
	if _, err := runIn(t, root, "release", "v0.6.4"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, changelogFile))
	if err != nil {
		t.Fatal(err)
	}
	want := `# Changelog

Intro.

## [Unreleased]

## [0.6.4] - 2026-11-07

The ` + "`jamb`" + ` command is released with it.

### Added

- A feature.

### Fixed

- First fix.
- Second fix,
  wrapped.

## [0.6.3] - 2026-11-06

### Fixed

- Old fix.

[Unreleased]: https://github.com/tyler-builds/jamb/compare/v0.6.4...HEAD
[0.6.4]: https://github.com/tyler-builds/jamb/compare/v0.6.3...v0.6.4
[0.6.3]: https://github.com/tyler-builds/jamb/compare/v0.6.2...v0.6.3
`
	if string(got) != want {
		t.Errorf("CHANGELOG.md =\n%s\nwant\n%s", got, want)
	}
	left, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(fragmentDir)))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("%d fragments left after release", len(left))
	}
}

func TestReleaseDateFlag(t *testing.T) {
	root := repo(t, testChangelog, "2026-11-07-a.md", "### Fixed\n\n- A fix.\n")
	if _, err := runIn(t, root, "release", "-date", "2026-12-01", "0.7.0"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, changelogFile))
	if !strings.Contains(string(got), "## [0.7.0] - 2026-12-01\n") {
		t.Errorf("no 0.7.0 heading dated 2026-12-01 in\n%s", got)
	}
}

func TestReleaseRefuses(t *testing.T) {
	frag := []string{"2026-11-07-a.md", "### Fixed\n\n- A fix.\n"}
	for _, tc := range []struct {
		name, changelog, version, want string
		fragments                      []string
	}{
		{"no fragments", testChangelog, "0.6.4", "no fragments", nil},
		{"bad version", testChangelog, "0.6", "not MAJOR.MINOR.PATCH", frag},
		{"existing version", testChangelog, "0.6.3", "already has", frag},
		{"hand-written entries", strings.Replace(testChangelog, "## [Unreleased]\n", "## [Unreleased]\n\n- Hand-written.\n", 1), "0.6.4", "has entries", frag},
		{"no link", strings.Replace(testChangelog, "[Unreleased]: ", "[Next]: ", 1), "0.6.4", "no \"[Unreleased]", frag},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := repo(t, tc.changelog, tc.fragments...)
			_, err := runIn(t, root, "release", tc.version)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one containing %q", err, tc.want)
			}
			got, _ := os.ReadFile(filepath.Join(root, changelogFile))
			if string(got) != tc.changelog {
				t.Error("a refused release changed CHANGELOG.md")
			}
		})
	}
}

func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		name, file, content, want string
	}{
		{"good", "2026-11-07-good.md", "### Fixed\n\n- A fix.\n", ""},
		{"intro only", "2026-11-07-intro.md", "Released with cmd/jamb v0.4.2.\n", ""},
		{"bad name", "fix.md", "### Fixed\n\n- A fix.\n", "name it YYYY-MM-DD-slug.md"},
		{"upper case slug", "2026-11-07-Fix.md", "### Fixed\n\n- A fix.\n", "name it"},
		{"unknown section", "2026-11-07-a.md", "### Fixes\n\n- A fix.\n", `section "Fixes" is not one of`},
		{"version heading", "2026-11-07-a.md", "## [0.6.4]\n\n### Fixed\n\n- A fix.\n", "use only"},
		{"empty section", "2026-11-07-a.md", "### Fixed\n\n### Added\n\n- A.\n", "has no entries"},
		{"not a list", "2026-11-07-a.md", "### Fixed\n\nA fix.\n", "bullet list"},
		{"duplicate section", "2026-11-07-a.md", "### Fixed\n\n- A.\n\n### Fixed\n\n- B.\n", "appears twice"},
		{"empty", "2026-11-07-a.md", "\n\n", "empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := repo(t, testChangelog, tc.file, tc.content)
			_, err := runIn(t, root, "check")
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestCheckRejectsHandWrittenUnreleasedEntries(t *testing.T) {
	root := repo(t, strings.Replace(testChangelog, "## [Unreleased]\n", "## [Unreleased]\n\n### Fixed\n\n- Hand-written.\n", 1))
	if _, err := runIn(t, root, "check"); err == nil || !strings.Contains(err.Error(), "move them to a file") {
		t.Fatalf("err = %v, want one that asks for a fragment", err)
	}
}

func TestCheckWithoutFragmentDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, changelogFile), []byte(testChangelog), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runIn(t, root, "check"); err != nil {
		t.Fatal(err)
	}
}

func TestPreview(t *testing.T) {
	root := repo(t, testChangelog,
		"2026-11-07-b.md", "### Fixed\n\n- B.\n",
		"2026-11-06-a.md", "### Added\n\n- A.\n",
	)
	out, err := runIn(t, root, "preview")
	if err != nil {
		t.Fatal(err)
	}
	if want := "### Added\n\n- A.\n\n### Fixed\n\n- B.\n\n"; out != want {
		t.Errorf("preview = %q, want %q", out, want)
	}
}

// The repository's own changelog and fragments must pass the check CI runs.
func TestRepositoryChangelog(t *testing.T) {
	if _, err := runIn(t, filepath.Join("..", ".."), "check"); err != nil {
		t.Fatal(err)
	}
}
