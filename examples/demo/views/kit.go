package views

import (
	"time"

	"github.com/tyler-builds/jamb/access"
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

// Badge is a small pill with a dot and a label.
func Badge(t *ui.Theme, label string, c ui.Color) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(6).PaddingX(8).Height(ui.Px(22)).Radius(11).
		Background(tintOf(t, c, 0.14)).Shrink(0).
		Child(ui.Div().Size(ui.Px(6), ui.Px(6)).Radius(3).Background(c)).
		Child(ui.Text(label).Size(12).Medium().Color(c).NoWrap())
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

// ProgressBar draws a thin track with a filled portion.
func ProgressBar(t *ui.Theme, v float32, c ui.Color) *ui.DivElement {
	if t.HighContrast {
		// The track is an outlined empty box and the fill is solid Highlight, so the
		// empty part does not read as filled.
		return ui.Div().Height(ui.Px(8)).Radius(3).Background(t.Surface).Border(1, t.Text).Grow(1).ClipContent().
			Child(ui.Div().Height(ui.Full).Width(ui.Percent(v * 100)).Background(t.Accent))
	}
	return ui.Div().Height(ui.Px(6)).Radius(3).Background(t.SurfaceActive).Grow(1).ClipContent().
		Child(ui.Div().Height(ui.Full).Width(ui.Percent(v * 100)).Radius(3).Background(c))
}

// IconTile is a rounded square with a tinted icon.
func IconTile(t *ui.Theme, ic *icons.Icon, c ui.Color, size float32) *ui.DivElement {
	return ui.Div().Size(ui.Px(size), ui.Px(size)).Shrink(0).Radius(size / 4).
		Background(tintOf(t, c, 0.15)).Center().
		Child(ui.Icon(ic).Size(size / 2).Color(c))
}

// Toggle is an on/off switch bound to a bool signal.
func Toggle(t *ui.Theme, on bool, onChange func(bool)) *ui.DivElement {
	if t.HighContrast {
		// On is a solid Highlight track with an AccentText knob. Off is an outlined
		// empty track with a text-colored knob, so the two states differ in color
		// and in outline, not only in the knob's place.
		track, knob, knobX := t.Surface, t.Text, float32(3)
		if on {
			track, knob, knobX = t.Accent, t.AccentText, 19
		}
		return ui.Div().Size(ui.Px(38), ui.Px(22)).Shrink(0).Radius(11).Background(track).Border(2, t.Text).
			Role(access.RoleSwitch).Checked(on).
			OnClick(func() { onChange(!on) }).
			Child(ui.Div().Absolute().Left(knobX).Top(3).Size(ui.Px(16), ui.Px(16)).Radius(8).Background(knob))
	}
	knobX := float32(3)
	track := t.SurfaceActive
	if on {
		knobX = 19
		track = t.Accent
	}
	return ui.Div().Size(ui.Px(38), ui.Px(22)).Shrink(0).Radius(11).Background(track).
		Transition(fast).Hover(ui.Style{Background: track.Lighten(0.08)}).
		Role(access.RoleSwitch).Checked(on).
		OnClick(func() { onChange(!on) }).
		Child(ui.Div().Absolute().Left(knobX).Top(3).Size(ui.Px(16), ui.Px(16)).Radius(8).
			Background(ui.Hex("#ffffff")).Shadow(ui.ShadowSm))
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
