package ui

import (
	"sync"
	"time"
)

// dateLocale holds the formats that the date and time pickers use: the short date
// pattern, the first day of the week, the clock, and the names of the days and the
// months. On Windows they are read from the user's locale settings (see
// locale_windows.go). Elsewhere, and where they cannot be read, they are the English
// formats of en-US (see englishLocale).
type dateLocale struct {
	shortDate  string // the short date pattern, such as "M/d/yyyy" or "dd.MM.yyyy"
	firstDay   time.Weekday
	twelveHour bool   // the clock is 12 hours, with AM and PM
	am, pm     string // the designators of the 12 hour clock
	timeSep    string // between the hour and the minute, such as ":"
	// The names are formatted from a date, so that they follow the locale. longDate is
	// "Monday, October 13, 2026", monthYear is "October 2026", and weekdayLong and
	// weekdayShort are the names of a day of the week, such as "Monday" and "Mon".
	longDate     func(t time.Time) string
	monthYear    func(t time.Time) string
	weekdayLong  func(d time.Weekday) string
	weekdayShort func(d time.Weekday) string
}

// localeOverride replaces the locale of the system, for the tests of the pickers.
// It is nil in an application.
var localeOverride *dateLocale

var (
	sysLocaleOnce sync.Once
	sysLocale     *dateLocale
)

// currentLocale returns the locale the date and time pickers format and parse with.
// The system locale is read once.
func currentLocale() *dateLocale {
	if localeOverride != nil {
		return localeOverride
	}
	sysLocaleOnce.Do(func() { sysLocale = readSystemLocale() })
	return sysLocale
}

// englishLocale returns the formats of en-US: a short date such as 10/13/2026, a week
// that starts on Sunday, a 12 hour clock, and English names. It is the locale when the
// system locale cannot be read.
func englishLocale() *dateLocale {
	return &dateLocale{
		shortDate:    "M/d/yyyy",
		firstDay:     time.Sunday,
		twelveHour:   true,
		am:           "AM",
		pm:           "PM",
		timeSep:      ":",
		longDate:     func(t time.Time) string { return t.Format("Monday, January 2, 2006") },
		monthYear:    func(t time.Time) string { return t.Format("January 2006") },
		weekdayLong:  func(d time.Weekday) string { return d.String() },
		weekdayShort: func(d time.Weekday) string { return d.String()[:3] },
	}
}
