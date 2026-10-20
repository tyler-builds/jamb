package lexer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// The module's memory layout, from src/memory.wat of @pierre/highlights. The
// first page is a control block, tables and scratch space. The input of a
// whole-text run starts at the second page.
const (
	ctlLang      = 0 // u8: the language ID
	ctlMode      = 1 // u8: the output mode
	ctlInputLen  = 2 // u32: the input's length in bytes
	ctlOutStart  = 6 // u32: the output's address, written by the module
	ctlOutLen    = 10
	inputStart   = 65536
	inputSlack   = 96 // bytes after the input that the lexers may read ahead into
	modeRecords  = 3  // output mode: (end, token) records in UTF-16 units
	endOfLine    = 0xffffffff
	flagWide     = 4 // line flag: the line's records are (end, token) pairs, not packed
	statPhase    = 9 // liveStats: zero when no line waits to be tokenized
	statCursor   = 10
	allLines     = 0x7fffffff
	pageSize     = 65536
	maxRunRounds = 1 << 20
)

// Token is a run of text with one token ID. Start and End are byte offsets.
type Token struct {
	Start, End int
	ID         int
}

// LineToken ends a run of a line's text with one token ID. End is the run's end
// in UTF-16 code units from the start of the line; the run starts where the
// previous one ended, or at the start of the line.
type LineToken struct {
	End int
	ID  int
}

// Instance is one instance of the module. It can lex whole texts with Lex, and
// it holds one live document, opened with Open, that it relexes as it is edited.
// It is not safe for concurrent use. After a call returns an error, the instance
// may be in an unknown state and should be closed.
type Instance struct {
	mod api.Module
	fn  struct {
		highlight, stage, initDoc, applyEdits, run, stats api.Function
		lineCount, lineByteLen, lineTextPtr, lineFlags    api.Function
		lineTokPtr, lineTokCount                          api.Function
	}
}

// New compiles the module if this is its first use in the process, and returns
// a new instance of it.
func New() (*Instance, error) {
	mu.Lock()
	r, cm, err := module()
	var mod api.Module
	if err == nil {
		mod, err = r.InstantiateModule(context.Background(), cm, wazero.NewModuleConfig().WithName(""))
	}
	mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("lexer: instantiate module: %w", err)
	}
	in := &Instance{mod: mod}
	get := func(name string) api.Function {
		f := mod.ExportedFunction(name)
		if f == nil {
			err = errors.Join(err, fmt.Errorf("lexer: module does not export %s", name))
		}
		return f
	}
	f := &in.fn
	f.highlight, f.stage, f.initDoc = get("highlight"), get("liveStage"), get("liveInitDoc")
	f.applyEdits, f.run, f.stats = get("liveApplyEdits"), get("liveRun"), get("liveStats")
	f.lineCount, f.lineByteLen, f.lineTextPtr = get("liveLineCount"), get("liveLineByteLen"), get("liveLineTextPtr")
	f.lineFlags, f.lineTokPtr, f.lineTokCount = get("liveLineFlags"), get("liveLineTokPtr"), get("liveLineTokCount")
	if mod.Memory() == nil {
		err = errors.Join(err, errors.New("lexer: module exports no memory"))
	}
	if err != nil {
		in.Close()
		return nil, err
	}
	return in, nil
}

// Close releases the instance and its memory.
func (in *Instance) Close() error {
	mu.Lock()
	defer mu.Unlock()
	return in.mod.Close(context.Background())
}

// MemorySize returns the size of the instance's linear memory in bytes.
func (in *Instance) MemorySize() uint32 { return in.mod.Memory().Size() }

// call invokes f and returns its first result, or zero when it has none.
func (in *Instance) call(f api.Function, args ...uint64) (uint32, error) {
	r, err := f.Call(context.Background(), args...)
	if err != nil {
		return 0, err
	}
	if len(r) == 0 {
		return 0, nil
	}
	return uint32(r[0]), nil
}

// Sanitize returns a copy of text that the lexers can read: each byte that is not
// part of valid UTF-8 becomes '?', and each carriage return a space, so that only
// line feeds end lines. The copy has the same length, so offsets in it are
// offsets in text. A carriage return before a line feed, as in Windows text,
// becomes a space at the end of its line, which does not change the tokens.
func Sanitize(text []byte) []byte {
	out := make([]byte, len(text))
	for i := 0; i < len(text); {
		c := text[i]
		if c < utf8.RuneSelf {
			if c == '\r' {
				c = ' '
			}
			out[i] = c
			i++
			continue
		}
		r, n := utf8.DecodeRune(text[i:])
		if r == utf8.RuneError && n <= 1 {
			out[i] = '?'
			i++
			continue
		}
		copy(out[i:i+n], text[i:i+n])
		i += n
	}
	return out
}

