//go:build !windows && (!js || jamb_embedfonts)

package text

import (
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomediumitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"
)

// embeddedFonts are the Go fonts, since which system fonts exist varies by
// distribution. Italic faces fall back to the Go italics the same way upright
// ones do.
func embeddedFonts() []embeddedFont {
	return []embeddedFont{
		{FamilySans, faceKey{Regular, false}, "Go Regular", goregular.TTF},
		{FamilySans, faceKey{Medium, false}, "Go Medium", gomedium.TTF},
		{FamilySans, faceKey{Bold, false}, "Go Bold", gobold.TTF},
		{FamilyMono, faceKey{Regular, false}, "Go Mono", gomono.TTF},
		{FamilySans, faceKey{Regular, true}, "Go Italic", goitalic.TTF},
		{FamilySans, faceKey{Medium, true}, "Go Medium Italic", gomediumitalic.TTF},
		{FamilySans, faceKey{Bold, true}, "Go Bold Italic", gobolditalic.TTF},
		{FamilyMono, faceKey{Regular, true}, "Go Mono Italic", gomonoitalic.TTF},
		{FamilyMono, faceKey{Bold, true}, "Go Mono Bold Italic", gomonobolditalic.TTF},
	}
}

func isCJK(r rune) bool { return r >= 0x2E80 && r <= 0x9FFF || r >= 0xAC00 && r <= 0xD7AF }
