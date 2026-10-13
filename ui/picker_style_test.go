package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// The theme's DatePicker is under the instance's Styles, and both are under the defaults.
func TestDatePickerThemeThenInstance(t *testing.T) {
	th := *DarkTheme
	th.DatePicker = DatePickerStyle{CalendarButton: Style{TextColor: markFG}, Popup: Style{Padding: All(11)}}
	r := newThemedRig(&th, func(ctx *Context) Element {
		return datePartElement(NewSignal(time.Time{}), DatePickerStyle{CalendarButton: Style{TextColor: markBG}}, nil)
	})
	r.frame()
	if got := partNode(t, r.rt, "DatePicker.CalendarButton", "").style.TextColor; got != markBG {
		t.Errorf("the instance's TextColor is %v, want %v (it wins over the theme)", got, markBG)
	}
	r.focusKey(t, "due")
	r.press(input.KeyDown, input.ModAlt)
	if got := partNode(t, r.rt, "DatePicker.Popup", "").style.Padding; got != All(11) {
		t.Errorf("the theme's popup padding is %v, want %v", got, All(11))
	}
}

// Clear removes a default of a part, for a date picker and a time picker, as it does for the
// other widgets (see docs/styling.md, "Turning a default off").
func TestPickersClearRemovesDefaults(t *testing.T) {
	r := pickerRig(t, func(ctx *Context) Element {
		return datePartElement(NewSignal(day(2026, time.October, 13)), DatePickerStyle{
			DaySelected: Style{Clear: ClearBackground},
			Popup:       Style{Clear: ClearShadows},
		}, nil)
	})
	r.focusKey(t, "due")
	r.press(input.KeyDown, input.ModAlt)
	if n := accessNode(t, r, access.RoleDataItem, "Tuesday, October 13, 2026"); !n.style.Background.IsZero() {
		t.Error("the selected day keeps its background under ClearBackground")
	}
	if n := partNode(t, r.rt, "DatePicker.Popup", ""); len(n.style.Shadows) != 0 {
		t.Errorf("the calendar keeps %d shadows under ClearShadows", len(n.style.Shadows))
	}

	r = pickerRig(t, func(ctx *Context) Element {
		return timePartElement(NewSignal(clockOn(9, 30)), TimePickerStyle{
			OptionHighlighted: Style{Clear: ClearBackground},
			Popup:             Style{Clear: ClearShadows},
		}, false)
	})
	r.focusKey(t, "start")
	r.press(input.KeyDown, input.ModAlt)
	if n := accessNode(t, r, access.RoleListItem, "9:30 AM"); !n.style.Background.IsZero() {
		t.Error("the highlighted time keeps its background under ClearBackground")
	}
	if n := partNode(t, r.rt, "TimePicker.Popup", ""); len(n.style.Shadows) != 0 {
		t.Errorf("the list keeps %d shadows under ClearShadows", len(n.style.Shadows))
	}
}

