// Command themegen converts the predefined themes of GPUI Kit (formerly gpui-component) into jamb
// themes. It reads the theme sets in third_party/gpui-component-themes and
// writes ui/themes_gen.go. The ui package runs it with go generate:
//
//	go generate ./ui
//
// Each variant becomes one unexported constructor. The constructor starts from
// the built-in theme of the variant's mode (darkTheme or lightTheme) and sets the
// tokens below. A key is used when its value is a usable color: a hex color, or
// the first stop of a linear gradient. A value with no hex color, such as the
// name "green-600", is skipped, and the next key in the list is tried.
//
//	Token                                        Upstream keys, in order                           If none
//	Background                                   background
//	Panel                                        sidebar.background, title_bar.background          Background
//	Surface                                      popover.background, list.even.background          Background
//	SurfaceHover                                 list.hover.background                             Surface mixed halfway to SurfaceActive
//	SurfaceActive                                secondary.active.background, muted.background
//	Border                                       border
//	BorderStrong                                 input.border                                      Border mixed halfway to Text
//	Text                                         foreground
//	TextMuted, TextSubtle                        muted.foreground
//	Accent                                       primary.background, base.blue
//	AccentHover                                  primary.hover.background                          Accent mixed halfway to AccentActive
//	AccentActive                                 primary.active.background                         Accent darkened by 12%
//	AccentText                                   primary.foreground
//	AccentSoft                                   list.active.background                            Accent at alpha 0.15
//	Link                                         link.foreground, link.active.foreground, link     Accent
//	Disabled                                     TextSubtle
//	Success, Warning, Danger                     base.green, success.background; base.yellow,
//	                                             warning.background; base.red, danger.background
//	Scrollbar                                    scrollbar.thumb.background
//	FocusRing                                    ring                                              Accent
//	Scrim                                        dark: black at alpha 0.6; light: Text at alpha 0.35
//	Radius, FontSize                             from the built-in theme
//	Syntax                                       highlight.syntax; a capture with no color takes the editor foreground
//	CodeEditor.Background, Foreground, ActiveLine,  editor.background, editor.foreground,
//	CodeEditor.LineNumber, ActiveLineNumber        editor.active_line.background, editor.line_number,
//	                                               editor.active_line_number (the last falls back to the foreground)
//	CodeEditor.Selection                           selection.background                              Accent at alpha 0.32
//
// The colors are then moved in lightness, minimally, to meet the contrast
// criteria of the ui tests: text (Text, TextMuted, TextSubtle, Link) and syntax
// captures at 4.5:1 on the surfaces they sit on; the accent fills, the danger
// fill and the danger label at 4.5:1 (the label of a danger button is #ffffff,
// as the built-in widgets draw it); BorderStrong and FocusRing at 3:1 (non-text).
// Border is decorative (dividers and popup edges) and is not moved. The accent label is kept and the fill moves instead. A moved color
// is written with a trailing comment that gives its converted value.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	src := flag.String("src", "../third_party/gpui-component-themes", "directory of the upstream theme sets (JSON)")
	out := flag.String("out", "themes_gen.go", "file to write")
	flag.Parse()
	data, err := render(*src)
	if err == nil {
		err = os.WriteFile(*out, data, 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "themegen:", err)
		os.Exit(1)
	}
}

// Contrast criteria, as in ui/contrast_test.go. margin keeps a color that
// meets a criterion from failing on rounding.
const (
	minText    = 4.5
	minNonText = 3.0
	margin     = 0.01

	lightnessStep = 0.005
	maxSteps      = 500
)

// themeSet is one upstream file: a color scheme and its variants.
type themeSet struct {
	Name   string        `json:"name"`
	Author string        `json:"author"`
	URL    string        `json:"url"`
	Themes []themeConfig `json:"themes"`
}

// themeConfig is one variant. Its editor colors are in Highlight, next to the
// syntax captures, and are decoded from there.
type themeConfig struct {
	Name      string                     `json:"name"`
	Mode      string                     `json:"mode"`
	Colors    map[string]string          `json:"colors"`
	Highlight map[string]json.RawMessage `json:"highlight"`
}

