package ui

import (
	"slices"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
)

// commandPaletteRows is how many commands the list shows at once by default.
const commandPaletteRows = 8

// commandPaletteRowHeight is the height of a command's row at the Text size of 100
// percent, in logical pixels (its MinHeight). The list is at most MaxRows rows high.
const commandPaletteRowHeight = 32

// commandPaletteTop is the distance from the top of the window to the palette's
// panel, in logical pixels.
const commandPaletteTop = 48

// The keys of a command palette are actions, bound in the context of its search
// field (see DefaultBindings), so that they beat the field's own Up, Down, Enter and
// Escape.
var (
	commandPaletteUp       = NewAction("command-palette.up", "Previous command").Hide()
	commandPaletteDown     = NewAction("command-palette.down", "Next command").Hide()
	commandPalettePageUp   = NewAction("command-palette.page-up", "Command a page up").Hide()
	commandPalettePageDown = NewAction("command-palette.page-down", "Command a page down").Hide()
	commandPaletteRun      = NewAction("command-palette.run", "Run command").Hide()
	commandPaletteClose    = NewAction("command-palette.close", "Close command palette").Hide()
)

// CommandPaletteStyle is the look of each part of a command palette. A part's style
// merges over the built-in defaults, then over the theme's CommandPalette, then over
// the Styles of the instance; see docs/styling.md. Panel is the floating panel, 520 px
// wide by default; Input holds the search field; List is the list of commands; Item
// is each row's style and ItemHighlighted the row that the keys or the pointer are
// on. Label is a row's name, Match the text of it that matches the query (see
// HighlightMatches; under high contrast it is underlined instead of colored), and
// Keys a command's key binding. Detail is the detail text of a PaletteItem. Empty is
// the message shown when nothing matches.
type CommandPaletteStyle struct {
	// Panel is the floating panel that holds the field and the list.
	Panel Style
	// Input is the box around the search field.
	Input Style
	// List is the list of commands, which scrolls past MaxRows rows.
	List Style
	// Item is the style of each command's row.
	Item Style
	// ItemHighlighted is the style of the row that the keyboard or the pointer is on.
	ItemHighlighted Style
	// Label is the style of a command's name.
	Label Style
	// Match is the style of the part of a name that matches the typed text.
	Match Style
	// Keys is the style of a command's key binding, such as "Ctrl+N".
	Keys Style
	// Detail is the style of the detail text of a PaletteItem, such as a commit's author.
	Detail Style
	// Empty is the style of the message shown when nothing matches.
	Empty Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s CommandPaletteStyle) merge(o CommandPaletteStyle) CommandPaletteStyle {
	return CommandPaletteStyle{
		Panel:           s.Panel.Merge(o.Panel),
		Input:           s.Input.Merge(o.Input),
		List:            s.List.Merge(o.List),
		Item:            s.Item.Merge(o.Item),
		ItemHighlighted: s.ItemHighlighted.Merge(o.ItemHighlighted),
		Label:           s.Label.Merge(o.Label),
		Match:           s.Match.Merge(o.Match),
		Keys:            s.Keys.Merge(o.Keys),
		Detail:          s.Detail.Merge(o.Detail),
		Empty:           s.Empty.Merge(o.Empty),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s CommandPaletteStyle) plain() CommandPaletteStyle {
	return CommandPaletteStyle{
		Panel:           plainStyle(s.Panel),
		Input:           plainStyle(s.Input),
		List:            plainStyle(s.List),
		Item:            plainStyle(s.Item),
		ItemHighlighted: plainStyle(s.ItemHighlighted),
		Label:           plainStyle(s.Label),
		Match:           plainStyle(s.Match),
		Keys:            plainStyle(s.Keys),
		Detail:          plainStyle(s.Detail),
		Empty:           plainStyle(s.Empty),
	}
}

// commandPaletteDefaults returns the built-in look of a command palette, from the
// colors of t: the panel of a modal, and the rows of a menu. The highlighted row is
// the Highlight color under high contrast and a soft accent otherwise, as a menu's
// highlighted item is.
func commandPaletteDefaults(t *Theme) CommandPaletteStyle {
	d := CommandPaletteStyle{
		Panel: Style{Direction: Column, Width: Px(520), Background: t.Surface, BorderWidth: 1, BorderColor: t.Border,
			Radius: 10, Shadows: []Shadow{ShadowLg}, Padding: All(6), Gap: 4},
		Input: Style{Direction: Row, Align: Center, Padding: XY(10, 8)},
		List:  Style{Direction: Column, Gap: 2},
		Item: Style{Direction: Row, Align: Center, Gap: 12, MinHeight: Px(commandPaletteRowHeight), Padding: XY(10, 0),
			Radius: 6, NoShrink: true, TextColor: t.Text},
		Label:  Style{Grow: 1},
		Match:  Style{Background: t.Accent.Alpha(0.4), TextColor: t.Text},
		Keys:   Style{FontSize: 12, TextColor: t.TextSubtle, NoShrink: true},
		Detail: Style{FontSize: 12, TextColor: t.TextMuted, MaxWidth: Px(200), NoShrink: true},
		Empty:  Style{Padding: All(12), TextColor: t.TextMuted},
	}
	if t.HighContrast {
		d.ItemHighlighted = Style{Background: t.Accent, TextColor: t.AccentText}
		d.Match = Style{}
	} else {
		d.ItemHighlighted = Style{Background: t.AccentSoft}
	}
	return d
}

// resolveCommandPalette returns the parts of a command palette: the defaults, merged
// with the theme's CommandPalette and then the instance's Styles. Under high contrast
// the two overrides are plain (see plainStyle).
func resolveCommandPalette(ctx *Context, inst CommandPaletteStyle) CommandPaletteStyle {
	t := ctx.Theme()
	return layered(ctx, commandPaletteDefaults(t), CommandPaletteStyle.merge, CommandPaletteStyle.plain,
		themeLayer(t.CommandPalette), instanceLayer(inst))
}

// CommandPaletteElement is a floating panel that lists commands to run: a search
// field, and below it the commands whose names match what is typed, each with its
// key binding. It is open while its signal is true. Place it last in the window's
// root element, which must fill the window, as a Modal is placed; its panel floats
// near the top of the window, centered.
//
//	open := ui.NewSignal(false)
//	ui.Div().OnAction(showPalette, func() { open.Set(true) }).
//		Child(page).
//		Child(ui.CommandPalette(open))
//
// When it opens it lists the commands available where focus was, and the field takes
// focus. Up and Down move the highlight, PageUp and PageDown move it a list's height,
// Enter runs the highlighted command and a click runs the one clicked. The command
// runs where focus was before the palette opened, so a command that acts on the
// focused element acts on that element, and focus goes back there. Escape closes the
// palette and gives focus back too; a click outside closes it and leaves focus where
// the click put it.
type CommandPaletteElement struct {
	open        *Signal[bool]
	commands    func() []*Action
	items       func(query string) []PaletteItem // see Items
	itemsOnly   bool                             // see ItemsOnly
	placeholder string
	maxRows     int
	styles      CommandPaletteStyle
	key         string
}

// PaletteItem is a thing a command palette lists besides its commands, such as a
// branch, a tag or a commit, that the user opens or runs (see CommandPalette.Items).
// It is listed and matched as a command is: its Label is its name, and the palette
// keeps the items whose label, Detail or Keywords contain every word of the query.
type PaletteItem struct {
	// Key identifies the item in the list. It must be unique among the items, and stable.
	Key string
	// Label is the name of the item. The query matches it and highlights the match.
	Label string
	// Detail is short text after the label, such as a commit's short hash and author.
	// The query matches it too, but it is not highlighted.
	Detail string
	// Icon is an icon shown before the label.
	Icon *icons.Icon
	// Keywords are other words the query matches, which are not shown.
	Keywords []string
	// Run runs when the item is chosen. The palette has closed by then, and focus is
	// back where it was, as for a command.
	Run func()
}

// paletteEntry is one row of the list: an item the app gives, or a command.
type paletteEntry struct {
	item *PaletteItem
	act  *Action
}

// key is the key of the row.
func (e paletteEntry) key() string {
	if e.item != nil {
		return "jamb.palette/item/" + e.item.Key
	}
	return "jamb.palette/" + e.act.Name()
}

// label is the name of the row.
func (e paletteEntry) label() string {
	if e.item != nil {
		return e.item.Label
	}
	return e.act.Label()
}

// CommandPalette creates a command palette that is open while open is true. It sets
// open to false when it closes itself: after it runs a command, on Escape, and on a
// click outside it.
func CommandPalette(open *Signal[bool]) *CommandPaletteElement {
	return &CommandPaletteElement{open: open, maxRows: commandPaletteRows}
}

func (*CommandPaletteElement) isElement()           {}
func (p *CommandPaletteElement) elementKey() string { return p.key }

// Key identifies the palette among its siblings (see DivElement.Key).
func (p *CommandPaletteElement) Key(k string) *CommandPaletteElement { p.key = k; return p }

// Placeholder sets the text shown while the field is empty. The default is the
// window's Strings.CommandPalettePlaceholder, "Type a command".
func (p *CommandPaletteElement) Placeholder(s string) *CommandPaletteElement {
	p.placeholder = s
	return p
}

// MaxRows sets how many commands the list shows at once (8 by default): the list is
// at most that many rows high and scrolls to the rest. PageUp and PageDown move the
// highlight by that many rows.
func (p *CommandPaletteElement) MaxRows(n int) *CommandPaletteElement {
	if n > 0 {
		p.maxRows = n
	}
	return p
}

// Commands sets the function that gives the commands to list. It runs each time the
// palette opens, before the field takes focus. By default the palette lists the
// actions available where focus is (see Context.AvailableActions). Hidden actions
// are left out either way. A command runs where focus was when the palette opened,
// so one that no element there handles does nothing.
func (p *CommandPaletteElement) Commands(fn func() []*Action) *CommandPaletteElement {
	p.commands = fn
	return p
}

// Items sets the function that gives the items to list before the matching commands,
// for the query typed. It runs each time the query changes. The palette keeps the items
// whose label, detail or keywords contain every word of the query, ignoring case and in
// any order, so fn may return all its items or filter them itself, which is cheaper for
// a long list. Every item kept is built as a row, so fn should keep the list short. A
// query that starts with ">" in an ItemsOnly palette lists only commands, and fn is not
// called then.
func (p *CommandPaletteElement) Items(fn func(query string) []PaletteItem) *CommandPaletteElement {
	p.items = fn
	return p
}

// ItemsOnly hides the commands, so the palette is a quick open of the items, as VS
// Code's Ctrl+P is. Typing ">" first switches to the commands, which are matched
// against the rest of the query, as VS Code's palette does. Set a Placeholder that
// names what the palette opens.
func (p *CommandPaletteElement) ItemsOnly() *CommandPaletteElement {
	p.itemsOnly = true
	return p
}

// Styles sets the look of the parts of the palette, for this instance only. Set
// fields win over the theme's CommandPalette and the defaults; see
// CommandPaletteStyle and docs/styling.md. Calling Styles again merges with the
// earlier values.
func (p *CommandPaletteElement) Styles(st CommandPaletteStyle) *CommandPaletteElement {
	p.styles = p.styles.merge(st)
	return p
}

// paletteSession is what an open palette keeps from the moment it opened: the
// element that had focus, the commands, and the query and highlight, which start
// afresh each time it opens.
type paletteSession struct {
	open   bool
	back   *FocusHandle
	cmds   []*Action
	query  *Signal[string]
	hl     *Signal[int]
	reveal int // the row the keys have just moved to, plus one (zero for none); see Combobox
}

// paletteMatch reports whether every word of the query occurs in label, ignoring
// case, and returns the byte ranges of label where they occur, sorted and merged.
// A blank query matches every label.
func paletteMatch(label string, words []string) ([][2]int, bool) {
	var out [][2]int
	for _, w := range words {
		rs := matchRanges(label, w)
		if len(rs) == 0 {
			return nil, false
		}
		out = append(out, rs...)
	}
	slices.SortFunc(out, func(a, b [2]int) int { return a[0] - b[0] })
	merged := out[:0]
	for _, r := range out {
		if n := len(merged); n > 0 && r[0] <= merged[n-1][1] {
			merged[n-1][1] = max(merged[n-1][1], r[1])
			continue
		}
		merged = append(merged, r)
	}
	return merged, true
}

// paletteVisible returns the commands of cmds that match the query.
func paletteVisible(cmds []*Action, query string) []*Action {
	words := strings.Fields(query)
	var out []*Action
	for _, a := range cmds {
		if _, ok := paletteMatch(a.Label(), words); ok {
			out = append(out, a)
		}
	}
	return out
}

// commandsNow returns the commands to list as the palette opens, without the hidden
// ones.
func (p *CommandPaletteElement) commandsNow(ctx *Context) []*Action {
	var all []*Action
	if p.commands != nil {
		all = p.commands()
	} else {
		all = ctx.AvailableActions()
	}
	var out []*Action
	for _, a := range all {
		if a != nil && !a.Hidden() {
			out = append(out, a)
		}
	}
	return out
}

// sources reports whether a query lists the items the app gives, and whether it lists
// the commands, and the query the commands are matched against. In an ItemsOnly palette
// a query that starts with ">" lists the commands alone, matched against the rest.
func (p *CommandPaletteElement) sources(query string) (items, commands bool, cmdQuery string) {
	if !p.itemsOnly {
		return p.items != nil, true, query
	}
	if rest, ok := strings.CutPrefix(query, ">"); ok {
		return false, true, rest
	}
	return p.items != nil, false, query
}

// entries returns the rows a query lists, in order: the items the app gives that match,
// then the commands that match. The items are asked for with the whole query.
func (p *CommandPaletteElement) entries(cmds []*Action, query string) []paletteEntry {
	items, commands, cmdQuery := p.sources(query)
	var out []paletteEntry
	if items {
		words := strings.Fields(query)
		for _, it := range p.items(query) {
			text := strings.Join(append([]string{it.Label, it.Detail}, it.Keywords...), " ")
			if _, ok := paletteMatch(text, words); ok {
				out = append(out, paletteEntry{item: &it})
			}
		}
	}
	if commands {
		for _, a := range paletteVisible(cmds, cmdQuery) {
			out = append(out, paletteEntry{act: a})
		}
	}
	return out
}

func (p *CommandPaletteElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		s := Local(ctx, "session", &paletteSession{}).Peek()
		input := Local(ctx, "input", NewFocusHandle()).Peek()
		if !p.open.Get() {
			s.open = false
			return nil
		}
		if !s.open {
			// The palette has just opened: focus is still where the user was, so the
			// commands are the ones available there, and focus goes back there.
			*s = paletteSession{open: true, back: ctx.FocusedHandle(), cmds: p.commandsNow(ctx),
				query: NewSignal(""), hl: NewSignal(0)}
		}
		t := ctx.Theme()
		st := resolveCommandPalette(ctx, p.styles)
		strs := ctx.Strings()
		query := s.query.Get()
		vis := p.entries(s.cmds, query)
		hl := min(s.hl.Get(), len(vis)-1)

		// The handlers read the query when they run, not as it was at this render: keys
		// typed quickly can arrive before the next render.
		visible := func() []paletteEntry { return p.entries(s.cmds, s.query.Peek()) }
		closeFn := func(restore bool) {
			p.open.Set(false)
			if restore && s.back != nil {
				s.back.FocusNow()
			}
		}
		// run closes the palette, gives focus back, and runs the row: a command acts on the
		// element the user was on, not on the palette, and an item runs its Run.
		run := func(e paletteEntry) {
			closeFn(true)
			if e.item != nil {
				if e.item.Run != nil {
					e.item.Run()
				}
				return
			}
			ctx.Dispatch(e.act)
		}
		// move highlights row i, kept inside the list, and has it scrolled into view.
		// The highlight stops at the ends of the list.
		move := func(delta int) {
			n := len(visible())
			if n == 0 {
				return
			}
			i := min(max(min(s.hl.Peek(), n-1)+delta, 0), n-1)
			s.hl.Set(i)
			s.reveal = i + 1
		}
		placeholder := p.placeholder
		if placeholder == "" {
			placeholder = strs.CommandPalettePlaceholder
		}

		field := TextInput(s.query).Label(strs.CommandPalette).Placeholder(placeholder).Borderless().
			AutoFocus().FocusHandle(input).Width(Full).
			OnChange(func(string) { s.hl.Set(0) })
		// The field and the list are a combo box, as Combobox's are: focus stays in the
		// field, and the highlighted row is the focus that assistive technology reports.
		combo := Div().Column().Role(access.RoleComboBox).AccessExpanded(true).Label(strs.CommandPalette).
			OnAction(commandPaletteUp, func() { move(-1) }).
			OnAction(commandPaletteDown, func() { move(1) }).
			OnAction(commandPalettePageUp, func() { move(-p.maxRows) }).
			OnAction(commandPalettePageDown, func() { move(p.maxRows) }).
			OnAction(commandPaletteRun, func() {
				list, i := visible(), s.hl.Peek()
				if i >= 0 && i < len(list) {
					run(list[i])
				}
			}).
			OnAction(commandPaletteClose, func() { closeFn(true) }).
			Child(Div().asPart("CommandPalette.Input").Style(st.Input).Child(field))
		a := combo.access()
		a.value = query
		// Collapse from assistive technology closes the palette, as Escape does.
		a.setExpanded = func(o bool) {
			if !o {
				closeFn(true)
			}
		}

		reveal := s.reveal
		s.reveal = 0
		maxH := float32(p.maxRows)*commandPaletteRowHeight*textScale(ctx) + float32(p.maxRows-1)*st.List.Gap
		list := Div().asPart("CommandPalette.List").Style(st.List).MaxHeight(Px(maxH)).ScrollY().
			Role(access.RoleList).SelectionContainer().Label(strs.CommandPalette)
		words := strings.Fields(query)
		match := Highlight{Color: st.Match.TextColor, Background: st.Match.Background, Underline: t.HighContrast}
		for i, e := range vis {
			list.Child(p.row(ctx, st, e, words, match, i == hl, i == reveal-1, func() { s.hl.Set(i) }, run))
		}
		combo.Child(list)
		if len(vis) == 0 {
			msg := strs.NoMatchingCommands
			if items, _, _ := p.sources(query); items {
				msg = strs.NoResults // the app's items are listed too, so the message is not about commands
			}
			combo.Child(Div().asPart("CommandPalette.Empty").Style(st.Empty).Child(Text(msg)))
		}

		panel := Div().asPart("CommandPalette.Panel").Key("jamb.palette").KeyContext("CommandPalette").
			Overlay(BelowCenter).Style(st.Panel).
			Role(access.RoleDialog).Label(strs.CommandPalette).
			Enter(Fade().Slide(0, -6).Grow(0.98), Animation{Duration: t.Motion.enter(140 * time.Millisecond)}).
			Exit(Fade().Grow(0.98), Animation{Duration: t.Motion.exit(90 * time.Millisecond)}).
			// A press in the panel leaves the field focused, so a click on a row runs
			// it with the field's focus intact.
			KeepFocus().
			OnClickOutside(func() { closeFn(false) }).
			Child(combo)
		// The anchor is a line across the window near its top; the panel hangs from
		// its middle.
		return Div().Absolute().Top(commandPaletteTop).Left(0).Right(0).Height(Px(0)).Child(panel)
	})
}

