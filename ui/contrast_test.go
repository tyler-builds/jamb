package ui

import (
	"math"
	"testing"
)

// The built-in themes are checked against the WCAG 2 contrast criteria for
// the color pairs that the built-in widgets draw. Translucent colors are
// composited source-over onto the color they sit on before the ratio is
// computed. The criteria are:
//
//   - 1.4.3 Contrast (Minimum), normal text 4.5:1
//     https://www.w3.org/TR/WCAG21/#contrast-minimum
//   - 1.4.11 Non-text Contrast, UI components and graphical objects 3:1
//     https://www.w3.org/TR/WCAG21/#non-text-contrast
//
// The default font size is 14, so every text pair uses 4.5:1, including
// subtle and muted text. Disabled controls are exempt from 1.4.3, so the
// disabled pair is recorded only. Pairs with min 0 are recorded only.
const (
	minText    = 4.5
	minNonText = 3.0
)

// contrastPair is one foreground and background pair. A pair with a gap set
// is a known shortfall: its ratio is logged, not enforced, and the test fails
// if the gap starts to pass so the entry gets removed.
type contrastPair struct {
	name string
	role string
	fg   Color
	bg   Color
	min  float64
	gap  string
}

func TestContrast(t *testing.T) {
	for _, th := range []*Theme{DarkTheme(), LightTheme()} {
		t.Run(th.Name, func(t *testing.T) {
			for _, p := range contrastPairs(th) {
				ratio := contrastRatio(p.fg, p.bg)
				switch {
				case p.min == 0:
					t.Logf("%-30s %-62s %6.2f  recorded only", p.name, p.role, ratio)
				case p.gap != "":
					t.Logf("%-30s %-62s %6.2f  known gap (min %.1f): %s", p.name, p.role, ratio, p.min, p.gap)
					if ratio >= p.min {
						t.Errorf("%s: known gap now passes at %.2f; remove it from contrastPairs", p.name, ratio)
					}
				default:
					t.Run(p.name, func(t *testing.T) {
						t.Logf("%s (%s): %.2f, min %.1f", p.name, p.role, ratio, p.min)
						if ratio < p.min {
							t.Errorf("%s: contrast %.2f is below %.1f", p.name, ratio, p.min)
						}
					})
				}
			}
		})
	}
}

// contrastPairs lists the pairs used by the built-in widgets for one theme.
// Where a widget is named, it is the code that draws the pair.
func contrastPairs(th *Theme) []contrastPair {
	surfaces := []struct {
		name string
		c    Color
	}{
		{"Background", th.Background},
		{"Panel", th.Panel},
		{"Surface", th.Surface},
		{"SurfaceHover", th.SurfaceHover},
		{"SurfaceActive", th.SurfaceActive},
	}
	var ps []contrastPair
	for _, s := range surfaces {
		ps = append(ps,
			contrastPair{name: "Text_on_" + s.name, role: "body text (Text)", fg: th.Text, bg: s.c, min: minText},
			contrastPair{name: "TextMuted_on_" + s.name, role: "muted text: ghost button, window buttons, icons", fg: th.TextMuted, bg: s.c, min: minText},
			contrastPair{name: "TextSubtle_on_" + s.name, role: "subtle text: hints, metadata, menu shortcuts", fg: th.TextSubtle, bg: s.c, min: minText},
			contrastPair{name: "Link_on_" + s.name, role: "link text (Link, richtext.go)", fg: th.Link, bg: s.c, min: minText},
		)
	}
	ps = append(ps,

		// editor.go draws the placeholder in TextSubtle on the field's Surface.
		contrastPair{name: "Placeholder_on_Surface", role: "TextInput placeholder (TextSubtle)", fg: th.TextSubtle, bg: th.Surface, min: minText},

		// widgets.go: Opacity(0.5) is applied to the whole disabled control.
		contrastPair{name: "Disabled_Text_on_Surface", role: "disabled text: Opacity 0.5, exempt from 1.4.3", fg: th.Text.Alpha(0.5), bg: th.Surface},

		// widgets.go: Primary button label is AccentText on each fill state.
		contrastPair{name: "AccentText_on_Accent", role: "Primary button label, rest", fg: th.AccentText, bg: th.Accent, min: minText},
		contrastPair{name: "AccentText_on_AccentHover", role: "Primary button label, hover", fg: th.AccentText, bg: th.AccentHover, min: minText},
		contrastPair{name: "AccentText_on_AccentActive", role: "Primary button label, pressed", fg: th.AccentText, bg: th.AccentActive, min: minText},

		// widgets.go: Danger button fills with Danger and a hard-coded #ffffff label.
		contrastPair{name: "Label_on_Danger", role: "Danger button label (#ffffff), rest", fg: Hex("#ffffff"), bg: th.Danger, min: minText},
		contrastPair{name: "Label_on_DangerHover", role: "Danger button label (#ffffff), hover", fg: Hex("#ffffff"), bg: th.Danger.Lighten(0.08), min: minText},
		contrastPair{name: "Label_on_DangerPressed", role: "Danger button label (#ffffff), pressed", fg: Hex("#ffffff"), bg: th.Danger.Darken(0.1), min: minText},

		// menu.go: a highlighted row is drawn on AccentSoft over the menu Surface.
		contrastPair{name: "Text_on_AccentSoft", role: "highlighted menu row (AccentSoft over Surface)", fg: th.Text, bg: over(th.AccentSoft, th.Surface), min: minText},
		// editor.go: a selection is Accent at 0.32 alpha over the field's Surface.
		contrastPair{name: "Text_on_Selection", role: "editor selection (Accent at 0.32 over Surface)", fg: th.Text, bg: over(th.Accent.Alpha(0.32), th.Surface), min: minText},

		// Danger, Success and Warning are not used as text by the built-ins.
		contrastPair{name: "Danger_text_on_Surface", role: "status text, not used as text by built-ins", fg: th.Danger, bg: th.Surface},
		contrastPair{name: "Success_text_on_Surface", role: "status text, not used as text by built-ins", fg: th.Success, bg: th.Surface},
		contrastPair{name: "Warning_text_on_Surface", role: "status text, not used as text by built-ins", fg: th.Warning, bg: th.Surface},
	)
	ps = append(ps, badgePairs(th)...)
	ps = append(ps, progressPairs(th)...)
	for _, s := range surfaces[:3] {
		ps = append(ps,
			// widgets.go: the focus ring is a spread shadow in FocusRing.
			contrastPair{name: "FocusRing_on_" + s.name, role: "focus ring (FocusRing spread shadow)", fg: th.FocusRing, bg: s.c, min: minNonText},
			// widgets.go: BorderStrong is the hover border of inputs and secondary buttons.
			contrastPair{name: "BorderStrong_on_" + s.name, role: "control border at rest (BorderStrong)", fg: th.BorderStrong, bg: s.c, min: minNonText},
			contrastPair{name: "TextMuted_border_on_" + s.name, role: "control border on hover (TextMuted)", fg: th.TextMuted, bg: s.c, min: minNonText},
		)
	}
	ps = append(ps,
		// Dividers and the edges of menus and tooltips use Border. They are
		// decorative, so no ratio is required; controls use BorderStrong.
		contrastPair{name: "Border_on_Background", role: "divider line (Border), decorative", fg: th.Border, bg: th.Background},
		// scrollbar.go: the thumb is drawn at alpha 0.5 at rest, 0.6 hovered.
		contrastPair{name: "ScrollThumb_on_Surface", role: "scrollbar thumb at rest (Scrollbar at 0.5)", fg: th.Scrollbar.Alpha(0.5), bg: th.Surface, min: minNonText},
		contrastPair{name: "ScrollThumbHover_on_Surface", role: "scrollbar thumb hovered (Scrollbar at 0.6)", fg: th.Scrollbar.Alpha(0.6), bg: th.Surface},
		// modal.go: the Scrim is drawn over the window and the dialog is on Surface.
		contrastPair{name: "Text_through_Scrim", role: "window Background as seen through Scrim, against Text", fg: th.Text, bg: over(th.Scrim, th.Background)},
	)
	return ps
}