// grow makes the memory at least size bytes.
func (in *Instance) grow(size uint64) error {
	mem := in.mod.Memory()
	if have := uint64(mem.Size()); have < size {
		if _, ok := mem.Grow(uint32((size - have + pageSize - 1) / pageSize)); !ok {
			return errors.New("lexer: module is out of memory")
		}
	}
	return nil
}

// Lex lexes the whole of text, in the language with the given ID, and appends
// its tokens to dst. Text without a token (ID 0) and line ends are left out.
// text must come from Sanitize.
func (in *Instance) Lex(lang int, text []byte, dst []Token) ([]Token, error) {
	if err := in.grow(inputStart + uint64(len(text)) + inputSlack); err != nil {
		return dst, err
	}
	mem := in.mod.Memory()
	mem.Write(inputStart, text)
	mem.WriteByte(inputStart+uint32(len(text)), 0) // the lexers stop at a NUL at the end
	mem.WriteByte(ctlLang, byte(lang))
	mem.WriteByte(ctlMode, modeRecords)
	mem.WriteUint32Le(ctlInputLen, uint32(len(text)))
	if _, err := in.call(in.fn.highlight); err != nil {
		return dst, fmt.Errorf("lexer: highlight: %w", err)
	}
	mem = in.mod.Memory()
	out, _ := mem.ReadUint32Le(ctlOutStart)
	n, _ := mem.ReadUint32Le(ctlOutLen)
	rec, ok := mem.Read(out, n)
	if !ok {
		return dst, errors.New("lexer: output outside module memory")
	}
	// The records end in UTF-16 units from the start of the text; walk the text to
	// turn them into byte offsets.
	pos, units := 0, 0
	for i := 0; i+8 <= len(rec); i += 8 {
		end := int(binary.LittleEndian.Uint32(rec[i:]))
		id := binary.LittleEndian.Uint32(rec[i+4:])
		start := pos
		for units < end && pos < len(text) {
			r, size := utf8.DecodeRune(text[pos:])
			pos += size
			units += utf16Len(r)
		}
		if id != endOfLine && id != 0 && pos > start {
			dst = append(dst, Token{Start: start, End: pos, ID: int(id)})
		}
	}
	return dst, nil
}

// Open makes text, in the language with the given ID, the instance's live
// document. Its lines wait to be tokenized; see Run. text must come from Sanitize.
func (in *Instance) Open(lang int, text []byte) error {
	p, err := in.stage(text)
	if err != nil {
		return err
	}
	if _, err := in.call(in.fn.initDoc, uint64(p), uint64(len(text)), uint64(lang)); err != nil {
		return fmt.Errorf("lexer: open document: %w", err)
	}
	return nil
}

// stage copies b into a block of the live heap and returns its address.
func (in *Instance) stage(b []byte) (uint32, error) {
	p, err := in.call(in.fn.stage, uint64(len(b)))
	if err != nil {
		return 0, fmt.Errorf("lexer: stage: %w", err)
	}
	if !in.mod.Memory().Write(p, b) {
		return 0, errors.New("lexer: stage outside module memory")
	}
	return p, nil
}

// Edit replaces the text of the live document from column sc of line sl to
// column ec of line el with text. Columns count UTF-16 code units from the start
// of the line, and a column at the end of a line is before its line feed. The
// changed lines wait to be tokenized; see Run. text must come from Sanitize.
func (in *Instance) Edit(sl, sc, el, ec int, text []byte) error {
	const head = 4 + 24 // the edit count, then one record of six u32s
	block := make([]byte, head+len(text))
	binary.LittleEndian.PutUint32(block, 1)
	for i, v := range []int{sl, sc, el, ec, head, len(text)} {
		binary.LittleEndian.PutUint32(block[4+4*i:], uint32(v))
	}
	copy(block[head:], text)
	p, err := in.stage(block)
	if err != nil {
		return err
	}
	if _, err := in.call(in.fn.applyEdits, uint64(p), 0); err != nil {
		return fmt.Errorf("lexer: edit: %w", err)
	}
	return nil
}

