package lexer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestMain keeps the tests' compiled-module cache in a temporary directory, so
// that they do not write to the user's cache.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "jamb-test-cache")
	if err != nil {
		panic(err)
	}
	for _, k := range []string{"LOCALAPPDATA", "XDG_CACHE_HOME", "HOME"} {
		os.Setenv(k, dir)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newInstance(t testing.TB) *Instance {
	t.Helper()
	in, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { in.Close() })
	return in
}

func langID(t testing.TB, name string) int {
	t.Helper()
	id, ok := LanguageID(name)
	if !ok {
		t.Fatalf("no language %q", name)
	}
	return id
}

func TestModuleMatchesTables(t *testing.T) {
	sum := sha256.Sum256(wasm)
	if got := hex.EncodeToString(sum[:]); got != wasmSHA256 {
		t.Fatalf("highlights.wasm has SHA-256 %s, and tables.go was written for %s; run go generate", got, wasmSHA256)
	}
	if TokenNames()[0] != "none" {
		t.Fatalf("token 0 is %q, want none", TokenNames()[0])
	}
}

func TestLanguageID(t *testing.T) {
	for _, n := range []string{"go", "Golang", " TS ", "python", "py", "rust", "c++", "plain"} {
		if _, ok := LanguageID(n); !ok {
			t.Errorf("LanguageID(%q) is not known", n)
		}
	}
	if id, _ := LanguageID("ts"); id != langID(t, "typescript") {
		t.Error("ts and typescript have different IDs")
	}
	if _, ok := LanguageID("klingon"); ok {
		t.Error("LanguageID knows klingon")
	}
	if names := LanguageNames(); !slices.Contains(names, "go") || len(names) != len(languageIDs) {
		t.Errorf("LanguageNames: %d names", len(names))
	}
}

// tokenAt returns the name of the token that covers the first match of word in src.
func tokenAt(t *testing.T, toks []Token, src, word string) string {
	t.Helper()
	at := strings.Index(src, word)
	if at < 0 {
		t.Fatalf("no %q in the source", word)
	}
	for _, tk := range toks {
		if tk.Start <= at && at < tk.End {
			return TokenNames()[tk.ID]
		}
	}
	return "none"
}

func TestLexGo(t *testing.T) {
	in := newInstance(t)
	src := "package main\n\n// Hello says hi.\nfunc Hello() string {\n\treturn \"hi\\n\" + strconv.Itoa(42)\n}\n"
	toks, err := in.Lex(langID(t, "go"), Sanitize([]byte(src)), nil)
	if err != nil {
		t.Fatal(err)
	}
	for word, want := range map[string]string{
		"package": "keyword", "func": "keyword", "// Hello": "comment", `"hi`: "string",
		"42": "number", "Hello()": "function", "return": "keyword",
	} {
		if got := tokenAt(t, toks, src, word); !strings.HasPrefix(got, want) {
			t.Errorf("%q is %q, want %s", word, got, want)
		}
	}
	for i, tk := range toks {
		if tk.Start >= tk.End || tk.End > len(src) || i > 0 && tk.Start < toks[i-1].End || tk.ID == 0 {
			t.Fatalf("token %d is %+v", i, tk)
		}
	}
}

