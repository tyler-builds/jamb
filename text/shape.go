package text

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-text/typesetting/di"
	tsfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Style selects font and size for a layout. Sizes are logical pixels.
type Style struct {
	Family string
	Size   float32
	Weight Weight
	// Italic selects the italic face of the family (upright when it has none).
	Italic bool
	// LineHeight is a multiple of Size; 0 uses the font's natural height.
	LineHeight float32
	// MaxLines truncates with an ellipsis after this many lines; 0 = no limit.
	MaxLines int
	// NoWrap keeps text on one line (it may still be truncated by MaxLines).
	NoWrap bool
}

// Span gives a byte range of a text its own font family, weight or slant.
// Only attributes that change shaping belong here; colors and decorations are
// painted (see DrawRich). Zero fields inherit the Style: Family "" keeps the
// style's family, Weight 0 keeps its weight, and Italic is OR'd with its
// Italic. Line metrics still come from the Style's font.
//
// Spans must be sorted, non-overlapping, inside the text and on rune
// boundaries; the ui package guarantees this.
type Span struct {
	Start, End int // byte offsets
	Family     string
	Weight     Weight
	Italic     bool
}

// Glyph is one shaped glyph. Positions are relative to its run's left edge
// and the baseline (Y grows downward).
type Glyph struct {
	font *Font
	ID   sfnt.GlyphIndex
	// Pen is the pen position before the glyph; X/Y add the shaper's offset.
	Pen, X, Y float32
	Adv       float32
	// Cluster is the index of the first rune of the glyph's cluster and
	// Runes the number of runes the cluster covers (a ligature covers several).
	Cluster, Runes int
}

// Run is a stretch of a line in one font, script and direction.
type Run struct {
	X, Width           float32 // within the line
	RTL                bool
	RuneStart, RuneEnd int
	Glyphs             []Glyph // visual order, left to right
	// Span is the index of the span the run belongs to in the spans passed to
	// LayoutSpans, or -1 outside every span.
	Span     int
	ellipsis bool
}

// Line is one laid-out line; Runs are in visual order.
type Line struct {
	Runs               []Run
	Width              float32 // excluding trailing whitespace
	Top, Baseline      float32
	RuneStart, RuneEnd int
	// hardEnd is set when the line ends a paragraph (newline or end of text),
	// so a caret at RuneEnd belongs to this line rather than the next.
	hardEnd bool
}

// Layout is shaped, wrapped text. Layouts are cached and shared: treat them
// as read-only. Positions in the public API are byte offsets into Text.
type Layout struct {
	Text       string
	Style      Style
	Lines      []Line
	Width      float32
	Height     float32
	LineHeight float32
	Ascent     float32
	Truncated  bool

	runes  []rune
	byteOf []int // rune index -> byte offset, len(runes)+1 entries
	// runeSpans is the span list in rune offsets, index-aligned with the spans
	// passed to LayoutSpans. Nil when there are no spans.
	runeSpans []runeSpan
}

// runeSpan is a Span in rune offsets with its attributes resolved against the Style.
type runeSpan struct {
	start, end int
	family     string
	weight     Weight
	italic     bool
}

// setSpans fills runeSpans from spans, resolving inherited attributes.
func (l *Layout) setSpans(spans []Span) {
	if len(spans) == 0 {
		return
	}
	st := l.Style
	l.runeSpans = make([]runeSpan, len(spans))
	for i, sp := range spans {
		rs := runeSpan{start: l.runeAt(sp.Start), end: l.runeAt(sp.End), family: sp.Family, weight: sp.Weight, italic: sp.Italic || st.Italic}
		if rs.family == "" {
			rs.family = st.Family
		}
		if rs.weight == 0 {
			rs.weight = st.Weight
		}
		l.runeSpans[i] = rs
	}
}

// spanAt returns the index of the span covering rune r, or -1.
func (l *Layout) spanAt(r int) int {
	i := sort.Search(len(l.runeSpans), func(i int) bool { return l.runeSpans[i].end > r })
	if i < len(l.runeSpans) && l.runeSpans[i].start <= r {
		return i
	}
	return -1
}

const unbounded = 1 << 24