// syntaxStyle is one capture of highlight.syntax. Empty or zero fields are unset.
type syntaxStyle struct {
	Color      string  `json:"color"`
	FontStyle  string  `json:"font_style"`
	FontWeight float64 `json:"font_weight"`
}

// rgba is an 8-bit color with straight alpha.
type rgba struct{ r, g, b, a uint8 }

func (c rgba) hex() string {
	if c.a == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.r, c.g, c.b, c.a)
}

var (
	white = rgba{255, 255, 255, 255}
	black = rgba{0, 0, 0, 255}
	hexRE = regexp.MustCompile(`#[0-9a-fA-F]{6}(?:[0-9a-fA-F]{2})?`)
)

// parseColor returns the color of an upstream value: its first hex color. A
// gradient gives its first stop. A value with no hex color is not usable.
func parseColor(s string) (rgba, bool) {
	m := hexRE.FindString(s)
	if m == "" {
		return rgba{}, false
	}
	var v []uint8
	for i := 1; i+2 <= len(m); i += 2 {
		n, err := strconv.ParseUint(m[i:i+2], 16, 8)
		if err != nil {
			return rgba{}, false
		}
		v = append(v, uint8(n))
	}
	c := rgba{v[0], v[1], v[2], 255}
	if len(v) == 4 {
		c.a = v[3]
	}
	return c, true
}

// palette looks up the upstream keys of one variant. The first usable value
// of a key list wins. A missing color that has no fallback is an error,
// reported once at the end.
type palette struct {
	name   string
	colors map[string]string
	err    error
}

func (p *palette) first(keys ...string) (rgba, bool) {
	for _, k := range keys {
		if s, ok := p.colors[k]; ok {
			if c, ok := parseColor(s); ok {
				return c, true
			}
		}
	}
	return rgba{}, false
}

func (p *palette) must(keys ...string) rgba {
	c, ok := p.first(keys...)
	if !ok && p.err == nil {
		p.err = fmt.Errorf("%s: no usable color for %s", p.name, strings.Join(keys, " or "))
	}
	return c
}

func (p *palette) firstOr(def rgba, keys ...string) rgba {
	if c, ok := p.first(keys...); ok {
		return c
	}
	return def
}

// token is one Theme field (or CodeEditorColors field) of a generated theme. was is
// the converted color before it was moved for contrast, when adjusted is set.
type token struct {
	field    string
	c        rgba
	was      rgba
	adjusted bool
}

func newToken(field string, orig, final rgba) token {
	return token{field: field, c: final, was: orig, adjusted: orig != final}
}

// syntaxToken is one capture of the Syntax map.
type syntaxToken struct {
	name                   string
	c, was                 rgba
	adjusted, italic, bold bool
}

// converted is the output of one variant, ready to emit.
type converted struct {
	name, family, mode string
	tokens             []token // Theme fields, in the order of the Theme struct
	syntax             []syntaxToken
	codeEditor         []token // CodeEditorColors fields
}

// converter moves colors in lightness until they meet a criterion.
type converter struct {
	p *palette
}

// fit steps c in lightness by dir until ok holds, and returns the first color
// that holds. A color that already holds is returned unchanged. what names the
// color in the error when no step holds.
func (cv *converter) fit(what string, c rgba, dir float64, ok func(rgba) bool) rgba {
	out := c
	for i := 0; i < maxSteps && !ok(out); i++ {
		out = stepLightness(out, dir)
	}
	if !ok(out) && cv.p.err == nil {
		cv.p.err = fmt.Errorf("%s: %s cannot reach the contrast criterion", cv.p.name, what)
	}
	return out
}

// fitFill moves a fill in lightness, away from the label that sits on it, until
// the label meets minText on the fill.
func (cv *converter) fitFill(what string, fill, label rgba) rgba {
	dir := 1.0
	if luminance(label) > luminance(fill) {
		dir = -1
	}
	return cv.fit(what, fill, dir, func(c rgba) bool { return ratio(label, c) >= minText+margin })
}

