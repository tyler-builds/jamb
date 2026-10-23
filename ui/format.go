package ui

import (
	"math"
	"strconv"
	"strings"
	"time"
)

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

// FormatNumber returns v rounded to decimals places, with the group separators, the
// decimal point and the minus sign of the user's locale, such as 12,345.68 in en-US
// or 12.345,68 in de-DE. A negative decimals is 0. A value that rounds to zero is
// shown without a minus sign. A NaN is the empty string, and an infinity is "∞" or
// "-∞".
func FormatNumber(v float64, decimals int) string {
	return formatNumber(v, decimals, currentLocale())
}

// formatNumber returns the text of FormatNumber in the symbols of loc.
func formatNumber(v float64, decimals int, loc *dateLocale) string {
	switch {
	case math.IsNaN(v):
		return ""
	case math.IsInf(v, 1):
		return "∞"
	case math.IsInf(v, -1):
		return loc.minus + "∞"
	}
	decimals = max(decimals, 0)
	raw := strconv.FormatFloat(v, 'f', decimals, 64)
	if strings.Trim(raw, "-0.") == "" {
		raw = strings.TrimPrefix(raw, "-") // -0.001 at two decimals is 0.00, not -0.00
	}
	// The same presentation as a NumberFormat field, so that the two agree.
	f := InputFormat{number: true, opts: NumberOptions{Decimals: decimals, Signed: true}}
	return f.presentNumber(raw, loc).shown
}

// FormatBytes returns the size n in bytes as Windows Explorer and Task Manager show
// it: in 1024-based units, with three significant digits, such as "512 bytes",
// "1.00 KB", "12.3 MB", "123 MB" and "1.23 GB". A size of 1000 units or more is shown
// in the next unit, as Windows does, so 1024000 is "0.97 MB", and the digits are cut,
// not rounded, so 1048575 is "0.99 MB". The sizes go up to PB. The decimal point is the
// user's locale's, and the words are in the language of the system (see SystemStrings).
// Unlike Windows, one byte is "1 byte".
func FormatBytes(n uint64) string { return formatBytes(n, currentLocale(), systemStrings()) }

// formatBytes returns the text of FormatBytes, with the decimal point of loc and the
// words of s.
func formatBytes(n uint64, loc *dateLocale, s *Strings) string {
	if n < 1024 {
		if n == 1 {
			return "1 " + s.Byte
		}
		return strconv.FormatUint(n, 10) + " " + s.Bytes
	}
	units := [...]string{s.KB, s.MB, s.GB, s.TB, s.PB}
	i, size := 0, uint64(1024)
	for i < len(units)-1 && n >= size*1000 {
		size *= 1024
		i++
	}
	whole, rem := n/size, n%size
	var num string
	switch {
	case whole < 10:
		num = strconv.FormatUint(whole, 10) + loc.decimal + twoDigits(rem*100/size)
	case whole < 100:
		num = strconv.FormatUint(whole, 10) + loc.decimal + strconv.FormatUint(rem*10/size, 10)
	default:
		num = strconv.FormatUint(whole, 10)
	}
	return num + " " + units[i]
}

// twoDigits returns d, from 0 to 99, as two digits, such as "05".
func twoDigits(d uint64) string {
	return string([]byte{byte('0' + d/10), byte('0' + d%10)})
}
