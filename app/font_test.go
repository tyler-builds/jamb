package app

import (
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/tyler-builds/jamb/ui"
)

// App.Font registers each face in the text system, and data that is not a
// font is an error that names the font.
func TestAppFontRegistersFaces(t *testing.T) {
	a := New().Font("Brand", ui.FontRegular, goregular.TTF).Font("Brand", ui.FontBold, gobold.TTF)
	res := loadText(a.fonts, 14)
	if res.err != nil {
		t.Fatal(res.err)
	}
	reg := res.ts.Registry()
	if reg.Font("Brand", ui.FontRegular, false) == reg.Font("Brand", ui.FontBold, false) {
		t.Fatal("the regular and bold faces of the family are the same font")
	}

	bad := New().Font("Broken", ui.FontRegular, []byte("not a font"))
	res = loadText(bad.fonts, 14)
	if res.err == nil || !strings.Contains(res.err.Error(), `"Broken"`) {
		t.Fatalf("error %v, want one that names the font", res.err)
	}
}
