package ui

import "time"

// These format dates and times as the date and time pickers do, in the user's
// locale: the short date and the clock that Windows' region settings give, such
// as 10/13/2026 and 3:04 PM in en-US, or 13.10.2026 and 15:04 in de-DE.

// FormatDate returns t as a short date in the user's locale.
func FormatDate(t time.Time) string { return formatDate(t, currentLocale()) }

// FormatTime returns the time of day of t, to the minute, in the user's
// locale's clock.
func FormatTime(t time.Time) string { return formatClock(clockOf(t), currentLocale()) }

// FormatMonthYear returns the month and year of t in the user's locale, such
// as October 2026.
func FormatMonthYear(t time.Time) string { return currentLocale().monthYear(t) }
