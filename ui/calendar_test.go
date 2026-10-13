package ui

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.Local) }

func TestAddMonthsKeepsOrClampsTheDay(t *testing.T) {
	for _, c := range []struct {
		from time.Time
		n    int
		want time.Time
	}{
		{day(2026, time.January, 31), 1, day(2026, time.February, 28)},
		{day(2024, time.January, 31), 1, day(2024, time.February, 29)},
		{day(2026, time.March, 31), -1, day(2026, time.February, 28)},
		{day(2026, time.October, 13), 1, day(2026, time.November, 13)},
		{day(2026, time.October, 13), 12, day(2027, time.October, 13)},
		{day(2024, time.February, 29), 12, day(2025, time.February, 28)},
		{day(2026, time.January, 15), -13, day(2024, time.December, 15)},
	} {
		if got := addMonths(c.from, c.n); !got.Equal(c.want) {
			t.Errorf("addMonths(%v, %d) = %v, want %v", c.from, c.n, got, c.want)
		}
	}
}

func TestWeekStartForEachFirstDay(t *testing.T) {
	// October 13, 2026 is a Tuesday.
	tue := day(2026, time.October, 13)
	if got := weekStart(tue, time.Sunday); !got.Equal(day(2026, time.October, 11)) {
		t.Errorf("Sunday-first week of Tuesday Oct 13 starts %v, want Oct 11", got)
	}
	if got := weekStart(tue, time.Monday); !got.Equal(day(2026, time.October, 12)) {
		t.Errorf("Monday-first week of Tuesday Oct 13 starts %v, want Oct 12", got)
	}
	// A Sunday starts its own week when weeks start on Sunday, and the week before one that
	// starts on Monday.
	sun := day(2026, time.October, 11)
	if got := weekStart(sun, time.Sunday); !got.Equal(sun) {
		t.Errorf("a Sunday-first week that starts on the Sunday starts %v", got)
	}
	if got := weekStart(sun, time.Monday); !got.Equal(day(2026, time.October, 5)) {
		t.Errorf("Monday-first week of Sunday Oct 11 starts %v, want Oct 5", got)
	}
}

func TestMonthGridSundayFirst(t *testing.T) {
	// October 1, 2026 is a Thursday, so a Sunday-first grid starts on Sunday, September 27
	// and October 1 is in the fifth column of the first row.
	g := monthGrid(day(2026, time.October, 1), time.Sunday)
	if !g[0][0].Equal(day(2026, time.September, 27)) {
		t.Fatalf("first cell = %v, want Sept 27", g[0][0])
	}
	if !g[0][4].Equal(day(2026, time.October, 1)) {
		t.Fatalf("Oct 1 is at %v, want the fifth column", g[0][4])
	}
	if !g[5][6].Equal(day(2026, time.November, 7)) {
		t.Fatalf("last cell = %v, want Nov 7", g[5][6])
	}
}

func TestMonthGridMondayFirstAndAlwaysSixWeeks(t *testing.T) {
	g := monthGrid(day(2026, time.October, 1), time.Monday)
	if !g[0][0].Equal(day(2026, time.September, 28)) || !g[0][3].Equal(day(2026, time.October, 1)) {
		t.Fatalf("Monday-first grid row 0 = %v..%v", g[0][0], g[0][6])
	}
	// February 2027 starts on a Monday and fills four weeks; the grid still has six rows.
	g = monthGrid(day(2027, time.February, 1), time.Monday)
	if !g[0][0].Equal(day(2027, time.February, 1)) || !g[5][6].Equal(day(2027, time.March, 14)) {
		t.Fatalf("February 2027 grid = %v .. %v", g[0][0], g[5][6])
	}
	for r := range 6 {
		for c := 1; c < 7; c++ {
			if !g[r][c].Equal(g[r][c-1].AddDate(0, 0, 1)) {
				t.Fatalf("row %d is not consecutive days at column %d", r, c)
			}
		}
	}
}

func TestMonthGridLeapFebruary(t *testing.T) {
	g := monthGrid(day(2028, time.February, 1), time.Sunday)
	found := false
	for _, row := range g {
		for _, d := range row {
			if sameDay(d, day(2028, time.February, 29)) {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("February 29, 2028 is not in the grid of February 2028")
	}
}

func TestSameDayIgnoresTheClock(t *testing.T) {
	if !sameDay(day(2026, time.October, 13).Add(15*time.Hour), day(2026, time.October, 13)) {
		t.Error("the same day at another hour is not the same day")
	}
	if sameDay(day(2026, time.October, 13), day(2026, time.October, 14)) {
		t.Error("two days are the same day")
	}
}
