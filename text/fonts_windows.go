//go:build windows

package text

import (
	"os"
	"path/filepath"
	"unicode"

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

func winFont(name string) string {
	dir := os.Getenv("WINDIR")
	if dir == "" {
		dir = `C:\Windows`
	}
	return filepath.Join(dir, "Fonts", name)
}

// embeddedFonts are the Go fonts, used only for faces no system font
// provides (see pick).
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

func systemFonts() map[string]map[faceKey][]string {
	return map[string]map[faceKey][]string{
		FamilySans: {
			{Light, false}:    {winFont("segoeuil.ttf")},
			{Regular, false}:  {winFont("segoeui.ttf")},
			{Semibold, false}: {winFont("seguisb.ttf")},
			{Bold, false}:     {winFont("segoeuib.ttf")},
			{Light, true}:     {winFont("seguili.ttf")},
			{Regular, true}:   {winFont("segoeuii.ttf")},
			{Semibold, true}:  {winFont("seguisbi.ttf")},
			{Bold, true}:      {winFont("segoeuiz.ttf")},
		},
		FamilyMono: {
			{Regular, false}: {winFont("consola.ttf"), winFont("cour.ttf")},
			{Bold, false}:    {winFont("consolab.ttf")},
			{Regular, true}:  {winFont("consolai.ttf")},
			{Bold, true}:     {winFont("consolaz.ttf")},
		},
	}
}

func systemFallbackFonts() []fallbackSpec {
	// Segoe UI already covers Latin, Greek, Cyrillic, Arabic and Hebrew.
	// Fallbacks are loaded lazily, only when a character needs them.
	return []fallbackSpec{
		{winFont("seguiemj.ttf"), isEmoji}, // emoji, drawn monochrome
		{winFont("Nirmala.ttc"), inTables(unicode.Devanagari, unicode.Bengali, unicode.Gurmukhi, unicode.Gujarati,
			unicode.Oriya, unicode.Tamil, unicode.Telugu, unicode.Kannada, unicode.Malayalam, unicode.Sinhala)},
		{winFont("LeelawUI.ttf"), inTables(unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar)},
		// Yu Gothic Medium matches Segoe UI's weight on screen; Regular is thin.
		{winFont("YuGothM.ttc"), inTables(unicode.Hiragana, unicode.Katakana)},
		{winFont("malgun.ttf"), inTables(unicode.Hangul)},
		{winFont("msyh.ttc"), inTables(unicode.Han, unicode.Bopomofo)},
		{winFont("seguisym.ttf"), anyRune}, // symbols and everything else
	}
}
