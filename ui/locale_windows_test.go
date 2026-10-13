//go:build windows

package ui

import (
	"strings"
	"testing"
	"time"
)

// The system locale is read from Windows: its short date pattern, its first day of the week
// and its names are set, and the names are in the locale's words for the day they format.
func TestSystemLocaleIsReadFromWindows(t *testing.T) {
	l := readSystemLocale()
	if l.shortDate == "" || len(datePattern(l.shortDate)) == 0 {
		t.Fatalf("the short date pattern is %q", l.shortDate)
	}
	if len(fieldOrder(datePattern(l.shortDate))) != 3 {
		t.Fatalf("the short date pattern %q does not have a day, a month and a year", l.shortDate)
	}
	if strings.ContainsAny(l.timeSep, "0123456789") || l.am == "" || l.pm == "" {
		t.Fatalf("the clock settings are wrong: separator %q, AM %q, PM %q", l.timeSep, l.am, l.pm)
	}
	day := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.Local)
	if got := l.longDate(day); !strings.Contains(got, "2026") || !strings.Contains(got, "13") {
		t.Errorf("the long date of October 13, 2026 is %q", got)
	}
	if got := l.monthYear(day); !strings.Contains(got, "2026") {
		t.Errorf("the month and year of October 13, 2026 is %q", got)
	}
	if got := l.weekdayLong(time.Tuesday); got == "" {
		t.Error("the name of Tuesday is empty")
	}
	if got := l.weekdayShort(time.Tuesday); got == "" || len([]rune(got)) > len([]rune(l.weekdayLong(time.Tuesday))) {
		t.Errorf("the short name of Tuesday is %q", got)
	}
	// The formatted date of a locale's pattern has the day and month in it.
	if got := formatDate(day, l); !strings.Contains(got, "2026") {
		t.Errorf("formatDate in the system locale gives %q", got)
	}
}
