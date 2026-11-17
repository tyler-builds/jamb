package text

import (
	"encoding/binary"
	"slices"
	"unicode/utf16"
)

// notdefFont returns a TrueType font whose one glyph, empty and advance units
// wide in a font of upem units to the em, stands for every character but the
// controls and the default-ignorable ones: the box jamb draws for a character
// that none of its fonts has, the missing glyph of the text's own face. Last in
// a browser's list of fonts, it lays such characters out as jamb does, where
// the browser would otherwise draw them in a font of its own, of other widths
// (see FaceData). Its cmap is of format 13, many characters to one glyph,
// as the last-resort fonts of Apple's systems have.
func notdefFont(upem, advance uint16, ascent, descent int16) []byte {
	be := binary.BigEndian
	u16 := func(b []byte, v uint16) []byte { return be.AppendUint16(b, v) }
	u32 := func(b []byte, v uint32) []byte { return be.AppendUint32(b, v) }
	i16 := func(b []byte, v int16) []byte { return be.AppendUint16(b, uint16(v)) }

	head := u32(nil, 0x00010000)             // version
	head = u32(head, 0x00010000)             // fontRevision
	head = u32(head, 0)                      // checksumAdjustment, set below
	head = u32(head, 0x5F0F3CF5)             // magicNumber
	head = u16(head, 0x000B)                 // flags: baseline and left side bearing at 0, integer scaling
	head = u16(head, upem)                   // unitsPerEm
	head = append(head, make([]byte, 16)...) // created, modified
	head = i16(head, 0)                      // xMin
	head = i16(head, -descent)               // yMin
	head = i16(head, int16(advance))         // xMax
	head = i16(head, ascent)                 // yMax
	head = u16(head, 0)                      // macStyle
	head = u16(head, 8)                      // lowestRecPPEM
	head = i16(head, 2)                      // fontDirectionHint
	head = i16(head, 0)                      // indexToLocFormat: short offsets
	head = i16(head, 0)                      // glyphDataFormat

	hhea := u32(nil, 0x00010000)
	hhea = i16(hhea, ascent)
	hhea = i16(hhea, -descent)
	hhea = i16(hhea, 0)                      // lineGap
	hhea = u16(hhea, advance)                // advanceWidthMax
	hhea = append(hhea, make([]byte, 6)...)  // min side bearings, xMaxExtent
	hhea = i16(hhea, 1)                      // caretSlopeRise
	hhea = append(hhea, make([]byte, 14)...) // caretSlopeRun, caretOffset, reserved, metricDataFormat
	hhea = u16(hhea, 2)                      // numberOfHMetrics

	maxp := u32(nil, 0x00010000)
	maxp = u16(maxp, 2) // numGlyphs
	maxp = append(maxp, make([]byte, 26)...)
	be.PutUint16(maxp[14:], 2) // maxZones

	hmtx := u16(u16(u16(u16(nil, advance), 0), advance), 0)
	loca := make([]byte, 6) // three short offsets, all 0: both glyphs are empty
	glyf := make([]byte, 4)

	// The characters the glyph stands for: all but the controls, surrogates and
	// default-ignorable characters, which a browser lays out as nothing, as
	// jamb's shaper does.
	skip := [][2]rune{{0, 0x1F}, {0x7F, 0x9F}, {0xAD, 0xAD}, {0x34F, 0x34F}, {0x61C, 0x61C},
		{0x115F, 0x1160}, {0x17B4, 0x17B5}, {0x180B, 0x180F}, {0x200B, 0x200F}, {0x2028, 0x202E},
		{0x2060, 0x206F}, {0x3164, 0x3164}, {0xD800, 0xDFFF}, {0xFE00, 0xFE0F}, {0xFEFF, 0xFEFF},
		{0xFFA0, 0xFFA0}, {0xFFF0, 0xFFF8}, {0x1BCA0, 0x1BCA3}, {0x1D173, 0x1D17A}, {0xE0000, 0xE0FFF}}
	var groups [][2]rune
	next := rune(0)
	for _, s := range skip {
		if s[0] > next {
			groups = append(groups, [2]rune{next, s[0] - 1})
		}
		next = s[1] + 1
	}
	groups = append(groups, [2]rune{next, 0x10FFFF})
	sub := u16(nil, 13)
	sub = u16(sub, 0)
	sub = u32(sub, uint32(16+12*len(groups)))
	sub = u32(sub, 0) // language
	sub = u32(sub, uint32(len(groups)))
	for _, g := range groups {
		sub = u32(u32(u32(sub, uint32(g[0])), uint32(g[1])), 1)
	}
	cmap := u16(u16(nil, 0), 1)           // version, one subtable
	cmap = u32(u16(u16(cmap, 3), 10), 12) // Windows, full Unicode, at 12
	cmap = append(cmap, sub...)

	// The names: family, subfamily, unique name, full name and PostScript name.
	names := []string{"jamb notdef", "Regular", "jamb notdef", "jamb notdef", "jamb-notdef"}
	nameIDs := []uint16{1, 2, 3, 4, 6}
	var strs []byte
	name := u16(u16(u16(nil, 0), uint16(len(names))), uint16(6+12*len(names)))
	for i, s := range names {
		var enc []byte
		for _, c := range utf16.Encode([]rune(s)) {
			enc = u16(enc, c)
		}
		name = u16(u16(u16(u16(name, 3), 1), 0x409), nameIDs[i])
		name = u16(u16(name, uint16(len(enc))), uint16(len(strs)))
		strs = append(strs, enc...)
	}
	name = append(name, strs...)

	os2 := u16(nil, 4)                        // version
	os2 = i16(os2, int16(advance))            // xAvgCharWidth
	os2 = u16(os2, 400)                       // usWeightClass
	os2 = u16(os2, 5)                         // usWidthClass
	os2 = u16(os2, 0)                         // fsType: installable
	os2 = append(os2, make([]byte, 20)...)    // sub- and superscript, strikeout
	os2 = i16(os2, 0)                         // sFamilyClass
	os2 = append(os2, make([]byte, 10+16)...) // panose, ulUnicodeRange
	os2 = append(os2, "jamb"...)              // achVendID
	os2 = u16(os2, 0x40)                      // fsSelection: regular
	os2 = u16(os2, 0x20)                      // usFirstCharIndex
	os2 = u16(os2, 0xFFFF)                    // usLastCharIndex
	os2 = i16(os2, ascent)                    // sTypoAscender
	os2 = i16(os2, -descent)                  // sTypoDescender
	os2 = i16(os2, 0)                         // sTypoLineGap
	os2 = u16(os2, uint16(ascent))            // usWinAscent
	os2 = u16(os2, uint16(descent))           // usWinDescent
	os2 = append(os2, make([]byte, 8)...)     // ulCodePageRange
	os2 = append(os2, make([]byte, 4)...)     // sxHeight, sCapHeight
	os2 = u16(u16(u16(os2, 0), 0x20), 1)      // usDefaultChar, usBreakChar, usMaxContext

	post := u32(nil, 0x00030000)
	post = append(post, make([]byte, 28)...)

	tables := []struct {
		tag  string
		data []byte
	}{
		{"OS/2", os2}, {"cmap", cmap}, {"glyf", glyf}, {"head", head}, {"hhea", hhea},
		{"hmtx", hmtx}, {"loca", loca}, {"maxp", maxp}, {"name", name}, {"post", post},
	}
	slices.SortFunc(tables, func(a, b struct {
		tag  string
		data []byte
	}) int {
		return compareTags(a.tag, b.tag)
	})
	n := uint16(len(tables))
	search := uint16(1)
	sel := uint16(0)
	for search*2 <= n {
		search *= 2
		sel++
	}
	out := u32(nil, 0x00010000)
	out = u16(out, n)
	out = u16(out, search*16)
	out = u16(out, sel)
	out = u16(out, n*16-search*16)
	offset := uint32(12 + 16*int(n))
	headAt := 0
	for _, t := range tables {
		out = append(out, t.tag...)
		out = u32(out, checksum(t.data))
		out = u32(out, offset)
		out = u32(out, uint32(len(t.data)))
		offset += uint32((len(t.data) + 3) &^ 3)
	}
	for _, t := range tables {
		if t.tag == "head" {
			headAt = len(out)
		}
		out = append(out, t.data...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	be.PutUint32(out[headAt+8:], 0xB1B0AFBA-checksum(out))
	return out
}

// compareTags orders table tags as a font's table directory does: by their
// bytes.
func compareTags(a, b string) int {
	for i := range 4 {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}

// checksum is the sum of b as big-endian 32-bit words, the last padded with zeros.
func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:])
		sum += binary.BigEndian.Uint32(w[:])
	}
	return sum
}
