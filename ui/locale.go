package ui

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// dateLocale holds the formats that the date and time pickers use: the short date
// pattern, the first day of the week, the clock, and the names of the days and the
// months; and the symbols that a NumberFormat shows a number with. On Windows they are read from the user's locale settings (see
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
	// The symbols of a number: the decimal point, the group separator, the sizes of the
	// groups from the right (see parseGrouping), and the minus sign.
	decimal, group string
	grouping       []int
	minus          string
	// The currency of the locale (see FormatCurrency): its symbol, the digits after
	// its decimal point, the decimal point, group separator and grouping of amounts of
	// money, and Windows' patterns for a positive amount (LOCALE_ICURRENCY, 0 to 3)
	// and a negative one (LOCALE_INEGCURR, 0 to 15). Empty separators and a nil
	// grouping are those of numbers.
	currency              string
	currDigits            int
	monDecimal, monGroup  string
	monGrouping           []int
	currPositive, currNeg int
}

// money returns the locale with the decimal point, the group separator and the
// grouping of amounts of money in place of those of numbers.
func (l *dateLocale) money() *dateLocale {
	m := *l
	if l.monDecimal != "" {
		m.decimal = l.monDecimal
	}
	if l.monGroup != "" {
		m.group = l.monGroup
	}
	if l.monGrouping != nil {
		m.grouping = l.monGrouping
	}
	return &m
}

// localeOverride replaces the locale of the system, for the tests of the pickers and
// of the number format.
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
// that starts on Sunday, a 12 hour clock, English names, and numbers such as -1,234.5.
// It is the locale when the system locale cannot be read.
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
		decimal:      ".",
		group:        ",",
		grouping:     []int{3},
		minus:        "-",
		currency:     "$",
		currDigits:   2,
		currNeg:      1, // -$1.10
	}
}

// parseGrouping reads the grouping setting of Windows (LOCALE_SGROUPING): the sizes
// of the digit groups from the right, separated by semicolons, where a final 0 repeats
// the size before it. "3;0" groups by threes (1,234,567), "3;2;0" by three and then by
// twos (12,34,567, as in hi-IN), and "3" makes one group (1234,567). The result lists
// the sizes with the last one repeating, and ends with 0 when the grouping stops:
// [3], [3 2] and [3 0]. It reports false for a setting it can't read.
func parseGrouping(s string) ([]int, bool) {
	if s == "" {
		return nil, false
	}
	var sizes []int
	for _, part := range strings.Split(s, ";") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 9 {
			return nil, false
		}
		sizes = append(sizes, n)
	}
	switch last := len(sizes) - 1; {
	case sizes[last] == 0 && last > 0:
		sizes = sizes[:last] // the size before the 0 repeats
	case sizes[last] != 0:
		sizes = append(sizes, 0) // the grouping stops after the last size
	}
	return sizes, true
}
