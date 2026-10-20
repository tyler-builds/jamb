package codeedit

import "github.com/tyler-builds/jamb/ui"

// Style is the look of the parts of an Editor. A part's style merges over the built-in
// defaults, which come from the colors of the theme (Theme.CodeEditor and Theme.Syntax), then
// over the Style that the theme holds for every editor (set with ui.SetThemeStyle), and then
// over the Styles of the editor. Box is the frame of the editor: its Background,
// border, radius and padding. BoxFocused is the frame while the editor has focus. By default
// it is unset, as in code editors, where the caret shows focus: the frame does not change.
// Under high contrast it is a border of the Highlight color, so focus shows in the frame as
// well.
// Text is the text: its TextColor and FontSize. Gutter is the
// column of line numbers and fold markers: its TextColor, FontSize and Background.
// ActiveLine is the row of the caret, Selection the selected text, Match the other find
// matches and CurrentMatch the current one; their Background is the fill. Find is the box
// of the find and replace bar. DiagnosticError, DiagnosticWarning, DiagnosticInfo and
// DiagnosticHint are the marks of the diagnostics of each severity: their TextColor colors the
// wavy underline and the gutter marker, which the theme's Danger, Warning, Accent and TextMuted
// colors are by default.
//
// Under high contrast the colors of these parts are the system colors, whatever is set
// here or in the theme; their sizes and spacing still apply.
type Style struct {
	// Box is the frame of the editor and its background.
	Box ui.Style
	// BoxFocused is the frame while the editor has keyboard focus.
	BoxFocused ui.Style
	// Text is the look of the text itself.
	Text ui.Style
	// Gutter is the column of line numbers and fold markers.
	Gutter ui.Style
	// ActiveLine is the look of the line that holds the caret.
	ActiveLine ui.Style
	// Selection is the look of the selected text.
	Selection ui.Style
	// Match is the look of each find match.
	Match ui.Style
	// CurrentMatch is the look of the find match that is current.
	CurrentMatch ui.Style
	// Find is the look of the find bar.
	Find ui.Style
	// DiagnosticError is the look of the marks of an error: its TextColor.
	DiagnosticError ui.Style
	// DiagnosticWarning is the look of the marks of a warning: its TextColor.
	DiagnosticWarning ui.Style
	// DiagnosticInfo is the look of the marks of an information diagnostic: its TextColor.
	DiagnosticInfo ui.Style
	// DiagnosticHint is the look of the marks of a hint: its TextColor.
	DiagnosticHint ui.Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s Style) merge(o Style) Style {
	return Style{
		Box:          s.Box.Merge(o.Box),
		BoxFocused:   s.BoxFocused.Merge(o.BoxFocused),
		Text:         s.Text.Merge(o.Text),
		Gutter:       s.Gutter.Merge(o.Gutter),
		ActiveLine:   s.ActiveLine.Merge(o.ActiveLine),
		Selection:    s.Selection.Merge(o.Selection),
		Match:        s.Match.Merge(o.Match),
		CurrentMatch: s.CurrentMatch.Merge(o.CurrentMatch),
		Find:         s.Find.Merge(o.Find),

		DiagnosticError:   s.DiagnosticError.Merge(o.DiagnosticError),
		DiagnosticWarning: s.DiagnosticWarning.Merge(o.DiagnosticWarning),
		DiagnosticInfo:    s.DiagnosticInfo.Merge(o.DiagnosticInfo),
		DiagnosticHint:    s.DiagnosticHint.Merge(o.DiagnosticHint),
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
	ec := t.CodeEditor
	var focused ui.Style // the caret shows focus; the frame does not change
	if t.HighContrast {
		focused = ui.Style{BorderColor: t.Accent} // the Highlight color of the system
	}
	return Style{
		Box:          ui.Style{Background: ec.Background, BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius},
		BoxFocused:   focused,
		Text:         ui.Style{TextColor: ec.Foreground, FontSize: t.FontSize},
		Gutter:       ui.Style{TextColor: ec.LineNumber, FontSize: t.FontSize, Background: ec.Background},
		ActiveLine:   ui.Style{Background: ec.ActiveLine},
		Selection:    ui.Style{Background: ec.Selection},
		Match:        ui.Style{Background: t.Accent.Alpha(0.25)},
		CurrentMatch: ui.Style{Background: t.Accent.Alpha(0.55)},
		Find:         ui.Style{Background: t.Surface, BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius, Gap: 6, Padding: ui.All(6)},

		DiagnosticError:   ui.Style{TextColor: t.Danger},
		DiagnosticWarning: ui.Style{TextColor: t.Warning},
		DiagnosticInfo:    ui.Style{TextColor: t.Accent},
		DiagnosticHint:    ui.Style{TextColor: t.TextMuted},
	}
}

// plain returns s with each part without its colors (see plainPart).
func (s Style) plain() Style {
	return Style{
		Box:          plainPart(s.Box),
		BoxFocused:   plainPart(s.BoxFocused),
		Text:         plainPart(s.Text),
		Gutter:       plainPart(s.Gutter),
		ActiveLine:   plainPart(s.ActiveLine),
		Selection:    plainPart(s.Selection),
		Match:        plainPart(s.Match),
		CurrentMatch: plainPart(s.CurrentMatch),
		Find:         plainPart(s.Find),

		DiagnosticError:   plainPart(s.DiagnosticError),
		DiagnosticWarning: plainPart(s.DiagnosticWarning),
		DiagnosticInfo:    plainPart(s.DiagnosticInfo),
		DiagnosticHint:    plainPart(s.DiagnosticHint),
	}
}

// resolve returns the look of the parts in the theme t: the defaults, merged with the
// theme's codeedit.Style (see ui.ThemeStyle), then with the editor's Styles. Under high
// contrast the colors are the system colors, which the theme holds, so the colors of the
// theme's style and of the Styles are dropped.
func (s Style) resolve(ctx *ui.Context) Style {
	t := ctx.Theme()
	themed, _ := ui.ThemeStyle[Style](t)
	if t.HighContrast {
		themed, s = themed.plain(), s.plain()
	}
	st := ui.StyleLayer(ctx, editorDefaults(t), "built-in default").
		merge(ui.StyleLayer(ctx, themed, "theme")).
		merge(ui.StyleLayer(ctx, s, "instance Styles"))
	if t.HighContrast {
		st.Text = st.Text.Merge(ui.StyleLayer(ctx, ui.Style{TextColor: t.CodeEditor.Foreground}, "high contrast"))
		st.Gutter = st.Gutter.Merge(ui.StyleLayer(ctx, ui.Style{TextColor: t.CodeEditor.LineNumber}, "high contrast"))
		// The marks of the diagnostics take the system colors that the theme holds; the gutter
		// markers differ in shape, so the severities do not rest on color.
		hc := func(c ui.Color) ui.Style { return ui.StyleLayer(ctx, ui.Style{TextColor: c}, "high contrast") }
		st.DiagnosticError = st.DiagnosticError.Merge(hc(t.Danger))
		st.DiagnosticWarning = st.DiagnosticWarning.Merge(hc(t.Warning))
		st.DiagnosticInfo = st.DiagnosticInfo.Merge(hc(t.Accent))
		st.DiagnosticHint = st.DiagnosticHint.Merge(hc(t.TextMuted))
	}
	return st
}
