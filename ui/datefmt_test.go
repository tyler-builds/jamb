package ui

import (
	"testing"
	"time"
)

// Locales for the tests, as the pickers read them from Windows: en-US, en-GB (Monday first,
// a day-first date, a 24 hour clock), de-DE (a dotted date) and sv-SE (a year-first date).
func testLocaleEN() *dateLocale { return englishLocale() }

func testLocaleGB() *dateLocale {
	l := englishLocale()
	l.shortDate, l.firstDay, l.twelveHour = "dd/MM/yyyy", time.Monday, false
	return l
}

func testLocaleDE() *dateLocale {
	l := testLocaleGB()
	l.shortDate, l.timeSep = "dd.MM.yyyy", "."
	return l
}

func testLocaleSE() *dateLocale {
	l := testLocaleGB()
	l.shortDate, l.firstDay = "yyyy-MM-dd", time.Monday
	return l
}

// testNow is the day the tests treat as today: Monday, October 12, 2026.
var testNow = time.Date(2026, time.October, 12, 10, 0, 0, 0, time.Local)

func TestFormatDateInLocaleOrder(t *testing.T) {
	day := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.Local)
	for _, c := range []struct {
		name string
		l    *dateLocale
		want string
	}{
		{"en-US", testLocaleEN(), "10/13/2026"},
		{"en-GB", testLocaleGB(), "13/10/2026"},
		{"de-DE", testLocaleDE(), "13.10.2026"},
		{"sv-SE", testLocaleSE(), "2026-10-13"},
	} {
		if got := formatDate(day, c.l); got != c.want {
			t.Errorf("%s: formatDate = %q, want %q", c.name, got, c.want)
		}
	}
	if got := formatDate(time.Time{}, testLocaleEN()); got != "" {
		t.Errorf("a zero time formats as %q, want empty", got)
	}
}

func TestFormatDatePadsSingleDigitFields(t *testing.T) {
	day := time.Date(2026, time.March, 4, 0, 0, 0, 0, time.Local)
	if got := formatDate(day, testLocaleEN()); got != "3/4/2026" {
		t.Errorf("en-US short date = %q, want 3/4/2026 (the M/d pattern has no padding)", got)
	}
	if got := formatDate(day, testLocaleGB()); got != "04/03/2026" {
		t.Errorf("en-GB short date = %q, want 04/03/2026", got)
	}
}

func TestDateExampleIsTheSampleDayInTheLocale(t *testing.T) {
	if got := dateExample(testLocaleEN()); got != "10/13/2026" {
		t.Errorf("dateExample = %q", got)
	}
}

func TestParseDateTolerantSeparators(t *testing.T) {
	want := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.Local)
	for _, text := range []string{"10/13/2026", "10-13-2026", "10.13.2026", "10 13 2026", " 10/13/2026 ", "10132026"} {
		got, ok := parseDate(text, testLocaleEN(), testNow)
		if !ok || !got.Equal(want) {
			t.Errorf("parseDate(%q) = %v, %v; want %v", text, got, ok, want)
		}
	}
}

func TestParseDateInLocaleOrder(t *testing.T) {
	want := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.Local)
	if got, ok := parseDate("13.10.2026", testLocaleDE(), testNow); !ok || !got.Equal(want) {
		t.Errorf("de-DE parseDate = %v, %v", got, ok)
	}
	if got, ok := parseDate("2026-10-13", testLocaleSE(), testNow); !ok || !got.Equal(want) {
		t.Errorf("sv-SE parseDate = %v, %v", got, ok)
	}
	// The same text is another day in another locale: 10/13 is not a day in en-GB's order.
	if _, ok := parseDate("10/13/2026", testLocaleGB(), testNow); ok {
		t.Error("en-GB read 10/13/2026, which has month 13")
	}
}

func TestParseDateTwoDigitYears(t *testing.T) {
	// The century is that of now (2026); a year more than 20 years ahead is the century before.
	for _, c := range []struct {
		text string
		year int
	}{
		{"10/13/26", 2026},
		{"10/13/45", 2045},
		{"10/13/47", 1947},
		{"10/13/80", 1980},
		{"10/13/99", 1999},
	} {
		got, ok := parseDate(c.text, testLocaleEN(), testNow)
		if !ok || got.Year() != c.year {
			t.Errorf("parseDate(%q) year = %d, %v; want %d", c.text, got.Year(), ok, c.year)
		}
	}
}

func TestParseDateWithoutYearUsesThisYear(t *testing.T) {
	got, ok := parseDate("10/13", testLocaleEN(), testNow)
	if !ok || got.Year() != 2026 || got.Month() != time.October || got.Day() != 13 {
		t.Errorf("parseDate(10/13) = %v, %v", got, ok)
	}
}

func TestParseDateSixDigitsWithTwoDigitYear(t *testing.T) {
	got, ok := parseDate("101326", testLocaleEN(), testNow)
	if !ok || got.Year() != 2026 || got.Day() != 13 {
		t.Errorf("parseDate(101326) = %v, %v", got, ok)
	}
}

func TestParseDateRejectsInvalidText(t *testing.T) {
	for _, text := range []string{
		"", "abc", "13/10/2026x", "10/32/2026", "13/13/2026", "00/10/2026", "10/00/2026",
		"02/29/2026", "02/30/2028", "10/13/202", "10/13/20260", "10//", "Oct 13 2026", "10/13/2026/1",
	} {
		if got, ok := parseDate(text, testLocaleEN(), testNow); ok {
			t.Errorf("parseDate(%q) = %v, want not a date", text, got)
		}
	}
}

