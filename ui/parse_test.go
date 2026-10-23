package ui

import (
	"testing"
	"time"
)

func TestParseDateReadsTheLocaleAndISO(t *testing.T) {
	useLocale(t, testLocaleEN())
	want := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.Local)
	for _, text := range []string{"10/13/2026", "10-13-2026", " 10/13/26 ", "2026-10-13", "10132026"} {
		if got, ok := ParseDate(text); !ok || !got.Equal(want) {
			t.Errorf("ParseDate(%q) = %v, %v; want %v", text, got, ok, want)
		}
	}
	for _, text := range []string{"", "13/13/2026", "Oct 13", "2026-13-01"} {
		if _, ok := ParseDate(text); ok {
			t.Errorf("ParseDate(%q) read a date", text)
		}
	}
	useLocale(t, testLocaleGB())
	if got, ok := ParseDate("13/10/2026"); !ok || !got.Equal(want) {
		t.Errorf("en-GB: ParseDate(13/10/2026) = %v, %v", got, ok)
	}
}

func TestParseNumberReadsTheLocaleAndAmounts(t *testing.T) {
	for _, c := range []struct {
		name string
		loc  *dateLocale
		text string
		want float64
		ok   bool
	}{
		{"en-US", testLocaleEN(), "1,234.5", 1234.5, true},
		{"en-US minus", testLocaleEN(), "-12", -12, true},
		{"en-US plus", testLocaleEN(), "+12.25", 12.25, true},
		{"en-US currency", testLocaleEN(), "$1,234.56", 1234.56, true},
		{"en-US parentheses", testLocaleEN(), "($5.00)", -5, true},
		{"trailing minus", testLocaleEN(), "5.00-", -5, true},
		{"de-DE", localeDE(), "1.234,5", 1234.5, true},
		{"de-DE currency", currencyDE(), "-1.234,56 €", -1234.56, true},
		{"not a number", testLocaleEN(), "twelve", 0, false},
		{"empty", testLocaleEN(), "", 0, false},
		{"two minus signs", testLocaleEN(), "--5", 0, false},
	} {
		got, ok := parseNumber(c.text, c.loc)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%s: parseNumber(%q) = %v, %v; want %v, %v", c.name, c.text, got, ok, c.want, c.ok)
		}
	}
	// What FormatCurrency writes reads back, in every negative shape.
	for i := 0; i < 16; i++ {
		l := testLocaleEN()
		l.currNeg = i
		if got, ok := parseNumber(formatCurrency(-1.1, l), l); !ok || got != -1.1 {
			t.Errorf("negative pattern %d: %q reads as %v, %v", i, formatCurrency(-1.1, l), got, ok)
		}
	}
}
