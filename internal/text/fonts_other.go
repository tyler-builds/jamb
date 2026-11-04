//go:build !windows

package text

import (
	"os"

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

// readFontFile reads a font file into memory.
func readFontFile(path string) ([]byte, error) { return os.ReadFile(path) }

// Candidate system font files for macOS and Linux. Missing files are fine:
// the embedded Go fonts are used instead. On Linux, the faces fontconfig finds
// (fontconfig_linux.go) come first, and these paths are a last resort. A family without an italic file
// here uses the embedded Go italic, and one with neither uses the upright face.
func systemFonts() map[string]map[faceKey][]string {
	return map[string]map[faceKey][]string{
		FamilySans: {
			{Regular, false}: {
				"/System/Library/Fonts/Supplemental/Arial.ttf",
				"/usr/share/fonts/truetype/inter/Inter-Regular.ttf",
				"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
				"/usr/share/fonts/noto/NotoSans-Regular.ttf",
				"/usr/share/fonts/google-noto/NotoSans-Regular.ttf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			},
			{Bold, false}: {
				"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
				"/usr/share/fonts/truetype/inter/Inter-Bold.ttf",
				"/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf",
				"/usr/share/fonts/noto/NotoSans-Bold.ttf",
				"/usr/share/fonts/google-noto/NotoSans-Bold.ttf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
			},
			{Regular, true}: {
				"/System/Library/Fonts/Supplemental/Arial Italic.ttf",
				"/usr/share/fonts/truetype/inter/Inter-Italic.ttf",
				"/usr/share/fonts/truetype/noto/NotoSans-Italic.ttf",
				"/usr/share/fonts/noto/NotoSans-Italic.ttf",
				"/usr/share/fonts/google-noto/NotoSans-Italic.ttf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans-Oblique.ttf",
			},
			{Bold, true}: {
				"/System/Library/Fonts/Supplemental/Arial Bold Italic.ttf",
				"/usr/share/fonts/truetype/inter/Inter-BoldItalic.ttf",
				"/usr/share/fonts/truetype/noto/NotoSans-BoldItalic.ttf",
				"/usr/share/fonts/noto/NotoSans-BoldItalic.ttf",
				"/usr/share/fonts/google-noto/NotoSans-BoldItalic.ttf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans-BoldOblique.ttf",
			},
		},
		FamilyMono: {
			{Regular, false}: {
				"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
				"/usr/share/fonts/truetype/noto/NotoSansMono-Regular.ttf",
				"/usr/share/fonts/google-noto/NotoSansMono-Regular.ttf",
				"/usr/share/fonts/dejavu-sans-mono-fonts/DejaVuSansMono.ttf",
				"/usr/share/fonts/TTF/DejaVuSansMono.ttf",
			},
			{Regular, true}: {
				"/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Oblique.ttf",
			},
		},
	}
}

// systemFallbackFonts lists the fallback fonts at their paths on Debian and
// Ubuntu, Fedora and Arch. A missing file is skipped. On Linux, the emoji font
// and the font for any other character come from fontconfig.
func systemFallbackFonts() []fallbackSpec {
	return []fallbackSpec{
		{"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", isCJK},
		{"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc", isCJK},
		{"/usr/share/fonts/google-noto-sans-cjk-fonts/NotoSansCJK-Regular.ttc", isCJK},
		{"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc", isCJK},
		{"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", anyRune},
		{"/usr/share/fonts/dejavu-sans-fonts/DejaVuSans.ttf", anyRune},
		{"/usr/share/fonts/TTF/DejaVuSans.ttf", anyRune},
	}
}

func isCJK(r rune) bool { return r >= 0x2E80 && r <= 0x9FFF || r >= 0xAC00 && r <= 0xD7AF }