// convertVariant converts one variant. It fails when a color is missing or when
// a criterion cannot be met.
func convertVariant(family string, th themeConfig) (converted, error) {
	if th.Mode != "dark" && th.Mode != "light" {
		return converted{}, fmt.Errorf("%s: mode %q is neither dark nor light", th.Name, th.Mode)
	}
	p := &palette{name: th.Name, colors: merge(th)}
	cv := &converter{p: p}
	dark := th.Mode == "dark"

	bg := p.must("background")
	if bg.a != 255 {
		return converted{}, fmt.Errorf("%s: background is not opaque", th.Name)
	}
	sidebar := opaque(p.firstOr(bg, "sidebar.background", "title_bar.background"), bg)
	surface := opaque(p.firstOr(bg, "popover.background", "list.even.background"), bg)
	surfaceActive, ok := p.first("secondary.active.background")
	if !ok {
		surfaceActive = p.must("muted.background")
	}
	surfaceActive = opaque(surfaceActive, bg)
	surfaceHover := mix(surface, surfaceActive, 0.5)
	if c, ok := p.first("list.hover.background"); ok {
		surfaceHover = opaque(c, surface)
	}

	text := p.must("foreground")
	muted := p.must("muted.foreground")
	onSurfaces := []rgba{bg, sidebar, surface, surfaceHover, surfaceActive}
	okText := func(c rgba) bool {
		for _, s := range onSurfaces {
			if ratio(c, s) < minText+margin {
				return false
			}
		}
		return true
	}
	dirBG := fitDir(bg)
	textFit := cv.fit("Text", text, dirBG, okText)
	mutedFit := cv.fit("TextMuted", muted, dirBG, okText)
	subtleFit := cv.fit("TextSubtle", muted, dirBG, okText)

	// The accent fills move, and the label stays as the palette has it.
	accentText := p.must("primary.foreground")
	accent, ok := p.first("primary.background")
	if !ok {
		accent = p.must("base.blue")
	}
	active, ok := p.first("primary.active.background")
	if !ok {
		active = accent.darken(0.12)
	}
	hover, ok := p.first("primary.hover.background")
	if !ok {
		hover = mix(accent, active, 0.5)
	}
	accentFit := cv.fitFill("Accent", accent, accentText)
	activeFit := cv.fitFill("AccentActive", active, accentText)
	hoverFit := cv.fitFill("AccentHover", hover, accentText)

	accentSoft, ok := p.first("list.active.background")
	if !ok {
		accentSoft = withAlpha(accentFit, 0.15)
	}

	link, ok := p.first("link.foreground", "link.active.foreground", "link")
	if !ok {
		link = accentFit
	}
	linkFit := cv.fit("Link", link, dirBG, okText)

	// Danger is a fill with the label #ffffff, and it is darkened until the
	// label meets the criterion on the fill and on its hover and pressed states.
	danger := p.must("base.red", "danger.background")
	okDanger := func(c rgba) bool {
		return ratio(white, c) >= minText+margin &&
			shiftedRatio(white, c, 0.08, 1) >= minText+margin &&
			shiftedRatio(white, c, 0.1, 0) >= minText+margin
	}
	dangerFit := cv.fit("Danger", danger, -1, okDanger)

	focus, ok := p.first("ring")
	if !ok {
		focus = accentFit
	}
	onBackgrounds := []rgba{bg, sidebar, surface}
	okNonText := func(c rgba) bool {
		for _, s := range onBackgrounds {
			if ratio(c, s) < minNonText+margin {
				return false
			}
		}
		return true
	}
	focusFit := cv.fit("FocusRing", focus, dirBG, okNonText)

	// Border is a divider and the edge of a popup, which are decorative, so it
	// keeps its upstream value. Controls are outlined with BorderStrong.
	border := opaque(p.must("border"), bg)
	borderStrong, ok := p.first("input.border")
	if ok {
		borderStrong = opaque(borderStrong, bg)
	} else {
		borderStrong = mix(border, text, 0.5)
	}
	borderStrongFit := cv.fit("BorderStrong", borderStrong, dirBG, okNonText)

	scrollbar := p.must("scrollbar.thumb.background")

	scrim := rgba{0, 0, 0, 153}
	if !dark {
		scrim = rgba{textFit.r, textFit.g, textFit.b, 89}
	}

	// The editor colors. Its foreground meets the criterion on its background,
	// and the syntax captures meet it on the editor background and the window one.
	edBg := opaque(p.firstOr(bg, "editor.background"), bg)
	edFg := opaque(p.firstOr(textFit, "editor.foreground"), edBg)
	edFgFit := cv.fit("Editor.Foreground", edFg, fitDir(edBg), func(c rgba) bool { return ratio(c, edBg) >= minText+margin })
	activeLine := p.firstOr(surfaceHover, "editor.active_line.background")
	lineNumber := p.firstOr(mutedFit, "editor.line_number")
	activeLineNumber := p.firstOr(edFgFit, "editor.active_line_number")
	selection := p.firstOr(withAlpha(accentFit, 0.32), "selection.background")

	if p.err != nil {
		return converted{}, p.err
	}

	c := converted{name: th.Name, family: family, mode: th.Mode}
	c.tokens = []token{
		newToken("Background", bg, bg),
		newToken("Panel", sidebar, sidebar),
		newToken("Surface", surface, surface),
		newToken("SurfaceHover", surfaceHover, surfaceHover),
		newToken("SurfaceActive", surfaceActive, surfaceActive),
		newToken("Border", border, border),
		newToken("BorderStrong", borderStrong, borderStrongFit),
		newToken("Text", text, textFit),
		newToken("TextMuted", muted, mutedFit),
		newToken("TextSubtle", muted, subtleFit),
		newToken("Accent", accent, accentFit),
		newToken("Link", link, linkFit),
		newToken("Disabled", muted, subtleFit),
		newToken("AccentHover", hover, hoverFit),
		newToken("AccentActive", active, activeFit),
		newToken("AccentText", accentText, accentText),
		newToken("AccentSoft", accentSoft, accentSoft),
	}
	success := p.must("base.green", "success.background")
	warning := p.must("base.yellow", "warning.background")
	c.tokens = append(c.tokens,
		newToken("Success", success, success),
		newToken("Warning", warning, warning),
		newToken("Danger", danger, dangerFit),
		newToken("Scrollbar", scrollbar, scrollbar),
		newToken("FocusRing", focus, focusFit),
		newToken("Scrim", scrim, scrim),
	)
	c.codeEditor = []token{
		newToken("Background", edBg, edBg),
		newToken("Foreground", edFg, edFgFit),
		newToken("ActiveLine", activeLine, activeLine),
		newToken("LineNumber", lineNumber, lineNumber),
		newToken("ActiveLineNumber", activeLineNumber, activeLineNumber),
		newToken("Selection", selection, selection),
	}
	syntax, err := convertSyntax(th, p, cv, bg, edBg, edFgFit)
	if err != nil {
		return converted{}, err
	}
	c.syntax = syntax
	return c, nil
}