// fontmap resolves the font for each rune: the primary font when it has the
// glyph, else a fallback.
//
// Fallback is sticky within a paragraph: once a fallback font has been used,
// later characters it covers stay in it. Japanese kana pick Yu Gothic, and
// the kanji that follow then come from Yu Gothic too rather than from the
// Chinese font Han characters default to, so one word never mixes faces.
type fontmap struct {
	reg     *Registry
	primary *Font
	last    *Font // the fallback used most recently in this paragraph
}

func (m *fontmap) ResolveFace(r rune) *tsfont.Face {
	if m.primary.has(r) {
		return m.primary.face
	}
	if m.last != nil && m.last.has(r) {
		return m.last.face
	}
	if f := m.reg.fallbackFor(r); f != nil {
		m.last = f
		return f.face
	}
	return m.primary.face
}

func fx(v fixed.Int26_6) float32 { return float32(v) / 64 }

// build shapes, wraps and positions s with spans. maxWidth may be +Inf.
func (sys *System) build(s string, st Style, spans []Span, maxWidth float32) *Layout {
	primary := sys.reg.Font(st.Family, st.Weight, st.Italic)
	scale := st.Size / primary.upem
	l := &Layout{Text: s, Style: st}
	l.Ascent = primary.ascent * scale
	natural := (primary.ascent + primary.descent) * scale
	l.LineHeight = natural
	if st.LineHeight > 0 {
		l.LineHeight = st.LineHeight * st.Size
	}
	l.runes = []rune(s)
	l.byteOf = make([]int, 0, len(l.runes)+1)
	for i := range s {
		l.byteOf = append(l.byteOf, i)
	}
	l.byteOf = append(l.byteOf, len(s))
	l.setSpans(spans)

	wrapW := maxWidth
	if st.NoWrap && st.MaxLines == 0 || math.IsInf(float64(wrapW), 1) || wrapW > unbounded {
		wrapW = unbounded
	}
	size := fixed.Int26_6(st.Size*64 + 0.5)

	for start := 0; ; {
		if st.MaxLines > 0 && len(l.Lines) >= st.MaxLines {
			l.Truncated = true
			break
		}
		end := start
		for end < len(l.runes) && l.runes[end] != '\n' {
			end++
		}
		switch {
		case end == start:
			l.Lines = append(l.Lines, Line{RuneStart: start, RuneEnd: start, hardEnd: true})
		case st.MaxLines == 0:
			// Paragraphs are shaped independently and cached, so editing one
			// paragraph of a long document re-shapes only that paragraph.
			l.Lines = sys.cachedParagraph(l, primary, size, start, end, wrapW)
		default:
			sys.wrapParagraph(l, start, end, size, wrapW, sys.pieces(l, start, end))
		}
		if end >= len(l.runes) {
			break
		}
		start = end + 1
	}

	half := (l.LineHeight - natural) / 2
	for i := range l.Lines {
		ln := &l.Lines[i]
		ln.Top = float32(i) * l.LineHeight
		ln.Baseline = ln.Top + half + l.Ascent
		l.Width = max(l.Width, ln.Width)
	}
	l.Height = float32(max(len(l.Lines), 1)) * l.LineHeight
	return l
}

type paraKey struct {
	text  string
	font  *Font
	size  fixed.Int26_6
	width float32
	spans string // paraSpanKey: the spans inside the paragraph
}

type cachedPara struct {
	lines    []Line // rune indices relative to the paragraph start
	lastUsed int
}

