//go:build !windows

package ui

// readSystemLocale returns the English formats. Only Windows has the locale settings
// that the pickers read (see locale_windows.go).
func readSystemLocale() *dateLocale { return englishLocale() }

// systemLanguage returns no language, so the text is English. Only Windows has the
// display language that SystemStrings reads (see locale_windows.go).
func systemLanguage() string { return "" }
