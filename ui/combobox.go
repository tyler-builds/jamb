package ui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// comboboxMaxVisible is how many suggestions the list shows by default.
const comboboxMaxVisible = 8

// comboboxAnnounceDelay is the pause after the last change of the suggestions
// before their count is announced, so that typing does not announce each one.
const comboboxAnnounceDelay = 300 * time.Millisecond

// comboboxRowHeight is the height of a suggestion row at the Text size of 100 percent, in
// logical pixels (its MinHeight). The list is at most MaxVisible rows high.
const comboboxRowHeight = 28

// The keys of a combobox are actions, bound in the context of its editor (see
// DefaultBindings). An action runs only while its handler is on the focus path,
// so Up, Down, Enter and Escape keep their editing meaning when they do not apply.
var (
	comboboxDown   = NewAction("combobox.down", "Next suggestion").Hide()
	comboboxUp     = NewAction("combobox.up", "Previous suggestion").Hide()
	comboboxEnter  = NewAction("combobox.choose", "Choose suggestion").Hide()
	comboboxEscape = NewAction("combobox.escape", "Close suggestions").Hide()
)

// ComboboxStyle is the look of each part of a combobox. A part's style merges over
// the built-in defaults, then over the theme's Combobox, then over the Styles of the
// instance; see docs/styling.md. Field is the text frame of the editor, a FieldStyle
// (see TextInput) with the same parts and states. Popup is the open list, Option each
// suggestion, OptionHighlighted the suggestion that the keys have reached, and
// OptionDisabled a suggestion that cannot be chosen. Match is the style of the text
// that matches the query in a suggestion's label (see HighlightMatches); under high
// contrast it is underlined instead of colored. Empty is the "No results" slot, and
// Loading the row with the ProgressRing while suggestions are being found. Error is the
// row that says the suggestions could not be found (see ErrorText), and the message
// under the field that says what was reverted or cleared (see AllowFree); both have the
// look of a field's error message.
type ComboboxStyle struct {
	// Field is the text frame of the combo box.
	Field FieldStyle
	// Popup is the panel that holds the list of options.
	Popup Style
	// Option is the style of each option in the list.
	Option Style
	// OptionHighlighted is the style of the option that the keyboard or the pointer is on.
	OptionHighlighted Style
	// OptionDisabled is the style of an option that cannot be chosen.
	OptionDisabled Style
	// Match is the style of the part of an option that matches the typed text.
	Match Style
	// Empty is the style of the message shown when no option matches.
	Empty Style
	// Loading is the style of the message shown while the suggestions load.
	Loading Style
	// Error is the style of the message shown when the suggestions cannot be loaded.
	Error Style
}