func TestLexNonASCII(t *testing.T) {
	in := newInstance(t)
	// Text before the string has characters of one, two and three bytes, and one
	// outside the BMP (two UTF-16 units), so the byte offsets are checked.
	src := "// é ☃ 😀\nconst café = \"ü\"; // done\n"
	toks, err := in.Lex(langID(t, "js"), Sanitize([]byte(src)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := tokenAt(t, toks, src, `"ü"`); got != "string" {
		t.Errorf(`"ü" is %q, want string`, got)
	}
	if got := tokenAt(t, toks, src, "// done"); got != "comment" {
		t.Errorf("// done is %q, want comment", got)
	}
	for _, tk := range toks {
		if s := src[tk.Start:tk.End]; strings.HasPrefix(TokenNames()[tk.ID], "string") && s != `"ü"` {
			t.Errorf("string token %q", s)
		}
	}
}

func TestEveryLanguage(t *testing.T) {
	in := newInstance(t)
	src := Sanitize([]byte("# x\nfn a(b) { return \"c\" + 1; } // d\n<e f=\"g\">h</e>\n/* i */ -- j\n"))
	seen := map[int]bool{}
	for _, name := range LanguageNames() {
		id, _ := LanguageID(name)
		if seen[id] {
			continue
		}
		seen[id] = true
		if _, err := in.Lex(id, src, nil); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		live := newInstance(t)
		if err := live.Open(id, src); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := live.Run(0); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		live.Close()
	}
	if len(seen) < 70 {
		t.Fatalf("only %d languages", len(seen))
	}
}

// liveIDs returns the token ID of each byte of the live document's text, and
// the text itself, read back from the instance.
func liveIDs(t *testing.T, in *Instance) ([]int, []byte) {
	t.Helper()
	n, err := in.LineCount()
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	var text []byte
	var toks []LineToken
	for i := range n {
		line, err := in.LineText(i)
		if err != nil {
			t.Fatal(err)
		}
		line = bytes.Clone(line)
		toks, err = in.LineTokens(i, toks[:0])
		if err != nil {
			t.Fatal(err)
		}
		row := make([]int, len(line))
		start := 0
		for _, tk := range toks {
			end := ByteOffset(line, tk.End)
			for b := start; b < end; b++ {
				row[b] = tk.ID
			}
			start = max(start, end)
		}
		ids = append(ids, row...)
		text = append(text, line...)
		if i < n-1 {
			ids = append(ids, 0)
			text = append(text, '\n')
		}
	}
	return ids, text
}

// lexIDs returns the token ID of each byte of text, from a whole-text lex.
func lexIDs(t *testing.T, in *Instance, lang int, text []byte) []int {
	t.Helper()
	toks, err := in.Lex(lang, text, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int, len(text))
	for _, tk := range toks {
		for b := tk.Start; b < tk.End; b++ {
			ids[b] = tk.ID
		}
	}
	return ids
}

// lineCol returns the line of byte offset at in text and its column in UTF-16 units.
func lineCol(text []byte, at int) (int, int) {
	line := bytes.Count(text[:at], []byte{'\n'})
	start := bytes.LastIndexByte(text[:at], '\n') + 1
	return line, UTF16Len(text[start:at])
}

// TestLiveEditsMatchWholeLex applies random edits to the live documents of real
// files and checks after each one that the document's text and tokens are those
// of a whole-text lex of the same text: that the edits reach the module at the
// right places, and that relexing from the changed lines gives the same tokens.
func TestLiveEditsMatchWholeLex(t *testing.T) {
	const alphabet = "xyz _(){}[]\n\"'`=;:.,+-*/<>!&|1\t#é😀"
	for _, file := range []struct{ path, lang string }{
		{"../../editor.go", "go"},
		{"../../../docs/markdown.md", "markdown"},
		{"../../../README.md", "md"},
	} {
		t.Run(filepath.Base(file.path), func(t *testing.T) {
			src, err := os.ReadFile(file.path)
			if err != nil {
				t.Fatal(err)
			}
			text := Sanitize(src)
			lang := langID(t, file.lang)
			live, whole := newInstance(t), newInstance(t)
			if err := live.Open(lang, text); err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewPCG(1, uint64(len(text))))
			runes := []rune(alphabet)
			for step := range 300 {
				// An edit at a rune boundary: delete up to 8 bytes, insert up to 6 runes.
				at := rng.IntN(len(text) + 1)
				for at > 0 && at < len(text) && text[at]&0xC0 == 0x80 {
					at--
				}
				end := at
				if rng.IntN(2) == 0 {
					end = min(len(text), at+rng.IntN(9))
					for end < len(text) && text[end]&0xC0 == 0x80 {
						end++
					}
				}
				var ins []byte
				for range rng.IntN(7) {
					ins = append(ins, string(runes[rng.IntN(len(runes))])...)
				}
				sl, sc := lineCol(text, at)
				el, ec := lineCol(text, end)
				if err := live.Edit(sl, sc, el, ec, ins); err != nil {
					t.Fatal(err)
				}
				text = slices.Concat(text[:at], ins, text[end:])
				// Run up to a random line first, as the highlighter does for the
				// visible lines, then the rest.
				if err := live.Run(rng.IntN(40) + 1); err != nil {
					t.Fatal(err)
				}
				if err := live.Run(0); err != nil {
					t.Fatal(err)
				}
				if pending, _, _ := live.Pending(); pending {
					t.Fatal("lines wait after Run(0)")
				}
				got, gotText := liveIDs(t, live)
				if !bytes.Equal(gotText, text) {
					t.Fatalf("step %d: the live text differs from the edited text", step)
				}
				want := lexIDs(t, whole, lang, text)
				for b := range text {
					if c := text[b]; c == ' ' || c == '\t' || c == '\n' {
						continue // spaces take the token of their neighbor, which differs by mode
					}
					if got[b] != want[b] {
						lo := max(0, b-30)
						t.Fatalf("step %d: byte %d is %s live and %s in a whole lex, near %q",
							step, b, TokenNames()[got[b]], TokenNames()[want[b]], text[lo:min(len(text), b+30)])
					}
				}
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	in := []byte("a\r\nb\rc\xffd\xe2\x82e😀")
	got := Sanitize(in)
	want := []byte("a \nb c?d??e😀")
	if !bytes.Equal(got, want) {
		t.Fatalf("Sanitize = %q, want %q", got, want)
	}
}

func TestUTF16(t *testing.T) {
	b := []byte("aé☃😀z")
	if n := UTF16Len(b); n != 6 {
		t.Fatalf("UTF16Len = %d, want 6", n)
	}
	for u, want := range map[int]int{0: 0, 1: 1, 2: 3, 3: 6, 5: 10, 6: 11, 9: 11} {
		if got := ByteOffset(b, u); got != want {
			t.Errorf("ByteOffset(%d) = %d, want %d", u, got, want)
		}
	}
}

func TestIdentifierClasses(t *testing.T) {
	for r, want := range map[rune][2]bool{
		'é': {true, true}, 'ж': {true, true}, '中': {true, true}, '١': {false, true},
		'́': {false, true}, '‿': {false, true}, '☃': {false, false}, '·': {false, true},
		'℘': {true, true}, '«': {false, false},
	} {
		if got := [2]bool{idStart(r), idContinue(r)}; got != want {
			t.Errorf("%q: ID_Start, ID_Continue = %v, want %v", r, got, want)
		}
	}
}

func TestOpenCacheWritesOnlyUnderItsDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	r, err := newRuntime(openCache(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close(context.Background())
	if _, err := r.CompileModule(context.Background(), wasm); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("compiling the module wrote nothing to the cache directory")
	}
}

func TestOpenCacheFallsBackToMemory(t *testing.T) {
	// A regular file where the cache directory should be makes MkdirAll fail.
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c := openCache(filepath.Join(blocker, "cache"))
	if _, err := os.Stat(filepath.Join(blocker, "cache")); err == nil {
		t.Fatal("openCache created a directory under a file")
	}
	r, err := newRuntime(c)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close(context.Background())
	if _, err := r.CompileModule(context.Background(), wasm); err != nil {
		t.Fatal(err)
	}
}