// cachedParagraph appends the lines of paragraph [start, end), shaping it
// only if this text, font, size, width and spans have not been seen recently.
func (sys *System) cachedParagraph(l *Layout, primary *Font, size fixed.Int26_6, start, end int, wrapW float32) []Line {
	k := paraKey{string(l.runes[start:end]), primary, size, wrapW, paraSpanKey(l, start, end)}
	c, ok := sys.paras[k]
	if !ok {
		// Shape against a scratch layout holding just this paragraph.
		scratch := &Layout{Style: l.Style, runes: l.runes[start:end]}
		sys.wrapParagraph(scratch, 0, end-start, size, wrapW, sys.pieces(l, start, end))
		c = &cachedPara{lines: scratch.Lines}
		sys.paras[k] = c
	}
	c.lastUsed = sys.frame
	for _, ln := range c.lines {
		ln.RuneStart += start
		ln.RuneEnd += start
		runs := make([]Run, len(ln.Runs))
		for i, r := range ln.Runs {
			r.RuneStart += start
			r.RuneEnd += start
			// Cached runs carry the span indices of the layout they were
			// shaped for, so look the span up in this one.
			r.Span = l.spanAt(r.RuneStart)
			gs := make([]Glyph, len(r.Glyphs))
			for j, g := range r.Glyphs {
				g.Cluster += start
				gs[j] = g
			}
			r.Glyphs = gs
			runs[i] = r
		}
		ln.Runs = runs
		l.Lines = append(l.Lines, ln)
	}
	return l.Lines
}

// paraSpanKey encodes the spans that touch runes [start, end), with offsets
// relative to start, so a paragraph's cache entry depends on its spans but not
// on where it sits in the text.
func paraSpanKey(l *Layout, start, end int) string {
	if len(l.runeSpans) == 0 {
		return ""
	}
	var b strings.Builder
	for _, sp := range l.runeSpans {
		if sp.start >= end || sp.end <= sp.start || sp.end <= start {
			continue
		}
		fmt.Fprintf(&b, "%d:%d:%q:%d:%t;", max(sp.start, start)-start, min(sp.end, end)-start, sp.family, sp.weight, sp.italic)
	}
	return b.String()
}

// piece is a stretch of a paragraph shaped in one font and belonging to one
// span (or to none).
type piece struct {
	start, end int // rune offsets relative to the paragraph start
	span       int // index into the layout's spans, or -1
	font       *Font
}

// pieces splits runes [start, end) at every span boundary. The stretches
// between spans take the layout's own style. Offsets are relative to start.
func (sys *System) pieces(l *Layout, start, end int) []piece {
	st := l.Style
	plain := sys.reg.Font(st.Family, st.Weight, st.Italic)
	var out []piece
	pos := start
	for i, sp := range l.runeSpans {
		a, b := max(sp.start, start), min(sp.end, end)
		if a >= b {
			continue
		}
		if a > pos {
			out = append(out, piece{start: pos - start, end: a - start, span: -1, font: plain})
		}
		out = append(out, piece{start: a - start, end: b - start, span: i, font: sys.reg.Font(sp.family, sp.weight, sp.italic)})
		pos = b
	}
	if pos < end {
		out = append(out, piece{start: pos - start, end: end - start, span: -1, font: plain})
	}
	return out
}

// pieceIndex returns the index of the piece holding relative rune r.
func pieceIndex(pcs []piece, r int) int {
	return min(sort.Search(len(pcs), func(i int) bool { return pcs[i].end > r }), len(pcs)-1)
}

// shapePieces shapes the paragraph in its pieces. Direction and script are
// resolved once for the whole paragraph, as plain text is, so a span never
// splits a bidi run. The resulting inputs are then split at the piece
// boundaries, and each part takes its piece's face per rune. The line wrapper
// sees all outputs as one paragraph.
func (sys *System) shapePieces(text []rune, plain *Font, pcs []piece, size fixed.Int26_6) []shaping.Output {
	outs := sys.outs[:0]
	fm := fontmap{reg: sys.reg, primary: plain} // fallback stickiness is per paragraph
	input := shaping.Input{
		Text: text, RunStart: 0, RunEnd: len(text), Direction: di.DirectionLTR,
		Face: plain.face, Size: size, Language: language.DefaultLanguage(),
	}
	ins := sys.seg.Split(input, &fm)
	if len(pcs) == 1 && pcs[0].font == plain {
		for _, in := range ins {
			outs = append(outs, sys.shaper.Shape(in))
		}
		sys.outs = outs
		return outs
	}
	for _, in := range ins {
		for _, p := range pcs {
			a, b := max(in.RunStart, p.start), min(in.RunEnd, p.end)
			if a >= b {
				continue
			}
			fm.primary = p.font
			// Group the runes of this part by the face they resolve to.
			emit := func(from, to int, face *tsfont.Face) {
				sub := in
				sub.RunStart, sub.RunEnd, sub.Face = from, to, face
				outs = append(outs, sys.shaper.Shape(sub))
			}
			from := a
			var face *tsfont.Face
			for i := a; i < b; i++ {
				f := fm.ResolveFace(text[i])
				if i > from && f != face {
					emit(from, i, face)
					from = i
				}
				face = f
			}
			emit(from, b, face)
		}
	}
	sys.outs = outs
	return outs
}

