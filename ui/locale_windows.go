//go:build windows

package ui

import (
	"syscall"
	"time"
	"unsafe"
)

// The date and time pickers read the user's locale settings. These calls only read
// them: GetLocaleInfoEx for the patterns and the clock, and GetDateFormatEx for the
// names of the days and the months, so that they are in the user's language.
var (
	kernel32Locale                  = syscall.NewLazyDLL("kernel32.dll")
	procGetLocaleInfoEx             = kernel32Locale.NewProc("GetLocaleInfoEx")
	procGetDateFormatEx             = kernel32Locale.NewProc("GetDateFormatEx")
	procGetUserPreferredUILanguages = kernel32Locale.NewProc("GetUserPreferredUILanguages")
)

// The LOCALE_* and DATE_* values from winnls.h. A null locale name is the user's
// default locale.
const (
	lcShortDate       = 0x001F // LOCALE_SSHORTDATE
	lcTimeSep         = 0x001E // LOCALE_STIME
	lcITime           = 0x0023 // LOCALE_ITIME: 0 for a 12 hour clock, 1 for 24 hours
	lcAM              = 0x0028 // LOCALE_S1159
	lcPM              = 0x0029 // LOCALE_S2359
	lcFirstDayOfWeek  = 0x100C // LOCALE_IFIRSTDAYOFWEEK: 0 is Monday, 6 is Sunday
	lcReturnNumber    = 0x20000000
	dateLongDate      = 0x0002 // DATE_LONGDATE
	muiLanguageName   = 0x0008 // MUI_LANGUAGE_NAME: the languages as tags such as "it-IT"
	localeBufferChars = 80
	dateBufferChars   = 128
)

// systemLanguage returns the first of the user's preferred UI languages, which is the
// display language of Windows, as a tag such as "it-IT" or "zh-TW". It is "" when the
// languages cannot be read. The UI language is not the region format, which the date
// and time settings above follow.
func systemLanguage() string {
	var n, size uint32
	// The first call, with no buffer, sets the size of the list in characters.
	procGetUserPreferredUILanguages.Call(muiLanguageName, uintptr(unsafe.Pointer(&n)), 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return ""
	}
	buf := make([]uint16, size)
	ok, _, _ := procGetUserPreferredUILanguages.Call(muiLanguageName, uintptr(unsafe.Pointer(&n)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf) // the list is null separated; the first is the preferred one
}

// systemTime is the SYSTEMTIME structure that GetDateFormatEx takes.
type systemTime struct {
	year, month, dayOfWeek, day, hour, minute, second, milliseconds uint16
}

// readSystemLocale reads the formats of the user's locale. A setting that cannot be
// read keeps its English value.
func readSystemLocale() *dateLocale {
	l := englishLocale()
	if s := localeString(lcShortDate); s != "" {
		l.shortDate = s
	}
	if v, ok := localeNumber(lcFirstDayOfWeek); ok {
		l.firstDay = time.Weekday((v + 1) % 7) // Windows counts from Monday; Go from Sunday
	}
	if v, ok := localeNumber(lcITime); ok {
		l.twelveHour = v == 0
	}
	if s := localeString(lcAM); s != "" {
		l.am = s
	}
	if s := localeString(lcPM); s != "" {
		l.pm = s
	}
	if s := localeString(lcTimeSep); s != "" {
		l.timeSep = s
	}
	english := englishLocale()
	l.longDate = func(t time.Time) string {
		return orElse(formatSystemDate(t, dateLongDate, ""), english.longDate(t))
	}
	l.monthYear = func(t time.Time) string {
		return orElse(formatSystemDate(t, 0, "MMMM yyyy"), english.monthYear(t))
	}
	l.weekdayLong = func(d time.Weekday) string {
		return orElse(formatSystemDate(weekdayRef(d), 0, "dddd"), english.weekdayLong(d))
	}
	l.weekdayShort = func(d time.Weekday) string {
		return orElse(formatSystemDate(weekdayRef(d), 0, "ddd"), english.weekdayShort(d))
	}
	return l
}

// orElse returns s, or fallback when s is empty.
func orElse(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// weekdayRef returns a day of the week d, for a format that names the day. January 4,
// 2026 was a Sunday.
func weekdayRef(d time.Weekday) time.Time {
	return time.Date(2026, time.January, 4+int(d), 0, 0, 0, 0, time.Local)
}

// localeString returns the string setting lctype of the user's locale, or "".
func localeString(lctype uint32) string {
	buf := make([]uint16, localeBufferChars)
	n, _, _ := procGetLocaleInfoEx.Call(0, uintptr(lctype), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n-1]) // n counts the terminating null
}

// localeNumber returns the numeric setting lctype of the user's locale, and whether
// it was read.
func localeNumber(lctype uint32) (uint32, bool) {
	var v uint32
	// With LOCALE_RETURN_NUMBER the buffer is one UINT, which is two characters.
	n, _, _ := procGetLocaleInfoEx.Call(0, uintptr(lctype|lcReturnNumber), uintptr(unsafe.Pointer(&v)), 2)
	return v, n != 0
}

// formatSystemDate formats t with GetDateFormatEx: with the long date when flags is
// dateLongDate and format is empty, or with the custom pattern format otherwise.
func formatSystemDate(t time.Time, flags uint32, format string) string {
	st := systemTime{year: uint16(t.Year()), month: uint16(t.Month()), dayOfWeek: uint16(t.Weekday()), day: uint16(t.Day())}
	var fp *uint16
	if format != "" {
		fp, _ = syscall.UTF16PtrFromString(format)
	}
	buf := make([]uint16, dateBufferChars)
	n, _, _ := procGetDateFormatEx.Call(0, uintptr(flags), uintptr(unsafe.Pointer(&st)),
		uintptr(unsafe.Pointer(fp)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n-1])
}
