package text

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	tsfont "github.com/go-text/typesetting/font"
)

// usesPointAnchors reports whether the composite glyph g, the bytes of its glyf
// entry, places a component by matching points (its flags lack ARGS_ARE_XY_VALUES),
// and whether g could be read at all. Components placed by offsets need no points.
func usesPointAnchors(g []byte) (anchored, ok bool) {
	const (
		argsAreWords = 0x0001
		argsAreXY    = 0x0002
		haveScale    = 0x0008
		moreComps    = 0x0020
		xyScale      = 0x0040
		twoByTwo     = 0x0080
	)
	be := binary.BigEndian
	for i := 10; ; { // the component records start after the glyph header
		if i+4 > len(g) {
			return false, false
		}
		flags := be.Uint16(g[i:])
		i += 4 // the flags and the glyph index
		if flags&argsAreWords != 0 {
			i += 4
		} else {
			i += 2
		}
		if flags&argsAreXY == 0 {
			return true, true
		}
		switch {
		case flags&haveScale != 0:
			i += 2
		case flags&xyScale != 0:
			i += 4
		case flags&twoByTwo != 0:
			i += 8
		}
		if flags&moreComps == 0 {
			return false, true
		}
	}
}

// shapingFile is the font file as go-text reads it, with its glyph outlines cut:
// the table directory of the font, with the offsets of the loca and glyf tables
// changed; then the rest of the file as it is; then the two rebuilt tables. It
// implements the Resource interface that go-text's font package reads.
//
// jamb shapes with go-text/typesetting, but draws outlines with DirectWrite or
// x/image/font/sfnt, which read the font file themselves. go-text parses every
// glyph of the glyf table into memory, although shaping needs only each glyph's
// bounding box and metrics. The view gives go-text each simple glyph as an empty
// outline with the same bounding box, so the outlines are not kept in memory twice;
// composite glyphs, which are small, are kept whole. The font file is not copied.
//
// A font is viewed this way only when shaping gives the same results with the view
// (see shapingResource). Contour points are not needed: the GPOS anchors and the
// GDEF caret values that refer to them fall back to the design units in go-text
// 0.3.5, whether or not the outline is cut (its Font.GetGlyphContourPoint always
// reports false). Phantom points come from the bounding box, the side bearing and
// the advances, which the view keeps. TestShapingFileMatchesFullFonts checks the
// results, and TestGoTextHasNoContourPoints fails if go-text starts to resolve
// contour points, which would mean the view must not be used.
type shapingFile struct {
	head  []byte // the table directory; replaces the first len(head) bytes of data
	data  []byte // the font file
	extra []byte // the padding, the new loca table and the new glyf table, from len(data)
	pos   int64
}

// shapingResource returns the file that go-text parses for the font data: a
// shapingFile when the font can be cut, and the bytes otherwise. A variable font
// (fvar) is not cut, because go-text applies its variations to the outlines when it
// measures the glyphs; a collection (ttcf), a font with CFF outlines, and a font
// with a composite glyph that places a component by points are not cut either.
func shapingResource(data []byte) tsfont.Resource {
	if f := trimGlyf(data); f != nil {
		return f
	}
	return bytes.NewReader(data)
}

func (f *shapingFile) size() int64 { return int64(len(f.data) + len(f.extra)) }

func (f *shapingFile) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("text: negative offset in a font")
	}
	n := 0
	for n < len(p) {
		o := off + int64(n)
		var src []byte
		switch {
		case o < int64(len(f.head)):
			src = f.head[o:]
		case o < int64(len(f.data)):
			src = f.data[o:]
		case o < f.size():
			src = f.extra[o-int64(len(f.data)):]
		default:
			return n, io.EOF
		}
		n += copy(p[n:], src)
	}
	return n, nil
}

func (f *shapingFile) Read(p []byte) (int, error) {
	n, err := f.ReadAt(p, f.pos)
	f.pos += int64(n)
	if n > 0 && err == io.EOF {
		err = nil
	}
	return n, err
}

func (f *shapingFile) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		off += f.pos
	case io.SeekEnd:
		off += f.size()
	}
	if off < 0 {
		return 0, errors.New("text: negative position in a font")
	}
	f.pos = off
	return off, nil
}

