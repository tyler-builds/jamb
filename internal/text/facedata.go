package text

import (
	"math"
	"strconv"

	tsfont "github.com/go-text/typesetting/font"
	"golang.org/x/image/math/fixed"
)

// A web page on iOS has an editable element lie over a text field, which the
// keyboard's trackpad moves the caret in by where its characters are on screen
// (see internal/platform/web). The element is built from the field's layout:
// Pieces splits a layout into stretches that a browser lays out exactly as
// jamb does, each in the face jamb drew it with (FaceData), and says what the
// browser must add for the rest: the width of a tab, the kerning go-text
// applied to a character.

// FaceData is the font file of a face that text is drawn with, for laying the
// same text out elsewhere.
type FaceData struct {
	// Key names the face, the same for as long as the font is loaded and
	// unique to it: "jamb-face-" and a number.
	Key string
	// Data is the font file, which must not be changed.
	Data []byte
	// Weight is where a variable font is drawn on its weight axis, or 0 for
	// its default instance.
	Weight float32
	// NotdefKey and Notdef are a font of one empty glyph as wide as the
	// missing glyph of the face, for every character but the controls and the
	// default-ignorable ones (see notdefFont): after the face in a browser's
	// list of fonts, it lays a character the face lacks out as the box jamb
	// draws for it.
	NotdefKey string
	Notdef    []byte
}

// faceData returns the FaceData of f, made once.
func (sys *System) faceData(f *Font) FaceData {
	if sys.faceSets == nil {
		sys.faceSets = map[*Font]FaceData{}
	}
	if d, ok := sys.faceSets[f]; ok {
		return d
	}
	id := strconv.FormatUint(uint64(f.id), 10)
	d := FaceData{Key: "jamb-face-" + id, Data: f.data, NotdefKey: "jamb-notdef-" + id}
	if f.varied {
		d.Weight = f.wght
	}
	adv := f.face.HorizontalAdvance(0)
	d.Notdef = notdefFont(uint16(f.upem), uint16(max(adv, 0)+0.5), int16(f.ascent), int16(f.descent))
	sys.faceSets[f] = d
	return d
}

// LayoutSize returns the size whose advances a layout of text of the size
// size has: go-text scales a font to whole pixels, rounding up, so text of
// 13.5 pixels is laid out, and its caret placed, as text of 14 is, though its
// glyphs are drawn at 13.5. A browser that lays the same text out takes this
// size, to put each character where jamb does.
func LayoutSize(size float32) float32 {
	return float32(fixed.Int26_6(size*64 + 0.5).Ceil())
}

// PieceKind is what a Piece is.
type PieceKind uint8

const (
	// PieceText is text in one face, at the font's own advances, but for its
	// Spacing.
	PieceText PieceKind = iota
	// PieceTab is a tab, Width wide.
	PieceTab
	// PieceBreak is the line break that ends a paragraph.
	PieceBreak
	// PieceNone is text that the layout gives no width, such as a control
	// character with no glyph.
	PieceNone
)

// Piece is a stretch of a layout's text that a browser lays out as jamb does,
// given the face, the size (LayoutSize of the layout's) and the Spacing.
type Piece struct {
	Start, End int // byte offsets in the layout's text
	Kind       PieceKind
	Line       int // the line the piece is on
	// Face is the index of the piece's face among those Pieces returns, or -1
	// for a piece of no face (a break, or nothing).
	Face int
	// Width is the piece's advance in the layout.
	Width float32
	// Spacing is what the layout adds after each character of the piece to
	// the font's own advance: the kerning go-text applied, or the rounding of
	// the advances before it to 1/64 of a pixel once it adds up (see
	// driftMax), which a piece of its own, of one character, carries. 0 for
	// most pieces.
	Spacing float32
}

// driftMax is how far, in pixels, the layout's advances may run ahead of or
// behind the font's own before a piece takes up the difference: go-text rounds
// each advance to 1/64 of a pixel, where a browser does not, and kerning
// changes a few.
const driftMax = 0.2

