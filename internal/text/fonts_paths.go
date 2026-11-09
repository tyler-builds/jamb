//go:build !windows && !js

package text

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
