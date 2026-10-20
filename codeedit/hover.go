package codeedit

import (
	"context"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
)

// A Hover is what a HoverProvider says about a range of the text: plain text, or an element of
// its own, which the editor shows in a popup under the range, after the diagnostics there.
type Hover struct {
	// From and To are the byte range the hover is about. The popup closes when the pointer
	// leaves it. An empty range is the word under the pointer.
	From, To int
	// Text is the text of the hover, which is also what is announced when the hover is opened
	// from the keyboard.
	Text string
	// Content, when set, builds the element shown in place of Text, such as rendered Markdown.
	Content func() ui.Element
}

// A HoverProvider answers what the text at an offset is, as a language server's hover does. Its
// Hover runs on a goroutine of its own, with a context that is canceled when the answer is no
// longer wanted. An empty Hover, or an error, shows nothing but the diagnostics there.
type HoverProvider interface {
	Hover(ctx context.Context, q Query) (Hover, error)
}

// HoverFunc is a function that is a HoverProvider.
type HoverFunc func(ctx context.Context, q Query) (Hover, error)

// Hover calls f.
func (f HoverFunc) Hover(ctx context.Context, q Query) (Hover, error) { return f(ctx, q) }

// defaultHoverDelay is how long the pointer rests on the text before the hover opens.
const defaultHoverDelay = 500 * time.Millisecond

// afterFunc runs f after d on a goroutine of its own, as time.AfterFunc does. The tests replace
// it, so that they do not wait.
var afterFunc = func(d time.Duration, f func()) { time.AfterFunc(d, f) }

// Hover sets the provider of the hover popup. When the pointer rests on the text, or on Ctrl+K
// Ctrl+I at the caret, the editor asks p about the offset and shows its answer under the range,
// with the messages of the diagnostics there. Without a provider the popup shows the diagnostics
// alone. Escape, an edit, the loss of focus and a move of the pointer off the range close it.
func (e *EditorElement) Hover(p HoverProvider) *EditorElement { e.hoverP = p; return e }

// HoverDelay sets how long the pointer rests on the text before the hover opens. It is 500
// milliseconds by default.
func (e *EditorElement) HoverDelay(d time.Duration) *EditorElement {
	e.hoverDelay = max(d, 0)
	return e
}

// hoverState is the hover popup of an editor: whether it is open, the range it is about, the
// diagnostics and the provider's answer it shows, the request on its way, and the pointer rest
// that will open it, which a later move makes stale.
type hoverState struct {
	open     bool
	from, to int
	diags    []Diagnostic
	answer   Hover
	slot     asyncSlot[Hover]
	rest     uint64
}

// hoverRest schedules the hover at the byte offset p, where the pointer has come to rest, after
// the hover delay. A pointer on the range of the open hover keeps it; a pointer elsewhere closes it.
func (vs *viewState) hoverRest(p int) {
	hv := &vs.hover
	if hv.open && p >= hv.from && p <= hv.to {
		return
	}
	vs.closeHover()
	hv.rest++
	if vs.hoverP == nil && len(vs.doc.DiagnosticsAt(p)) == 0 {
		return
	}
	rest := hv.rest
	afterFunc(vs.hoverDelay, func() {
		if c := vs.latest.Load(); c != nil {
			c.Post(func() {
				if hv.rest == rest {
					vs.openHover(p, false)
				}
			})
		}
	})
}

// pointerRest schedules the hover at the text under the pointer of ev, or closes the hover when
// the pointer is off the text: in the gutter, or past the end of a line.
func (vs *viewState) pointerRest(ev *ui.MouseEvent) {
	if !vs.hover.open && vs.hoverP == nil && len(vs.doc.diags) == 0 {
		return // nothing to show
	}
	_, p, _, ok := vs.hitAt(ev.LocalX, ev.LocalY)
	if !ok || ev.LocalX < vs.geo.textX || ev.LocalX > vs.caretRect(p).X+vs.geo.charW {
		vs.closeHover()
		return
	}
	vs.hoverRest(p)
}

