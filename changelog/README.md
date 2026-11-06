# Unreleased changelog entries

Each change that users would notice adds one file to `changelog/unreleased/`, instead of an entry
under `## [Unreleased]` in [CHANGELOG.md](../CHANGELOG.md). Pull requests made at the same time
then never edit the same lines. The files are merged into CHANGELOG.md when a version is released.

Name the file `YYYY-MM-DD-slug.md`, with today's date and a slug of lowercase letters, digits and
hyphens, such as `2026-11-07-caret-after-paste.md`. Write the entry as it will read in CHANGELOG.md:
one or more [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) sections (`Added`, `Changed`,
`Deprecated`, `Removed`, `Fixed`, `Security`), each a `### ` heading and a bullet list.

```markdown
### Fixed

- A text field keeps its caret in view after a paste.
```

Text before the first heading is an introduction, placed under the version's heading. Use it for a
note such as which release of `cmd/jamb` comes with the version.

From the repository's root:

```
go run ./internal/changelog check            # checks every file; CI runs it
go run ./internal/changelog preview          # prints the section the files make
go run ./internal/changelog release 0.6.4    # merges them into a 0.6.4 section dated today, and deletes them
```

`release` keeps the order of the file names, so entries of a section appear in the order they were
written. It also updates the comparison links at the end of CHANGELOG.md. `-date YYYY-MM-DD` sets
another date.
