//go:build !linux

package text

// Fontconfig is Linux only. Windows lists its own fonts (fonts_windows.go),
// and macOS uses the fixed paths of fonts_other.go.

func fontconfigFaces() map[string]map[faceKey]fontFile { return nil }

func fontconfigFallbacks() []fileFallback { return nil }

func fontconfigRune(rune) (fontFile, bool) { return fontFile{}, false }
