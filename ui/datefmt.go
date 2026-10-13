package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// datePart is one part of a short date pattern: a field (d, M or y, with its width,
// as in "dd" or "yyyy") or a literal character.
type datePart struct {
	field byte // 'd', 'M' or 'y'; zero for a literal
	width int
	lit   string
}

// datePattern splits a short date pattern, such as "dd.MM.yyyy", into its parts. The
// letters that are not a day, month or year are dropped. A pattern that does not have
// all three fields is read as the English M/d/yyyy.
func datePattern(p string) []datePart {
	parts := splitDatePattern(p)
	if len(fieldOrder(parts)) != 3 {
		parts = splitDatePattern("M/d/yyyy")
	}
	return parts
}

// splitDatePattern splits the pattern into its parts, without the check of datePattern.
func splitDatePattern(p string) []datePart {
	var parts []datePart
	rs := []rune(p)
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case c == 'd' || c == 'M' || c == 'y':
			j := i
			for j < len(rs) && rs[j] == c {
				j++
			}
			parts = append(parts, datePart{field: byte(c), width: j - i})
			i = j
		case unicode.IsLetter(c):
			i++ // a letter the pattern does not use, such as the era
		default:
			parts = append(parts, datePart{lit: string(c)})
			i++
		}
	}
	return parts
}

// fieldOrder returns the fields of the parts in the order they come, such as "Mdy" for
// M/d/yyyy. Each field is listed once.
func fieldOrder(parts []datePart) string {
	var order []byte
	for _, p := range parts {
		if p.field != 0 && !strings.ContainsRune(string(order), rune(p.field)) {
			order = append(order, p.field)
		}
	}
	return string(order)
}

// formatDate returns the text of the day of t, in the short date pattern of l. The
// time of day is not shown. A zero time is no date, and its text is empty.
func formatDate(t time.Time, l *dateLocale) string {
	if t.IsZero() {
		return ""
	}
	var b strings.Builder
	for _, p := range datePattern(l.shortDate) {
		switch p.field {
		case 0:
			b.WriteString(p.lit)
		case 'd':
			fmt.Fprintf(&b, "%0*d", p.width, t.Day())
		case 'M':
			fmt.Fprintf(&b, "%0*d", p.width, int(t.Month()))
		case 'y':
			if p.width <= 2 {
				fmt.Fprintf(&b, "%02d", t.Year()%100)
			} else {
				fmt.Fprintf(&b, "%04d", t.Year())
			}
		}
	}
	return b.String()
}

// dateExample is the text of a sample date in l, which a message gives as an example
// of what the field takes.
func dateExample(l *dateLocale) string {
	return formatDate(time.Date(2026, time.October, 13, 0, 0, 0, 0, time.Local), l)
}

// parseDate returns the day that text holds, at local midnight, and whether it is a
// date. The groups of digits are read in the order of the short date pattern of l,
// and they may be separated by any punctuation or spaces. A year of two digits is in
// the century of now, or the one before when it would be more than 20 years ahead.
// Without a year, the date is in the year of now. Eight digits (or six, with a
// two-digit year) are read as the fields in the pattern's widths, such as 10132026.
// Month names are not read.
func parseDate(text string, l *dateLocale, now time.Time) (time.Time, bool) {
	var groups []string
	cur := ""
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			cur += string(r)
		case unicode.IsLetter(r):
			return time.Time{}, false
		default:
			if cur != "" {
				groups = append(groups, cur)
				cur = ""
			}
		}
	}
	if cur != "" {
		groups = append(groups, cur)
	}
	order := []byte(fieldOrder(datePattern(l.shortDate))) // three fields, as datePattern guarantees
	vals := map[byte]string{}
	switch {
	case len(groups) == 3:
		for i := range order {
			vals[order[i]] = groups[i]
		}
	case len(groups) == 2:
		// A date without a year: the two fields that are not the year.
		var rest []byte
		for i := range order {
			if order[i] != 'y' {
				rest = append(rest, order[i])
			}
		}
		vals[rest[0]], vals[rest[1]] = groups[0], groups[1]
	case len(groups) == 1 && (len(groups[0]) == 8 || len(groups[0]) == 6):
		g := groups[0]
		yearWidth := len(g) - 4
		for _, f := range order {
			w := 2
			if f == 'y' {
				w = yearWidth
			}
			vals[f], g = g[:w], g[w:]
		}
	default:
		return time.Time{}, false
	}
	dv, mv := vals['d'], vals['M']
	day, err1 := strconv.Atoi(dv)
	month, err2 := strconv.Atoi(mv)
	if dv == "" || mv == "" || err1 != nil || err2 != nil {
		return time.Time{}, false
	}
	year := now.Year()
	if yv, ok := vals['y']; ok {
		n, err := strconv.Atoi(yv)
		if err != nil {
			return time.Time{}, false
		}
		switch len(yv) {
		case 2:
			year = now.Year()/100*100 + n
			if year > now.Year()+20 {
				year -= 100
			}
		case 4:
			year = n
		default:
			return time.Time{}, false
		}
	}
	if month < 1 || month > 12 || day < 1 || day > daysIn(year, time.Month(month)) {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local), true
}

