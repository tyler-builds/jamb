package ui

import (
	"fmt"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/icons"
	"github.com/tyler-builds/jamb/input"
)

// Step is one step of a Stepper: its title, an optional description under the
// title and an optional icon, which the step's indicator shows in place of its
// number until the step is complete.
type Step struct {
	// Title is the name of the step.
	Title string
	// Description is a line of text under the title.
	Description string
	// Icon is shown in the indicator in place of the step's number.
	Icon *icons.Icon
}

// StepperStyle is the look of each part of a stepper. A part's style merges over
// the built-in defaults, then over the theme's Stepper, then over the Styles of
// the instance; see docs/styling.md. Stepper is the row (or the column, when
// Vertical) that holds the steps, and Step each step: its indicator and its text.
// StepHover, StepFocusVisible and StepDisabled are the states of a step that can
// be chosen (see OnSelect). Indicator is the circle with the step's number or
// icon, the look of a step that is not reached yet; IndicatorComplete is drawn
// over it for a step before the current one, which shows a check mark, and
// IndicatorCurrent for the current step. Title and Description are the texts of
// a step. Connector is the line between two steps, and ConnectorComplete is drawn
// over it after a complete step.
type StepperStyle struct {
	Stepper           Style // the row, or the column, that holds the steps
	Step              Style // each step: its indicator and its text
	StepHover         Style // a step the pointer is over, if it can be chosen
	StepFocusVisible  Style // a step that has keyboard focus
	StepDisabled      Style // a step of a disabled stepper that has OnSelect
	Indicator         Style // the circle with the number or the icon of a step not reached yet
	IndicatorComplete Style // the indicator of a step before the current one, over Indicator
	IndicatorCurrent  Style // the indicator of the current step, over Indicator
	Title             Style // the title of a step
	Description       Style // the description under the title
	Connector         Style // the line between two steps
	ConnectorComplete Style // the line after a complete step, over Connector
}

// merge returns s with the set fields of o applied on top, part by part.
func (s StepperStyle) merge(o StepperStyle) StepperStyle {
	return StepperStyle{
		Stepper:           s.Stepper.Merge(o.Stepper),
		Step:              s.Step.Merge(o.Step),
		StepHover:         s.StepHover.Merge(o.StepHover),
		StepFocusVisible:  s.StepFocusVisible.Merge(o.StepFocusVisible),
		StepDisabled:      s.StepDisabled.Merge(o.StepDisabled),
		Indicator:         s.Indicator.Merge(o.Indicator),
		IndicatorComplete: s.IndicatorComplete.Merge(o.IndicatorComplete),
		IndicatorCurrent:  s.IndicatorCurrent.Merge(o.IndicatorCurrent),
		Title:             s.Title.Merge(o.Title),
		Description:       s.Description.Merge(o.Description),
		Connector:         s.Connector.Merge(o.Connector),
		ConnectorComplete: s.ConnectorComplete.Merge(o.ConnectorComplete),
	}
}

// plain returns s as high contrast allows it (see plainStyle).
func (s StepperStyle) plain() StepperStyle {
	return StepperStyle{
		Stepper:           plainStyle(s.Stepper),
		Step:              plainStyle(s.Step),
		StepHover:         plainStyle(s.StepHover),
		StepFocusVisible:  plainStyle(s.StepFocusVisible),
		StepDisabled:      plainStyle(s.StepDisabled),
		Indicator:         plainStyle(s.Indicator),
		IndicatorComplete: plainStyle(s.IndicatorComplete),
		IndicatorCurrent:  plainStyle(s.IndicatorCurrent),
		Title:             plainStyle(s.Title),
		Description:       plainStyle(s.Description),
		Connector:         plainStyle(s.Connector),
		ConnectorComplete: plainStyle(s.ConnectorComplete),
	}
}

