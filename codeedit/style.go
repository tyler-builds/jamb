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
// colors are by default. Popup is the panel of the hover and of the completions, PopupItem one
// entry of it, and PopupItemSelected the completion that Enter accepts. Lightbulb is the mark
// of a line that has a code action, its TextColor; PopupItemDisabled is a code action that is
// disabled, and PopupGroup the header of a group in the menu of code actions, each by its TextColor.
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
	// Popup is the panel of the hover and of the list of completions.
	Popup ui.Style
	// PopupItem is one entry of a popup: a message of the hover, or a completion.
	PopupItem ui.Style
	// PopupItemSelected is the completion that Enter accepts.
	PopupItemSelected ui.Style
	// Lightbulb is the mark of a line that has a code action: its TextColor.
	Lightbulb ui.Style
	// PopupItemDisabled is the look of a code action that is disabled: its TextColor.
	PopupItemDisabled ui.Style
	// PopupGroup is the look of the header of a group in the menu of code actions: its TextColor.
	PopupGroup ui.Style
}

// mergeFrom applies the set fields of o on top of s, part by part, in place. With
// plain, o's parts go without their colors (see mergePart).
func (s *Style) mergeFrom(o *Style, plain bool) {
	mergePart(&s.Box, &o.Box, plain)
	mergePart(&s.BoxFocused, &o.BoxFocused, plain)
	mergePart(&s.Text, &o.Text, plain)
	mergePart(&s.Gutter, &o.Gutter, plain)
	mergePart(&s.ActiveLine, &o.ActiveLine, plain)
	mergePart(&s.Selection, &o.Selection, plain)
	mergePart(&s.Match, &o.Match, plain)
	mergePart(&s.CurrentMatch, &o.CurrentMatch, plain)
	mergePart(&s.Find, &o.Find, plain)
	mergePart(&s.DiagnosticError, &o.DiagnosticError, plain)
	mergePart(&s.DiagnosticWarning, &o.DiagnosticWarning, plain)
	mergePart(&s.DiagnosticInfo, &o.DiagnosticInfo, plain)
	mergePart(&s.DiagnosticHint, &o.DiagnosticHint, plain)
	mergePart(&s.Popup, &o.Popup, plain)
	mergePart(&s.PopupItem, &o.PopupItem, plain)
	mergePart(&s.PopupItemSelected, &o.PopupItemSelected, plain)
	mergePart(&s.Lightbulb, &o.Lightbulb, plain)
	mergePart(&s.PopupItemDisabled, &o.PopupItemDisabled, plain)
	mergePart(&s.PopupGroup, &o.PopupGroup, plain)
}

// mergePart applies the set fields of src on top of dst. With plain, src goes
// without the colors, gradients and shadows, which high contrast takes from the
// system colors. It is never inlined, so that a web build has one copy of its code
// (see ui.Style.MergeFrom).
//
//go:noinline
func mergePart(dst, src *ui.Style, plain bool) {
	if plain {
		p := *src
		p.Background, p.BorderColor, p.TextColor = ui.Color{}, ui.Color{}, ui.Color{}
		p.Gradient, p.Shadows, p.Opacity = ui.Gradient{}, nil, 0
		src = &p
	}
	dst.MergeFrom(src)
}

// editorDefaults sets st to the built-in look of the parts, from the colors of t.
func editorDefaults(st *Style, t *ui.Theme) {
	ec := t.CodeEditor
	*st = Style{
		Box:          ui.Style{Background: ec.Background, BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius},
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

		Popup: ui.Style{Background: t.Surface, TextColor: t.Text, BorderWidth: 1, BorderColor: t.Border,
			Radius: t.Radius, Padding: ui.All(4), Gap: 2, FontSize: t.FontSize},
		PopupItem:         ui.Style{Padding: ui.Edges{Top: 2, Right: 6, Bottom: 2, Left: 6}, Radius: t.Radius},
		PopupItemSelected: ui.Style{Background: t.Accent, TextColor: t.AccentText},
		Lightbulb:         ui.Style{TextColor: t.Warning},
		PopupItemDisabled: ui.Style{TextColor: t.Disabled},
		PopupGroup:        ui.Style{TextColor: t.TextMuted},
	}
	// The caret shows focus; the frame changes only under high contrast.
	if t.HighContrast {
		st.BoxFocused.BorderColor = t.Accent // the Highlight color of the system
	}
}

// resolve sets st to the look of the parts in the theme of ctx: the defaults, merged
// with the theme's codeedit.Style (see ui.ThemeStyle), then with s, the editor's
// Styles. Under high contrast the colors are the system colors, which the theme
// holds, so the colors of the theme's style and of the Styles are dropped. The
// layers are read where they are and merged into st: a Style is 19 parts of 109
// values each, which a web build cannot afford to copy (see ui.Style.MergeFrom).
func (s *Style) resolve(ctx *ui.Context, st *Style) {
	t := ctx.Theme()
	hc := t.HighContrast
	var base Style
	editorDefaults(&base, t)
	*st = Style{}
	st.mergeFrom(ui.StyleLayerRef(ctx, &base, "built-in default"), false)
	if themed := ui.ThemeStyleRef[Style](t); themed != nil {
		st.mergeFrom(ui.StyleLayerRef(ctx, themed, "theme"), hc)
	}
	st.mergeFrom(ui.StyleLayerRef(ctx, s, "instance Styles"), hc)
	if hc {
		// system merges a system color that the theme holds over a part's text color.
		system := func(part *ui.Style, c ui.Color) {
			part.MergeFrom(ui.StyleLayerRef(ctx, &ui.Style{TextColor: c}, "high contrast"))
		}
		system(&st.Text, t.CodeEditor.Foreground)
		system(&st.Gutter, t.CodeEditor.LineNumber)
		// The marks of the diagnostics take the system colors that the theme holds; the gutter
		// markers differ in shape, so the severities do not rest on color.
		system(&st.DiagnosticError, t.Danger)
		system(&st.DiagnosticWarning, t.Warning)
		system(&st.DiagnosticInfo, t.Accent)
		system(&st.DiagnosticHint, t.TextMuted)
		// The lightbulb and the group headers take the text color, and a disabled action the
		// disabled color, which is GrayText; the shape of the lightbulb marks it, not the color.
		system(&st.Lightbulb, t.Text)
		system(&st.PopupGroup, t.Text)
		system(&st.PopupItemDisabled, t.Disabled)
	}
}