func TestParseDateLeapDay(t *testing.T) {
	if got, ok := parseDate("02/29/2028", testLocaleEN(), testNow); !ok || got.Day() != 29 {
		t.Errorf("2028 is a leap year: parseDate = %v, %v", got, ok)
	}
	if _, ok := parseDate("02/29/2100", testLocaleEN(), testNow); ok {
		t.Error("2100 is not a leap year, but 02/29/2100 was accepted")
	}
}

func TestDaysInMonth(t *testing.T) {
	for _, c := range []struct {
		year  int
		month time.Month
		want  int
	}{
		{2024, time.February, 29}, {2026, time.February, 28}, {2000, time.February, 29},
		{2100, time.February, 28}, {2026, time.October, 31}, {2026, time.April, 30},
	} {
		if got := daysIn(c.year, c.month); got != c.want {
			t.Errorf("daysIn(%d, %v) = %d, want %d", c.year, c.month, got, c.want)
		}
	}
}

func TestParseClockFormsAndDesignators(t *testing.T) {
	for _, c := range []struct {
		text string
		want clock
	}{
		{"9:30 AM", clock{9, 30}},
		{"9:30pm", clock{21, 30}},
		{"9:30 p", clock{21, 30}},
		{"12 am", clock{0, 0}},
		{"12pm", clock{12, 0}},
		{"12:15 PM", clock{12, 15}},
		{"930", clock{9, 30}},
		{"1430", clock{14, 30}},
		{"21:30", clock{21, 30}},
		{"9", clock{9, 0}},
		{"9.5", clock{9, 5}},
		{"0:00", clock{0, 0}},
		{"23:59", clock{23, 59}},
	} {
		got, ok := parseClock(c.text, testLocaleEN())
		if !ok || got != c.want {
			t.Errorf("parseClock(%q) = %v, %v; want %v", c.text, got, ok, c.want)
		}
	}
}

func TestParseClockRejectsInvalidTimes(t *testing.T) {
	for _, text := range []string{"", "abc", "13:00 pm", "0 am", "9:60", "24:00", "9:30:15", "99", "1:2:3:4"} {
		if got, ok := parseClock(text, testLocaleEN()); ok {
			t.Errorf("parseClock(%q) = %v, want not a time", text, got)
		}
	}
}

func TestFormatClockByLocaleClock(t *testing.T) {
	for _, c := range []struct {
		l    *dateLocale
		at   clock
		want string
	}{
		{testLocaleEN(), clock{9, 5}, "9:05 AM"},
		{testLocaleEN(), clock{0, 0}, "12:00 AM"},
		{testLocaleEN(), clock{12, 30}, "12:30 PM"},
		{testLocaleEN(), clock{21, 45}, "9:45 PM"},
		{testLocaleGB(), clock{9, 5}, "09:05"},
		{testLocaleGB(), clock{21, 45}, "21:45"},
		{testLocaleDE(), clock{9, 5}, "09.05"},
	} {
		if got := formatClock(c.at, c.l); got != c.want {
			t.Errorf("formatClock(%v) = %q, want %q", c.at, got, c.want)
		}
	}
}

func TestClockRoundTrip(t *testing.T) {
	for m := 0; m < 24*60; m += 7 {
		c := clockAt(m)
		for _, l := range []*dateLocale{testLocaleEN(), testLocaleGB()} {
			got, ok := parseClock(formatClock(c, l), l)
			if !ok || got != c {
				t.Fatalf("%v in %q parsed back as %v, %v", c, formatClock(c, l), got, ok)
			}
		}
	}
}

func TestCurrentLocaleHonorsOverride(t *testing.T) {
	prev := localeOverride
	localeOverride = testLocaleGB()
	t.Cleanup(func() { localeOverride = prev })
	if currentLocale().shortDate != "dd/MM/yyyy" {
		t.Fatal("currentLocale ignores the override")
	}
}

func TestDatePatternFallsBackWithoutAllFields(t *testing.T) {
	parts := datePattern("MMMM yyyy")
	if len(fieldOrder(parts)) != 3 {
		t.Fatalf("a pattern without a day falls back to M/d/yyyy, got fields %q", fieldOrder(parts))
	}
}

// The default placeholders follow the locale's formats: the date hint of the short date
// pattern, and the time hint of the clock.
func TestPlaceholdersFollowTheLocale(t *testing.T) {
	for _, c := range []struct {
		name     string
		l        *dateLocale
		date, tm string
	}{
		{"en-US", testLocaleEN(), "MM/DD/YYYY", "h:mm AM"},
		{"en-GB", testLocaleGB(), "DD/MM/YYYY", "HH:mm"},
		{"de-DE", testLocaleDE(), "DD.MM.YYYY", "HH.mm"},
		{"sv-SE", testLocaleSE(), "YYYY-MM-DD", "HH:mm"},
	} {
		if got := c.l.datePlaceholder(); got != c.date {
			t.Errorf("%s: date placeholder %q, want %q", c.name, got, c.date)
		}
		if got := c.l.timePlaceholder(); got != c.tm {
			t.Errorf("%s: time placeholder %q, want %q", c.name, got, c.tm)
		}
	}
}