// row is one row of the list: its name, with the matches of the query highlighted. A
// command shows its key binding at the right; an item shows its icon before the name
// and its detail after it.
func (p *CommandPaletteElement) row(ctx *Context, st CommandPaletteStyle, e paletteEntry, words []string, match Highlight,
	hi, reveal bool, highlight func(), run func(paletteEntry)) Element {
	style := st.Item
	if hi {
		style = style.Merge(st.ItemHighlighted)
	}
	label := Text(e.label()).NoWrap().Truncate()
	if rs, _ := paletteMatch(e.label(), words); len(rs) > 0 {
		for _, r := range rs {
			label.Highlight(r[0], r[1], match)
		}
	}
	r := Div().asPart("CommandPalette.Item").Key(e.key()).Style(style).
		Role(access.RoleListItem).AccessSelected(hi).Label(e.label())
	if e.item != nil && e.item.Icon != nil {
		r.Child(Icon(e.item.Icon).Size(16))
	}
	r.Child(Div().asPart("CommandPalette.Label").Style(st.Label).Child(label))
	r.revealRow = reveal
	r.access().highlighted = hi
	if e.item != nil {
		if d := e.item.Detail; d != "" {
			// The detail is read after the name, as the description of the row.
			r.Description(d)
			r.Child(Div().asPart("CommandPalette.Detail").Style(st.Detail).Child(Text(d).NoWrap().Truncate()))
		}
	} else if keys := ctx.KeysFor(e.act); keys != "" {
		// The binding is read after the name, as the description of the row.
		r.Description(keys)
		r.Child(Div().asPart("CommandPalette.Keys").Style(st.Keys).Child(Text(keys).NoWrap()))
	}
	return r.OnMouseEnter(highlight).OnClick(func() { run(e) })
}