// wrapParagraph shapes runes [start, end) (no newlines) and appends its lines.
// pcs splits the paragraph into pieces; their offsets are relative to start.
func (sys *System) wrapParagraph(l *Layout, start, end int, size fixed.Int26_6, wrapW float32, pcs []piece) {
	text := l.runes[start:end]
	outs := sys.shapePieces(text, sys.reg.Font(l.Style.Family, l.Style.Weight, l.Style.Italic), pcs, size)
	cfg := shaping.WrapConfig{Direction: di.DirectionLTR, DisableTrailingWhitespaceTrim: true}
	ell := pcs[0]
	if st := l.Style; st.MaxLines > 0 {
		cfg.TruncateAfterLines = st.MaxLines - len(l.Lines)
		if len(pcs) > 1 {
			ell = sys.truncationPiece(text, outs, pcs, cfg.TruncateAfterLines, wrapW)
		}
		cfg.Truncator = sys.ellipsis(ell.font, size)
	}
	lines, truncated := sys.wrapper.WrapParagraphF(cfg, fixed.Int26_6(wrapW*64), text, shaping.NewSliceIterator(outs))
	if truncated > 0 {
		l.Truncated = true
	}
	for i, ln := range lines {
		last := i == len(lines)-1
		l.Lines = append(l.Lines, l.convertLine(ln, start, pcs, ell.span, last && truncated == 0, last && truncated > 0))
	}
}

// truncationPiece returns the piece the ellipsis of a truncated paragraph
// follows: the piece of the last run on the line that truncation cuts. It
// wraps once without truncation to find that line.
func (sys *System) truncationPiece(text []rune, outs []shaping.Output, pcs []piece, keep int, wrapW float32) piece {
	cfg := shaping.WrapConfig{Direction: di.DirectionLTR, DisableTrailingWhitespaceTrim: true}
	lines, _ := sys.wrapper.WrapParagraphF(cfg, fixed.Int26_6(wrapW*64), text, shaping.NewSliceIterator(outs))
	if keep < 1 || keep > len(lines) {
		return pcs[0]
	}
	last := -1
	for _, o := range lines[keep-1] {
		last = max(last, o.Runes.Offset)
	}
	if last < 0 {
		return pcs[0]
	}
	return pcs[pieceIndex(pcs, last)]
}

// convertLine turns a wrapped line into positioned runs in visual order. The
// ellipsis run takes the span ellSpan.
func (l *Layout) convertLine(ln shaping.Line, base int, pcs []piece, ellSpan int, hardEnd, truncated bool) Line {
	order := make([]int, len(ln))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return ln[order[a]].VisualIndex < ln[order[b]].VisualIndex })
	out := Line{hardEnd: hardEnd, RuneStart: math.MaxInt, RuneEnd: -1}
	var x float32
	for _, idx := range order {
		o := ln[idx]
		f := fontOf(o.Face)
		run := Run{
			X:         x,
			RTL:       o.Direction == di.DirectionRTL,
			RuneStart: base + o.Runes.Offset,
			RuneEnd:   base + o.Runes.Offset + o.Runes.Count,
			Span:      pcs[pieceIndex(pcs, o.Runes.Offset)].span,
			// The truncator is appended last in logical order.
			ellipsis: truncated && idx == len(ln)-1,
		}
		if run.ellipsis {
			run.Span = ellSpan
		}
		var pen float32
		run.Glyphs = make([]Glyph, 0, len(o.Glyphs))
		for _, g := range o.Glyphs {
			adv := fx(g.Advance)
			run.Glyphs = append(run.Glyphs, Glyph{
				font: f, ID: sfnt.GlyphIndex(g.GlyphID),
				Pen: pen, X: pen + fx(g.XOffset), Y: -fx(g.YOffset), Adv: adv,
				Cluster: base + g.ClusterIndex, Runes: max(g.RuneCount, 1),
			})
			pen += adv
		}
		run.Width = pen
		x += pen
		if !run.ellipsis {
			out.RuneStart = min(out.RuneStart, run.RuneStart)
			out.RuneEnd = max(out.RuneEnd, run.RuneEnd)
		}
		out.Runs = append(out.Runs, run)
	}
	if out.RuneEnd < 0 {
		out.RuneStart, out.RuneEnd = base, base
	}
	// Width without trailing whitespace (in logical order).
	lastInk := out.RuneEnd - 1
	for lastInk >= out.RuneStart && unicode.IsSpace(l.runes[lastInk]) {
		lastInk--
	}
	var trailing float32
	for _, r := range out.Runs {
		if r.ellipsis {
			continue
		}
		for _, g := range r.Glyphs {
			if g.Cluster > lastInk {
				trailing += g.Adv
			}
		}
	}
	out.Width = x - trailing
	return out
}