// convertSyntax converts the syntax captures, sorted by name. A capture with
// no color takes the editor foreground, as in an editor that has no color for it.
func convertSyntax(th themeConfig, p *palette, cv *converter, bg, edBg, edFg rgba) ([]syntaxToken, error) {
	raw, ok := th.Highlight["syntax"]
	if !ok {
		return nil, fmt.Errorf("%s: no syntax captures", th.Name)
	}
	var caps map[string]syntaxStyle
	if err := json.Unmarshal(raw, &caps); err != nil {
		return nil, fmt.Errorf("%s: syntax: %w", th.Name, err)
	}
	names := make([]string, 0, len(caps))
	for name := range caps {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []syntaxToken
	for _, name := range names {
		s := caps[name]
		orig := edFg
		if s.Color != "" {
			c, ok := parseColor(s.Color)
			if !ok {
				return nil, fmt.Errorf("%s: syntax %s: %q is not a hex color", th.Name, name, s.Color)
			}
			orig = c
		}
		okSyn := func(c rgba) bool {
			return ratio(c, bg) >= minText+margin && ratio(c, edBg) >= minText+margin
		}
		fit := cv.fit("syntax "+name, orig, fitDir(edBg), okSyn)
		out = append(out, syntaxToken{
			name: name, c: fit, was: orig, adjusted: fit != orig,
			italic: s.FontStyle == "italic", bold: s.FontWeight >= 600,
		})
	}
	return out, p.err
}

// merge returns the color keys of a variant, with its editor colors from the
// highlight block. The colors block wins where a key is in both.
func merge(th themeConfig) map[string]string {
	m := make(map[string]string, len(th.Colors)+len(th.Highlight))
	for k, raw := range th.Highlight {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			m[k] = s
		}
	}
	for k, v := range th.Colors {
		m[k] = v
	}
	return m
}

