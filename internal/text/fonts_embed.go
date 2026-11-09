//go:build !js || jamb_embedfonts

package text

import "golang.org/x/image/font/gofont/goregular"

// A build for the desktop compiles its fonts in (see embeddedFonts), as a js
// build does with the jamb_embedfonts tag: there is nothing to fetch, and
// nothing to wait for.

func addPlatformFonts(*Registry) {}

func waitPlatformFonts(*Registry) error { return nil }

// lastResort is the font of last resort: the embedded Go Regular.
func lastResort(*Registry) *lazyFont {
	return &lazyFont{load: func() (*Font, error) { return ParseFont("Go Regular", goregular.TTF) }}
}
