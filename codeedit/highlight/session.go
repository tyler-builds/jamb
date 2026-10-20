// Package highlight colors the text of a code editor with @pierre/highlights, a
// syntax highlighter of the Pierre Computer Company with a lexer for each of 76
// languages, which runs in WebAssembly (see codeedit/internal/lexer). Languages
// are named as code fences name them, such as "go", "python" or "ts"; Languages
// lists the names.
//
// A Session holds one open document. The typing thread calls Edit for every
// change and SetViewport when the visible range moves. Edit returns at once: it
// shifts the spans it has, so colors stay in place, and queues the change for a
// background worker. The worker owns the document's WebAssembly instance. It
// applies every queued change, relexes the changed lines and the lines after them
// that the change affects, up to the end of the visible range, and publishes the
// spans of that range; it relexes the rest of the document afterwards, a slice at
// a time. Spans reports the latest published spans, and Options.Notify tells the
// editor when new ones arrive, so it can request a frame.
//
// Highlight colors a text that is not edited, such as a code block, in one call.
package highlight

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tyler-builds/jamb/codeedit/internal/lexer"
)

// DefaultMaxBytes is the largest document a Session highlights when
// Options.MaxBytes is zero. The lexer's copy of a document takes about five
// times its size in memory.
const DefaultMaxBytes = 16 << 20

// margin is how far beyond the visible range the worker computes spans, in
// bytes, so that a short scroll shows colors at once.
const margin = 4096

// sliceLines is how many lines the worker relexes in one step of the work it
// does after publishing, outside the visible range: about a millisecond, so a
// new edit or Close waits no longer than that.
const sliceLines = 2048

// ErrTooLarge is returned by NewSession for a document over the size limit. The
// editor then shows the document without highlighting.
var ErrTooLarge = errors.New("highlight: document is larger than the highlighting limit")

// ErrClosed is returned by Edit and SetViewport after Close.
var ErrClosed = errors.New("highlight: session is closed")

// ErrUnknownLanguage is returned, wrapped with the name, by NewSession and
// Highlight for a language that the highlighter does not know.
var ErrUnknownLanguage = errors.New("highlight: unknown language")

// extraAliases are names that code fences use for a language, in addition to the
// highlighter's own names and aliases.
var extraAliases = map[string]string{
	"node": "javascript", "python3": "python", "console": "bash", "h++": "cpp", "xhtml": "html",
}

// languageID returns the lexer's ID for a language name, in any case.
func languageID(lang string) (int, error) {
	name := strings.ToLower(strings.TrimSpace(lang))
	if alias, ok := extraAliases[name]; ok {
		name = alias
	}
	if id, ok := lexer.LanguageID(name); ok {
		return id, nil
	}
	return 0, fmt.Errorf("%w %q", ErrUnknownLanguage, lang)
}

