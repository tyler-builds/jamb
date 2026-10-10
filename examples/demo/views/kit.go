package views

import (
	"time"

	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/ui"
)

// Small reusable pieces built only from public framework APIs.

const fast = 120 * time.Millisecond

// Card is the standard surface for grouped content.
func Card(t *ui.Theme) *ui.DivElement {
	return ui.Div().Padding(18).Gap(12).Radius(12).
		Background(t.Surface).Border(1, t.Border).Shadow(ui.ShadowSm)
}

// SectionTitle is a small heading above a group of content.
func SectionTitle(t *ui.Theme, title string) *ui.DivElement {
	return ui.Div().Heading(2).Child(ui.Text(title).Size(15).Semibold().Color(t.Text))
}

// settingLabel is the key of a setting's title. The setting's control is
// labelled by the title, so that a switch reads as "Dark mode, switch".
func settingLabel(title string) string { return "setting:" + title }

// fieldLabel is the key of a form field's label above it.
func fieldLabel(label string) string { return "field:" + label }

// statusBadge is the badge that names a project's status.
func statusBadge(s Status) *ui.BadgeElement {
	return ui.BadgeText(s.String()).Variant(statusVariant(s)).Dot()
}

// statusVariant maps a project status to a badge variant.
func statusVariant(s Status) ui.BadgeVariant {
	switch s {
	case StatusPaused:
		return ui.BadgeWarning
	case StatusDone:
		return ui.BadgeSuccess
	}
	return ui.BadgeAccent
}

// StatusColor maps a project status to a theme color.
func StatusColor(t *ui.Theme, s Status) ui.Color {
	switch s {
	case StatusPaused:
		return t.Warning
	case StatusDone:
		return t.Success
	}
	return t.Accent.Lighten(0.25)
}

// IconTile is a rounded square with a tinted icon.
func IconTile(t *ui.Theme, ic *icons.Icon, c ui.Color, size float32) *ui.DivElement {
	return ui.Div().Size(ui.Px(size), ui.Px(size)).Shrink(0).Radius(size / 4).
		Background(tintOf(t, c, 0.15)).Center().
		Child(ui.Icon(ic).Size(size / 2).Color(c))
}

// SettingRow is a label/description pair with a control on the right.
func SettingRow(t *ui.Theme, title, desc string, control ui.Element) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(16).PaddingY(4).
		Child(ui.Div().Grow(1).Gap(3).
			Child(ui.Text(title).Medium().Key(settingLabel(title))).
			Child(ui.Text(desc).Size(13).Color(t.TextMuted))).
		Child(control)
}

// tintOf is the fill of a tinted chip: the color at alpha a. High contrast draws
// no tints, so the chip takes the surface instead.
func tintOf(t *ui.Theme, c ui.Color, a float32) ui.Color {
	if t.HighContrast {
		return t.Surface
	}
	return c.Alpha(a)
}

// barFill is the fill of a chart bar. High contrast draws it solid, so that the
// bar keeps its contrast against the surface.
func barFill(t *ui.Theme, c ui.Color) ui.Color {
	if t.HighContrast {
		return c
	}
	return c.Alpha(0.85)
}

// starIcon is the star of a starred item. High contrast draws the star filled
// when it is on, since the color of the star is the text color there.
func starIcon(t *ui.Theme, on bool) *icons.Icon {
	if on && t.HighContrast {
		return icons.StarFilled
	}
	return icons.Star
}

// progressColor is a project's color as the fill of its progress bar. The
// palette is light, and in the light theme its fills fall below 3:1 against the
// track, so they are darkened there by 0.35, which clears 3:1 for every color in
// the palette (see ui.ProgressBar.Color, where the contrast is the app's).
func progressColor(t *ui.Theme, c ui.Color) ui.Color {
	if t.Dark || t.HighContrast {
		return c
	}
	return c.Darken(0.35)
}
