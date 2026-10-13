//go:build !windows

package ui

// readSystemLocale returns the English formats. Only Windows has the locale settings
// that the pickers read (see locale_windows.go).
func readSystemLocale() *dateLocale { return englishLocale() }
