package views

import (
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// dateTimeSection shows the date picker and the time picker: a date with its limits and a
// rule that disables the weekends, a time in steps of 15 minutes between two limits, a date and
// a time that share one value, and a required date in a form. The fields take the locale's
// hints as their placeholders.
func dateTimeSection(ctx *ui.Context, t *ui.Theme) ui.Element {
	due := ui.Local(ctx, "dt.due", time.Time{})
	start := ui.Local(ctx, "dt.start", time.Time{})
	when := ui.Local(ctx, "dt.when", time.Time{})
	review := ui.Local(ctx, "dt.review", time.Time{})
	form := ui.Local(ctx, "dt.form", ui.NewForm()).Peek()
	result := ui.Local(ctx, "dt.result", "")
	today := startOfToday()
	form.OnSubmit(func() error {
		result.Set("Saved a review date of " + review.Peek().Format("Jan 2, 2006") + ".")
		return nil
	})
	meeting := "No date and time set."
	if !when.Get().IsZero() {
		meeting = "Set to " + when.Get().Format("Jan 2, 2006 at 3:04 PM") + "."
	}
	return section(t, "Date and time",
		sample(t, "Date, from today for 90 days, weekends disabled",
			ui.Field("Due date", ui.DatePicker(due).
				Min(today).Max(today.AddDate(0, 0, 90)).DisableDays(weekendDay)).Width(ui.Px(200))),
		sample(t, "Time, in steps of 15 minutes, between two limits",
			ui.Field("Start time", ui.TimePicker(start).MinuteStep(15).
				Min(clockOn(8, 0)).Max(clockOn(18, 0))).Width(ui.Px(160))),
		sample(t, "A date and a time on one value",
			ui.Div().Column().Gap(8).
				Child(ui.Div().Row().Gap(16).Align(ui.Start).
					Child(ui.Field("Meeting date", ui.DatePicker(when)).Width(ui.Px(200))).
					Child(ui.Field("Meeting time", ui.TimePicker(when)).Width(ui.Px(160)))).
				Child(ui.Text(meeting).FontSize(13).TextColor(t.TextMuted))),
		sample(t, "In a form, required",
			ui.Div().Column().Gap(10).
				Child(form.Summary()).
				Child(form.Field("review", "Review date", ui.DatePicker(review).
					Validate(ui.Required[time.Time]())).Required()).
				Child(ui.Button("Save review").Key("dt-save").Primary().OnClick(form.Submit)).
				Child(ui.Text(result.Get()).FontSize(13).TextColor(t.TextMuted))))
}

// startOfToday returns today at local midnight.
func startOfToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

// weekendDay reports whether day is a Saturday or a Sunday, for DisableDays.
func weekendDay(day time.Time) bool {
	return day.Weekday() == time.Saturday || day.Weekday() == time.Sunday
}

// clockOn returns a time on the day of the Go zero date at h:m, for a time picker's limits,
// where only the clock is read.
func clockOn(h, m int) time.Time {
	return time.Date(2000, time.January, 1, h, m, 0, 0, time.Local)
}