// ellipsis returns a shaped "…" for truncated lines.
func (sys *System) ellipsis(f *Font, size fixed.Int26_6) shaping.Output {
	k := ellipsisKey{f, size}
	if o, ok := sys.ellipses[k]; ok {
		return o
	}
	face := f.face
	if !f.has('…') {
		if fb := sys.reg.fallbackFor('…'); fb != nil {
			face = fb.face
		}
	}
	o := sys.shaper.Shape(shaping.Input{
		Text: []rune{'…'}, RunEnd: 1, Direction: di.DirectionLTR, Face: face, Size: size,
		Language: language.DefaultLanguage(),
	})
	sys.ellipses[k] = o
	return o
}

type ellipsisKey struct {
	f    *Font
	size fixed.Int26_6
}

// ---- positions ----

// runeAt converts a byte offset to a rune index (rounding down inside a rune).
func (l *Layout) runeAt(pos int) int {
	i := sort.SearchInts(l.byteOf, pos)
	if i < len(l.byteOf) && l.byteOf[i] == pos {
		return i
	}
	return max(i-1, 0)
}

func (l *Layout) byteAt(r int) int { return l.byteOf[min(max(r, 0), len(l.byteOf)-1)] }

// LineOf returns the index of the line a caret at byte offset pos is on.
func (l *Layout) LineOf(pos int) int {
	r := l.runeAt(pos)
	for i, ln := range l.Lines {
		if r >= ln.RuneStart && r < ln.RuneEnd || r == ln.RuneEnd && (ln.hardEnd || i == len(l.Lines)-1) {
			return i
		}
		if r < ln.RuneStart {
			return max(i-1, 0)
		}
	}
	return max(len(l.Lines)-1, 0)
}

// LineBounds returns the byte range [start, end) of line i, excluding a
// trailing newline.
func (l *Layout) LineBounds(i int) (start, end int) {
	if len(l.Lines) == 0 {
		return 0, 0
	}
	ln := l.Lines[min(max(i, 0), len(l.Lines)-1)]
	return l.byteAt(ln.RuneStart), l.byteAt(ln.RuneEnd)
}

// Caret returns where to draw the caret for byte offset pos: x from the
// layout's left edge and the top of its line.
func (l *Layout) Caret(pos int) (x, top float32) {
	if len(l.Lines) == 0 {
		return 0, 0
	}
	li := l.LineOf(pos)
	ln := &l.Lines[li]
	return l.caretX(ln, l.runeAt(pos)), ln.Top
}

// CaretX is Caret's x and line index, kept for single-line callers.
func (l *Layout) CaretX(pos int) (float32, int) {
	x, _ := l.Caret(pos)
	return x, l.LineOf(pos)
}

func (l *Layout) caretX(ln *Line, r int) float32 {
	if r < ln.RuneEnd {
		if x, ok := l.edge(ln, r, false); ok {
			return x
		}
	}
	if r > ln.RuneStart {
		if x, ok := l.edge(ln, r-1, true); ok {
			return x
		}
	}
	if len(ln.Runs) > 0 && ln.Runs[0].RTL {
		return ln.Width
	}
	return 0
}