// Pending reports whether lines of the live document wait to be tokenized, and
// the first of them.
func (in *Instance) Pending() (bool, int, error) {
	phase, err := in.call(in.fn.stats, statPhase)
	if err != nil || phase == 0 {
		return false, 0, err
	}
	cursor, err := in.call(in.fn.stats, statCursor)
	return true, int(cursor), err
}

// Run tokenizes the waiting lines of the live document that come before line
// end, in order, or all of them when end is zero. Lines after a change are
// tokenized again until the lexer's state at the end of one matches the state
// it had before, so a change that opens a comment retokenizes up to where the
// comment closes.
func (in *Instance) Run(end int) error {
	for range maxRunRounds {
		pending, cursor, err := in.Pending()
		if err != nil || !pending || end > 0 && cursor >= end {
			return err
		}
		more, err := in.call(in.fn.run, allLines, uint64(end), 0)
		if err != nil {
			return fmt.Errorf("lexer: run: %w", err)
		}
		if more == 0 {
			return nil
		}
	}
	return errors.New("lexer: run made no progress")
}

// RunLines tokenizes at most n of the waiting lines of the live document.
func (in *Instance) RunLines(n int) error {
	if _, err := in.call(in.fn.run, uint64(max(n, 1)), 0, 0); err != nil {
		return fmt.Errorf("lexer: run: %w", err)
	}
	return nil
}

// LineCount returns the number of lines of the live document. Text that ends in
// a line feed has an empty last line.
func (in *Instance) LineCount() (int, error) {
	n, err := in.call(in.fn.lineCount)
	return int(n), err
}

// LineText returns the bytes of line i of the live document, without its line
// feed. The slice is a view of the module's memory, valid until the next call.
func (in *Instance) LineText(i int) ([]byte, error) {
	p, err := in.call(in.fn.lineTextPtr, uint64(i))
	if err != nil {
		return nil, err
	}
	n, err := in.call(in.fn.lineByteLen, uint64(i))
	if err != nil {
		return nil, err
	}
	b, ok := in.mod.Memory().Read(p, n)
	if !ok {
		return nil, errors.New("lexer: line outside module memory")
	}
	return b, nil
}

// LineTokens appends the tokens of line i of the live document to dst. A line
// that waits to be tokenized has the tokens it had before, and a new one none.
func (in *Instance) LineTokens(i int, dst []LineToken) ([]LineToken, error) {
	count, err := in.call(in.fn.lineTokCount, uint64(i))
	if err != nil || count == 0 {
		return dst, err
	}
	flags, err := in.call(in.fn.lineFlags, uint64(i))
	if err != nil {
		return dst, err
	}
	p, err := in.call(in.fn.lineTokPtr, uint64(i))
	if err != nil {
		return dst, err
	}
	wide := flags&flagWide != 0
	size := count * 4
	if wide {
		size *= 2
	}
	b, ok := in.mod.Memory().Read(p, size)
	if !ok {
		return dst, errors.New("lexer: tokens outside module memory")
	}
	for k := range int(count) {
		if wide {
			dst = append(dst, LineToken{End: int(binary.LittleEndian.Uint32(b[8*k:])), ID: int(binary.LittleEndian.Uint32(b[8*k+4:]))})
		} else {
			v := binary.LittleEndian.Uint32(b[4*k:])
			dst = append(dst, LineToken{End: int(v & 0xffffff), ID: int(v >> 24)})
		}
	}
	return dst, nil
}

// utf16Len returns the number of UTF-16 code units of r.
func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// UTF16Len returns the number of UTF-16 code units of b, which must be valid UTF-8.
func UTF16Len(b []byte) int {
	n := 0
	for i := 0; i < len(b); {
		if b[i] < utf8.RuneSelf {
			n++
			i++
			continue
		}
		r, size := utf8.DecodeRune(b[i:])
		n += utf16Len(r)
		i += size
	}
	return n
}

// ByteOffset returns the byte offset in b, which must be valid UTF-8, of the
// UTF-16 offset u, or len(b) when b is shorter.
func ByteOffset(b []byte, u int) int {
	i, n := 0, 0
	for i < len(b) && n < u {
		if b[i] < utf8.RuneSelf {
			n++
			i++
			continue
		}
		r, size := utf8.DecodeRune(b[i:])
		n += utf16Len(r)
		i += size
	}
	return i
}
