package ui

import "github.com/tyler-builds/jamb/access"

// StatusBarStyle is the look of each part of a status bar. A part's style merges
// over the built-in defaults, then over the theme's StatusBar, then over the
// Styles of the instance; see docs/styling.md. Bar is the strip, with the size and
// color of its text, which the items inherit; Start holds the items at its start
// (the left), and End the items at its end (the right).
type StatusBarStyle struct {
	Bar   Style // the strip: its height, padding, fill, top border and text
	Start Style // the row of items at the start, which takes the free space
	End   Style // the row of items at the end
}

// statusBarDefaults sets d to the built-in look of a status bar, from the colors of
// t: a compact strip in the panel color with a border along its top, and small
// muted text. Under high contrast the strip is the window color, its border and
// its text the text color.
func statusBarDefaults(d *StatusBarStyle, t *Theme) {
	*d = StatusBarStyle{
		Bar: Style{Direction: Row, Align: Center, Gap: 16, Padding: XY(10, 3), MinHeight: Px(24), NoShrink: true,
			Background: t.Panel, BorderEdges: Edges{Top: 1}, BorderColor: t.Border, FontSize: 12, TextColor: t.TextMuted},
		Start: Style{Direction: Row, Align: Center, Gap: 12, Grow: 1, MinWidth: Px(0)},
		End:   Style{Direction: Row, Align: Center, Gap: 12, NoShrink: true},
	}
	if t.HighContrast {
		d.Bar.Background, d.Bar.BorderColor, d.Bar.TextColor = t.Surface, t.Text, t.Text
	}
}

// resolveStatusBar sets st to the look of a status bar: the defaults, merged with the
// theme's StatusBar and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveStatusBar(ctx *Context, inst, st *StatusBarStyle) {
	t := ctx.Theme()
	statusBarDefaults(st, t)
	st.layered(ctx,
		themeLayer(&t.StatusBar), instanceLayer(inst))
}

// StatusBarElement is the bar along the bottom of a window, which shows the state
// of what the window holds, such as the current page, a count or the position of
// the caret. Its items sit at its start and at its end. Assistive technology sees
// a status bar (UI Automation's StatusBar control type) that holds the items.
//
// A status bar is not a live region: a change to its text is not announced. An app
// that changes it after an action the user should hear about announces it too,
// with Announce.
type StatusBarElement struct {
	start  []Element
	end    []Element
	label  string
	key    string
	styles StatusBarStyle
	size   sizing // the bar's sizes, from the sizing methods (see Sizing in docs/styling.md)
}

// StatusBar creates an empty status bar. A status bar fills the width of its
// container by default; put it last in the window's column.
func StatusBar() *StatusBarElement { return &StatusBarElement{} }

func (*StatusBarElement) isElement()           {}
func (s *StatusBarElement) elementKey() string { return s.key }

// Key identifies the status bar among its siblings (see DivElement.Key).
func (s *StatusBarElement) Key(k string) *StatusBarElement { s.key = k; return s }

// Start appends items at the start of the bar, the left. nil items are ignored.
func (s *StatusBarElement) Start(els ...Element) *StatusBarElement {
	for _, e := range els {
		if !isNil(e) {
			s.start = append(s.start, e)
		}
	}
	return s
}

// End appends items at the end of the bar, the right. nil items are ignored.
func (s *StatusBarElement) End(els ...Element) *StatusBarElement {
	for _, e := range els {
		if !isNil(e) {
			s.end = append(s.end, e)
		}
	}
	return s
}

// Label names the status bar for assistive technology. A status bar needs no
// name, since its control type says what it is; a window with two status bars
// names them apart.
func (s *StatusBarElement) Label(name string) *StatusBarElement { s.label = name; return s }

// The sizing methods give the status bar's outer box a size. See Sizing in docs/styling.md.
func (s *StatusBarElement) Width(l Length) *StatusBarElement { s.size.setWidth(l); return s }

// MinWidth sets the smallest width of the status bar.
func (s *StatusBarElement) MinWidth(l Length) *StatusBarElement { s.size.setMinWidth(l); return s }

// MaxWidth sets the largest width of the status bar.
func (s *StatusBarElement) MaxWidth(l Length) *StatusBarElement { s.size.setMaxWidth(l); return s }

// Grow makes the status bar take a share of the free space on its parent's main axis.
func (s *StatusBarElement) Grow(f float32) *StatusBarElement { s.size.setGrow(f); return s }

// GridSpan sets how many columns and rows the status bar covers in its parent's grid.
func (s *StatusBarElement) GridSpan(cols, rows int) *StatusBarElement {
	s.size.setSpan(cols, rows)
	return s
}

// Styles sets the look of the parts of the status bar, for this instance only.
// Set fields win over the theme's StatusBar and the defaults; see StatusBarStyle
// and docs/styling.md. Calling Styles again merges with the earlier values.
func (s *StatusBarElement) Styles(st StatusBarStyle) *StatusBarElement { return s.stylesOf(&st) }

func (s *StatusBarElement) stylesOf(st *StatusBarStyle) *StatusBarElement {
	s.styles.mergeFrom(st, false)
	return s
}

func (s *StatusBarElement) expand(ctx *Context) Element {
	var st StatusBarStyle
	resolveStatusBar(ctx, &s.styles, &st)
	bar := Div().asPart("StatusBar.Bar").styleOf(&st.Bar).Role(access.RoleStatusBar).Label(s.label).
		Child(Div().asPart("StatusBar.Start").styleOf(&st.Start).Children(s.start...)).
		Child(Div().asPart("StatusBar.End").styleOf(&st.End).Children(s.end...))
	return s.size.apply(bar)
}
