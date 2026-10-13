package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// pickerRig is an access rig for a part test: a picker keyed "due" (a date picker) or "start"
// (a time picker), with the en-US locale and today as October 12, 2026.
func pickerRig(t *testing.T, root func(*Context) Element) *accessRig {
	t.Helper()
	prev := localeOverride
	localeOverride = testLocaleEN()
	t.Cleanup(func() { localeOverride = prev })
	r := newAccessRig(root)
	r.now = testNow
	r.frame()
	return r
}

// datePartElement is the date picker of the part tests, with the instance style st, the
// value v, and cfg applied to it.
func datePartElement(v *Signal[time.Time], st DatePickerStyle, cfg func(*DatePickerElement) *DatePickerElement) Element {
	p := DatePicker(v).Key("due").Label("Due date").Placeholder("MM/DD/YYYY").Styles(st)
	if cfg != nil {
		p = cfg(p)
	}
	return Div().Column().Child(p)
}

// timePartElement is the time picker of the part tests, with the instance style st and the value v.
func timePartElement(v *Signal[time.Time], st TimePickerStyle, disabled bool) Element {
	return Div().Column().Child(TimePicker(v).Key("start").Label("Start time").Placeholder("hh:mm").Styles(st).Disabled(disabled))
}

// testDatePickerParts sets a distinctive style on each part of the date picker, puts the
// state of the part in effect, and asserts that the style reaches the node that draws it.
func testDatePickerParts(t *testing.T, applied map[string]bool) {
	type dpCase struct {
		part string
		set  func(*DatePickerStyle, Style)
		run  func(t *testing.T, r *accessRig)
	}
	frame := func(t *testing.T, r *accessRig, part string) {
		wantBackground(t, partNode(t, r.rt, "DatePicker.Frame", ""), part, markBG)
	}
	openDay := func(t *testing.T, r *accessRig) {
		r.focusKey(t, "due")
		r.rt.keyboardModality = true
		r.press(input.KeyDown, input.ModAlt)
	}
	cases := []dpCase{
		{part: "DatePicker.Frame", set: func(s *DatePickerStyle, m Style) { s.Field.Frame = m },
			run: func(t *testing.T, r *accessRig) { frame(t, r, "DatePicker.Frame") }},
		{part: "DatePicker.FrameHover", set: func(s *DatePickerStyle, m Style) { s.Field.FrameHover = m },
			run: func(t *testing.T, r *accessRig) {
				hoverNode(r, partNode(t, r.rt, "DatePicker.Frame", ""))
				frame(t, r, "DatePicker.FrameHover")
			}},
		{part: "DatePicker.FrameFocusWithin", set: func(s *DatePickerStyle, m Style) { s.Field.FrameFocusWithin = m },
			run: func(t *testing.T, r *accessRig) {
				focusNode(r, editorNode(t, r.rt))
				frame(t, r, "DatePicker.FrameFocusWithin")
			}},
		{part: "DatePicker.FrameDisabled", set: func(s *DatePickerStyle, m Style) { s.Field.FrameDisabled = m },
			run: func(t *testing.T, r *accessRig) { frame(t, r, "DatePicker.FrameDisabled") }},
		{part: "DatePicker.FrameInvalid", set: func(s *DatePickerStyle, m Style) { s.Field.FrameInvalid = m },
			run: func(t *testing.T, r *accessRig) {
				r.focusKey(t, "due")
				r.typeText("13/45/2026")
				r.press(input.KeyEnter, 0)
				frame(t, r, "DatePicker.FrameInvalid")
			}},
		{part: "DatePicker.Text", set: func(s *DatePickerStyle, m Style) { s.Field.Text = m },
			run: func(t *testing.T, r *accessRig) { wantTextColor(t, editorNode(t, r.rt), "DatePicker.Text", markFG) }},
		{part: "DatePicker.Placeholder", set: func(s *DatePickerStyle, m Style) { s.Field.Placeholder = m },
			run: func(t *testing.T, r *accessRig) {
				if got := editorNode(t, r.rt).editor.placeholderColor; got != markFG {
					t.Errorf("DatePicker.Placeholder: color %v, want %v", got, markFG)
				}
			}},
		{part: "DatePicker.Selection", set: func(s *DatePickerStyle, m Style) { s.Field.Selection = m },
			run: func(t *testing.T, r *accessRig) {
				if got := editorNode(t, r.rt).editor.selection; got != markBG {
					t.Errorf("DatePicker.Selection: color %v, want %v", got, markBG)
				}
			}},
		{part: "DatePicker.CalendarButton", set: func(s *DatePickerStyle, m Style) { s.CalendarButton = m },
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, partNode(t, r.rt, "DatePicker.CalendarButton", ""), "DatePicker.CalendarButton", markBG)
			}},
		{part: "DatePicker.Message", set: func(s *DatePickerStyle, m Style) { s.Message = m },
			run: func(t *testing.T, r *accessRig) {
				r.focusKey(t, "due")
				r.typeText("13/45/2026")
				r.press(input.KeyEnter, 0)
				wantBackground(t, partNode(t, r.rt, "DatePicker.Message", ""), "DatePicker.Message", markBG)
			}},
		{part: "DatePicker.Popup", set: func(s *DatePickerStyle, m Style) { s.Popup = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, partNode(t, r.rt, "DatePicker.Popup", ""), "DatePicker.Popup", markBG)
			}},
		{part: "DatePicker.Header", set: func(s *DatePickerStyle, m Style) { s.Header = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, partNode(t, r.rt, "DatePicker.Header", ""), "DatePicker.Header", markBG)
			}},
		{part: "DatePicker.NavButton", set: func(s *DatePickerStyle, m Style) { s.NavButton = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackgroundAll(t, r, "DatePicker.NavButton")
			}},
		{part: "DatePicker.WeekdayLabel", set: func(s *DatePickerStyle, m Style) { s.WeekdayLabel = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackgroundAll(t, r, "DatePicker.WeekdayLabel")
			}},
		{part: "DatePicker.Day", set: func(s *DatePickerStyle, m Style) { s.Day = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Thursday, October 15, 2026"), "DatePicker.Day", markBG)
			}},
		{part: "DatePicker.DayHover", set: func(s *DatePickerStyle, m Style) { s.DayHover = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				n := accessNode(t, r, access.RoleDataItem, "Thursday, October 15, 2026")
				hoverNode(r, n)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Thursday, October 15, 2026"), "DatePicker.DayHover", markBG)
			}},
		{part: "DatePicker.DaySelected", set: func(s *DatePickerStyle, m Style) { s.DaySelected = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Tuesday, October 13, 2026"), "DatePicker.DaySelected", markBG)
			}},
		{part: "DatePicker.DayToday", set: func(s *DatePickerStyle, m Style) { s.DayToday = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Monday, October 12, 2026"), "DatePicker.DayToday", markBG)
			}},
		{part: "DatePicker.DayDisabled", set: func(s *DatePickerStyle, m Style) { s.DayDisabled = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Saturday, October 17, 2026"), "DatePicker.DayDisabled", markBG)
			}},
		{part: "DatePicker.DayOutsideMonth", set: func(s *DatePickerStyle, m Style) { s.DayOutsideMonth = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r)
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Sunday, September 27, 2026"), "DatePicker.DayOutsideMonth", markBG)
			}},
		{part: "DatePicker.DayFocusVisible", set: func(s *DatePickerStyle, m Style) { s.DayFocusVisible = m },
			run: func(t *testing.T, r *accessRig) {
				openDay(t, r) // with no date, the keys start on today
				wantBackground(t, accessNode(t, r, access.RoleDataItem, "Monday, October 12, 2026"), "DatePicker.DayFocusVisible", markBG)
			}},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st DatePickerStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			value := time.Time{}
			if c.part == "DatePicker.DaySelected" {
				value = day(2026, time.October, 13)
			}
			if c.part == "DatePicker.Selection" {
				value = day(2026, time.October, 13)
			}
			r := pickerRig(t, func(*Context) Element {
				return datePartElement(NewSignal(value), st, func(p *DatePickerElement) *DatePickerElement {
					if c.part == "DatePicker.FrameDisabled" {
						return p.Disabled(true)
					}
					if c.part == "DatePicker.DayDisabled" {
						return p.DisableDays(func(d time.Time) bool { return d.Day() == 17 && d.Month() == time.October })
					}
					return p
				})
			})
			if c.part == "DatePicker.Selection" || c.part == "DatePicker.Text" || c.part == "DatePicker.Placeholder" ||
				c.part == "DatePicker.FrameFocusWithin" {
				r.focusKey(t, "due")
			}
			c.run(t, r)
		})
	}
}

