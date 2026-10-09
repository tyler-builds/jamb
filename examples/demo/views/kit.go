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
func SectionTitle(t *ui.Theme, title string) *ui.TextElement {
	return ui.Text(title).Size(15).Semibold().Color(t.Text)
}

// Badge is a small pill with a dot and a label.
func Badge(label string, c ui.Color) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(6).PaddingX(8).Height(ui.Px(22)).Radius(11).
		Background(c.Alpha(0.14)).Shrink(0).
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
	return ui.Div().Height(ui.Px(6)).Radius(3).Background(t.SurfaceActive).Grow(1).ClipContent().
		Child(ui.Div().Height(ui.Full).Width(ui.Percent(v * 100)).Radius(3).Background(c))
}

// IconTile is a rounded square with a tinted icon.
func IconTile(ic *icons.Icon, c ui.Color, size float32) *ui.DivElement {
	return ui.Div().Size(ui.Px(size), ui.Px(size)).Shrink(0).Radius(size / 4).
		Background(c.Alpha(0.15)).Center().
		Child(ui.Icon(ic).Size(size / 2).Color(c))
}

// Toggle is an on/off switch bound to a bool signal.
func Toggle(t *ui.Theme, on bool, onChange func(bool)) *ui.DivElement {
	knobX := float32(3)
	track := t.SurfaceActive
	if on {
		knobX = 19
		track = t.Accent
	}
	return ui.Div().Size(ui.Px(38), ui.Px(22)).Shrink(0).Radius(11).Background(track).
		Transition(fast).Hover(ui.Style{Background: track.Lighten(0.08)}).
		OnClick(func() { onChange(!on) }).
		Child(ui.Div().Absolute().Left(knobX).Top(3).Size(ui.Px(16), ui.Px(16)).Radius(8).
			Background(ui.Hex("#ffffff")).Shadow(ui.ShadowSm))
}

// SettingRow is a label/description pair with a control on the right.
func SettingRow(t *ui.Theme, title, desc string, control ui.Element) ui.Element {
	return ui.Div().Row().Align(ui.Center).Gap(16).PaddingY(4).
		Child(ui.Div().Grow(1).Gap(3).
			Child(ui.Text(title).Medium()).
			Child(ui.Text(desc).Size(13).Color(t.TextMuted))).
		Child(control)
}