// stepperDefaults returns the built-in look of a stepper, laid out in a column
// when vertical, from the colors of t. A step not reached yet has an outlined
// circle in the muted text color, a complete step a circle filled with Accent and
// a check mark, and the current step an Accent outline on AccentSoft. Under high
// contrast the circles are outlined in the text color, a complete one is filled
// with Highlight, and the current one has a thicker Highlight outline, so the
// states differ by shape as well as by color.
func stepperDefaults(t *Theme, vertical bool) StepperStyle {
	ring := Shadow{Spread: 2, Color: t.FocusRing}
	d := StepperStyle{
		Stepper: Style{Direction: Row, Align: Center, Gap: 8},
		// A horizontal step can shrink, down to its indicator, so the stepper fits its
		// width; its title and description then end with an ellipsis.
		Step: Style{Direction: Row, Align: Center, Gap: 10, Padding: XY(6, 4), Radius: t.Radius / 2,
			MinWidth: Px(0), Transition: 110 * time.Millisecond},
		StepHover:        Style{Background: t.SurfaceHover},
		StepFocusVisible: Style{Shadows: []Shadow{ring}},
		Indicator: Style{Direction: Row, Align: Center, Justify: Center, Width: Px(28), Height: Px(28),
			NoShrink: true, Radius: 14, BorderWidth: 2, BorderColor: t.BorderStrong, Background: t.Surface,
			FontSize: 13, TextColor: t.TextMuted},
		IndicatorComplete: Style{Background: t.Accent, BorderColor: t.Accent, TextColor: t.AccentText},
		IndicatorCurrent:  Style{Background: t.AccentSoft, BorderColor: t.Accent, TextColor: t.Accent},
		Title:             Style{FontSize: 14, TextColor: t.Text},
		Description:       Style{FontSize: 12, TextColor: t.TextMuted},
		Connector:         Style{Height: Px(2), MinWidth: Px(24), Grow: 1, Radius: 1, Background: t.Border},
		ConnectorComplete: Style{Background: t.Accent},
	}
	if vertical {
		// The steps are stacked, and the line runs down from each indicator to the next.
		d.Stepper = Style{Direction: Column, Align: Start}
		d.Step.Align, d.Step.NoShrink = Start, true
		d.Connector = Style{Width: Px(2), MinHeight: Px(16), Grow: 1, Radius: 1, Margin: Edges{Top: 4, Bottom: 4},
			Background: t.Border}
	}
	if t.HighContrast {
		d.Indicator.BorderColor, d.Indicator.TextColor = t.Text, t.Text
		d.IndicatorCurrent = Style{Background: t.Surface, BorderColor: t.Accent, BorderWidth: 3, TextColor: t.Text}
		d.Connector.Background = t.Text
		// A disabled step is GrayText under high contrast, not dimmed.
		d.StepDisabled = Style{TextColor: t.Disabled}
	} else {
		d.StepDisabled = Style{Opacity: 0.5}
	}
	return d
}

// resolveStepper returns the look of a stepper: the defaults, merged with the
// theme's Stepper and then the instance's Styles. Under high contrast the two
// overrides are plain (see plainStyle).
func resolveStepper(ctx *Context, vertical bool, inst StepperStyle) StepperStyle {
	t := ctx.Theme()
	return layered(ctx, stepperDefaults(t, vertical), StepperStyle.merge, StepperStyle.plain,
		themeLayer(t.Stepper), instanceLayer(inst))
}

// StepperElement shows the progress through the steps of a process, such as the
// pages of a checkout. The steps before the current one are complete, and show a
// check mark; the current one is highlighted, and the later ones are muted. The
// steps are numbered from 0 in the current signal, and a current step equal to
// the number of steps marks every step complete.
//
// Without OnSelect the stepper only shows the progress, and it takes no focus.
// With OnSelect each step can be chosen: the stepper is one Tab stop, at the
// current step, the arrow keys move the focus between the steps, and Enter, Space
// or a click chooses the focused step.
//
// The stepper is a list named by its Label, and each step is an item of it named
// by its position, title and state, such as "Step 2 of 4: Shipping, current".
// The current step is selected in the list.
type StepperElement struct {
	current  *Signal[int]
	steps    []Step
	vertical bool
	key      string
	name     string // the accessible name of the list, see Label
	disabled bool
	onSelect func(int)
	styles   StepperStyle
	size     sizing // the stepper's sizes, from the sizing methods (see Sizing in docs/styling.md)
	tooltip  string // see Tooltip
	// focusHandle is the handle of the step that Tab reaches, see FocusHandle.
	focusHandle *FocusHandle
}

// Stepper creates a stepper over steps, shown left to right, with the step at
// index current (from 0) as the current one.
func Stepper(current *Signal[int], steps ...Step) *StepperElement {
	return &StepperElement{current: current, steps: steps}
}

func (*StepperElement) isElement()           {}
func (s *StepperElement) elementKey() string { return s.key }