// edge returns the x of rune r's leading (or trailing) edge in reading
// direction. Runes inside a ligature get proportional positions.
func (l *Layout) edge(ln *Line, r int, trailing bool) (float32, bool) {
	for i := range ln.Runs {
		run := &ln.Runs[i]
		if run.ellipsis || r < run.RuneStart || r >= run.RuneEnd {
			continue
		}
		x0, x1 := float32(math.Inf(1)), float32(math.Inf(-1))
		cstart, cn := 0, 1
		for _, g := range run.Glyphs {
			if r >= g.Cluster && r < g.Cluster+g.Runes {
				x0 = min(x0, run.X+g.Pen)
				x1 = max(x1, run.X+g.Pen+g.Adv)
				cstart, cn = g.Cluster, g.Runes
			}
		}
		if x1 < x0 {
			continue
		}
		k := r - cstart
		if trailing {
			k++
		}
		f := float32(k) / float32(cn)
		if run.RTL {
			return x1 - f*(x1-x0), true
		}
		return x0 + f*(x1-x0), true
	}
	return 0, false
}

// SpanAt returns the index of the span whose run is under (x, y), in layout
// coordinates (left-aligned, from the top-left), or -1 when none is.
func (l *Layout) SpanAt(x, y float32) int {
	if len(l.Lines) == 0 || l.LineHeight <= 0 {
		return -1
	}
	li := int(math.Floor(float64(y / l.LineHeight)))
	if li < 0 || li >= len(l.Lines) {
		return -1
	}
	for _, run := range l.Lines[li].Runs {
		if x >= run.X && x < run.X+run.Width {
			return run.Span
		}
	}
	return -1
}

// HitTest returns the byte offset of the caret position nearest to (x, y),
// relative to the top-left of the layout.
func (l *Layout) HitTest(x, y float32) int {
	if len(l.Lines) == 0 {
		return 0
	}
	li := int(math.Floor(float64(y / l.LineHeight)))
	return l.PositionInLine(min(max(li, 0), len(l.Lines)-1), x)
}

// PositionInLine returns the byte offset on line i nearest to x. It is how
// Up/Down keep the caret's horizontal position.
func (l *Layout) PositionInLine(i int, x float32) int {
	ln := &l.Lines[min(max(i, 0), len(l.Lines)-1)]
	last := ln.RuneEnd
	if !ln.hardEnd && last > ln.RuneStart {
		last-- // a caret at a soft wrap belongs to the next line
	}
	best, bestD := ln.RuneStart, float32(math.Inf(1))
	for r := ln.RuneStart; r <= last; r++ {
		if r > ln.RuneStart && r < len(l.runes) && isMark(l.runes[r]) {
			continue // not a caret stop
		}
		d := l.caretX(ln, r) - x
		if d < 0 {
			d = -d
		}
		if d < bestD {
			best, bestD = r, d
		}
	}
	return l.byteAt(best)
}

func isMark(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || r == 0x200D || r == 0xFE0F
}

// Rect is a rectangle in layout coordinates.
type Rect struct{ X, Y, W, H float32 }

// SelectionRects returns rectangles covering the text between byte offsets
// a and b. Mixed-direction text may need several rectangles per line.
func (l *Layout) SelectionRects(a, b int, dst []Rect) []Rect {
	ra, rb := l.runeAt(min(a, b)), l.runeAt(max(a, b))
	for i := range l.Lines {
		ln := &l.Lines[i]
		if rb < ln.RuneStart || ra > ln.RuneEnd {
			continue
		}
		lo, hi := max(ra, ln.RuneStart), min(rb, ln.RuneEnd)
		start := len(dst)
		for r := lo; r < hi; r++ {
			x0, ok0 := l.edge(ln, r, false)
			x1, ok1 := l.edge(ln, r, true)
			if !ok0 || !ok1 {
				continue
			}
			if x1 < x0 {
				x0, x1 = x1, x0
			}
			dst = mergeRect(dst, start, Rect{x0, ln.Top, x1 - x0, l.LineHeight})
		}
		// Show a selected line break as a small block at the end of the line.
		if ln.hardEnd && rb > ln.RuneEnd && ra <= ln.RuneEnd && ln.RuneEnd < len(l.runes) {
			x := l.caretX(ln, ln.RuneEnd)
			dst = mergeRect(dst, start, Rect{x, ln.Top, l.Style.Size * 0.35, l.LineHeight})
		}
	}
	return dst
}