// render returns ui/themes_gen.go for the theme sets in dir.
func render(dir string) ([]byte, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no theme sets in %s", dir)
	}
	var body, table strings.Builder
	seen := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var set themeSet
		if err := json.Unmarshal(data, &set); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		for _, th := range set.Themes {
			c, err := convertVariant(set.Name, th)
			if err != nil {
				return nil, err
			}
			name := varName(th.Name)
			if seen[name] {
				return nil, fmt.Errorf("%s: duplicate theme variable %s", th.Name, name)
			}
			seen[name] = true
			info := fmt.Sprintf("ThemeInfo{Name: %s, Family: %s, Author: %s, URL: %s, Dark: %t}",
				strconv.Quote(th.Name), strconv.Quote(set.Name), strconv.Quote(set.Author), strconv.Quote(set.URL), th.Mode == "dark")
			emitSpec(&body, name, info, c)
			fmt.Fprintf(&table, "\t&%s,\n", name)
		}
	}
	var out strings.Builder
	out.WriteString(header)
	out.WriteString(body.String())
	out.WriteString("// generatedThemes lists the themes converted from GPUI Kit, in the order of their\n")
	out.WriteString("// files and of the variants in each file.\n")
	out.WriteString("var generatedThemes = []*themeSpec{\n")
	out.WriteString(table.String())
	out.WriteString("}\n")
	return format.Source([]byte(out.String()))
}

const header = `// Code generated by internal/themegen from third_party/gpui-component-themes. DO NOT EDIT.

// Regenerate with go generate ./ui. Each theme is a themeSpec (see themes.go): the
// colors it sets over the built-in theme of its mode, as hexadecimal strings. A
// color with a trailing comment was moved in lightness to meet the contrast
// criteria of the generator, and the comment gives its converted value. The
// upstream scheme of each theme is credited in third_party/gpui-component-themes.

package ui

`

// emitSpec writes the themeSpec of one variant.
func emitSpec(b *strings.Builder, name, info string, c converted) {
	kind := "dark"
	if c.mode == "light" {
		kind = "light"
	}
	fmt.Fprintf(b, "// %s is the %s theme, a %s variant of the %s scheme.\n", name, c.name, kind, c.family)
	fmt.Fprintf(b, "var %s = themeSpec{\n", name)
	fmt.Fprintf(b, "\tInfo: %s,\n\n", info)
	for _, tk := range c.tokens {
		writeToken(b, "\t", tk)
	}
	b.WriteString("\n\tSyntax: []syntaxSpec{\n")
	for _, s := range c.syntax {
		style := "plain"
		switch {
		case s.italic && s.bold:
			style = "boldItalic"
		case s.italic:
			style = "italic"
		case s.bold:
			style = "bold"
		}
		fmt.Fprintf(b, "\t\t{%s, %q, %s},", strconv.Quote(s.name), s.c.hex(), style)
		if s.adjusted {
			fmt.Fprintf(b, " // was %s", s.was.hex())
		}
		b.WriteString("\n")
	}
	b.WriteString("\t},\n")
	b.WriteString("\tCodeEditor: codeEditorSpec{\n")
	for _, tk := range c.codeEditor {
		writeToken(b, "\t\t", tk)
	}
	b.WriteString("\t},\n")
	b.WriteString("}\n\n")
}