// daysIn returns the number of days in the month of the year.
func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

// clock is a time of day, to the minute.
type clock struct{ hour, minute int }

// minutes returns the minutes after midnight.
func (c clock) minutes() int { return c.hour*60 + c.minute }

// clockAt returns the clock of m minutes after midnight.
func clockAt(m int) clock { return clock{hour: m / 60, minute: m % 60} }

// clockOf returns the clock of t.
func clockOf(t time.Time) clock { return clock{hour: t.Hour(), minute: t.Minute()} }

// formatClock returns the text of c in l: "9:05 AM" on a 12 hour clock, "09:05"
// otherwise.
func formatClock(c clock, l *dateLocale) string {
	if !l.twelveHour {
		return fmt.Sprintf("%02d%s%02d", c.hour, l.timeSep, c.minute)
	}
	h := c.hour % 12
	if h == 0 {
		h = 12
	}
	des := l.am
	if c.hour >= 12 {
		des = l.pm
	}
	return fmt.Sprintf("%d%s%02d %s", h, l.timeSep, c.minute, des)
}

// parseClock returns the clock that text holds, and whether it is a time. The hour and
// the minute are separated by any punctuation or a space, or they come as digits, such
// as "930" or "1430". An AM or PM designator, in full or as a or p, makes the hour a
// 12 hour one. Without a designator the hour is on the 24 hour clock.
func parseClock(text string, l *dateLocale) (clock, bool) {
	s := strings.ToLower(strings.TrimSpace(text))
	designated, pm := false, false
	if strings.IndexFunc(s, unicode.IsLetter) >= 0 {
		switch {
		case strings.Contains(s, strings.ToLower(l.pm)) && l.pm != "", strings.Contains(s, "p"):
			pm, designated = true, true
		case strings.Contains(s, strings.ToLower(l.am)) && l.am != "", strings.Contains(s, "a"):
			designated = true
		default:
			return clock{}, false
		}
	}
	rest := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return ' '
		}
		return r
	}, s)
	var groups []string
	cur := ""
	for _, r := range rest {
		switch {
		case r >= '0' && r <= '9':
			cur += string(r)
		default:
			if cur != "" {
				groups = append(groups, cur)
				cur = ""
			}
		}
	}
	if cur != "" {
		groups = append(groups, cur)
	}
	var h, m int
	var err error
	switch {
	case len(groups) == 1 && (len(groups[0]) == 3 || len(groups[0]) == 4):
		g := groups[0]
		h, err = strconv.Atoi(g[:len(g)-2])
		if err == nil {
			m, err = strconv.Atoi(g[len(g)-2:])
		}
	case len(groups) == 1 && len(groups[0]) <= 2:
		h, err = strconv.Atoi(groups[0])
	case len(groups) == 2 && len(groups[0]) <= 2 && len(groups[1]) <= 2:
		h, err = strconv.Atoi(groups[0])
		if err == nil {
			m, err = strconv.Atoi(groups[1])
		}
	default:
		return clock{}, false
	}
	if err != nil || m < 0 || m > 59 {
		return clock{}, false
	}
	if designated {
		if h < 1 || h > 12 {
			return clock{}, false
		}
		h %= 12
		if pm {
			h += 12
		}
	} else if h < 0 || h > 23 {
		return clock{}, false
	}
	return clock{hour: h, minute: m}, true
}

// withClock returns the day of day at the clock c, in local time.
func withClock(day time.Time, c clock) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), c.hour, c.minute, 0, 0, time.Local)
}

// datePlaceholder returns the hint a date field shows while it is empty: the short date
// pattern of l with its fields as DD, MM and YYYY, such as "MM/DD/YYYY" for en-US and
// "DD.MM.YYYY" for de-DE.
func (l *dateLocale) datePlaceholder() string {
	var b strings.Builder
	for _, p := range datePattern(l.shortDate) {
		switch p.field {
		case 0:
			b.WriteString(p.lit)
		case 'd':
			b.WriteString("DD")
		case 'M':
			b.WriteString("MM")
		case 'y':
			if p.width <= 2 {
				b.WriteString("YY")
			} else {
				b.WriteString("YYYY")
			}
		}
	}
	return b.String()
}

// timePlaceholder returns the hint a time field shows while it is empty: "h:mm AM" on a 12
// hour clock (with the locale's separator and designator), and "HH:mm" on a 24 hour one.
func (l *dateLocale) timePlaceholder() string {
	if !l.twelveHour {
		return "HH" + l.timeSep + "mm"
	}
	return "h" + l.timeSep + "mm " + l.am
}