// merge returns s with the set fields of o applied on top, part by part.
func (s ComboboxStyle) merge(o ComboboxStyle) ComboboxStyle {
	return ComboboxStyle{
		Field:             s.Field.merge(o.Field),
		Popup:             s.Popup.Merge(o.Popup),
		Option:            s.Option.Merge(o.Option),
		OptionHighlighted: s.OptionHighlighted.Merge(o.OptionHighlighted),
		OptionDisabled:    s.OptionDisabled.Merge(o.OptionDisabled),
		Match:             s.Match.Merge(o.Match),
		Empty:             s.Empty.Merge(o.Empty),
		Loading:           s.Loading.Merge(o.Loading),
		Error:             s.Error.Merge(o.Error),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s ComboboxStyle) plain() ComboboxStyle {
	return ComboboxStyle{
		Field:             s.Field.plain(),
		Popup:             plainStyle(s.Popup),
		Option:            plainStyle(s.Option),
		OptionHighlighted: plainStyle(s.OptionHighlighted),
		OptionDisabled:    plainStyle(s.OptionDisabled),
		Match:             plainStyle(s.Match),
		Empty:             plainStyle(s.Empty),
		Loading:           plainStyle(s.Loading),
		Error:             plainStyle(s.Error),
	}
}

// comboboxDefaults returns the built-in look of the parts of a combobox, from the
// colors of t. The highlighted suggestion is the Highlight color under high contrast
// and a soft accent otherwise, as a select's highlighted option is.
func comboboxDefaults(t *Theme) ComboboxStyle {
	d := ComboboxStyle{
		Popup: Style{Padding: All(4), Background: t.Surface, BorderWidth: 1,
			BorderColor: t.Border, Radius: t.Radius, Shadows: []Shadow{ShadowMd}},
		Option: Style{Direction: Row, Align: Center, Gap: 8, MinHeight: Px(28), Padding: XY(8, 0),
			Radius: 4, NoShrink: true, FontSize: 13, TextColor: t.Text},
		OptionDisabled: Style{TextColor: t.Disabled},
		Match:          Style{Background: t.Accent.Alpha(0.4), TextColor: t.Text},
		Empty:          Style{Padding: XY(8, 6), FontSize: 13, TextColor: t.TextMuted},
		Loading:        Style{Direction: Row, Align: Center, Gap: 8, Padding: XY(8, 6), TextColor: t.TextMuted},
		Error:          messageStyle(t).Merge(Style{Padding: XY(8, 6)}),
	}
	if t.HighContrast {
		d.OptionHighlighted = Style{Background: t.Accent, TextColor: t.AccentText}
		d.OptionDisabled = Style{TextColor: t.Disabled}
		d.Match = Style{}
	} else {
		d.OptionHighlighted = Style{Background: t.AccentSoft}
	}
	return d
}

// resolveCombobox returns the parts of a combobox: the defaults, merged with the
// theme's Combobox and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle). The Field is resolved by the editor, over
// the theme's Field.
func resolveCombobox(ctx *Context, inst ComboboxStyle) ComboboxStyle {
	t := ctx.Theme()
	return layered(ctx, comboboxDefaults(t), ComboboxStyle.merge, ComboboxStyle.plain,
		themeLayer(t.Combobox), instanceLayer(inst))
}

// comboItems are the suggestions of a query, as the last request returned them.
type comboItems[T comparable] struct {
	query  string
	opts   []Option[T]
	failed bool // the request for query failed: there are no opts, and the error row shows
	done   bool // the suggestions of query arrived; the zero value has asked for nothing yet
}

// comboState is what a combobox keeps between renders, on the UI thread: the
// sequence numbers that tell a current request, edit and announcement from a stale
// one, the cancel of the pending async request, and the option last chosen.
type comboState[T comparable] struct {
	inputSeq    uint64
	reqSeq      uint64
	announceSeq uint64
	cancel      context.CancelFunc
	picked      *Option[T]
	// revealIdx is the row the keys have just moved the highlight to, plus one (zero for
	// none): that row is scrolled into view by the next paint. It is cleared when read.
	revealIdx int
}

// ComboboxElement is an editable text field with a list of suggestions below it. As
// the user types, the suggestions for the text are found and the list opens; the
// highlighted suggestion is the one the keys have reached, and focus stays in the
// text (see docs/widgets-selection.md, Combobox). The text is the signal the field reads and writes.
//
// The suggestions come from a function of the query (see Combobox), or from a
// function that runs in the background (see SuggestAsync), whose results are
// discarded when a newer query has started.
type ComboboxElement[T comparable] struct {
	text        *Signal[string]
	suggest     func(query string) []Option[T]
	async       func(ctx context.Context, query string) ([]Option[T], error)
	onPick      func(T)
	free        bool
	minChars    int
	debounce    time.Duration
	maxVisible  int
	empty       string
	errorText   string
	placeholder string
	name        string // the accessible name, see Label
	labelledBy  []string
	icon        *icons.Icon
	key         string
	disabled    bool
	size        sizing
	styles      ComboboxStyle
	render      func(Option[T], PartState) Element
	rules       []Validator[string] // the validators of the text (see Validate)
	onChange    func(string)
	focus       *FocusHandle // the editor's focus handle, see FocusHandle
	wire        fieldWire    // what a Field gives the field (see Field)
	ctlSize     controlSize  // see Small and Large
	tooltip     string       // see Tooltip
}

// Tooltip shows s below the field after the pointer rests on it. Assistive
// technology reads it as the field's description, unless it has another.
func (c *ComboboxElement[T]) Tooltip(s string) *ComboboxElement[T] {
	c.tooltip = s
	return c
}

// Small makes the field compact: 28 px high with smaller text, like
// Button.Small and TextInput.Small, for toolbars and dense forms.
func (c *ComboboxElement[T]) Small() *ComboboxElement[T] {
	c.ctlSize = sizeSmall
	return c
}

// Large makes the field larger: 40 px high with 16 px text, like
// Button.Large and TextInput.Large, for touch-friendly or prominent forms.
// Small and Large replace each other.
func (c *ComboboxElement[T]) Large() *ComboboxElement[T] {
	c.ctlSize = sizeLarge
	return c
}

// Combobox creates an editable text field that reads and writes text, with the
// suggestions that suggest returns for the text the user has typed. suggest runs on
// the UI thread at each change of the text, unless Debounce or SuggestAsync says
// otherwise. A nil suggest is allowed with SuggestAsync.
//
// Free text is allowed by default. AllowFree(false) makes the field choose only from
// its suggestions (see AllowFree).
func Combobox[T comparable](text *Signal[string], suggest func(query string) []Option[T]) *ComboboxElement[T] {
	return &ComboboxElement[T]{text: text, suggest: suggest, free: true, maxVisible: comboboxMaxVisible}
}

// formCheck and formWire make a combobox a formControl (see Field).
func (c *ComboboxElement[T]) formCheck() error { return firstError(c.text.Get(), c.rules) }

func (c *ComboboxElement[T]) formWire(w fieldWire) Element {
	cc := *c
	cc.wire = w
	return &cc
}

func (*ComboboxElement[T]) isElement()           {}
func (c *ComboboxElement[T]) elementKey() string { return c.key }

// Key identifies the combobox among its siblings (see DivElement.Key).
func (c *ComboboxElement[T]) Key(k string) *ComboboxElement[T] { c.key = k; return c }

// Label names the field for assistive technology, such as "City". The placeholder
// is not used as the name.
func (c *ComboboxElement[T]) Label(name string) *ComboboxElement[T] { c.name = name; return c }

// LabelledBy names the field by the text of the elements with the given keys; see
// DivElement.LabelledBy.
func (c *ComboboxElement[T]) LabelledBy(keys ...string) *ComboboxElement[T] {
	c.labelledBy = keys
	return c
}

// Placeholder is the text shown while the field is empty.
func (c *ComboboxElement[T]) Placeholder(s string) *ComboboxElement[T] { c.placeholder = s; return c }

// Icon puts ic at the start of the field, as TextInput.Icon does.
func (c *ComboboxElement[T]) Icon(ic *icons.Icon) *ComboboxElement[T] { c.icon = ic; return c }

// Disabled shows the field, with its text, and does not let it change.
func (c *ComboboxElement[T]) Disabled(d bool) *ComboboxElement[T] { c.disabled = d; return c }

// OnChange runs fn with the text after each change the user makes to it, as TextInput's
// OnChange does. It does not run for a suggestion that is chosen, which sets the text.
func (c *ComboboxElement[T]) OnChange(fn func(string)) *ComboboxElement[T] { c.onChange = fn; return c }

// FocusHandle lets code focus the text (see NewFocusHandle).
func (c *ComboboxElement[T]) FocusHandle(h *FocusHandle) *ComboboxElement[T] { c.focus = h; return c }

// AllowFree sets whether the text may be something other than a suggestion. With
// free text (the default), the text is kept as the user leaves it, and OnPick runs
// only when a suggestion is chosen. With AllowFree(false), the field chooses only
// from its suggestions: when it loses focus, a text that is not the label of the
// option last chosen (or of a suggestion that matches it exactly, ignoring case) is
// reverted to that option's label, or cleared when no option was chosen. The
// reverted text is what a Validate rule and a Form see, so Required makes an
// unchosen field an error.
func (c *ComboboxElement[T]) AllowFree(on bool) *ComboboxElement[T] { c.free = on; return c }

// MinChars is the number of characters the text needs before the field suggests
// anything. The list is closed until then. The default is 0.
func (c *ComboboxElement[T]) MinChars(n int) *ComboboxElement[T] { c.minChars = n; return c }

// Debounce waits for the user to stop typing for d before it asks for suggestions,
// for a suggest function that is expensive. Zero (the default) asks at once. Keys
// that need the suggestions ask at once too.
func (c *ComboboxElement[T]) Debounce(d time.Duration) *ComboboxElement[T] { c.debounce = d; return c }

// MaxVisible sets how many rows the list shows at once (8 by default): the list is at
// most that many rows high and scrolls to the rest. Every suggestion is in the list and
// in the accessibility tree; the rows are not virtualized, so a suggest function should
// still narrow its results to a few dozen. The keys scroll the highlighted row into view.
func (c *ComboboxElement[T]) MaxVisible(n int) *ComboboxElement[T] {
	if n > 0 {
		c.maxVisible = n
	}
	return c
}

// EmptyText sets the text of the slot shown when there are no suggestions, such as
// "No matching projects". The default is "No results".
func (c *ComboboxElement[T]) EmptyText(s string) *ComboboxElement[T] { c.empty = s; return c }

// ErrorText sets the text of the row shown when an async suggest function fails, such as
// "Could not reach the server". The default is "Couldn't load suggestions". The text is
// announced politely. The field stays usable: its text is free, and editing it asks again.
func (c *ComboboxElement[T]) ErrorText(s string) *ComboboxElement[T] { c.errorText = s; return c }

// OnPick runs fn with the value of the suggestion that the user chose, by a click or
// by Enter. The text is set to the suggestion's label first.
func (c *ComboboxElement[T]) OnPick(fn func(T)) *ComboboxElement[T] { c.onPick = fn; return c }

// SuggestAsync finds the suggestions in the background, instead of with the suggest
// function of Combobox. fn runs on its own goroutine with a context that is canceled
// when a newer query starts, and it must return promptly once that happens. Its
// results are shown only if they are for the latest query; the others are dropped.
// An error shows the error row (see ErrorText). The list shows a ProgressRing while fn
// runs.
func (c *ComboboxElement[T]) SuggestAsync(fn func(ctx context.Context, query string) ([]Option[T], error)) *ComboboxElement[T] {
	c.async = fn
	return c
}

// Validate sets the validators of the text: the field shows the first error of them
// once the user has changed the text and left the field, and on submit when it is in
// a Form (see Field and Validator). An empty text passes MinLength and Pattern, so
// pair those with Required when the field must be filled in.
func (c *ComboboxElement[T]) Validate(rules ...Validator[string]) *ComboboxElement[T] {
	c.rules = rules
	return c
}

// RenderOption replaces the label of each suggestion with what fn returns, as
// Select.RenderOption does. The state's Focused means the suggestion is highlighted.
// The content is hidden from assistive technology, which reads the label.
func (c *ComboboxElement[T]) RenderOption(fn func(opt Option[T], state PartState) Element) *ComboboxElement[T] {
	c.render = fn
	return c
}

// The sizing methods give the field a size. The field is textFieldWidth wide at the
// Text size of 100 percent by default; Width(Full) fills its container. The list is
// at least as wide as the field. See Sizing in docs/styling.md.
func (c *ComboboxElement[T]) Width(l Length) *ComboboxElement[T] { c.size.setWidth(l); return c }

// Height sets the height of the combobox.
func (c *ComboboxElement[T]) Height(l Length) *ComboboxElement[T] { c.size.setHeight(l); return c }

// MinWidth sets the smallest width of the combobox.
func (c *ComboboxElement[T]) MinWidth(l Length) *ComboboxElement[T] { c.size.setMinWidth(l); return c }

// MinHeight sets the smallest height of the combobox.
func (c *ComboboxElement[T]) MinHeight(l Length) *ComboboxElement[T] {
	c.size.setMinHeight(l)
	return c
}

// MaxWidth sets the largest width of the combobox.
func (c *ComboboxElement[T]) MaxWidth(l Length) *ComboboxElement[T] { c.size.setMaxWidth(l); return c }

// MaxHeight sets the largest height of the combobox.
func (c *ComboboxElement[T]) MaxHeight(l Length) *ComboboxElement[T] {
	c.size.setMaxHeight(l)
	return c
}

// Grow makes the combobox take a share of the free space on its parent's main axis.
func (c *ComboboxElement[T]) Grow(f float32) *ComboboxElement[T] { c.size.setGrow(f); return c }

// GridSpan sets how many columns and rows the combobox covers in its parent's grid.
func (c *ComboboxElement[T]) GridSpan(cols, rows int) *ComboboxElement[T] {
	c.size.setSpan(cols, rows)
	return c
}

// Styles sets the look of the parts of the combobox, for this instance only. Set
// fields win over the theme's Combobox and the defaults; see ComboboxStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (c *ComboboxElement[T]) Styles(st ComboboxStyle) *ComboboxElement[T] {
	c.styles = c.styles.merge(st)
	return c
}

