// Package lexer runs @pierre/highlights, the syntax highlighter of the Pierre
// Computer Company (https://github.com/pierrecomputer/pierre, Apache-2.0). The
// highlighter is a WebAssembly module, written in WebAssembly Text,
// with a lexer for each of 76 languages. It reads UTF-8 text and gives each line
// a list of tokens, without a syntax tree, and it keeps the state of its lexers
// at the end of each line, so an edit relexes only the lines it changes, and the
// lines after them until the state matches what it was.
//
// highlights.wasm is the module of the npm package's release named by Version,
// unchanged. The update command (go generate) downloads it and writes the
// tables of language and token names that match it; see README.md.
//
// The package compiles the module once per process. It runs it in wazero, which
// keeps the compiled code in a per-user cache directory, except in a js build
// (a web page, or Node), which has no wazero and runs it in the WebAssembly of
// the JavaScript host (module_js.go). An Instance holds one module instance. It
// is not safe for concurrent use.
package lexer

//go:generate go run ./update -version 0.1.3

import (
	_ "embed"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed highlights.wasm
var wasm []byte

// LanguageID returns the module's ID for a language name or alias, such as "go",
// "golang" or "TS", in any case, and false for a name it does not know.
func LanguageID(name string) (int, bool) {
	id, ok := languageIDs[strings.ToLower(strings.TrimSpace(name))]
	return id, ok
}

// LanguageNames returns every name and alias that LanguageID knows, in lower
// case, in no particular order.
func LanguageNames() []string {
	out := make([]string, 0, len(languageIDs))
	for n := range languageIDs {
		out = append(out, n)
	}
	return out
}

// TokenNames returns the names of the token IDs, indexed by ID, such as
// "keyword.control" or "string.escape". ID 0 is "none", which marks text that
// has no token. The caller must not change the slice.
func TokenNames() []string { return tokenNames }

// idStartAt and idContinueAt are the module's imports env.is_id_start and
// env.is_id_continue, given the bytes of the character. The ECMAScript lexers
// call them for a non-ASCII character, the n bytes of UTF-8 at p, to ask whether
// it can start or continue an identifier: whether it has the Unicode property
// ID_Start, or ID_Continue (or is U+200C or U+200D).
func idStartAt(b []byte) uint32 {
	r, _ := utf8.DecodeRune(b)
	return boolU32(idStart(r))
}

func idContinueAt(b []byte) uint32 {
	r, _ := utf8.DecodeRune(b)
	return boolU32(r == 0x200C || r == 0x200D || idContinue(r))
}

// idStart reports whether r has the Unicode property ID_Start: a letter, a
// letter number or Other_ID_Start, but not Pattern_Syntax or Pattern_White_Space.
func idStart(r rune) bool {
	if r == utf8.RuneError || unicode.In(r, unicode.Pattern_Syntax, unicode.Pattern_White_Space) {
		return false
	}
	return unicode.In(r, unicode.L, unicode.Nl, unicode.Other_ID_Start)
}

// idContinue reports whether r has the Unicode property ID_Continue: ID_Start, a
// combining mark, a decimal digit, a connector or Other_ID_Continue, but not
// Pattern_Syntax or Pattern_White_Space.
func idContinue(r rune) bool {
	if idStart(r) {
		return true
	}
	if r == utf8.RuneError || unicode.In(r, unicode.Pattern_Syntax, unicode.Pattern_White_Space) {
		return false
	}
	return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Other_ID_Continue)
}

func boolU32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}
