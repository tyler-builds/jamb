//go:build windows

package text

import (
	"os"
	"path/filepath"
	"syscall"
	"unicode"
	"unsafe"

	"golang.org/x/image/font/gofont/goregular"
)

// readFontFile maps a font file read-only instead of copying it onto the Go
// heap. The pages are shared with the system font cache and other processes,
// so they do not count as this process's private memory. Fonts are never
// unloaded, so the view stays mapped for the life of the process.
func readFontFile(path string) ([]byte, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	const share = syscall.FILE_SHARE_READ | syscall.FILE_SHARE_DELETE
	f, err := syscall.CreateFile(name, syscall.GENERIC_READ, share, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return os.ReadFile(path) // reports a missing file; copes with a writer
	}
	defer syscall.CloseHandle(f)
	var st syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(f, &st); err != nil {
		return nil, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	size := int64(st.FileSizeHigh)<<32 | int64(st.FileSizeLow)
	if size == 0 || size > 1<<30 {
		return os.ReadFile(path) // nothing to map, or not a plausible font
	}
	m, err := syscall.CreateFileMapping(f, nil, syscall.PAGE_READONLY, 0, 0, nil)
	if err != nil {
		return os.ReadFile(path)
	}
	defer syscall.CloseHandle(m) // the view keeps the mapping alive
	addr, err := syscall.MapViewOfFile(m, syscall.FILE_MAP_READ, 0, 0, 0)
	if err != nil {
		return os.ReadFile(path)
	}
	return unsafe.Slice((*byte)(*(*unsafe.Pointer)(unsafe.Pointer(&addr))), int(size)), nil
}

func winFont(name string) string {
	dir := os.Getenv("WINDIR")
	if dir == "" {
		dir = `C:\Windows`
	}
	return filepath.Join(dir, "Fonts", name)
}

// embeddedFonts is only Go Regular, the last resort the registry always has:
// every supported Windows version ships Segoe UI and Consolas (see
// systemFonts). The other Go faces would add about 1 MB to every binary.
func embeddedFonts() []embeddedFont {
	return []embeddedFont{{FamilySans, faceKey{Regular, false}, "Go Regular", goregular.TTF}}
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