func mergeRect(dst []Rect, from int, r Rect) []Rect {
	for i := from; i < len(dst); i++ {
		d := &dst[i]
		if r.X <= d.X+d.W+0.5 && r.X+r.W >= d.X-0.5 {
			x0, x1 := min(d.X, r.X), max(d.X+d.W, r.X+r.W)
			d.X, d.W = x0, x1-x0
			return dst
		}
	}
	return append(dst, r)
}

// ---- boundaries (byte offsets into a string) ----

// graphemes returns the byte offsets of grapheme cluster boundaries in s,
// including 0 and len(s).
func (sys *System) graphemes(s string) []int {
	if sys.graphemeText == s && sys.graphemeStops != nil {
		return sys.graphemeStops
	}
	stops := sys.graphemeStops[:0]
	if err := sys.gseg.InitWithString(s); err == nil {
		it := sys.gseg.GraphemeIterator()
		for it.Next() {
			stops = append(stops, it.Grapheme().OffsetInBytes)
		}
	}
	stops = append(stops, len(s))
	if stops[0] != 0 {
		stops = slices.Insert(stops, 0, 0)
	}
	sys.graphemeText, sys.graphemeStops = s, stops
	return stops
}

// NextGrapheme returns the byte offset after the user-perceived character at pos.
func (sys *System) NextGrapheme(s string, pos int) int {
	for _, b := range sys.graphemes(s) {
		if b > pos {
			return b
		}
	}
	return len(s)
}

// PrevGrapheme returns the byte offset of the user-perceived character before pos.
func (sys *System) PrevGrapheme(s string, pos int) int {
	stops := sys.graphemes(s)
	for i := len(stops) - 1; i >= 0; i-- {
		if stops[i] < pos {
			return stops[i]
		}
	}
	return 0
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || unicode.Is(unicode.Mn, r)
}

// NextWord returns the end of the word at or after pos (Ctrl+Right).
func NextWord(s string, pos int) int {
	for pos < len(s) {
		r, n := utf8.DecodeRuneInString(s[pos:])
		if isWordRune(r) {
			break
		}
		pos += n
	}
	for pos < len(s) {
		r, n := utf8.DecodeRuneInString(s[pos:])
		if !isWordRune(r) {
			break
		}
		pos += n
	}
	return pos
}

// PrevWord returns the start of the word before pos (Ctrl+Left).
func PrevWord(s string, pos int) int {
	for pos > 0 {
		r, n := utf8.DecodeLastRuneInString(s[:pos])
		if isWordRune(r) {
			break
		}
		pos -= n
	}
	for pos > 0 {
		r, n := utf8.DecodeLastRuneInString(s[:pos])
		if !isWordRune(r) {
			break
		}
		pos -= n
	}
	return pos
}

// WordAt returns the byte range of the word (or run of other characters)
// around pos, for double-click selection.
func WordAt(s string, pos int) (int, int) {
	pos = min(max(pos, 0), len(s))
	in := func(p int) bool {
		r, _ := utf8.DecodeRuneInString(s[p:])
		return isWordRune(r)
	}
	if pos == len(s) && pos > 0 {
		_, n := utf8.DecodeLastRuneInString(s)
		pos -= n
	}
	if pos >= len(s) {
		return pos, pos
	}
	word := in(pos)
	a, b := pos, pos
	for a > 0 {
		r, n := utf8.DecodeLastRuneInString(s[:a])
		if isWordRune(r) != word || r == '\n' {
			break
		}
		a -= n
	}
	for b < len(s) {
		r, n := utf8.DecodeRuneInString(s[b:])
		if isWordRune(r) != word || r == '\n' {
			break
		}
		b += n
	}
	return a, b
}
