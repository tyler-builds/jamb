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

// mergeFrom applies the set fields of o on top of s, part by part, in place. With
// plain, o's parts go without their colors (see mergePart).
func (s *Style) mergeFrom(o *Style, plain bool) {
	mergePart(&s.Document, &o.Document, plain)
	mergePart(&s.Paragraph, &o.Paragraph, plain)
	mergePart(&s.Heading1, &o.Heading1, plain)
	mergePart(&s.Heading2, &o.Heading2, plain)
	mergePart(&s.Heading3, &o.Heading3, plain)
	mergePart(&s.Heading4, &o.Heading4, plain)
	mergePart(&s.Heading5, &o.Heading5, plain)
	mergePart(&s.Heading6, &o.Heading6, plain)
	mergePart(&s.Link, &o.Link, plain)
	mergePart(&s.LinkHover, &o.LinkHover, plain)
	mergePart(&s.InlineCode, &o.InlineCode, plain)
	mergePart(&s.CodeBlock, &o.CodeBlock, plain)
	mergePart(&s.Quote, &o.Quote, plain)
	mergePart(&s.List, &o.List, plain)
	mergePart(&s.ListMarker, &o.ListMarker, plain)
	mergePart(&s.Task, &o.Task, plain)
	mergePart(&s.TaskChecked, &o.TaskChecked, plain)
	mergePart(&s.Table, &o.Table, plain)
	mergePart(&s.TableHeader, &o.TableHeader, plain)
	mergePart(&s.TableCell, &o.TableCell, plain)
	mergePart(&s.Rule, &o.Rule, plain)
	mergePart(&s.ImagePlaceholder, &o.ImagePlaceholder, plain)
}

// mergePart applies the set fields of src on top of dst. With plain, src goes
// without the colors, gradients, shadows and opacity that high contrast takes from
// the system colors, as the ui's own parts do. It is never inlined, so that a web
// build has one copy of its code (see ui.Style.MergeFrom).
//
//go:noinline
func mergePart(dst, src *ui.Style, plain bool) {
	if plain {
		p := *src
		plainPart(&p)
		src = &p
	}
	dst.MergeFrom(src)
}

// plainPart drops from s the colors, gradients, shadows and opacity.
func plainPart(s *ui.Style) {
	s.Background, s.BorderColor, s.TextColor = ui.Color{}, ui.Color{}, ui.Color{}
	s.Gradient, s.Shadows, s.Opacity = ui.Gradient{}, nil, 0
}

// plain drops the colors of each part of s (see plainPart).
func (s *Style) plain() {
	plainPart(&s.Document)
	plainPart(&s.Paragraph)
	plainPart(&s.Heading1)
	plainPart(&s.Heading2)
	plainPart(&s.Heading3)
	plainPart(&s.Heading4)
	plainPart(&s.Heading5)
	plainPart(&s.Heading6)
	plainPart(&s.Link)
	plainPart(&s.LinkHover)
	plainPart(&s.InlineCode)
	plainPart(&s.CodeBlock)
	plainPart(&s.Quote)
	plainPart(&s.List)
	plainPart(&s.ListMarker)
	plainPart(&s.Task)
	plainPart(&s.TaskChecked)
	plainPart(&s.Table)
	plainPart(&s.TableHeader)
	plainPart(&s.TableCell)
	plainPart(&s.Rule)
	plainPart(&s.ImagePlaceholder)
}

// heading returns the part of the headings of level 1 to 6.
func (s *Style) heading(level int) *ui.Style {
	return [...]*ui.Style{&s.Heading1, &s.Heading2, &s.Heading3, &s.Heading4, &s.Heading5, &s.Heading6}[min(max(level, 1), 6)-1]
}

// defaults sets st to the built-in look of the parts, from the colors and the text
// size of t. Headings are semibold by their content; no part changes its weight for
// a state such as hover or focus.
func defaults(st *Style, t *ui.Theme) {
	fs := t.FontSize
	if fs <= 0 {
		fs = 14
	}
	ec := t.CodeEditor
	*st = Style{
		Document:  ui.Style{Direction: ui.Column, Gap: 12},
		Paragraph: ui.Style{LineHeight: 1.5},
		Heading1: ui.Style{FontSize: fs * 2, FontWeight: ui.FontSemibold, BorderEdges: ui.Edges{Bottom: 1},
			BorderColor: t.Border, Padding: ui.Edges{Bottom: 6}, Margin: ui.Edges{Top: 6}},
		Heading2: ui.Style{FontSize: fs * 1.5, FontWeight: ui.FontSemibold, BorderEdges: ui.Edges{Bottom: 1},
			BorderColor: t.Border, Padding: ui.Edges{Bottom: 6}, Margin: ui.Edges{Top: 6}},
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

// highContrast sets st to the system colors of the parts under high contrast, which
// the high contrast theme t holds: text, links and borders in the text, link and
// highlight colors, with no fills but the code block's and a done task's.
func highContrast(st *Style, t *ui.Theme) {
	*st = Style{
		Heading1:         ui.Style{BorderColor: t.Text},
		Heading2:         ui.Style{BorderColor: t.Text},
		Heading6:         ui.Style{TextColor: t.Text},
		Link:             ui.Style{TextColor: t.Link},
		InlineCode:       ui.Style{TextColor: t.Text},
		CodeBlock:        ui.Style{Background: t.CodeEditor.Background, TextColor: t.CodeEditor.Foreground, BorderColor: t.Text},
		Quote:            ui.Style{BorderColor: t.Text, TextColor: t.Text},
		ListMarker:       ui.Style{TextColor: t.Text},
		Task:             ui.Style{BorderColor: t.Text},
		TaskChecked:      ui.Style{Background: t.Accent, BorderColor: t.Accent, TextColor: t.AccentText},
		Table:            ui.Style{BorderColor: t.Text},
		Rule:             ui.Style{Background: t.Text},
		ImagePlaceholder: ui.Style{BorderColor: t.Text, TextColor: t.Text},
	}
}

// resolveStyle sets st to the look of the parts in the theme of ctx: the defaults,
// merged with the theme's markdown.Style (see ui.ThemeStyle), then with the view's
// Styles. Under high contrast the colors of the theme's style and of the Styles are
// dropped, and the parts take the system colors (see highContrast). The layers are
// read where they are and merged into st: a Style is 22 parts of 109 values each,
// which a web build cannot afford to copy (see ui.Style.MergeFrom).
func (v *ViewElement) resolveStyle(ctx *ui.Context, st *Style) {
	t := ctx.Theme()
	hc := t.HighContrast
	var base Style
	defaults(&base, t)
	if hc {
		base.plain()
	}
	*st = Style{}
	st.mergeFrom(ui.StyleLayerRef(ctx, &base, "built-in default"), false)
	if themed := ui.ThemeStyleRef[Style](t); themed != nil {
		st.mergeFrom(ui.StyleLayerRef(ctx, themed, "theme"), hc)
	}
	st.mergeFrom(ui.StyleLayerRef(ctx, &v.styles, "instance Styles"), hc)
	if hc {
		var system Style
		highContrast(&system, t)
		st.mergeFrom(ui.StyleLayerRef(ctx, &system, "high contrast"), false)
	}
}
