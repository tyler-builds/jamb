//go:build ignore

// fonts.go writes the font files that a web page serves next to its app: the Go
// fonts, which a js build fetches instead of compiling them in (see
// internal/text/fonts_js.go). Run it from the repository's root:
//
//	go run scripts/web/fonts.go DIR
//
// build.sh runs it for the page's fonts directory.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomediumitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/tyler-builds/jamb/internal/text"
)

var data = map[string][]byte{
	"Go-Regular.ttf":          goregular.TTF,
	"Go-Medium.ttf":           gomedium.TTF,
	"Go-Bold.ttf":             gobold.TTF,
	"Go-Mono.ttf":             gomono.TTF,
	"Go-Italic.ttf":           goitalic.TTF,
	"Go-Medium-Italic.ttf":    gomediumitalic.TTF,
	"Go-Bold-Italic.ttf":      gobolditalic.TTF,
	"Go-Mono-Italic.ttf":      gomonoitalic.TTF,
	"Go-Mono-Bold-Italic.ttf": gomonobolditalic.TTF,
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/web/fonts.go DIR")
		os.Exit(2)
	}
	dir := os.Args[1]
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, name := range text.WebFontFiles() {
		b, ok := data[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "fonts.go: no data for %s, which the text package fetches\n", name)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
