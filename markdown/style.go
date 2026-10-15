package markdown

import "github.com/tyler-builds/jamb/ui"

// Style is the look of the parts of a Markdown view. A part's style merges over the
// built-in defaults, then over the Style that the theme holds for every view (set
// with ui.SetThemeStyle), then over the Styles of the view, field by field.
//
// Under high contrast the colors of these parts are the system colors, whatever is
// set here or in the theme; their sizes, spacing and fonts still apply.
type Style struct {
	// Document is the column that holds the blocks: its Gap is the space between
	// them, and its padding, background and text apply to the whole document.
	Document ui.Style
	// Paragraph is the text of a paragraph: its size, color and line height.
	Paragraph ui.Style
	// Heading1 to Heading6 are the boxes of the headings of each level, and their
	// text: size, weight, color, and a border or padding.
	Heading1, Heading2, Heading3, Heading4, Heading5, Heading6 ui.Style
	// Link is the text of a link: its TextColor and Background.
	Link ui.Style
	// LinkHover is the text of a link under the pointer: its TextColor and
	// Background. A link is underlined while the pointer is over it.
	LinkHover ui.Style
	// InlineCode is the text of code in a line: its TextColor and Background. It is
	// always in the monospace font.
	InlineCode ui.Style
	// CodeBlock is the box of a code block, which scrolls sideways when a line is
	// long, and its text: background, border, radius, padding and font.
	CodeBlock ui.Style
	// Quote is the box of a block quote: its border, padding, gap and text color.
	Quote ui.Style
	// List is the column of the items of a tight list: its Gap is the space between
	// them. A loose list spaces its items as the Document spaces its blocks.
	List ui.Style
	// ListMarker is the bullet or the number of a list item: its color, size and
	// width.
	ListMarker ui.Style
	// Task is the box of a task's check box, and TaskChecked its look when the task
	// is done, over Task. The check mark takes TaskChecked's TextColor.
	Task, TaskChecked ui.Style
	// Table is the box of a table: its BorderColor and BorderWidth are also the lines
	// between the cells.
	Table ui.Style
	// TableHeader is a cell of the header row, over TableCell.
	TableHeader ui.Style
	// TableCell is a cell of a table: its padding, background and text.
	TableCell ui.Style
	// Rule is a thematic break: a box as wide as the document, of the rule's Height
	// and Background.
	Rule ui.Style
	// ImagePlaceholder is the box shown in place of an image that is loading or
	// cannot be shown, with its alternative text.
	ImagePlaceholder ui.Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s Style) merge(o Style) Style {
	return Style{
		Document:         s.Document.Merge(o.Document),
		Paragraph:        s.Paragraph.Merge(o.Paragraph),
		Heading1:         s.Heading1.Merge(o.Heading1),
		Heading2:         s.Heading2.Merge(o.Heading2),
		Heading3:         s.Heading3.Merge(o.Heading3),
		Heading4:         s.Heading4.Merge(o.Heading4),
		Heading5:         s.Heading5.Merge(o.Heading5),
		Heading6:         s.Heading6.Merge(o.Heading6),
		Link:             s.Link.Merge(o.Link),
		LinkHover:        s.LinkHover.Merge(o.LinkHover),
		InlineCode:       s.InlineCode.Merge(o.InlineCode),
		CodeBlock:        s.CodeBlock.Merge(o.CodeBlock),
		Quote:            s.Quote.Merge(o.Quote),
		List:             s.List.Merge(o.List),
		ListMarker:       s.ListMarker.Merge(o.ListMarker),
		Task:             s.Task.Merge(o.Task),
		TaskChecked:      s.TaskChecked.Merge(o.TaskChecked),
		Table:            s.Table.Merge(o.Table),
		TableHeader:      s.TableHeader.Merge(o.TableHeader),
		TableCell:        s.TableCell.Merge(o.TableCell),
		Rule:             s.Rule.Merge(o.Rule),
		ImagePlaceholder: s.ImagePlaceholder.Merge(o.ImagePlaceholder),
	}
}

// plain returns s with each part without the colors, gradients, shadows and opacity
// that high contrast takes from the system colors, as the ui's own parts do.
func (s Style) plain() Style {
	p := func(s ui.Style) ui.Style {
		s.Background, s.BorderColor, s.TextColor = ui.Color{}, ui.Color{}, ui.Color{}
		s.Gradient, s.Shadows, s.Opacity = ui.Gradient{}, nil, 0
		return s
	}
	return Style{
		Document: p(s.Document), Paragraph: p(s.Paragraph),
		Heading1: p(s.Heading1), Heading2: p(s.Heading2), Heading3: p(s.Heading3),
		Heading4: p(s.Heading4), Heading5: p(s.Heading5), Heading6: p(s.Heading6),
		Link: p(s.Link), LinkHover: p(s.LinkHover), InlineCode: p(s.InlineCode), CodeBlock: p(s.CodeBlock),
		Quote: p(s.Quote), List: p(s.List), ListMarker: p(s.ListMarker), Task: p(s.Task), TaskChecked: p(s.TaskChecked),
		Table: p(s.Table), TableHeader: p(s.TableHeader), TableCell: p(s.TableCell), Rule: p(s.Rule),
		ImagePlaceholder: p(s.ImagePlaceholder),
	}
}

// heading returns the part of the headings of level 1 to 6.
func (s *Style) heading(level int) ui.Style {
	return [...]ui.Style{s.Heading1, s.Heading2, s.Heading3, s.Heading4, s.Heading5, s.Heading6}[min(max(level, 1), 6)-1]
}

