//go:build js

package text

// A js build reads no system font files. A web page has none it can read, and
// under Node the os package would read the host's, so the same build would draw
// with different fonts there than in a browser. Nor does it compile the Go
// fonts in: they would be a third of a small app's download. It fetches them
// from the page's server, as a web page fetches its fonts, where the browser
// caches them across visits and versions of the app (see webFonts and
// fonts_fetch.go). The page waits for Go Regular before its first frame
// (System.WaitForFonts); every other face is fetched the first time text asks
// for it, and the text is drawn with the face nearest to it until it arrives
// (see System.OnFontLoaded).
//
// The files are in the fonts directory next to the page, or in the directory
// that the page's jambEnv.JAMB_FONTS names (a URL that ends in a slash). A host
// with no server to fetch from, such as Node, sets globalThis.jambFetchFont to
// a function that takes a file's name and returns a promise of its bytes, as
// an ArrayBuffer or a Uint8Array. The jamb_embedfonts build tag compiles the
// Go fonts in instead, as the other builds do, for an app that cannot serve
// them and for tests under Node.

func systemFonts() map[string]map[faceKey][]string { return map[string]map[faceKey][]string{} }

func systemFallbackFonts() []fallbackSpec { return nil }
