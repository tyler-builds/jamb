package text

// webFont is a face of the Go fonts that a js build fetches from the page's
// server rather than compiling it in (see fonts_js.go). The file is served
// next to the app, in its fonts directory (see scripts/web/fonts.go).
type webFont struct {
	family string
	key    faceKey
	name   string
	file   string
}

// webFonts are the faces a page fetches: the Go fonts that other builds embed
// (see fonts_other.go). The sans Regular comes first: a page waits for it
// before its first frame, and fetches each other face the first time text
// asks for it.
var webFonts = []webFont{
	{FamilySans, faceKey{Regular, false}, "Go Regular", "Go-Regular.ttf"},
	{FamilySans, faceKey{Medium, false}, "Go Medium", "Go-Medium.ttf"},
	{FamilySans, faceKey{Bold, false}, "Go Bold", "Go-Bold.ttf"},
	{FamilyMono, faceKey{Regular, false}, "Go Mono", "Go-Mono.ttf"},
	{FamilySans, faceKey{Regular, true}, "Go Italic", "Go-Italic.ttf"},
	{FamilySans, faceKey{Medium, true}, "Go Medium Italic", "Go-Medium-Italic.ttf"},
	{FamilySans, faceKey{Bold, true}, "Go Bold Italic", "Go-Bold-Italic.ttf"},
	{FamilyMono, faceKey{Regular, true}, "Go Mono Italic", "Go-Mono-Italic.ttf"},
	{FamilyMono, faceKey{Bold, true}, "Go Mono Bold Italic", "Go-Mono-Bold-Italic.ttf"},
}

// WebFontFiles returns the names of the font files that a web page serves in
// the fonts directory next to its app, in the order of webFonts: the first is
// the one the page waits for, which it may preload.
func WebFontFiles() []string {
	names := make([]string, len(webFonts))
	for i, f := range webFonts {
		names[i] = f.file
	}
	return names
}
