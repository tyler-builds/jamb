# GPUI Kit themes

The JSON files in this directory are the theme sets of
[GPUI Kit](https://github.com/longbridge/gpui-kit), formerly gpui-component, copied
from its `themes/` directory at tag `v0.8.0` (commit
`1676826d9d49b90f4d7b6808b69924f58796f6f1`). The schema is GPUI Kit's
`.theme-schema.json` at the same tag. The directory keeps the project's earlier name.
`matrix.json` is from gpui-component at commit
`f2f0f563292114fecf55fb8ab92a97e14989dbd0`; it was removed upstream after that
commit, and jamb keeps it.
GPUI Kit is Copyright 2024-2026 Longbridge and is licensed under the Apache License,
Version 2.0; the license text is in [LICENSE-APACHE](LICENSE-APACHE).

Each file names the color scheme it is taken from in its `name`, `author` and
`url` fields. The schemes are the work of their authors, for example Catppuccin
(Catppuccino), Gruvbox (Pavel Pertsev), Solarized (Ethan Schoonover) and Tokyo
Night (Folke Lemaitre). jamb credits each scheme in the `Family`, `Author` and
`URL` of its theme (see `ui.Themes`) and in the NOTICE file.

## Conversion

The files are unchanged, apart from their line endings, which are LF. The ui
package converts them with `go generate ./ui`, which runs `internal/themegen` and
writes `ui/themes_gen.go`: one constructor per variant, starting from the built-in
theme of the variant's mode. The mapping from the upstream keys to jamb's color
tokens is in the doc comment of `internal/themegen/main.go`. A key whose value is a
gradient gives its first stop, and a key that names a color, such as `green-600` in
Aurora Light, is skipped for the next key in the list.

Colors that fail the contrast criteria of the ui tests are moved in lightness, as
little as needed: text (`Text`, `TextMuted`, `TextSubtle`, `Link` and the syntax
captures) at 4.5:1; the accent fills and the danger fill with its white label at
4.5:1; and `BorderStrong` and `FocusRing` at 3:1 for non-text. The criterion for
`BorderStrong` is the one that applies to the outline of a control (WCAG 1.4.11).
`Border` is decorative (dividers and the edges of popups), so it is not moved and
keeps its upstream value; jamb's own dark theme has the same kind of line. The accent
label is kept and the fill moves. Every moved color is marked in `ui/themes_gen.go`
with a trailing comment that gives the converted value before the move.

The 237 moved values, by variant, are:

- Adventure: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, Danger
- Adventure Time: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, AccentActive
- Alduin: BorderStrong, TextMuted, TextSubtle, Accent, Disabled, AccentHover, AccentActive
- Asciinema: BorderStrong, TextMuted, TextSubtle, Accent, Disabled, AccentHover, AccentActive, Danger, FocusRing
- Aurora Light: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Ayu Light: BorderStrong, Text, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, AccentActive, Danger, FocusRing; syntax punctuation.list_marker
- Ayu Dark: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Catppuccin Latte: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, AccentActive, Danger, FocusRing; syntax keyword.conditional.ternary
- Catppuccin Frappe: BorderStrong, Danger
- Catppuccin Macchiato: BorderStrong, Danger
- Catppuccin Mocha: TextMuted, TextSubtle, Link, Disabled, Danger
- Everforest Light: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, AccentActive, Danger; syntax string.special.symbol
- Everforest Dark: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Fahrenheit: BorderStrong, TextMuted, TextSubtle, Link, Disabled, FocusRing
- Flexoki Light: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, AccentActive, Danger, FocusRing; syntax string.special.symbol
- Flexoki Dark: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled
- Gruvbox Light: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, AccentActive, Danger, FocusRing; syntax string.special.symbol
- Gruvbox Dark: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Harper: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Hybrid Light: BorderStrong, TextMuted, TextSubtle, Link, Disabled
- Hybrid Dark: BorderStrong, TextMuted, TextSubtle, Disabled, FocusRing
- Jellybeans: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Kibble: BorderStrong, TextMuted, TextSubtle, Disabled
- macOS Classic Light: TextMuted, TextSubtle, Link, Disabled
- macOS Classic Dark: Accent, Link, AccentHover, Danger
- Matrix: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Mellifluous Light: BorderStrong, TextMuted, TextSubtle, Link, Disabled, AccentHover, Danger; syntax string.special.symbol
- Mellifluous Dark: BorderStrong, TextMuted, TextSubtle, Link, Disabled, AccentHover, Danger
- Molokai Light: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, Danger, FocusRing; syntax string.special.symbol
- Molokai Dark: BorderStrong, TextMuted, TextSubtle, Link, Disabled, Danger
- Solarized Light: BorderStrong, Text, TextMuted, TextSubtle, Link, Disabled; syntax string.special.symbol
- Solarized Dark: BorderStrong, TextMuted, TextSubtle, Link, Disabled, FocusRing
- Spaceduck: BorderStrong, TextMuted, TextSubtle, Accent, Link, Disabled, AccentHover, AccentActive, Danger
- Tokyo Night: TextMuted, TextSubtle, Disabled, Danger
- Tokyo Storm: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Tokyo Moon: BorderStrong, TextMuted, TextSubtle, Disabled, Danger
- Twilight: BorderStrong, TextMuted, TextSubtle, Disabled, Danger

Disabled is TextSubtle in every theme, so it moves with it, and TextMuted and
TextSubtle are the same converted color, since upstream has a single muted key. This
list is a record of the generator in this repository; the trailing comments in
`ui/themes_gen.go` are the exact values.
