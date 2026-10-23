package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// clickableCard is a focusable Div with an OnClick and the given extra options.
func clickableCard(opts ...func(*ui.DivElement) *ui.DivElement) func(*ui.Context) ui.Element {
	return func(ctx *ui.Context) ui.Element {
		d := ui.Div().Key("card").Size(ui.Px(100), ui.Px(40)).Focusable().OnClick(func() {})
		for _, o := range opts {
			d = o(d)
		}
		return ui.Div().Column().Child(d)
	}
}

// A focusable Div with an OnClick shows the standard focus ring while it has keyboard
// focus, in the theme's focus color, as the built-in controls do.
func TestClickableDivShowsDefaultFocusRing(t *testing.T) {
	u := uitest.New(t, clickableCard())
	u.Press("tab")
	if !u.Get("card").FocusRing() {
		t.Fatal("no focus ring around a focusable Div with OnClick after Tab")
	}
	if got, want := u.Get("card").Style().Ring, ui.DarkTheme().FocusRing; got != want {
		t.Errorf("ring color %v, want the theme's FocusRing %v", got, want)
	}
}

// A click gives focus without keyboard focus, so the ring is not drawn.
func TestClickableDivRingIsNotShownAfterClick(t *testing.T) {
	u := uitest.New(t, clickableCard())
	u.Get("card").Click()
	if u.Get("card").FocusRing() {
		t.Error("focus ring after a click, which is not keyboard focus")
	}
}

// A FocusVisible style of the element's own replaces the default ring.
func TestClickableDivOwnFocusVisibleReplacesDefaultRing(t *testing.T) {
	u := uitest.New(t, clickableCard(func(d *ui.DivElement) *ui.DivElement {
		return d.FocusVisible(ui.Style{BorderWidth: 2, BorderColor: ui.Hex("#ff0000")})
	}))
	u.Press("tab")
	if u.Get("card").FocusRing() {
		t.Error("the default ring is drawn although the Div sets its own FocusVisible style")
	}
}

// ClearShadows on the element removes the default ring along with its shadows.
func TestClickableDivClearShadowsRemovesDefaultRing(t *testing.T) {
	u := uitest.New(t, clickableCard(func(d *ui.DivElement) *ui.DivElement {
		return d.Style(ui.Style{Clear: ui.ClearShadows})
	}))
	u.Press("tab")
	if u.Get("card").FocusRing() {
		t.Error("the default ring is drawn under ClearShadows")
	}
}

// Under high contrast the ring is the high contrast focus color, like other controls.
func TestClickableDivRingUnderHighContrast(t *testing.T) {
	colors := ui.HighContrastBlack()
	u := uitest.New(t, clickableCard(), uitest.Settings(ui.SystemSettings{HighContrast: true, Colors: colors}))
	u.Press("tab")
	if got := u.Get("card").Style().Ring; got != colors.Highlight {
		t.Errorf("high contrast ring color %v, want Highlight %v", got, colors.Highlight)
	}
}