// trimGlyf returns the view of a single font file whose glyph outlines are cut to
// their bounding boxes, or nil when the font cannot be cut (see shapingResource).
// It reads the table directory, then head (the loca format), maxp (the glyph count),
// loca and glyf, and returns nil for any table that is missing or out of the file.
func trimGlyf(data []byte) *shapingFile {
	be := binary.BigEndian
	if len(data) < 12 || bytes.HasPrefix(data, []byte("ttcf")) {
		return nil
	}
	n := int(be.Uint16(data[4:6]))
	dirEnd := 12 + 16*n
	if dirEnd > len(data) {
		return nil
	}
	type table struct{ entry, off, length int }
	tables := map[string]table{}
	for i := 0; i < n; i++ {
		e := 12 + 16*i
		tables[string(data[e:e+4])] = table{e, int(be.Uint32(data[e+8:])), int(be.Uint32(data[e+12:]))}
	}
	if _, ok := tables["fvar"]; ok {
		return nil
	}
	glyf, okG := tables["glyf"]
	loca, okL := tables["loca"]
	head, okH := tables["head"]
	maxp, okM := tables["maxp"]
	if !okG || !okL || !okH || !okM {
		return nil
	}
	inFile := func(t table) bool { return t.off >= 0 && t.length >= 0 && t.off+t.length <= len(data) }
	if !inFile(glyf) || !inFile(loca) || !inFile(head) || !inFile(maxp) || head.length < 54 || maxp.length < 6 {
		return nil
	}
	long := be.Uint16(data[head.off+50:]) == 1 // indexToLocFormat
	numGlyphs := int(be.Uint16(data[maxp.off+4:]))
	need := (numGlyphs + 1) * 2
	if long {
		need *= 2
	}
	if loca.length < need {
		return nil
	}
	locaAt := func(i int) int {
		if long {
			return int(be.Uint32(data[loca.off+4*i:]))
		}
		return 2 * int(be.Uint16(data[loca.off+2*i:]))
	}

	// Each glyph becomes a header with no contours, its bounding box, and no
	// instructions: 12 bytes, which keeps the offsets even for the short loca.
	var newGlyf, newLoca []byte
	putLoca := func(off int) {
		if long {
			newLoca = be.AppendUint32(newLoca, uint32(off))
		} else {
			newLoca = be.AppendUint16(newLoca, uint16(off/2))
		}
	}
	for i := 0; i < numGlyphs; i++ {
		putLoca(len(newGlyf))
		s, e := locaAt(i), locaAt(i+1)
		if e-s < 10 || e > glyf.length {
			continue // an empty glyph, or one whose header is not in the table
		}
		g := data[glyf.off+s : glyf.off+e]
		if int16(be.Uint16(g)) < 0 {
			// A composite glyph is kept whole: the phantom points of a composite
			// come from its components, which may be composites too. A component
			// that places itself by matching points reads the points of the glyph's
			// outline, which a cut outline does not have, so the font is not cut.
			if anchored, ok := usesPointAnchors(g); !ok || anchored {
				return nil
			}
			newGlyf = append(newGlyf, g...)
			if len(g)%2 != 0 {
				newGlyf = append(newGlyf, 0) // the short loca needs even offsets
			}
			continue
		}
		newGlyf = append(newGlyf, 0, 0) // numberOfContours: none
		newGlyf = append(newGlyf, g[2:10]...)
		newGlyf = append(newGlyf, 0, 0) // instructionLength: none
	}
	putLoca(len(newGlyf))
	for len(newLoca)%4 != 0 {
		newLoca = append(newLoca, 0)
	}

	// The directory is copied, and its loca and glyf entries point to the tables
	// after the end of the file. The tables are aligned to 4 bytes.
	dir := append([]byte(nil), data[:dirEnd]...)
	locaOff := (len(data) + 3) &^ 3
	extra := make([]byte, locaOff-len(data), locaOff-len(data)+len(newLoca)+len(newGlyf))
	extra = append(extra, newLoca...)
	extra = append(extra, newGlyf...)
	be.PutUint32(dir[loca.entry+8:], uint32(locaOff))
	be.PutUint32(dir[loca.entry+12:], uint32(len(newLoca)))
	be.PutUint32(dir[glyf.entry+8:], uint32(locaOff+len(newLoca)))
	be.PutUint32(dir[glyf.entry+12:], uint32(len(newGlyf)))
	return &shapingFile{head: dir, data: data, extra: extra}
}