// countText is the text announced for n suggestions.
func countText(n int, s *Strings) string {
	if n == 0 {
		return s.NoResults
	}
	return s.SuggestionCount(n)
}

func (c *ComboboxElement[T]) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		t := ctx.Theme()
		st := resolveCombobox(ctx, c.styles)
		lay := fieldLayout{width: Full, disabled: c.disabled, size: c.ctlSize}
		fst := resolveField(ctx, lay, st.Field)
		w := c.wire

		text := c.text.Get()
		strs := ctx.Strings()
		s := Local(ctx, "state", &comboState[T]{}).Peek()
		items := Local(ctx, "items", comboItems[T]{})
		open := Local(ctx, "open", false)
		loading := Local(ctx, "loading", false)
		hl := Local(ctx, "hl", -1)
		// notice is what the field says it reverted or cleared, on blur (see AllowFree).
		notice := Local(ctx, "notice", "")
		// ret is the editor's focus handle: the one a Field gave it, else this
		// field's own. The list hands focus back to it when the user presses in it.
		back := Local(ctx, "back", NewFocusHandle())
		ret := back.Peek()
		if c.focus != nil {
			ret = c.focus
		}
		if w.handle != nil {
			ret = w.handle
		}
		post := ctx.Post
		vis := items.Get().opts
		query := items.Get().query

		// shut closes the list. The highlight goes with it.
		shut := func() {
			open.Set(false)
			hl.Set(-1)
			s.revealIdx = 0
		}
		// move highlights row i (-1 for none), and has the row scrolled into view.
		move := func(i int) {
			hl.Set(i)
			if i >= 0 {
				s.revealIdx = i + 1
			}
		}
		// announce says how many suggestions there are, once they have settled.
		announce := func(n int) {
			s.announceSeq++
			seq := s.announceSeq
			time.AfterFunc(comboboxAnnounceDelay, func() {
				post(func() {
					if s.announceSeq == seq {
						Announce(countText(n, strs), access.Polite)
					}
				})
			})
		}
		// apply shows the suggestions of request seq, if it is still the latest. A failed
		// request shows the error row, which is announced at once.
		apply := func(seq uint64, q string, opts []Option[T], failed bool) {
			if seq != s.reqSeq {
				return
			}
			loading.Set(false)
			items.Set(comboItems[T]{query: q, opts: opts, failed: failed, done: true})
			if hl.Peek() >= len(opts) {
				hl.Set(-1)
			}
			switch {
			case failed && open.Peek():
				errText := c.errorText
				if errText == "" {
					errText = strs.CouldNotLoadSuggestions
				}
				Announce(errText, access.Polite)
			case open.Peek():
				announce(len(opts))
			}
		}
		// run asks for the suggestions of q: at once, or in the background for an
		// async function. A newer request makes the older one stale.
		run := func(q string) {
			s.reqSeq++
			seq := s.reqSeq
			if c.async == nil {
				if c.suggest != nil {
					apply(seq, q, c.suggest(q), false)
				}
				return
			}
			if s.cancel != nil {
				s.cancel()
			}
			rctx, cancel := context.WithCancel(context.Background())
			s.cancel = cancel
			loading.Set(true)
			fn := c.async
			go func() {
				opts, err := fn(rctx, q)
				post(func() {
					if rctx.Err() != nil {
						return // a newer request replaced this one
					}
					apply(seq, q, opts, err != nil)
				})
			}()
		}
		// pick chooses o: the text becomes its label, and the list closes.
		pick := func(o Option[T]) {
			if o.Disabled {
				return
			}
			s.picked = &o
			notice.Set("")
			shut()
			c.text.Set(o.Label)
			if w.onEdit != nil {
				w.onEdit()
			}
			if c.onPick != nil {
				c.onPick(o.Value)
			}
		}
		// typed runs when the user changes the text. The list opens for enough text,
		// and the suggestions come now or after the debounce.
		typed := func(v string) {
			if c.onChange != nil {
				c.onChange(v)
			}
			s.inputSeq++
			seq := s.inputSeq
			notice.Set("")
			hl.Set(-1)
			if utf8.RuneCountInString(v) < c.minChars {
				s.reqSeq++
				if s.cancel != nil {
					s.cancel()
					s.cancel = nil
				}
				loading.Set(false)
				shut()
				items.Set(comboItems[T]{query: v})
				return
			}
			open.Set(true)
			if c.debounce <= 0 {
				run(v)
				return
			}
			time.AfterFunc(c.debounce, func() {
				post(func() {
					if s.inputSeq == seq {
						run(v)
					}
				})
			})
		}
		// ensure asks for the suggestions of the current text when they are not
		// there yet, for a key that opens the list.
		ensure := func() bool {
			cur := c.text.Peek()
			if utf8.RuneCountInString(cur) < c.minChars {
				return false
			}
			if it := items.Peek(); !it.done || it.query != cur || loading.Peek() {
				s.inputSeq++ // a pending debounced request is now stale
				run(cur)
			}
			return true
		}
		// openList opens the list. dir moves the highlight to the first (1) or last (-1)
		// suggestion; 0 opens it with nothing highlighted, as Alt+Down does.
		openList := func(dir int) {
			if !ensure() {
				return
			}
			vis := items.Peek().opts
			if len(vis) == 0 && !loading.Peek() {
				return
			}
			open.Set(true)
			hl.Set(-1)
			if dir != 0 {
				move(listStep(vis, -1, dir))
			}
		}
		// restrict reverts the text when the field loses focus, for a field that
		// allows only its suggestions (see AllowFree).
		restrict := func() {
			if c.free {
				return
			}
			cur := c.text.Peek()
			if s.picked != nil && cur == s.picked.Label {
				return
			}
			for _, o := range items.Peek().opts {
				if !o.Disabled && strings.EqualFold(o.Label, cur) {
					pick(o)
					return
				}
			}
			// The text is not a choice. It is reverted to the last choice, or cleared, and
			// the change is said both aloud and on the field, so the user is not left to
			// wonder where the text went. An empty text is reverted without a message.
			msg := ""
			if cur != "" {
				msg = strs.NotInList
				if s.picked != nil {
					msg = strs.NotInListReverted(s.picked.Label)
				}
			}
			if s.picked != nil {
				c.text.Set(s.picked.Label)
			} else {
				c.text.Set("")
			}
			if msg != "" {
				notice.Set(msg)
				Announce(msg, access.Polite)
			}
		}
		// clearText empties the text on Escape, when the list is closed.
		clearText := func() {
			s.inputSeq++
			s.reqSeq++
			s.picked = nil
			notice.Set("")
			items.Set(comboItems[T]{})
			shut()
			c.text.Set("")
			if w.onEdit != nil {
				w.onEdit()
			}
		}

		// The field's own frame, the editor in it, and the access state the field reports.
		root := Div().Column().KeyContext("Combobox").asPart("Combobox.Root")
		if c.key != "" {
			root.Key(c.key)
		}
		if c.size.set&hasWidth == 0 {
			root.style.Width = Px(textFieldWidth * textScale(ctx))
		}
		a := root.access()
		root.Role(access.RoleComboBox).AccessExpanded(open.Get())
		if c.tooltip != "" && w.description == "" {
			root.Description(c.tooltip) // the combo box is the node assistive technology reads
		}
		name, labelledBy := c.name, c.labelledBy
		if w.label != "" {
			name, labelledBy = w.label, w.labelledBy
		}
		if name != "" {
			root.Label(name)
		}
		if len(labelledBy) > 0 {
			root.LabelledBy(labelledBy...)
		}
		// The field's description is its help text and what it last said it reverted, so
		// the message is read with the field too.
		if desc := describe(w.description, notice.Get()); desc != "" {
			root.Description(desc)
		}
		a.value = text
		// Expand and Collapse from assistive technology open and close the list, through
		// the pattern's own handler, not the click (see access.go, setExpanded).
		a.setExpanded = func(o bool) {
			if o {
				openList(0)
			} else {
				shut()
			}
		}
		a.invalid, a.required = w.invalid, w.required

		if c.disabled {
			a.disabled = true
			root.Cursor(CursorNotAllowed)
			frame := Div().asPart("Combobox.Frame").Style(fst.Frame).Style(fst.FrameDisabled).
				Child(Div().asPart("Combobox.Text").Style(fst.Text).Grow(1).Child(Text(text).NoWrap()))
			root.Child(frame)
			return c.size.apply(root)
		}

		fo := fieldOptions{tooltip: c.tooltip, value: c.text, placeholder: c.placeholder, name: name, labelledBy: labelledBy,
			icon: c.icon, widget: "Combobox", handle: ret, wire: w, onChange: typed,
			onBlur: func() {
				shut()
				restrict()
			}}
		fo.size.setWidth(Full)
		root.Child(fo.expand(ctx, fst, false, 0))
		// The message the field last said: the same look as a field's error.
		root.Child(Div().Display(notice.Get() != "").Row().Align(Center).Gap(6).Style(st.Error).
			Child(Icon(icons.Alert).Size(14).Color(messageIconColor(t, st.Error))).
			Child(Text(notice.Get()).NoWrap()))

		// Up and Down move the highlight, and open the list. They are actions, so the
		// editor's own caret keys do not take them.
		root.OnAction(comboboxDown, func() {
			if !open.Peek() {
				openList(1)
				return
			}
			move(listStep(vis, hl.Peek(), 1))
		})
		root.OnAction(comboboxUp, func() {
			if !open.Peek() {
				openList(-1)
				return
			}
			i := listStep(vis, hl.Peek(), -1)
			if i == hl.Peek() && i >= 0 {
				i = -1 // back to the text, from the first suggestion
			}
			move(i)
		})
		if open.Get() {
			// Enter chooses the highlighted suggestion, or keeps the text and closes.
			root.OnAction(comboboxEnter, func() {
				if i := hl.Peek(); i >= 0 && i < len(vis) {
					pick(vis[i])
					return
				}
				shut()
			})
		}
		if open.Get() || c.text.Get() != "" {
			root.OnAction(comboboxEscape, func() {
				if open.Peek() {
					shut()
					return
				}
				clearText()
			})
		}
		// A click opens the list, and never closes it: the press places the caret in the
		// text, and the list closes by Escape, Tab, Enter, a press outside it, or blur.
		root.OnClick(func() {
			if !open.Peek() {
				openList(0)
			}
		})
		root.OnKeyDown(func(e *KeyEvent) {
			switch {
			case e.Key == input.KeyTab:
				// Tab closes the list and leaves the text as it is: Enter chooses a
				// suggestion, and Tab only moves on (see docs/widgets-selection.md).
				shut()
			case e.Key == input.KeyDown && e.Mods == input.ModAlt && !e.Repeat:
				openList(0)
				e.StopPropagation()
			case e.Key == input.KeyUp && e.Mods == input.ModAlt && !e.Repeat:
				shut()
				e.StopPropagation()
			}
		})

		if open.Get() {
			// The row the keys reached last is scrolled into view, once.
			reveal := s.revealIdx
			s.revealIdx = 0
			maxH := float32(c.maxVisible)*comboboxRowHeight*textScale(ctx) + 8
			root.Child(c.popup(t, st, vis, query, hl.Get(), reveal, loading.Get(), items.Get().failed,
				c.errorText, maxH, s.picked, pick, shut, func(i int) { hl.Set(i) }, ret, strs))
		}
		return c.size.apply(root)
	})
}

