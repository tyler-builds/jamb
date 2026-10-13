package views

import (
	"strings"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// paletteRows is how many commands the palette lists at once.
const paletteRows = 8

// openPalette opens the command palette. Its commands are the actions that
// are available where focus is now, and focus goes back there when it closes.
func (a *App) openPalette(ctx *ui.Context) {
	a.paletteFocus = ctx.FocusedHandle()
	a.paletteActions = nil
	for _, act := range ctx.AvailableActions() {
		if !act.Hidden() {
			a.paletteActions = append(a.paletteActions, act)
		}
	}
	a.paletteQuery.Set("")
	a.paletteSel.Set(0)
	a.paletteOpen.Set(true)
}

// closePalette closes the palette and leaves focus where it is. A click
// outside does that: the click itself sets focus.
func (a *App) closePalette() { a.paletteOpen.Set(false) }

// dismissPalette closes the palette and gives focus back to the field it
// opened over.
func (a *App) dismissPalette() {
	a.closePalette()
	a.paletteFocus.FocusNow()
}

// visible returns the commands whose label contains q, ignoring case, up to
// paletteRows of them.
func (a *App) visible(q string) []*ui.Action {
	q = strings.ToLower(q)
	var out []*ui.Action
	for _, act := range a.paletteActions {
		if strings.Contains(strings.ToLower(act.Label()), q) {
			out = append(out, act)
		}
	}
	return out[:min(len(out), paletteRows)]
}

// palette is the anchor for the open palette, or nothing while it is closed.
// The palette floats below the top of the window, like a menu.
func (a *App) palette() ui.Element {
	return ui.Component(func(ctx *ui.Context) ui.Element {
		if !a.paletteOpen.Get() {
			return nil
		}
		t := ctx.Theme()
		matches := a.visible(a.paletteQuery.Get())
		sel := a.paletteSel.Get()

		rows := ui.Div().Gap(2)
		for i, act := range matches {
			rows.Child(a.paletteRow(ctx, t, act, i == sel))
		}
		if len(matches) == 0 {
			rows.Child(ui.Div().Padding(12).Child(ui.Text("No matching commands").TextColor(t.TextMuted)))
		}

		panel := ui.Div().Key("palette").KeyContext("Palette").Overlay(ui.BelowCenter).Width(ui.Px(520)).
			Background(t.Surface).Border(1, t.Border).Radius(10).Shadow(ui.ShadowLg).Padding(6).Gap(4).
			Enter(ui.Fade().Slide(0, -6).Grow(0.98), ui.Animation{Duration: 140 * time.Millisecond}).
			Exit(ui.Fade().Grow(0.98), ui.Animation{Duration: 90 * time.Millisecond}).
			OnClickOutside(a.closePalette).
			OnAction(PaletteUp, func() { a.paletteSel.Set(max(a.paletteSel.Peek()-1, 0)) }).
			OnAction(PaletteDown, func() {
				last := max(len(a.visible(a.paletteQuery.Peek()))-1, 0)
				a.paletteSel.Set(min(a.paletteSel.Peek()+1, last))
			}).
			OnAction(PaletteRun, func() {
				list, i := a.visible(a.paletteQuery.Peek()), a.paletteSel.Peek()
				a.dismissPalette()
				if i < len(list) {
					ctx.Dispatch(list[i]) // runs where the user was, not in the palette
				}
			}).
			OnAction(PaletteClose, a.dismissPalette).
			Child(ui.TextInput(a.paletteQuery).Label("Command").Borderless().AutoFocus().Placeholder("Type a command…").
				Style(ui.Style{Padding: ui.XY(10, 6)}).
				OnChange(func(string) { a.paletteSel.Set(0) })).
			Child(rows)

		return ui.Div().Absolute().Top(48).Left(0).Right(0).Height(ui.Px(0)).Child(panel)
	})
}

// paletteRow lists one command: its label, and its key binding on the right.
func (a *App) paletteRow(ctx *ui.Context, t *ui.Theme, act *ui.Action, selected bool) ui.Element {
	return ui.Div().Key("palette-"+act.Name()).Row().Align(ui.Center).Justify(ui.SpaceBetween).Gap(12).
		PaddingX(10).PaddingY(7).Radius(6).Transition(fast).
		Hover(ui.Style{Background: t.SurfaceHover}).
		When(selected, func(d *ui.DivElement) { d.Background(t.SurfaceActive) }).
		OnClick(func() {
			a.dismissPalette()
			ctx.Dispatch(act)
		}).
		Child(ui.Text(act.Label()).Grow(1).Truncate()).
		Child(ui.Text(ctx.KeysFor(act)).FontSize(12).TextColor(t.TextSubtle).NoWrap())
}
