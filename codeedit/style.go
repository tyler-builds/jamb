package codeedit

import "github.com/tyler-builds/jamb/ui"

// Style is the look of the parts of an Editor. A part's style merges over the built-in
// defaults, which come from the colors of the theme (Theme.Editor and Theme.Syntax), and
// then over the Styles of the editor. Box is the frame of the editor: its Background,
// border, radius and padding. Text is the text: its TextColor and FontSize. Gutter is the
// column of line numbers and fold markers: its TextColor, FontSize and Background.
// ActiveLine is the row of the caret, Selection the selected text, Match the other find
// matches and CurrentMatch the current one; their Background is the fill. Find is the box
// of the find and replace bar.
//
// Under high contrast the colors of these parts are the system colors, whatever is set
// here; their sizes and spacing still apply.
type Style struct {
	Box          ui.Style
	Text         ui.Style
	Gutter       ui.Style
	ActiveLine   ui.Style
	Selection    ui.Style
	Match        ui.Style
	CurrentMatch ui.Style
	Find         ui.Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s Style) merge(o Style) Style {
	return Style{
		Box:          s.Box.Merge(o.Box),
		Text:         s.Text.Merge(o.Text),
		Gutter:       s.Gutter.Merge(o.Gutter),
		ActiveLine:   s.ActiveLine.Merge(o.ActiveLine),
		Selection:    s.Selection.Merge(o.Selection),
		Match:        s.Match.Merge(o.Match),
		CurrentMatch: s.CurrentMatch.Merge(o.CurrentMatch),
		Find:         s.Find.Merge(o.Find),
	}
}

// plainPart returns s without the colors, gradients and shadows, which high contrast takes
// from the system colors.
func plainPart(s ui.Style) ui.Style {
	s.Background, s.BorderColor, s.TextColor = ui.Color{}, ui.Color{}, ui.Color{}
	s.Gradient, s.Shadows, s.Opacity = ui.Gradient{}, nil, 0
	return s
}

// editorDefaults returns the built-in look of the parts, from the colors of t.
func editorDefaults(t *ui.Theme) Style {
	ec := t.Editor
	return Style{
		Box:          ui.Style{Background: ec.Background, BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius},
		Text:         ui.Style{TextColor: ec.Foreground, FontSize: t.FontSize},
		Gutter:       ui.Style{TextColor: ec.LineNumber, FontSize: t.FontSize, Background: ec.Background},
		ActiveLine:   ui.Style{Background: ec.ActiveLine},
		Selection:    ui.Style{Background: ec.Selection},
		Match:        ui.Style{Background: t.Accent.Alpha(0.25)},
		CurrentMatch: ui.Style{Background: t.Accent.Alpha(0.55)},
		Find:         ui.Style{Background: t.Surface, BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius, Gap: 6, Padding: ui.All(6)},
	}
}

// resolve returns the look of the parts in the theme t: the defaults, merged with the
// editor's Styles. Under high contrast the colors are the system colors, which the theme
// holds, so the Styles' colors are dropped.
func (s Style) resolve(t *ui.Theme) Style {
	st := editorDefaults(t).merge(s)
	if t.HighContrast {
		st = editorDefaults(t).merge(Style{
			Box:          plainPart(s.Box),
			Text:         plainPart(s.Text),
			Gutter:       plainPart(s.Gutter),
			ActiveLine:   plainPart(s.ActiveLine),
			Selection:    plainPart(s.Selection),
			Match:        plainPart(s.Match),
			CurrentMatch: plainPart(s.CurrentMatch),
			Find:         plainPart(s.Find),
		})
		st.Text.TextColor = t.Editor.Foreground
		st.Gutter.TextColor = t.Editor.LineNumber
	}
	return st
}
