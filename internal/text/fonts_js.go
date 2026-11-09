//go:build js

package text

// A js build reads no system font files. A web page has none it can read, and
// under Node the os package would read the host's, so the same build would draw
// with different fonts there than in a browser. It uses the embedded Go fonts,
// and the fonts that the app registers.

func systemFonts() map[string]map[faceKey][]string { return map[string]map[faceKey][]string{} }

func systemFallbackFonts() []fallbackSpec { return nil }