// popup returns the open list: an overlay below the field (above it when there is no
// room), at least as wide as the field and at most maxH high, so that it scrolls. Its rows
// are list items, the one the keys have reached is the highlighted row, and the one the
// text matches exactly is the selected one. The list takes no focus; a press in it hands
// focus back to the text. reveal is the row (plus one, zero for none) that the keys moved
// to, and it is scrolled into view. A failed request shows the error row instead of the
// empty slot.
func (c *ComboboxElement[T]) popup(t *Theme, st ComboboxStyle, vis []Option[T], query string, hl, reveal int,
	loading, failed bool, errorText string, maxH float32, picked *Option[T], pick func(Option[T]), shut func(),
	highlight func(int), ret *FocusHandle, strs *Strings) Element {
	name := c.name
	if name == "" {
		name = strs.Suggestions
	}
	panel := Div().asPart("Combobox.Popup").Key("jamb.listbox").Column().Style(st.Popup).
		Enter(Fade().Slide(0, -6).Grow(0.98), Animation{Duration: t.Motion.enter(140 * time.Millisecond)}).
		Exit(Fade().Grow(0.98), Animation{Duration: t.Motion.exit(90 * time.Millisecond)}).
		Overlay(BelowStart).OverlayGap(4).
		Role(access.RoleList).SelectionContainer().Label(name).
		MaxHeight(Px(maxH)).ScrollY().
		OnClickOutside(shut).
		OnClick(func() {}). // a press on the list's padding must not reach the field
		KeepFocus().        // a press in the list leaves the text focused, so the list stays open for the click
		OnMouseDown(func(*MouseEvent) { ret.Focus() })
	panel.matchAnchor = true // at least as wide as the field, whatever its width
	if loading {
		panel.Child(Div().asPart("Combobox.Loading").Style(st.Loading).
			Child(ProgressRing().Size(RingSmall).Label(strs.LoadingSuggestions)))
	}
	switch {
	case failed && !loading:
		if errorText == "" {
			errorText = strs.CouldNotLoadSuggestions
		}
		panel.Child(Div().asPart("Combobox.Error").Style(st.Error).
			Child(Icon(icons.Alert).Size(14).Color(messageIconColor(t, st.Error))).
			Child(Text(errorText).NoWrap()))
	case len(vis) == 0 && !loading:
		empty := c.empty
		if empty == "" {
			empty = strs.NoResults
		}
		panel.Child(Div().asPart("Combobox.Empty").Style(st.Empty).Child(Text(empty).NoWrap()))
	}
	hlStyle := Highlight{Color: st.Match.TextColor, Background: st.Match.Background, Underline: t.HighContrast}
	for i, o := range vis {
		i, o := i, o
		on := picked != nil && picked.Value == o.Value
		hi := i == hl && !o.Disabled
		reveals := i == reveal-1
		draw := func(ps PartState) *DivElement {
			style := st.Option
			if hi {
				style = style.Merge(st.OptionHighlighted)
			}
			if o.Disabled {
				style = style.Merge(st.OptionDisabled)
			}
			mark := Div().Size(Px(16), Px(16)).Center().Shrink(0)
			if on {
				mark.Child(Icon(icons.Check).Size(14))
			}
			row := Div().asPart("Combobox.Option").Style(style).
				Role(access.RoleListItem).AccessSelected(on).Label(o.Label).Child(mark)
			row.revealRow = reveals
			reveals = false // a render slot's later renders do not scroll again
			if c.render != nil {
				row.Child(Div().AccessHidden().Child(c.render(o, ps)))
			} else {
				row.Child(Text(o.Label).NoWrap().HighlightMatches(query, hlStyle))
			}
			row.access().highlighted = hi
			if o.Disabled {
				row.access().disabled = true
			} else {
				row.OnMouseEnter(func() { highlight(i) }).OnClick(func() { pick(o) })
			}
			return row
		}
		if c.render == nil {
			panel.Child(draw(PartState{}))
			continue
		}
		// With a render slot the row is its own component, so that it re-renders when
		// its pointer state changes, and the slot gets the new state.
		panel.Child(Component(func(ctx *Context) Element {
			return draw(PartState{Hovered: ctx.hovered(), Pressed: ctx.pressed(), Focused: hi,
				Selected: on, Disabled: o.Disabled, HighContrast: t.HighContrast})
		}).Key(fmt.Sprintf("jamb.listbox/%d", i)))
	}
	return panel
}
