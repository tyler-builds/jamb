# Contributing to jamb

Thanks for your interest. jamb has one maintainer, so this page says what gets reviewed quickly and what
needs a conversation first.

## Issues

Bug reports and feature requests are welcome; the issue forms ask for what helps most. For a bug, a
small program that shows it is worth more than anything else. Report security problems privately, as
[SECURITY.md](SECURITY.md) describes.

## Pull requests

- Small fixes (a bug, a typo, a doc that is wrong) can go straight to a pull request.
- For a new feature, a new widget, an API change or a new platform, open an issue first. jamb keeps its
  API consistent on purpose, and a short discussion avoids work that can't be merged.
- macOS and Linux are planned. Their adapters plug in behind `platform.Window` and `render.Renderer`
  (see [docs/architecture.md](docs/architecture.md)); please start with an issue.

## Rules the code follows

- Pure Go: no cgo.
- `ui`, `app` and the packages they import use no `crypto/...` package, even for non-security hashing,
  since any import links Go's FIPS module and a 32 MB buffer into every app. Use `hash/fnv`,
  `hash/maphash` or `math/rand/v2`. `TestCorePackagesLinkNoCrypto` checks this.
- Tests are headless: they need no display and don't touch the system clipboard. A test that has to open
  a window skips itself unless `JAMB_LIVE_TESTS=1` is set.
- Default styles never change font weight for hover, selection or focus; state is shown with fills,
  outlines and indicators.
- Exported names have doc comments, and docs in `docs/` change with the behavior they describe.

## Checks

Development needs Go 1.27.1 or later and Windows. Before sending a change, run:

```
gofmt -l .
go vet ./... ./cmd/jamb/... ./examples/demo/... ./examples/notes/... ./examples/hello/... ./examples/git/... ./examples/monitor/...
go test ./... ./cmd/jamb/... ./examples/demo/... ./examples/notes/... ./examples/hello/... ./examples/git/... ./examples/monitor/...
```

`gofmt -l .` should print nothing. CI runs the same checks, the race detector, the window tests and
builds for Linux and macOS.

## Commits

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/), for
example `fix(ui): keep the caret visible after paste`. Mark a breaking change with `!`
(`feat(ui)!: ...`). One logical change per commit.

Pull requests are squash-merged, so the pull request's title becomes the commit on `main`: write the
title in the same form, and the commits inside it can be as rough as you like. A pull request whose
commits are each one clean change in that form may be rebase-merged instead, keeping them. `main` has
a linear history; there are no merge commits.

## License

jamb is licensed under the Apache License, Version 2.0. A contribution submitted for inclusion is
licensed under the same terms, as section 5 of the license says.