// testTimePickerParts is testDatePickerParts for the time picker.
func testTimePickerParts(t *testing.T, applied map[string]bool) {
	type tpCase struct {
		part string
		set  func(*TimePickerStyle, Style)
		run  func(t *testing.T, r *accessRig)
	}
	frame := func(t *testing.T, r *accessRig, part string) {
		wantBackground(t, partNode(t, r.rt, "TimePicker.Frame", ""), part, markBG)
	}
	openList := func(t *testing.T, r *accessRig) {
		r.focusKey(t, "start")
		r.rt.keyboardModality = true
		r.press(input.KeyDown, input.ModAlt)
	}
	cases := []tpCase{
		{part: "TimePicker.Frame", set: func(s *TimePickerStyle, m Style) { s.Field.Frame = m },
			run: func(t *testing.T, r *accessRig) { frame(t, r, "TimePicker.Frame") }},
		{part: "TimePicker.FrameHover", set: func(s *TimePickerStyle, m Style) { s.Field.FrameHover = m },
			run: func(t *testing.T, r *accessRig) {
				hoverNode(r, partNode(t, r.rt, "TimePicker.Frame", ""))
				frame(t, r, "TimePicker.FrameHover")
			}},
		{part: "TimePicker.FrameFocusWithin", set: func(s *TimePickerStyle, m Style) { s.Field.FrameFocusWithin = m },
			run: func(t *testing.T, r *accessRig) {
				focusNode(r, editorNode(t, r.rt))
				frame(t, r, "TimePicker.FrameFocusWithin")
			}},
		{part: "TimePicker.FrameDisabled", set: func(s *TimePickerStyle, m Style) { s.Field.FrameDisabled = m },
			run: func(t *testing.T, r *accessRig) { frame(t, r, "TimePicker.FrameDisabled") }},
		{part: "TimePicker.FrameInvalid", set: func(s *TimePickerStyle, m Style) { s.Field.FrameInvalid = m },
			run: func(t *testing.T, r *accessRig) {
				r.focusKey(t, "start")
				r.typeText("13:00 pm")
				r.press(input.KeyEnter, 0)
				frame(t, r, "TimePicker.FrameInvalid")
			}},
		{part: "TimePicker.Text", set: func(s *TimePickerStyle, m Style) { s.Field.Text = m },
			run: func(t *testing.T, r *accessRig) { wantTextColor(t, editorNode(t, r.rt), "TimePicker.Text", markFG) }},
		{part: "TimePicker.Placeholder", set: func(s *TimePickerStyle, m Style) { s.Field.Placeholder = m },
			run: func(t *testing.T, r *accessRig) {
				if got := editorNode(t, r.rt).editor.placeholderColor; got != markFG {
					t.Errorf("TimePicker.Placeholder: color %v, want %v", got, markFG)
				}
			}},
		{part: "TimePicker.Selection", set: func(s *TimePickerStyle, m Style) { s.Field.Selection = m },
			run: func(t *testing.T, r *accessRig) {
				if got := editorNode(t, r.rt).editor.selection; got != markBG {
					t.Errorf("TimePicker.Selection: color %v, want %v", got, markBG)
				}
			}},
		{part: "TimePicker.Button", set: func(s *TimePickerStyle, m Style) { s.Button = m },
			run: func(t *testing.T, r *accessRig) {
				wantBackground(t, partNode(t, r.rt, "TimePicker.Button", ""), "TimePicker.Button", markBG)
			}},
		{part: "TimePicker.Message", set: func(s *TimePickerStyle, m Style) { s.Message = m },
			run: func(t *testing.T, r *accessRig) {
				r.focusKey(t, "start")
				r.typeText("13:00 pm")
				r.press(input.KeyEnter, 0)
				wantBackground(t, partNode(t, r.rt, "TimePicker.Message", ""), "TimePicker.Message", markBG)
			}},
		{part: "TimePicker.Popup", set: func(s *TimePickerStyle, m Style) { s.Popup = m },
			run: func(t *testing.T, r *accessRig) {
				openList(t, r)
				wantBackground(t, partNode(t, r.rt, "TimePicker.Popup", ""), "TimePicker.Popup", markBG)
			}},
		{part: "TimePicker.Option", set: func(s *TimePickerStyle, m Style) { s.Option = m },
			run: func(t *testing.T, r *accessRig) {
				openList(t, r)
				wantBackground(t, accessNode(t, r, access.RoleListItem, "9:45 AM"), "TimePicker.Option", markBG)
			}},
		{part: "TimePicker.OptionHighlighted", set: func(s *TimePickerStyle, m Style) { s.OptionHighlighted = m },
			run: func(t *testing.T, r *accessRig) {
				openList(t, r)
				r.press(input.KeyDown, 0)
				wantBackground(t, accessNode(t, r, access.RoleListItem, "9:45 AM"), "TimePicker.OptionHighlighted", markBG)
			}},
		{part: "TimePicker.OptionSelected", set: func(s *TimePickerStyle, m Style) { s.OptionSelected = m },
			run: func(t *testing.T, r *accessRig) {
				openList(t, r)
				wantBackground(t, accessNode(t, r, access.RoleListItem, "9:30 AM"), "TimePicker.OptionSelected", markBG)
			}},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st TimePickerStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := pickerRig(t, func(*Context) Element {
				return timePartElement(NewSignal(clockOn(9, 30)), st, c.part == "TimePicker.FrameDisabled")
			})
			if c.part == "TimePicker.Selection" || c.part == "TimePicker.Text" || c.part == "TimePicker.Placeholder" ||
				c.part == "TimePicker.FrameFocusWithin" {
				r.focusKey(t, "start")
			}
			c.run(t, r)
		})
	}
}

// wantBackgroundAll asserts that every node of the part has the Background markBG, for a
// part that is drawn once for each of several elements (the navigation buttons, the names of
// the weekdays).
func wantBackgroundAll(t *testing.T, r *accessRig, part string) {
	t.Helper()
	nodes := partNodes(r.rt, part)
	if len(nodes) == 0 {
		t.Fatalf("%s is not drawn", part)
	}
	for _, n := range nodes {
		wantBackground(t, n, part, markBG)
	}
}