// Under high contrast the app's colors for the pickers are not used: the selected day is
// drawn in the system Highlight colors, the focused day and today are outlined, and a
// disabled day is GrayText. The sizes and the padding of the app's styles still apply.
func TestPickersHighContrastGuardrails(t *testing.T) {
	th := *DarkTheme
	th.DatePicker = DatePickerStyle{
		Day:         Style{Background: markBG},
		DaySelected: Style{Background: markBG, TextColor: markFG},
	}
	r := themedPickerRig(t, &th, func(ctx *Context) Element {
		return datePartElement(NewSignal(day(2026, time.October, 13)), DatePickerStyle{Day: Style{Padding: All(7)}}, nil)
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	sys := r.rt.theme
	r.focusKey(t, "due")
	r.press(input.KeyDown, input.ModAlt)
	sel := accessNode(t, r, access.RoleDataItem, "Tuesday, October 13, 2026")
	if sel.style.Background != sys.Accent {
		t.Errorf("high contrast: the selected day is %v, want the system Highlight %v", sel.style.Background, sys.Accent)
	}
	if sel.style.TextColor != sys.AccentText {
		t.Errorf("high contrast: the selected day text is %v, want %v", sel.style.TextColor, sys.AccentText)
	}
	if sel.style.Padding != All(7) {
		t.Errorf("high contrast: the app's padding did not apply to a day: %v", sel.style.Padding)
	}
	focus := accessNode(t, r, access.RoleDataItem, "Tuesday, October 13, 2026")
	if focus.style.BorderWidth != 2 || focus.style.BorderColor != sys.FocusRing {
		t.Errorf("high contrast: the focused day is not outlined in the focus color (border %v in %v)",
			focus.style.BorderWidth, focus.style.BorderColor)
	}
	if n := accessNode(t, r, access.RoleDataItem, "Monday, October 12, 2026"); n.style.BorderWidth != 1 {
		t.Errorf("high contrast: today is not outlined (border %v)", n.style.BorderWidth)
	}
	if n := accessNode(t, r, access.RoleDataItem, "Monday, October 12, 2026"); n.style.Background == markBG {
		t.Error("high contrast: the app's day background was used")
	}
}

// The high contrast default of a disabled day is GrayText, with no dimming.
func TestDatePickerDisabledDayIsGrayTextUnderHighContrast(t *testing.T) {
	th := *DarkTheme
	th.HighContrast = true
	r := newThemedRig(&th, func(ctx *Context) Element {
		return datePartElement(NewSignal(day(2026, time.October, 13)), DatePickerStyle{}, func(p *DatePickerElement) *DatePickerElement {
			return p.DisableDays(weekend)
		})
	})
	r.frame()
	r.focusKey(t, "due")
	r.press(input.KeyDown, input.ModAlt)
	sat := accessNode(t, r, access.RoleDataItem, "Saturday, October 17, 2026")
	if sat.style.TextColor != r.rt.theme.Disabled {
		t.Errorf("a disabled day is %v under high contrast, want GrayText %v", sat.style.TextColor, r.rt.theme.Disabled)
	}
	if sat.style.Opacity != 0 {
		t.Errorf("a disabled day is dimmed (opacity %v) under high contrast", sat.style.Opacity)
	}
}

// The time picker's highlighted time is drawn in the Highlight colors under high contrast.
func TestTimePickerHighContrastHighlight(t *testing.T) {
	th := *DarkTheme
	th.TimePicker = TimePickerStyle{OptionHighlighted: Style{Background: markBG}}
	r := themedPickerRig(t, &th, func(ctx *Context) Element {
		return timePartElement(NewSignal(clockOn(9, 30)), TimePickerStyle{}, false)
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	r.focusKey(t, "start")
	r.press(input.KeyDown, input.ModAlt)
	n := accessNode(t, r, access.RoleListItem, "9:30 AM")
	if n.style.Background != r.rt.theme.Accent || n.style.TextColor != r.rt.theme.AccentText {
		t.Errorf("high contrast: the highlighted time is %v on %v, want the Highlight colors",
			n.style.TextColor, n.style.Background)
	}
}

// The date picker's and time picker's Clear and instance overrides reach the field's frame too.
func TestPickersFieldFrameFollowsTheTextFrame(t *testing.T) {
	r := pickerRig(t, func(ctx *Context) Element {
		return datePartElement(NewSignal(time.Time{}), DatePickerStyle{Field: FieldStyle{Frame: Style{Clear: ClearBorder}}}, nil)
	})
	if n := partNode(t, r.rt, "DatePicker.Frame", ""); n.style.BorderWidth != 0 {
		t.Errorf("the frame keeps its border under ClearBorder: %v", n.style.BorderWidth)
	}
}

// themedPickerRig is newThemedRig for the picker tests: the locale is en-US, and today is
// October 12, 2026.
func themedPickerRig(t *testing.T, th *Theme, root func(*Context) Element) *accessRig {
	t.Helper()
	prev := localeOverride
	localeOverride = testLocaleEN()
	t.Cleanup(func() { localeOverride = prev })
	r := newThemedRig(th, root)
	r.now = testNow
	r.frame()
	return r
}
