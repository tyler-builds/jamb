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

// ParseDate returns the day that text holds, at local midnight, and whether it is a
// date: a short date in the user's locale, read as a date picker reads typed text
// (the numbers in the locale's order, with any punctuation between them; a year of
// two digits in this century or the one before; no year meaning this year), or an
// ISO 8601 date such as 2026-10-13, which files from other programs often hold. It is
// the counterpart of FormatDate. Month names are not read.
func ParseDate(text string) (time.Time, bool) {
	text = strings.TrimSpace(text)
	if t, err := time.ParseInLocation("2006-01-02", text, time.Local); err == nil {
		return t, true
	}
	return parseDate(text, currentLocale(), time.Now())
}

// ParseNumber returns the number that text holds, written in the user's locale, and
// whether it is one: with the locale's decimal point and group separators (or those
// of amounts of money), and a leading minus or plus. It is the counterpart of
// FormatNumber, and reads what FormatCurrency writes too: a currency symbol before or
// after the number is dropped, and parentheses or a trailing minus make it negative,
// as some programs write amounts. So "1,234.5" is 1234.5 in en-US and "1.234,5" is
// 1234.5 in de-DE, and "($5.00)" is -5.
func ParseNumber(text string) (float64, bool) { return parseNumber(text, currentLocale()) }

// parseNumber is ParseNumber in loc.
func parseNumber(text string, loc *dateLocale) (float64, bool) {
	s := strings.TrimSpace(text)
	neg := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg, s = true, strings.TrimSpace(s[1:len(s)-1])
	}
	if loc.currency != "" {
		s = strings.TrimSpace(strings.ReplaceAll(s, loc.currency, ""))
	}
	for _, m := range []string{loc.minus, "-"} {
		if m != "" && strings.HasSuffix(s, m) && len(s) > len(m) {
			neg, s = !neg, strings.TrimSpace(strings.TrimSuffix(s, m))
			break
		}
	}
	raw, ok := numberFromDisplay(s, loc)
	if !ok {
		raw, ok = numberFromDisplay(s, loc.money())
	}
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		v = -v
	}
	return v, true
}

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
	raw := roundDecimal(math.Abs(v), decimals)
	if v < 0 && strings.Trim(raw, "0.") != "" {
		raw = "-" + raw // -0.001 at two decimals is 0.00, not -0.00
	}
	// The same presentation as a NumberFormat field, so that the two agree.
	f := InputFormat{number: true, opts: NumberOptions{Decimals: decimals, Signed: true}}
	return f.presentNumber(raw, loc).shown
}

// FormatCurrency returns amount as an amount of money in the currency of the user's
// region settings, as Windows formats it: rounded to the currency's digits, with its
// decimal point and group separator, and its symbol and its sign where the settings
// put them, such as $1,234.56 and -$5.00 in en-US, 1.234,56 € and -5,00 € in de-DE, or
// ($5.00) where the settings show negative amounts in parentheses. An amount that
// rounds to zero has no sign. A NaN is the empty string. Elsewhere than on Windows
// the currency is the US dollar.
func FormatCurrency(amount float64) string { return formatCurrency(amount, currentLocale()) }

// CurrencySymbol returns the symbol of the currency of the user's region settings,
// such as "$" or "€", and reports whether it goes after the amount, as in 5,00 €.
func CurrencySymbol() (symbol string, after bool) {
	loc := currentLocale()
	return loc.currency, loc.currPositive == 1 || loc.currPositive == 3
}

// formatCurrency returns the text of FormatCurrency in the currency of loc.
func formatCurrency(v float64, loc *dateLocale) string {
	if math.IsNaN(v) {
		return ""
	}
	num := formatNumber(math.Abs(v), loc.currDigits, loc.money())
	neg := v < 0 && strings.Trim(roundDecimal(-v, loc.currDigits), "0.") != ""
	sym, m := loc.currency, loc.minus
	if !neg {
		switch loc.currPositive {
		case 1:
			return num + sym
		case 2:
			return sym + " " + num
		case 3:
			return num + " " + sym
		}
		return sym + num
	}
	// The sixteen shapes of a negative amount, as LOCALE_INEGCURR numbers them.
	switch loc.currNeg {
	case 0:
		return "(" + sym + num + ")"
	case 2:
		return sym + m + num
	case 3:
		return sym + num + m
	case 4:
		return "(" + num + sym + ")"
	case 5:
		return m + num + sym
	case 6:
		return num + m + sym
	case 7:
		return num + sym + m
	case 8:
		return m + num + " " + sym
	case 9:
		return m + sym + " " + num
	case 10:
		return num + " " + sym + m
	case 11:
		return sym + " " + num + m
	case 12:
		return sym + " " + m + num
	case 13:
		return num + m + " " + sym
	case 14:
		return "(" + sym + " " + num + ")"
	case 15:
		return "(" + num + " " + sym + ")"
	}
	return m + sym + num
}

// roundDecimal returns v, which is not negative, rounded to decimals places, as
// Windows rounds numbers and amounts: half away from zero, on the shortest decimal
// form of v. So 1.005 is 1.01 at two places, as it reads, although its float is a
// little less than 1.005, which strconv rounds down.
func roundDecimal(v float64, decimals int) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) <= decimals {
		frac += strings.Repeat("0", decimals-len(frac))
	} else {
		up := frac[decimals] >= '5'
		digits := []byte(whole + frac[:decimals])
		for i := len(digits) - 1; up && i >= 0; i-- {
			if digits[i] == '9' {
				digits[i] = '0'
				continue
			}
			digits[i]++
			up = false
		}
		if up {
			digits = append([]byte{'1'}, digits...)
		}
		whole, frac = string(digits[:len(digits)-decimals]), string(digits[len(digits)-decimals:])
	}
	if decimals == 0 {
		return whole
	}
	return whole + "." + frac
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
