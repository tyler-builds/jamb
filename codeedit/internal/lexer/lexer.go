// Package lexer runs @pierre/highlights, the syntax highlighter of the Pierre
// Computer Company (https://github.com/pierrecomputer/pierre, Apache-2.0), in
// wazero. The highlighter is a WebAssembly module, written in WebAssembly Text,
// with a lexer for each of 76 languages. It reads UTF-8 text and gives each line
// a list of tokens, without a syntax tree, and it keeps the state of its lexers
// at the end of each line, so an edit relexes only the lines it changes, and the
// lines after them until the state matches what it was.
//
// highlights.wasm is the module of the npm package's release named by Version,
// unchanged. The update command (go generate) downloads it and writes the
// tables of language and token names that match it; see README.md.
//
// The package compiles the module once per process, and keeps the compiled code
// in a per-user cache directory. An Instance holds one module instance. It is
// not safe for concurrent use.
package lexer

//go:generate go run ./update -version 0.1.3

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed highlights.wasm
var wasm []byte

// cacheSubdir is the directory, under os.UserCacheDir, that holds the compiled
// module.
var cacheSubdir = filepath.Join("jamb", "wasm")

// mu guards the process's runtime and the compiled module, and every call that
// instantiates or closes a module. Calls into a running instance do not take it.
var (
	mu       sync.Mutex
	rt       wazero.Runtime
	compiled wazero.CompiledModule
)

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

// module returns the compiled module, compiling it on first use. The caller
// holds mu.
func module() (wazero.Runtime, wazero.CompiledModule, error) {
	if compiled != nil {
		return rt, compiled, nil
	}
	if rt == nil {
		r, err := newRuntime(defaultCache())
		if err != nil {
			return nil, nil, err
		}
		rt = r
	}
	cm, err := rt.CompileModule(context.Background(), wasm)
	if err != nil {
		return nil, nil, fmt.Errorf("lexer: compile module: %w", err)
	}
	compiled = cm
	return rt, compiled, nil
}

// newRuntime returns a runtime with the host functions that the module imports.
// A nil cache leaves the runtime to compile the module from source.
func newRuntime(cache wazero.CompilationCache) (wazero.Runtime, error) {
	cfg := wazero.NewRuntimeConfig()
	if cache != nil {
		cfg = cfg.WithCompilationCache(cache)
	}
	ctx := context.Background()
	r := wazero.NewRuntimeWithConfig(ctx, cfg)
	_, err := r.NewHostModuleBuilder("env").
		NewFunctionBuilder().WithFunc(isIDStart).Export("is_id_start").
		NewFunctionBuilder().WithFunc(isIDContinue).Export("is_id_continue").
		Instantiate(ctx)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("lexer: host functions: %w", err)
	}
	return r, nil
}

// defaultCache returns the process's compilation cache: a directory named jamb
// under the user's cache directory, or memory when that directory is not
// available.
func defaultCache() wazero.CompilationCache {
	base, err := os.UserCacheDir()
	if err != nil {
		return wazero.NewCompilationCache()
	}
	return openCache(filepath.Join(base, cacheSubdir))
}

// openCache returns a compilation cache in dir, creating dir if needed. It
// falls back to an in-memory cache when dir cannot be used, so an unwritable
// cache never prevents highlighting. Nothing is written outside dir.
func openCache(dir string) wazero.CompilationCache {
	if err := os.MkdirAll(dir, 0o755); err == nil {
		if c, err := wazero.NewCompilationCacheWithDir(dir); err == nil {
			return c
		}
	}
	return wazero.NewCompilationCache()
}

// isIDStart and isIDContinue are the module's imports env.is_id_start and
// env.is_id_continue. The ECMAScript lexers call them for a non-ASCII character,
// the n bytes of UTF-8 at p, to ask whether it can start or continue an
// identifier: whether it has the Unicode property ID_Start, or ID_Continue (or
// is U+200C or U+200D).
func isIDStart(_ context.Context, m api.Module, p, n uint32) uint32 {
	return boolU32(idStart(runeAt(m, p, n)))
}

func isIDContinue(_ context.Context, m api.Module, p, n uint32) uint32 {
	r := runeAt(m, p, n)
	return boolU32(r == 0x200C || r == 0x200D || idContinue(r))
}

// runeAt decodes the character of n bytes at p in m's memory, or returns
// utf8.RuneError.
func runeAt(m api.Module, p, n uint32) rune {
	b, ok := m.Memory().Read(p, n)
	if !ok {
		return utf8.RuneError
	}
	r, _ := utf8.DecodeRune(b)
	return r
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
