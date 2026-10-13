package ui

import "time"

// dayOf returns the day of t, at local midnight.
func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// sameDay reports whether a and b fall on the same day.
func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// addMonths returns the day n months from t. It keeps the day of the month, or the last
// day of the month when the new month is shorter, so that January 31 plus one month is
// February 28 (or 29), not March 3.
func addMonths(t time.Time, n int) time.Time {
	first := time.Date(t.Year(), t.Month()+time.Month(n), 1, 0, 0, 0, 0, time.Local)
	d := min(t.Day(), daysIn(first.Year(), first.Month()))
	return time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, time.Local)
}

// weekStart returns the first day of the week of t, for a week that starts on first.
func weekStart(t time.Time, first time.Weekday) time.Time {
	d := dayOf(t)
	return d.AddDate(0, 0, -((int(d.Weekday()) - int(first) + 7) % 7))
}

// monthGrid returns the days shown for the month of t: six weeks of seven days, from the
// week that holds the first of the month. The days of the previous and the next month
// that fill the weeks are in it too, so a month always takes the same six rows.
func monthGrid(t time.Time, first time.Weekday) [6][7]time.Time {
	start := weekStart(time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local), first)
	var g [6][7]time.Time
	for r := range 6 {
		for c := range 7 {
			g[r][c] = start.AddDate(0, 0, r*7+c)
		}
	}
	return g
}