// Pieces splits the layout l into pieces, in the order of its text, which
// cover it all: runs of text in one face at the font's own advances; each
// character whose advance the layout changed (kerning) on its own, with the
// change as its Spacing; tabs, with their width; and the line breaks between
// paragraphs. It returns the faces the pieces are in too. A browser that lays
// the pieces out in turn, each in its face at the font's own advances plus
// Spacing (no kerning, the default ligatures), a tab as a box of its width,
// breaks the lines where they break here (it wraps at the same width), and puts
// each character where the layout puts it. Right-to-left runs are pieces of
// their own, without spacing. Pieces assumes the layout is not truncated.
func (sys *System) Pieces(l *Layout) ([]Piece, []FaceData) {
	var pieces []Piece
	var faces []FaceData
	faceIndex := map[*Font]int{}
	face := func(f *Font) int {
		if f == nil || len(f.data) == 0 {
			return -1
		}
		i, ok := faceIndex[f]
		if !ok {
			i = len(faces)
			faceIndex[f] = i
			faces = append(faces, sys.faceData(f))
		}
		return i
	}
	size := LayoutSize(l.Style.Size)
	// nominal returns the advance the font gives glyph g at the layout's
	// size, unrounded, as a browser lays it out.
	nominal := func(g *Glyph) float64 {
		if g.font == nil || g.font.upem == 0 {
			return float64(g.Adv)
		}
		return float64(g.font.face.HorizontalAdvance(tsfont.GID(g.ID))) * float64(size) / float64(g.font.upem)
	}
	at := 0 // the rune the pieces have reached
	push := func(p Piece) {
		if n := len(pieces); n > 0 && p.Kind == PieceText && p.Spacing == 0 {
			q := &pieces[n-1]
			if q.Kind == PieceText && q.Spacing == 0 && q.Face == p.Face && q.Line == p.Line && q.End == p.Start {
				q.End, q.Width = p.End, q.Width+p.Width
				return
			}
		}
		pieces = append(pieces, p)
	}
	// fill covers the runes from at to r, which no glyph covers, as nothing.
	fill := func(r, line int) {
		if r > at {
			push(Piece{Start: l.byteAt(at), End: l.byteAt(r), Kind: PieceNone, Line: line, Face: -1})
			at = r
		}
	}
	for li := range l.Lines {
		ln := &l.Lines[li]
		runs := make([]*Run, 0, len(ln.Runs))
		for i := range ln.Runs {
			if !ln.Runs[i].ellipsis {
				runs = append(runs, &ln.Runs[i])
			}
		}
		// Logical order: the runs of a line are kept in visual order.
		for i := 1; i < len(runs); i++ {
			for j := i; j > 0 && runs[j].RuneStart < runs[j-1].RuneStart; j-- {
				runs[j], runs[j-1] = runs[j-1], runs[j]
			}
		}
		if li > 0 && ln.RuneStart > at {
			// The line break between paragraphs, and anything before it.
			fill(ln.RuneStart-1, li-1)
			kind := PieceBreak
			if l.runes[at] != '\n' {
				kind = PieceNone
			}
			push(Piece{Start: l.byteAt(at), End: l.byteAt(at + 1), Kind: kind, Line: li - 1, Face: -1})
			at++
		}
		for _, run := range runs {
			fill(run.RuneStart, li)
			drift := 0.0 // how far the run's advances are ahead of the font's
			if run.RTL {
				f := -1
				if len(run.Glyphs) > 0 {
					f = face(run.Glyphs[0].font)
				}
				push(Piece{Start: l.byteAt(run.RuneStart), End: l.byteAt(run.RuneEnd), Line: li, Face: f, Width: run.Width})
				at = run.RuneEnd
				continue
			}
			for gi := 0; gi < len(run.Glyphs); {
				// A cluster: the glyphs of the same runes.
				g := &run.Glyphs[gi]
				adv, nom := float32(0), 0.0
				gj := gi
				for ; gj < len(run.Glyphs) && run.Glyphs[gj].Cluster == g.Cluster; gj++ {
					adv += run.Glyphs[gj].Adv
					nom += nominal(&run.Glyphs[gj])
				}
				start, end := g.Cluster, min(g.Cluster+g.Runes, run.RuneEnd)
				if gj < len(run.Glyphs) {
					end = min(end, run.Glyphs[gj].Cluster)
				} else {
					end = run.RuneEnd
				}
				fill(start, li)
				p := Piece{Start: l.byteAt(start), End: l.byteAt(end), Line: li, Face: face(g.font), Width: adv}
				drift += float64(adv) - nom
				switch {
				case end-start == 1 && l.runes[start] == '\t':
					p.Kind, p.Face = PieceTab, -1
					drift = 0
				case math.Abs(drift) > driftMax:
					p.Spacing = float32(drift) / float32(end-start)
					drift = 0
				}
				push(p)
				at, gi = end, gj
			}
		}
	}
	fill(len(l.runes), max(len(l.Lines)-1, 0))
	return pieces, faces
}
