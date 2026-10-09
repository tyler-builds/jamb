package views

import "github.com/tyler-builds/jamb/ui"

// matchHighlight is how a search match looks: strong enough to find at a
// glance on both themes (AccentSoft was too faint on the dark one). It is the
// same as the notes app's.
func matchHighlight(t *ui.Theme) ui.Highlight {
	return ui.Highlight{Background: t.Accent.Alpha(0.4), Color: t.Text}
}
