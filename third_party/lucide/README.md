# Lucide

The icons of the `icons` package are those of [Lucide](https://lucide.dev), release
1.61.0 (tag `1.61.0`, commit `c29c276f16590cf017126671535ad8c792be64d8`). Lucide is
Copyright (c) 2026 Lucide Icons and Contributors, licensed under the ISC License; some
of its icons are derived from Feather, Copyright (c) 2013-present Cole Bemis, licensed
under the MIT License. Both notices are in [LICENSE](LICENSE).

The SVG files are not kept in this repository. `internal/lucidegen` downloads the
release's source archive from GitHub, checks its `icons/*.svg` files against a SHA-256
recorded in the generator, and writes `icons/lucide_gen.go`; `go generate ./icons` runs
it. To update Lucide, change the tag, commit and hash in `internal/lucidegen/main.go`,
run `go generate ./icons`, and update this file. `-zip` reads a downloaded archive
instead.