// Key gives the stepper a stable identity. It also scopes the keys of its steps, so
// give two steppers over the same signal different keys. Without a key the signal's
// identity scopes them.
func (s *StepperElement) Key(k string) *StepperElement { s.key = k; return s }

// Label names the stepper for assistive technology, such as "Checkout steps". It
// defaults to the Steps text of the window's Strings.
func (s *StepperElement) Label(name string) *StepperElement { s.name = name; return s }

// Vertical stacks the steps from top to bottom, with the line between them running
// down from each indicator to the next, and the up and down arrows moving between
// them.
func (s *StepperElement) Vertical() *StepperElement { s.vertical = true; return s }

// OnSelect makes the steps choosable: a click, or Enter or Space on the focused
// step, runs fn with the index of the step. The current signal is not set: fn sets
// it when the step may be shown, so the app decides whether a later step can be
// reached yet. Left and Right (Up and Down when Vertical) move the focus to the
// previous and next step, without wrapping, and Home and End to the first and last.
func (s *StepperElement) OnSelect(fn func(i int)) *StepperElement { s.onSelect = fn; return s }

// Disabled shows the steps of a stepper with OnSelect dimmed, and does not let them
// be chosen or focused. A stepper without OnSelect is not changed by it.
func (s *StepperElement) Disabled(d bool) *StepperElement { s.disabled = d; return s }

// FocusHandle lets code focus the step that Tab reaches, the current one (see
// NewFocusHandle). It applies to a stepper with OnSelect.
func (s *StepperElement) FocusHandle(h *FocusHandle) *StepperElement {
	s.focusHandle = h
	return s
}

// Tooltip shows text below the stepper after the pointer rests on it. Assistive
// technology reads it as the list's description, unless it has another.
func (s *StepperElement) Tooltip(text string) *StepperElement { s.tooltip = text; return s }

// Styles sets the look of the parts of the stepper, for this instance only. Set
// fields win over the theme's Stepper and the defaults; see StepperStyle and
// docs/styling.md. Calling Styles again merges with the earlier values.
func (s *StepperElement) Styles(st StepperStyle) *StepperElement {
	s.styles = s.styles.merge(st)
	return s
}

// The sizing methods give the stepper's outer box a size. A horizontal stepper
// fills its container, and its lines share the room between the steps; a vertical
// one is as high as its steps. See Sizing in docs/styling.md.
func (s *StepperElement) Width(l Length) *StepperElement { s.size.setWidth(l); return s }

// Height sets the height of the stepper.
func (s *StepperElement) Height(l Length) *StepperElement { s.size.setHeight(l); return s }

// MinWidth sets the smallest width of the stepper.
func (s *StepperElement) MinWidth(l Length) *StepperElement { s.size.setMinWidth(l); return s }

// MaxWidth sets the largest width of the stepper.
func (s *StepperElement) MaxWidth(l Length) *StepperElement { s.size.setMaxWidth(l); return s }

// Grow makes the stepper take a share of the free space on its parent's main axis.
func (s *StepperElement) Grow(f float32) *StepperElement { s.size.setGrow(f); return s }

// GridSpan sets how many columns and rows the stepper covers in its parent's grid.
func (s *StepperElement) GridSpan(cols, rows int) *StepperElement {
	s.size.setSpan(cols, rows)
	return s
}

// scope is the prefix of the keys of this stepper's steps.
func (s *StepperElement) scope() string {
	if s.key != "" {
		return s.key
	}
	return fmt.Sprintf("stepper:%p", s.current)
}