// Languages returns the names that NewSession and Highlight accept, sorted: the
// names of the languages and their aliases, in lower case, such as "go",
// "golang", "python", "py", "typescript" and "ts". Names are matched in any case.
// "plain" (or "text") highlights nothing.
func Languages() []string {
	names := lexer.LanguageNames()
	for n := range extraAliases {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Supported reports whether lang names a language that NewSession and Highlight
// accept.
func Supported(lang string) bool {
	_, err := languageID(lang)
	return err == nil
}

// CaptureNames returns the names of the tokens, indexed by Span.Capture: the
// highlighter's token names, which are the syntax names of Zed's themes, such as
// "keyword", "string.escape" or "function.method", with a few of its own, such as
// "selector.class" for CSS. Map them to theme keys with Closest. Every language
// uses the same names. The caller must not change the slice.
func CaptureNames() []string { return lexer.TokenNames() }

// Span is a highlighted byte range [Start, End) and the index of its capture
// name in CaptureNames. The spans of a session do not overlap; for
// highlighters that make spans that do, a span later in the slice takes
// precedence over an earlier one that overlaps it.
type Span struct {
	// Start and End are the byte offsets of the span.
	Start, End int
	// Capture is the index of the capture name, in the names from CaptureNames.
	Capture int
}

// Options configures a Session.
type Options struct {
	// MaxBytes is the largest document that is highlighted. Zero means
	// DefaultMaxBytes, and a negative value means no limit. A larger document
	// gets ErrTooLarge from NewSession.
	MaxBytes int

	// Notify is called from the session's worker goroutine each time new spans
	// are published. It must not block, and it must not call Close on the
	// session, which waits for the worker. A typical Notify requests a repaint.
	Notify func()
}

// EditRecord replaces the bytes [Start, OldEnd) of the document with Text. An
// insertion has Start equal to OldEnd, and a deletion has empty Text. Offsets
// are bytes in the document as it is before the edit.
type EditRecord struct {
	// Start is the byte offset where the edit begins, and OldEnd is the end of the replaced bytes before the edit.
	Start, OldEnd int
	// Text is the new bytes that replace the old range.
	Text []byte
}

// Session highlights one document. Its methods are safe for concurrent use,
// although Edit and SetViewport are meant for the typing thread.
type Session struct {
	in     *lexer.Instance // owned by the worker from NewSession on
	lines  []int           // owned by the worker: the byte offset of each line of its text
	toks   []lexer.LineToken
	notify func()
	wake   chan struct{} // capacity one: a pending wake-up for the worker
	quit   chan struct{}
	done   chan struct{}

	mu        sync.Mutex
	closed    bool
	err       error        // the first error from the worker; highlighting stops
	length    int          // document length, as the typing thread sees it
	version   uint64       // the number of edits accepted
	synced    uint64       // the version of the last spans the worker published
	rev       uint64       // changes whenever spans does: at each accepted edit and each publish
	spans     []Span       // the spans to paint; replaced, never modified in place
	queue     []EditRecord // edits the worker has not applied yet; Text is owned here
	first     bool         // the worker has not published spans yet
	view      [2]int       // the range the worker computes spans for, with margin
	viewDirty bool         // view changed since the worker last computed spans
	busy      bool         // the worker has taken a round whose result is not published yet
	behind    bool         // lines outside the view wait to be relexed
	rounds    atomic.Int64 // the number of rounds that applied edits
}

// workHook, when set, runs in the worker before each round. Tests use it to hold
// the worker, so that they can check what the typing thread sees in between.
var workHook func()

// NewSession opens a document in the language lang, such as "go" (see
// Languages), and starts its worker. The worker lexes the text and publishes the
// first spans in the background. NewSession returns ErrTooLarge for a document
// over Options.MaxBytes, and an error wrapping ErrUnknownLanguage for a language
// it does not know.
func NewSession(lang string, text []byte, opts Options) (*Session, error) {
	id, err := languageID(lang)
	if err != nil {
		return nil, err
	}
	limit := opts.MaxBytes
	if limit == 0 {
		limit = DefaultMaxBytes
	}
	if limit > 0 && len(text) > limit {
		return nil, ErrTooLarge
	}
	in, err := lexer.New()
	if err != nil {
		return nil, fmt.Errorf("highlight: %w", err)
	}
	if err := in.Open(id, lexer.Sanitize(text)); err != nil {
		in.Close()
		return nil, fmt.Errorf("highlight: %w", err)
	}
	s := &Session{
		in:        in,
		lines:     lineStarts(text),
		notify:    opts.Notify,
		wake:      make(chan struct{}, 1),
		quit:      make(chan struct{}),
		done:      make(chan struct{}),
		length:    len(text),
		first:     true,
		view:      [2]int{0, min(len(text), margin)},
		viewDirty: true,
	}
	s.signal()
	go s.run()
	return s, nil
}

// lineStarts returns the byte offset of each line of text: 0, and each offset
// after a line feed.
func lineStarts(text []byte) []int {
	starts := []int{0}
	for i, c := range text {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// Edit applies a change to the document. It never waits for the worker: the
// existing spans are shifted at once, and the change is queued for the worker,
// which applies every queued change before it relexes. Edit returns an error,
// without changing anything, when the range is not inside the document.
func (s *Session) Edit(e EditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if s.err != nil {
		return s.err
	}
	if e.Start < 0 || e.OldEnd < e.Start || e.OldEnd > s.length {
		return fmt.Errorf("highlight: edit [%d, %d) is outside the document of %d bytes", e.Start, e.OldEnd, s.length)
	}
	e.Text = slices.Clone(e.Text)
	s.version++
	s.rev++
	s.spans = shift(s.spans, e)
	s.length += len(e.Text) - (e.OldEnd - e.Start)
	s.queue = append(s.queue, e)
	s.signal()
	return nil
}

// SetViewport tells the session which bytes are visible, [lo, hi). The worker
// computes spans for that range with a margin on each side. Until it publishes
// them, Spans shows the old spans, shifted by the edits.
func (s *Session) SetViewport(lo, hi int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if lo < 0 || hi < lo {
		return fmt.Errorf("highlight: viewport [%d, %d) is not a range", lo, hi)
	}
	lo = max(0, min(lo-margin, s.length))
	hi = min(s.length, hi+margin)
	if [2]int{lo, hi} == s.view {
		return nil
	}
	s.view = [2]int{lo, hi}
	s.viewDirty = true
	s.signal()
	return nil
}

// Spans returns the spans to paint and their revision. The revision changes
// whenever the spans do: when an edit shifts them, and when the worker publishes
// new ones, so a caller that keeps an index of the spans rebuilds it when the
// revision differs. The slice must not be modified. It is replaced, not changed,
// so a slice held by the caller stays valid.
func (s *Session) Spans() ([]Span, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spans, s.rev
}

// Synced returns the number of edits that the published spans reflect. Until it
// reaches the number of edits made, the spans of the latest edits are the earlier
// spans shifted into place.
func (s *Session) Synced() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.synced
}

// CaptureNames returns the names that Span.Capture indexes, the same for every
// session (see the package's CaptureNames). Map them to theme keys with Closest.
func (s *Session) CaptureNames() []string {
	return lexer.TokenNames()
}

// Err returns the error that stopped highlighting, or nil. After an error the
// session keeps accepting calls, but it publishes no more spans.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close stops the worker and releases the document's instance. It waits for the
// worker to finish its current step, which takes about a millisecond at most
// once the first spans are published. Calls after Close return ErrClosed. Close
// is safe to call more than once.
func (s *Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.quit)
	s.mu.Unlock()
	<-s.done
	return nil
}

// signal wakes the worker. It never blocks: one pending wake-up is enough,
// because the worker takes everything queued each time it runs.
func (s *Session) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// run is the worker goroutine. It owns the instance and closes it on exit.
func (s *Session) run() {
	defer close(s.done)
	defer s.in.Close()
	for {
		select {
		case <-s.quit:
			return
		case <-s.wake:
		}
		s.work()
	}
}

// work applies the queued edits, relexes up to the end of the viewport and
// publishes its spans. Edits that arrive during the work stay queued, and they
// shift the published spans before the worker's result is swapped in. With
// nothing new to do, it relexes a slice of the lines outside the viewport that
// still wait.
func (s *Session) work() {
	if workHook != nil {
		workHook()
	}
	s.mu.Lock()
	if s.closed || s.err != nil {
		s.mu.Unlock()
		return
	}
	edits := s.queue
	s.queue = nil
	dirty := s.viewDirty || s.first
	s.viewDirty = false
	view := s.view
	if len(edits) == 0 && !dirty {
		behind := s.behind
		s.mu.Unlock()
		if behind {
			s.catchUp()
		}
		return
	}
	s.busy = true
	s.mu.Unlock()

	err := s.compute(edits, view)

	s.mu.Lock()
	s.busy = false
	if s.closed {
		s.mu.Unlock()
		return
	}
	if err != nil {
		s.err = err
		s.spans = nil
		s.rev++
	} else if s.behind {
		s.signal() // relex the rest in the next rounds
	}
	s.mu.Unlock()
	if s.notify != nil {
		s.notify()
	}
}

// catchUp relexes a slice of the lines that wait, outside the viewport. It is
// called only from the worker.
func (s *Session) catchUp() {
	err := s.in.RunLines(sliceLines)
	var behind bool
	if err == nil {
		behind, _, err = s.in.Pending()
	}
	s.mu.Lock()
	if err != nil {
		s.err = fmt.Errorf("highlight: %w", err)
		s.spans = nil
		s.rev++
		s.mu.Unlock()
		if s.notify != nil {
			s.notify() // the editor paints the text without colors
		}
		return
	}
	s.behind = behind
	if behind {
		s.signal()
	}
	s.mu.Unlock()
}

// compute applies edits, relexes to the end of the view, and publishes the
// view's spans. It is called only from the worker.
func (s *Session) compute(edits []EditRecord, view [2]int) error {
	for _, e := range edits {
		if e.Start == e.OldEnd && len(e.Text) == 0 {
			continue
		}
		if err := s.apply(e); err != nil {
			return err
		}
	}
	if len(edits) > 0 {
		s.rounds.Add(1)
	}
	first, last := s.lineOf(view[0]), s.lineOf(view[1])
	if err := s.in.Run(last + 1); err != nil {
		return fmt.Errorf("highlight: %w", err)
	}
	spans, err := s.lineSpans(first, last)
	if err != nil {
		return err
	}
	behind, _, err := s.in.Pending()
	if err != nil {
		return fmt.Errorf("highlight: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.first = false
		s.behind = behind
		// Spans are for the version the worker started with. Edits made since
		// then are still in the queue, and they shift the spans into place.
		for _, e := range s.queue {
			spans = shift(spans, e)
		}
		s.spans = spans
		s.synced = s.version
		s.rev++
	}
	return nil
}

// lineOf returns the line that holds byte offset off of the worker's text.
func (s *Session) lineOf(off int) int {
	return sort.Search(len(s.lines), func(i int) bool { return s.lines[i] > off }) - 1
}

// position returns the line of byte offset off and its column in UTF-16 units,
// as the lexer counts them.
func (s *Session) position(off int) (int, int, error) {
	line := s.lineOf(off)
	text, err := s.in.LineText(line)
	if err != nil {
		return 0, 0, fmt.Errorf("highlight: %w", err)
	}
	col := min(off-s.lines[line], len(text))
	return line, lexer.UTF16Len(text[:col]), nil
}

// apply sends e to the instance and updates the line offsets. It is called only
// from the worker.
func (s *Session) apply(e EditRecord) error {
	sl, sc, err := s.position(e.Start)
	if err != nil {
		return err
	}
	el, ec, err := s.position(e.OldEnd)
	if err != nil {
		return err
	}
	if err := s.in.Edit(sl, sc, el, ec, lexer.Sanitize(e.Text)); err != nil {
		return fmt.Errorf("highlight: %w", err)
	}
	// The lines that start inside the old range go; those that start inside the
	// new text come; the later ones move by the change in length.
	delta := len(e.Text) - (e.OldEnd - e.Start)
	lines := make([]int, 0, len(s.lines)+8)
	lines = append(lines, s.lines[:sl+1]...)
	for i, c := range e.Text {
		if c == '\n' {
			lines = append(lines, e.Start+i+1)
		}
	}
	for _, start := range s.lines[el+1:] {
		lines = append(lines, start+delta)
	}
	s.lines = lines
	return nil
}

// lineSpans returns the spans of lines first to last. It is called only from the
// worker.
func (s *Session) lineSpans(first, last int) ([]Span, error) {
	var spans []Span
	for line := first; line <= last && line < len(s.lines); line++ {
		var err error
		if s.toks, err = s.in.LineTokens(line, s.toks[:0]); err != nil {
			return nil, fmt.Errorf("highlight: %w", err)
		}
		if len(s.toks) == 0 {
			continue
		}
		text, err := s.in.LineText(line)
		if err != nil {
			return nil, fmt.Errorf("highlight: %w", err)
		}
		base := s.lines[line]
		start, units := 0, 0
		for _, tk := range s.toks {
			end := start + lexer.ByteOffset(text[start:], tk.End-units)
			if lo, hi := trim(text, start, end); tk.ID != 0 && hi > lo {
				spans = append(spans, Span{Start: base + lo, End: base + hi, Capture: tk.ID})
			}
			if tk.End > units {
				units = tk.End
			}
			start = end
		}
	}
	return spans, nil
}

// pool holds idle instances for Highlight.
var pool struct {
	sync.Mutex
	idle []*lexer.Instance
}

// poolSize is how many idle instances Highlight keeps, and poolMemory the most
// memory an instance may hold to be kept: one that grew for a large text is
// closed.
const (
	poolSize   = 2
	poolMemory = 4 << 20
)

// Highlight returns the spans of code in the language lang, such as "go" (see
// Languages), lexed as a whole. It is for text that is not edited, such as the
// code blocks of a Markdown view, and takes about 10 ms per megabyte. It returns
// an error wrapping ErrUnknownLanguage for a language it does not know. It is safe
// for concurrent use.
func Highlight(lang string, code []byte) ([]Span, error) {
	id, err := languageID(lang)
	if err != nil {
		return nil, err
	}
	pool.Lock()
	var in *lexer.Instance
	if n := len(pool.idle); n > 0 {
		in, pool.idle = pool.idle[n-1], pool.idle[:n-1]
	}
	pool.Unlock()
	if in == nil {
		if in, err = lexer.New(); err != nil {
			return nil, fmt.Errorf("highlight: %w", err)
		}
	}
	toks, err := in.Lex(id, lexer.Sanitize(code), nil)
	if err != nil {
		in.Close() // its state is unknown
		return nil, fmt.Errorf("highlight: %w", err)
	}
	pool.Lock()
	if len(pool.idle) < poolSize && in.MemorySize() <= poolMemory {
		pool.idle = append(pool.idle, in)
		in = nil
	}
	pool.Unlock()
	if in != nil {
		in.Close()
	}
	spans := make([]Span, 0, len(toks))
	for _, tk := range toks {
		if lo, hi := trim(code, tk.Start, tk.End); hi > lo {
			spans = append(spans, Span{Start: lo, End: hi, Capture: tk.ID})
		}
	}
	return spans, nil
}

// trim returns the range [lo, hi) of text without its leading and trailing
// spaces, tabs and line ends. The lexer gives the space between two tokens to the
// first, so "package " is a keyword; the spans hold only the word.
func trim(text []byte, lo, hi int) (int, int) {
	for lo < hi && isSpace(text[lo]) {
		lo++
	}
	for hi > lo && isSpace(text[hi-1]) {
		hi--
	}
	return lo, hi
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