// openHover opens the hover at the byte offset p: at once with the diagnostics there, and with the
// provider's answer when it comes. announce says the popup's text politely when it is shown, for
// a hover opened from the keyboard.
func (vs *viewState) openHover(p int, announce bool) {
	hv := &vs.hover
	hv.rest++ // a pending rest is stale now
	hv.slot.stop()
	hv.open, hv.answer = false, Hover{}
	hv.diags = vs.doc.DiagnosticsAt(p)
	hv.from, hv.to = vs.wordAround(p)
	for _, g := range hv.diags {
		hv.from, hv.to = min(hv.from, g.From), max(hv.to, g.To)
	}
	show := func() {
		hv.open = len(hv.diags) > 0 || hv.answer.Text != "" || hv.answer.Content != nil
		if hv.open && announce {
			if s := vs.hoverText(); s != "" {
				ui.Announce(s, access.Polite)
			}
		}
		vs.touch()
	}
	if vs.hoverP == nil {
		show()
		return
	}
	if len(hv.diags) > 0 {
		hv.open = true // the diagnostics show while the provider answers
		vs.touch()
	}
	prov, q := vs.hoverP, vs.request(p)
	hv.slot.run(vs, func(ctx context.Context) (Hover, error) { return prov.Hover(ctx, q) }, func(h Hover, err error) {
		if err == nil {
			hv.answer = h
			if h.To > h.From {
				hv.from, hv.to = min(hv.from, h.From), max(hv.to, h.To)
			}
		}
		show()
	})
}

// closeHover closes the hover popup and drops its request, and reports whether it was open.
func (vs *viewState) closeHover() bool {
	hv := &vs.hover
	hv.rest++
	hv.slot.stop()
	if !hv.open {
		return false
	}
	hv.open = false
	vs.touch()
	return true
}

// wordAround returns the word at the byte offset p, or p alone when there is none.
func (vs *viewState) wordAround(p int) (int, int) {
	doc := vs.doc
	t := doc.text.Snapshot()
	row := doc.RowOf(p)
	ls, le := doc.LineStart(row), doc.LineEnd(row)
	from, to := p, p
	for from > ls {
		r, n := runeBefore(t, from)
		if charClass(r) != 1 {
			break
		}
		from -= n
	}
	for to < le {
		r, n := runeAt(t, to)
		if charClass(r) != 1 {
			break
		}
		to += n
	}
	return from, to
}

// hoverText returns the text of the open hover: the diagnostics, as they are announced, and the
// provider's text.
func (vs *viewState) hoverText() string {
	strs := vs.uiStrings()
	var parts []string
	for _, g := range vs.hover.diags {
		parts = append(parts, strs.CodeEditorDiagnostic(severityName(g.Severity, strs), vs.doc.RowOf(g.From)+1, g.Message))
	}
	if vs.hover.answer.Text != "" {
		parts = append(parts, vs.hover.answer.Text)
	}
	return strings.Join(parts, "\n")
}

// uiStrings returns the strings of the editor's latest render, or the English ones before one.
func (vs *viewState) uiStrings() *ui.Strings {
	if vs.ctx != nil {
		return vs.ctx.Strings()
	}
	return ui.EnglishStrings()
}

// hoverPopup returns the open hover popup: an overlay under the start of its range, with the
// messages of the diagnostics, each marked with its severity and its source, and then the
// provider's answer.
func (vs *viewState) hoverPopup(st Style) ui.Element {
	hv := &vs.hover
	strs := vs.uiStrings()
	panel := ui.Div().Key("codeedit.hover").Column().Style(st.Popup).
		Overlay(ui.BelowStart).OverlayGap(2).MaxWidth(ui.Px(560)).
		Role(access.RoleTooltip).
		OnClickOutside(func() { vs.closeHover() })
	for _, g := range hv.diags {
		msg := severityName(g.Severity, strs) + ": " + g.Message
		if src := strings.TrimSpace(g.Source + " " + g.Code); src != "" {
			msg += " (" + src + ")"
		}
		panel.Child(ui.Div().Style(st.PopupItem).TextColor(diagnosticStyle(st, g.Severity).TextColor).Child(ui.Text(msg)))
	}
	switch a := hv.answer; {
	case a.Content != nil:
		panel.Child(ui.Div().Style(st.PopupItem).Child(a.Content()))
	case a.Text != "":
		panel.Child(ui.Div().Style(st.PopupItem).Child(ui.Text(a.Text)))
	}
	return vs.popupAnchor(hv.from, panel)
}

// popupAnchor returns a box one pixel wide and a row high at the byte offset off, which a popup's
// overlay hangs from, so that it opens under the text. The box is placed from the geometry of
// the last paint.
func (vs *viewState) popupAnchor(off int, popup ui.Element) ui.Element {
	r := vs.caretRect(off)
	return ui.Div().Key("codeedit.anchor").Absolute().Left(max(r.X, 0)).Top(r.Y).
		Size(ui.Px(1), ui.Px(max(r.H, 1))).Child(popup)
}