func (s *StepperElement) expand(ctx *Context) Element {
	return Component(func(ctx *Context) Element {
		st := resolveStepper(ctx, s.vertical, s.styles)
		strs := ctx.Strings()
		n := len(s.steps)
		cur := min(max(s.current.Get(), 0), n)
		name := s.name
		if name == "" {
			name = strs.Steps
		}
		choosable := s.onSelect != nil && !s.disabled
		// handles holds the focus handle of each step, so the keys can move the focus
		// to a step without choosing it. The step that Tab reaches is the current one,
		// or the last when every step is complete.
		kept := Local(ctx, "handles", &[]*FocusHandle{}).Peek()
		for len(*kept) < n {
			*kept = append(*kept, NewFocusHandle())
		}
		handles := *kept
		stop := min(cur, n-1)
		handleOf := func(i int) *FocusHandle {
			if i == stop && s.focusHandle != nil {
				return s.focusHandle
			}
			return handles[i]
		}

		list := s.size.apply(Div().asPart("Stepper.Stepper").Role(access.RoleList).Label(name).
			SelectionContainer().Style(st.Stepper).Tooltip(s.tooltip))
		for i := range s.steps {
			last := i == n-1
			item := s.step(strs, st, i, cur, last)
			if s.onSelect != nil {
				item.AccessSelected(i == cur)
			} else if i == cur {
				item.AccessSelected(true)
			}
			if s.onSelect != nil && s.disabled {
				item.access().disabled = true
				item.Style(st.StepDisabled).Cursor(CursorNotAllowed)
			}
			if choosable {
				if i != stop {
					item.TabStop(false) // reached with the arrow keys, not Tab
				}
				back, next := input.KeyLeft, input.KeyRight
				if s.vertical {
					back, next = input.KeyUp, input.KeyDown
				}
				item.Hover(st.StepHover).Cursor(CursorPointer).Focusable().FocusHandle(handleOf(i)).
					FocusVisible(st.StepFocusVisible).
					OnClick(func() { s.onSelect(i) }).
					OnKeyDown(func(e *KeyEvent) {
						if e.Mods != 0 || e.Repeat {
							return
						}
						j := -1
						switch e.Key {
						case back:
							j = max(i-1, 0)
						case next:
							j = min(i+1, n-1)
						case input.KeyHome:
							j = 0
						case input.KeyEnd:
							j = n - 1
						default:
							return
						}
						e.StopPropagation()
						handleOf(j).Focus()
					})
			}
			list.Child(item)
			if !last && !s.vertical {
				list.Child(s.connector(st, i < cur))
			}
		}
		return list
	})
}

// step returns the item of the step at index i, with cur as the current step. In a
// vertical stepper the line to the next step is drawn under the indicator, inside
// the item, unless the step is the last.
func (s *StepperElement) step(strs *Strings, st StepperStyle, i, cur int, last bool) *DivElement {
	step := s.steps[i]
	n := len(s.steps)
	var label string
	ind := Div().asPart("Stepper.Indicator").Style(st.Indicator)
	switch {
	case i < cur:
		label = strs.StepCompleted(i+1, n, step.Title)
		ind.Style(st.IndicatorComplete).Child(Icon(icons.Check).Size(14))
	case i == cur:
		label = strs.StepCurrent(i+1, n, step.Title)
		ind.Style(st.IndicatorCurrent)
	default:
		label = strs.StepUpcoming(i+1, n, step.Title)
	}
	if i >= cur {
		if step.Icon != nil {
			ind.Child(Icon(step.Icon).Size(14))
		} else {
			ind.Child(Text(fmt.Sprint(i + 1)).NoWrap())
		}
	}
	// In a horizontal stepper the texts stay on one line and end with an ellipsis
	// when the steps do not fit; in a vertical one they wrap.
	text := func(str string) *TextElement {
		if s.vertical {
			return Text(str)
		}
		return Text(str).Truncate()
	}
	texts := Div().Column().Gap(2).MinWidth(Px(0)).
		Child(Div().asPart("Stepper.Title").Style(st.Title).Child(text(step.Title)))
	if step.Description != "" {
		texts.Child(Div().asPart("Stepper.Description").Style(st.Description).Child(text(step.Description)))
	}
	item := Div().asPart("Stepper.Step").Key(s.scope() + fmt.Sprintf("/step:%d", i)).Style(st.Step).
		Role(access.RoleListItem).Label(label)
	if !s.vertical {
		return item.Child(ind).Child(texts)
	}
	// The rail holds the indicator and the line under it, and is as high as the step,
	// so the line reaches the next step's indicator.
	rail := Div().Column().Align(Center).AlignSelf(Stretch).Child(ind)
	if !last {
		rail.Child(s.connector(st, i < cur))
		texts.PaddingEdges(Edges{Bottom: 16})
	}
	return item.Child(rail).Child(texts)
}

// connector returns the line after a step, drawn as complete after a complete step.
// It is decorative and hidden from assistive technology.
func (s *StepperElement) connector(st StepperStyle, complete bool) *DivElement {
	d := Div().asPart("Stepper.Connector").AccessHidden().Style(st.Connector)
	if complete {
		d.Style(st.ConnectorComplete)
	}
	return d
}
