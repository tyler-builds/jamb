//go:build !windows

package text

// Candidate system font files for macOS and Linux. Missing files are fine:
// the embedded Go fonts are used instead. A family without an italic file
// here uses the embedded Go italic, and one with neither uses the upright face.
func systemFonts() map[string]map[faceKey][]string {
	return map[string]map[faceKey][]string{
		FamilySans: {
			{Regular, false}: {
				"/System/Library/Fonts/Supplemental/Arial.ttf",
				"/usr/share/fonts/truetype/inter/Inter-Regular.ttf",
				"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
				"/usr/share/fonts/noto/NotoSans-Regular.ttf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			},
			{Bold, false}: {
				"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
				"/usr/share/fonts/truetype/inter/Inter-Bold.ttf",
				"/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf",
				"/usr/share/fonts/noto/NotoSans-Bold.ttf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
			},
			{Regular, true}: {
				"/System/Library/Fonts/Supplemental/Arial Italic.ttf",
				"/usr/share/fonts/truetype/inter/Inter-Italic.ttf",
				"/usr/share/fonts/truetype/noto/NotoSans-Italic.ttf",
				"/usr/share/fonts/noto/NotoSans-Italic.ttf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans-Oblique.ttf",
			},
			{Bold, true}: {
				"/System/Library/Fonts/Supplemental/Arial Bold Italic.ttf",
				"/usr/share/fonts/truetype/inter/Inter-BoldItalic.ttf",
				"/usr/share/fonts/truetype/noto/NotoSans-BoldItalic.ttf",
				"/usr/share/fonts/noto/NotoSans-BoldItalic.ttf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans-BoldOblique.ttf",
			},
		},
		FamilyMono: {
			{Regular, false}: {
				"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
				"/usr/share/fonts/truetype/noto/NotoSansMono-Regular.ttf",
			},
			{Regular, true}: {
				"/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Oblique.ttf",
			},
		},
	}
}

func systemFallbackFonts() []fallbackSpec {
	return []fallbackSpec{
		{"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc", isCJK},
		{"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", anyRune},
	}
}

func isCJK(r rune) bool { return r >= 0x2E80 && r <= 0x9FFF || r >= 0xAC00 && r <= 0xD7AF }