// writeToken writes one color field of a spec, with the color it was moved from.
func writeToken(b *strings.Builder, indent string, tk token) {
	fmt.Fprintf(b, "%s%s: %q,", indent, tk.field, tk.c.hex())
	if tk.adjusted {
		fmt.Fprintf(b, " // was %s", tk.was.hex())
	}
	b.WriteString("\n")
}

// varName returns the variable name of a variant's spec, such as
// themeCatppuccinMocha for "Catppuccin Mocha".
func varName(name string) string {
	var b strings.Builder
	b.WriteString("theme")
	for _, w := range strings.FieldsFunc(name, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	return b.String()
}

// The color operations below match the ones of package ui, so that a converted
// color meets the criteria in the ui tests.

func lum(r, g, b float64) float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// luminance returns the relative luminance of an opaque color.
func luminance(c rgba) float64 {
	return lum(float64(c.r)/255, float64(c.g)/255, float64(c.b)/255)
}

// ratio returns the WCAG 2 contrast ratio of fg drawn on bg. A translucent fg is
// composited over bg first.
func ratio(fg, bg rgba) float64 {
	a := float64(fg.a) / 255
	r := float64(fg.r)/255*a + float64(bg.r)/255*(1-a)
	g := float64(fg.g)/255*a + float64(bg.g)/255*(1-a)
	b := float64(fg.b)/255*a + float64(bg.b)/255*(1-a)
	l1, l2 := lum(r, g, b), luminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// fitDir returns the direction in which a color on bg gains contrast: lighter on
// a dark background, darker on a light one.
func fitDir(bg rgba) float64 {
	if ratio(white, bg) >= ratio(black, bg) {
		return 1
	}
	return -1
}

// opaque composites c over under, so that the result is opaque.
func opaque(c, under rgba) rgba {
	if c.a == 255 {
		return c
	}
	return mix(under, rgba{c.r, c.g, c.b, 255}, float64(c.a)/255)
}

// mix returns a moved by t toward b, channel by channel, rounded to 8 bits.
func mix(a, b rgba, t float64) rgba {
	f := func(x, y uint8) uint8 { return quant(float64(x) + (float64(y)-float64(x))*t) }
	return rgba{f(a.r, b.r), f(a.g, b.g), f(a.b, b.b), f(a.a, b.a)}
}

func (c rgba) darken(t float64) rgba { return mix(c, rgba{0, 0, 0, c.a}, t) }

// shiftedRatio returns the ratio of the opaque color fg on c mixed toward the
// channel value target (1 for white, 0 for black) by t. It is computed in
// floating point, as Color.Lighten and Color.Darken are, so that the result is
// the one the ui tests compute.
func shiftedRatio(fg, c rgba, t, target float64) float64 {
	shift := func(v uint8) float64 {
		x := float64(v) / 255
		return x + (target-x)*t
	}
	l1, l2 := luminance(fg), lum(shift(c.r), shift(c.g), shift(c.b))
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// withAlpha returns c with its alpha multiplied by f.
func withAlpha(c rgba, f float64) rgba {
	c.a = quant(float64(c.a) * f)
	return c
}

func quant(v float64) uint8 {
	return uint8(math.Round(math.Min(1, math.Max(0, v/255)) * 255))
}

// stepLightness returns c with its HSL lightness moved by dir times lightnessStep.
// The hue and the saturation are kept, and so is the alpha.
func stepLightness(c rgba, dir float64) rgba {
	h, s, l := toHSL(float64(c.r)/255, float64(c.g)/255, float64(c.b)/255)
	l = math.Min(1, math.Max(0, l+dir*lightnessStep))
	r, g, b := fromHSL(h, s, l)
	return rgba{quant(r * 255), quant(g * 255), quant(b * 255), c.a}
}

func toHSL(r, g, b float64) (h, s, l float64) {
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

func fromHSL(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		return l, l, l
	}
	q := l + s - l*s
	if l < 0.5 {
		q = l * (1 + s)
	}
	p := 2*l - q
	return hueToRGB(p, q, h+1.0/3), hueToRGB(p, q, h), hueToRGB(p, q, h-1.0/3)
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}