// contrastRatio returns the WCAG 2 contrast ratio of fg drawn on bg. A
// translucent fg is composited over bg first. bg must be opaque.
func contrastRatio(fg, bg Color) float64 {
	l1, l2 := relLuminance(over(fg, bg)), relLuminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// over composites top over below (source-over). The result is opaque.
func over(top, below Color) Color {
	a := top.A
	return Color{
		R: top.R*a + below.R*(1-a),
		G: top.G*a + below.G*(1-a),
		B: top.B*a + below.B*(1-a),
		A: 1,
	}
}

// relLuminance returns the WCAG 2 relative luminance of an opaque color.
// See https://www.w3.org/TR/WCAG21/#dfn-relative-luminance
func relLuminance(c Color) float64 {
	lin := func(v float32) float64 {
		x := float64(v)
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// badgePairs lists the text and fill of each badge variant (badge.go). Badge
// text is 11 px, so every pair uses 4.5:1.
func badgePairs(th *Theme) []contrastPair {
	var ps []contrastPair
	for _, v := range []struct {
		name string
		v    BadgeVariant
	}{
		{"Neutral", BadgeNeutral}, {"Accent", BadgeAccent}, {"Success", BadgeSuccess},
		{"Warning", BadgeWarning}, {"Danger", BadgeDanger},
	} {
		fill, ink := badgeColors(th, v.v)
		ps = append(ps, contrastPair{name: "Badge_" + v.name, role: "badge text (badge.go badgeColors) on its fill", fg: ink, bg: fill, min: minText})
	}
	return ps
}

// TestBadgeContrastHighContrast checks the badge pairs under the two
// representative high contrast schemes. A badge is outlined there, so its text
// is the text color on the surface.
func TestBadgeContrastHighContrast(t *testing.T) {
	for _, sc := range []struct {
		name string
		c    SystemColors
	}{{"black", HighContrastBlack()}, {"white", HighContrastWhite()}} {
		th := HighContrastTheme(sc.c, DarkTheme())
		for _, p := range append(badgePairs(th), progressPairs(th)...) {
			r := contrastRatio(p.fg, p.bg)
			t.Logf("high contrast %s: %s %.2f", sc.name, p.name, r)
			if r < p.min {
				t.Errorf("high contrast %s: %s is %.2f, below %.1f", sc.name, p.name, r, p.min)
			}
		}
	}
}

// progressPairs lists the fill of a determinate progress bar against its track
// (progress.go). The fill is a graphical object, so it needs 3:1. Under high
// contrast the fill is Highlight on an outlined Surface track.
func progressPairs(th *Theme) []contrastPair {
	track, fill := th.SurfaceActive, th.Accent
	if th.HighContrast {
		track = th.Surface
	}
	return []contrastPair{
		{name: "ProgressFill_on_Track", role: "determinate bar fill (Accent) on its track (SurfaceActive)", fg: fill, bg: track, min: minNonText},
	}
}