// defaults returns the built-in look of the parts, from the colors and the text size
// of t. Headings are semibold by their content; no part changes its weight for a
// state such as hover or focus.
func defaults(t *ui.Theme) Style {
	fs := t.FontSize
	if fs <= 0 {
		fs = 14
	}
	ruled := func(size float32) ui.Style {
		return ui.Style{FontSize: size, FontWeight: ui.FontSemibold, BorderEdges: ui.Edges{Bottom: 1},
			BorderColor: t.Border, Padding: ui.Edges{Bottom: 6}, Margin: ui.Edges{Top: 6}}
	}
	ec := t.CodeEditor
	return Style{
		Document:   ui.Style{Direction: ui.Column, Gap: 12},
		Paragraph:  ui.Style{LineHeight: 1.5},
		Heading1:   ruled(fs * 2),
		Heading2:   ruled(fs * 1.5),
		Heading3:   ui.Style{FontSize: fs * 1.25, FontWeight: ui.FontSemibold, Margin: ui.Edges{Top: 4}},
		Heading4:   ui.Style{FontSize: fs, FontWeight: ui.FontSemibold},
		Heading5:   ui.Style{FontSize: fs * 0.9, FontWeight: ui.FontSemibold},
		Heading6:   ui.Style{FontSize: fs * 0.85, FontWeight: ui.FontSemibold, TextColor: t.TextMuted},
		Link:       ui.Style{TextColor: t.Link},
		InlineCode: ui.Style{TextColor: t.Text, Background: t.SurfaceActive},
		CodeBlock: ui.Style{Background: ec.Background, TextColor: ec.Foreground, FontFamily: "mono", LineHeight: 1.4,
			BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius, Padding: ui.All(12)},
		Quote:       ui.Style{Direction: ui.Column, Gap: 8, BorderEdges: ui.Edges{Left: 3}, BorderColor: t.Border, Padding: ui.Edges{Left: 12}, TextColor: t.TextMuted},
		List:        ui.Style{Direction: ui.Column, Gap: 4},
		ListMarker:  ui.Style{TextColor: t.TextMuted, MinWidth: ui.Px(18), TextAlign: ui.End},
		Task:        ui.Style{Width: ui.Px(14), Height: ui.Px(14), BorderWidth: 1, BorderColor: t.TextMuted, Radius: 3, Margin: ui.Edges{Top: 3}},
		TaskChecked: ui.Style{Background: t.Accent, BorderColor: t.Accent, TextColor: t.AccentText},
		Table:       ui.Style{BorderWidth: 1, BorderColor: t.Border},
		TableHeader: ui.Style{Background: t.Surface, FontWeight: ui.FontSemibold},
		TableCell:   ui.Style{Padding: ui.XY(10, 6)},
		Rule:        ui.Style{Height: ui.Px(1), Background: t.Border, Margin: ui.XY(0, 4)},
		ImagePlaceholder: ui.Style{Padding: ui.XY(10, 6), BorderWidth: 1, BorderColor: t.Border, Radius: t.Radius,
			Background: t.Surface, TextColor: t.TextMuted, AlignSelf: ui.Start},
	}
}

// highContrast returns the system colors of the parts under high contrast, which
// the high contrast theme t holds: text, links and borders in the text, link and
// highlight colors, with no fills but the code block's and a done task's.
func highContrast(t *ui.Theme) Style {
	line := ui.Style{BorderColor: t.Text}
	return Style{
		Heading1: line, Heading2: line,
		Heading6:         ui.Style{TextColor: t.Text},
		Link:             ui.Style{TextColor: t.Link},
		InlineCode:       ui.Style{TextColor: t.Text},
		CodeBlock:        ui.Style{Background: t.CodeEditor.Background, TextColor: t.CodeEditor.Foreground, BorderColor: t.Text},
		Quote:            ui.Style{BorderColor: t.Text, TextColor: t.Text},
		ListMarker:       ui.Style{TextColor: t.Text},
		Task:             ui.Style{BorderColor: t.Text},
		TaskChecked:      ui.Style{Background: t.Accent, BorderColor: t.Accent, TextColor: t.AccentText},
		Table:            line,
		Rule:             ui.Style{Background: t.Text},
		ImagePlaceholder: ui.Style{BorderColor: t.Text, TextColor: t.Text},
	}
}

// resolveStyle returns the look of the parts in the theme of ctx: the defaults,
// merged with the theme's markdown.Style (see ui.ThemeStyle), then with the view's
// Styles. Under high contrast the colors of the theme's style and of the Styles are
// dropped, and the parts take the system colors (see highContrast).
func (v *ViewElement) resolveStyle(ctx *ui.Context) Style {
	t := ctx.Theme()
	themed, _ := ui.ThemeStyle[Style](t)
	own := v.styles
	if t.HighContrast {
		themed, own = themed.plain(), own.plain()
	}
	base := defaults(t)
	if t.HighContrast {
		base = base.plain()
	}
	st := ui.StyleLayer(ctx, base, "built-in default").
		merge(ui.StyleLayer(ctx, themed, "theme")).
		merge(ui.StyleLayer(ctx, own, "instance Styles"))
	if t.HighContrast {
		st = st.merge(ui.StyleLayer(ctx, highContrast(t), "high contrast"))
	}
	return st
}
