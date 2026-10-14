package rope

import (
	"bytes"
	"errors"
	"io"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// modelPoint returns the point of byte offset off in the plain byte model m.
func modelPoint(m []byte, off int) Point {
	return Point{
		Row:    bytes.Count(m[:off], lineFeed),
		Column: off - (bytes.LastIndexByte(m[:off], '\n') + 1),
	}
}

// modelLineStarts returns the start offset of every line of m.
func modelLineStarts(m []byte) []int {
	starts := []int{0}
	for i, c := range m {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// modelUnits returns the UTF-16 length of m, counted by decoding runes. It is an
// independent reference for utf16Units.
func modelUnits(m []byte) int {
	n := 0
	for _, c := range string(m) {
		if c >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// modelOffset returns the byte offset of p in m, following the rules of
// OffsetOf: the columns of a row run to its line feed, inclusive, and the last
// row runs to the end of m.
func modelOffset(m []byte, p Point) (int, error) {
	starts := modelLineStarts(m)
	if p.Row < 0 || p.Row >= len(starts) || p.Column < 0 {
		return 0, ErrRange
	}
	end := len(m)
	if p.Row+1 < len(starts) {
		end = starts[p.Row+1] - 1
	}
	if p.Column > end-starts[p.Row] {
		return 0, ErrRange
	}
	off := starts[p.Row] + p.Column
	if off < len(m) && m[off]&0xC0 == 0x80 {
		return 0, ErrBoundary
	}
	return off, nil
}

// modelBoundaries returns every valid edit offset of m, including len(m).
func modelBoundaries(m []byte) []int {
	var b []int
	for i := 0; i <= len(m); i++ {
		if i == len(m) || m[i]&0xC0 != 0x80 {
			b = append(b, i)
		}
	}
	return b
}

// testPieces are fragments of valid UTF-8 that cover one, two, three, and four
// byte sequences, and both line break forms.
var testPieces = []string{"a", "bc", " ", "\n", "\r\n", "\r", "é", "€", "𝄞", "\t", "xyz\n"}

// randomText returns a random valid UTF-8 string, occasionally large enough to
// span several leaves.
func randomText(rng *rand.Rand) []byte {
	n := rng.IntN(20)
	if rng.IntN(15) == 0 {
		n = 500 + rng.IntN(3500)
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(testPieces[rng.IntN(len(testPieces))])
	}
	return []byte(sb.String())
}

// checkNode verifies the structure of the subtree at n and returns its
// aggregates. Leaves must hold at least leafMin bytes; pass 0 for a lone leaf.
func checkNode(t *testing.T, n *node, leafMin int) (bytesN, lines, units, h int) {
	t.Helper()
	if n == nil {
		return 0, 0, 0, 0
	}
	if n.isLeaf() {
		if len(n.data) == 0 || len(n.data) > maxLeaf || len(n.data) < leafMin {
			t.Fatalf("leaf of %d bytes outside [%d, %d]", len(n.data), leafMin, maxLeaf)
		}
		if !utf8.Valid(n.data) {
			t.Fatalf("leaf is not valid UTF-8")
		}
		if n.height != 1 || n.bytes != len(n.data) || n.lines != bytes.Count(n.data, lineFeed) || n.units != modelUnits(n.data) {
			t.Fatalf("leaf aggregates are wrong")
		}
		return n.bytes, n.lines, n.units, 1
	}
	if n.right == nil {
		t.Fatalf("internal node has a nil child")
	}
	lb, ll, lu, lh := checkNode(t, n.left, leafMin)
	rb, rl, ru, rh := checkNode(t, n.right, leafMin)
	if d := lh - rh; d < -1 || d > 1 {
		t.Fatalf("AVL balance violated: heights %d and %d", lh, rh)
	}
	if n.height != max(lh, rh)+1 || n.bytes != lb+rb || n.lines != ll+rl || n.units != lu+ru {
		t.Fatalf("internal aggregates are wrong")
	}
	return n.bytes, n.lines, n.units, n.height
}

// checkTree verifies the structure of the whole tree of r.
func checkTree(t *testing.T, root *node) {
	t.Helper()
	if root == nil {
		return
	}
	leafMin := minLeaf
	if root.isLeaf() {
		leafMin = 0
	}
	checkNode(t, root, leafMin)
}

// checkRope compares r with the byte model m. Every step checks a sample of
// offsets and lines. Every 25th step checks all of them.
func checkRope(t *testing.T, step int, r *Rope, m []byte, rng *rand.Rand) {
	t.Helper()
	full := step%25 == 0
	if r.Len() != len(m) {
		t.Fatalf("step %d: Len = %d, want %d", step, r.Len(), len(m))
	}
	if r.String() != string(m) {
		t.Fatalf("step %d: text differs from the model", step)
	}
	checkTree(t, r.root)

	starts := modelLineStarts(m)
	if r.LineCount() != len(starts) {
		t.Fatalf("step %d: LineCount = %d, want %d", step, r.LineCount(), len(starts))
	}
	lines := len(starts)
	samples := 16
	if full {
		samples = lines
	}
	for i := 0; i < samples; i++ {
		line := i
		if !full {
			line = rng.IntN(lines)
		}
		if got := r.LineStart(line); got != starts[line] {
			t.Fatalf("step %d: LineStart(%d) = %d, want %d", step, line, got, starts[line])
		}
	}

	if r.UTF16Len() != modelUnits(m) {
		t.Fatalf("step %d: UTF16Len = %d, want %d", step, r.UTF16Len(), modelUnits(m))
	}
	bounds := modelBoundaries(m)
	for i := 0; i < 32; i++ {
		off := rng.IntN(len(m) + 1)
		if got := r.PointAt(off); got != modelPoint(m, off) {
			t.Fatalf("step %d: PointAt(%d) = %+v, want %+v", step, off, got, modelPoint(m, off))
		}
		b := bounds[rng.IntN(len(bounds))]
		u := r.ByteToUTF16(b)
		if u != modelUnits(m[:b]) {
			t.Fatalf("step %d: ByteToUTF16(%d) = %d, want %d", step, b, u, modelUnits(m[:b]))
		}
		back, err := r.UTF16ToByte(u)
		if err != nil || back != b {
			t.Fatalf("step %d: UTF16ToByte(%d) = %d, %v, want %d", step, u, back, err, b)
		}
		from := bounds[rng.IntN(len(bounds))]
		to := from + rng.IntN(len(m)-from+1)
		if got := r.Slice(from, to); !bytes.Equal(got, m[from:to]) {
			t.Fatalf("step %d: Slice(%d, %d) differs from the model", step, from, to)
		}
		// Point to offset, for random points that may be valid or invalid.
		p := Point{Row: rng.IntN(lines+2) - 1, Column: rng.IntN(16) - 1}
		wantOff, wantErr := modelOffset(m, p)
		if gotOff, gotErr := r.OffsetOf(p); gotOff != wantOff || gotErr != wantErr {
			t.Fatalf("step %d: OffsetOf(%+v) = %d, %v, want %d, %v", step, p, gotOff, gotErr, wantOff, wantErr)
		}
		// Offset to point and back: boundaries round-trip, and other offsets
		// report ErrBoundary.
		off = rng.IntN(len(m) + 1)
		isBoundary := off == len(m) || m[off]&0xC0 != 0x80
		rt, rtErr := r.OffsetOf(r.PointAt(off))
		if isBoundary && (rtErr != nil || rt != off) || !isBoundary && rtErr != ErrBoundary {
			t.Fatalf("step %d: round trip of %d = %d, %v", step, off, rt, rtErr)
		}
	}
	if full {
		var got []byte
		for c := range r.Chunks(0) {
			got = append(got, c...)
		}
		if !bytes.Equal(got, m) {
			t.Fatalf("step %d: Chunks differ from the model", step)
		}
		data, err := io.ReadAll(r.Reader(0))
		if err != nil || !bytes.Equal(data, m) {
			t.Fatalf("step %d: Reader differs from the model: %v", step, err)
		}
		// Every character boundary maps to its UTF-16 offset and back. The
		// running count is built incrementally, and long texts are sampled at a
		// fixed stride to bound the cost of the check.
		stride := 1 + len(bounds)/2000
		units, prev := 0, 0
		for i, b := range bounds {
			units += modelUnits(m[prev:b])
			prev = b
			if i%stride != 0 {
				continue
			}
			if got := r.ByteToUTF16(b); got != units {
				t.Fatalf("step %d: ByteToUTF16(%d) = %d, want %d", step, b, got, units)
			}
			if q, err := r.OffsetOf(r.PointAt(b)); err != nil || q != b {
				t.Fatalf("step %d: OffsetOf(PointAt(%d)) = %d, %v", step, b, q, err)
			}
			if u, err := r.UTF16ToByte(units); err != nil || u != b {
				t.Fatalf("step %d: UTF16ToByte(%d) = %d, %v, want %d", step, units, u, err, b)
			}
		}
	}
}

// TestRandomEdits applies random inserts, deletes, and replacements, mixing
// multi-byte text and line breaks, and compares every result with a plain byte
// model. It also checks the edit records against the points computed from the
// model.
func TestRandomEdits(t *testing.T) {
	for seed := uint64(1); seed <= 3; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
		r, err := New(nil)
		if err != nil {
			t.Fatal(err)
		}
		var model []byte
		for step := 1; step <= 1500; step++ {
			bounds := modelBoundaries(model)
			a := rng.IntN(len(bounds))
			b := a
			if rng.IntN(2) == 0 {
				b = a + rng.IntN(min(len(bounds)-a, 4000))
			}
			from, to := bounds[a], bounds[b]
			var ins []byte
			if rng.IntN(5) != 0 && len(model) < 150000 {
				ins = randomText(rng)
			}
			// Cycle through the three entry points. Insert and Delete are Replace
			// with one side empty, so each must produce the same edit record.
			mode := rng.IntN(4)
			if mode == 0 {
				to = from
			}
			if mode == 1 {
				ins = nil
			}
			next := make([]byte, 0, len(model)-(to-from)+len(ins))
			next = append(next, model[:from]...)
			next = append(next, ins...)
			next = append(next, model[to:]...)

			var edit Edit
			var err error
			switch mode {
			case 0:
				edit, err = r.Insert(from, ins)
			case 1:
				edit, err = r.Delete(from, to)
			default:
				edit, err = r.Replace(from, to, ins)
			}
			if err != nil {
				t.Fatalf("seed %d step %d: replace(%d, %d): %v", seed, step, from, to, err)
			}
			want := Edit{
				StartByte:   from,
				OldEndByte:  to,
				NewEndByte:  from + len(ins),
				StartPoint:  modelPoint(model, from),
				OldEndPoint: modelPoint(model, to),
				NewEndPoint: modelPoint(next, from+len(ins)),
			}
			if edit != want {
				t.Fatalf("seed %d step %d: edit = %+v, want %+v", seed, step, edit, want)
			}
			model = next
			checkRope(t, step, r, model, rng)
		}
	}
}

// TestEmptyRope covers the zero value and the empty text.
func TestEmptyRope(t *testing.T) {
	var r Rope
	if r.Len() != 0 || r.LineCount() != 1 || r.UTF16Len() != 0 || r.String() != "" {
		t.Fatalf("zero rope is not empty")
	}
	if r.LineStart(0) != 0 || r.PointAt(0) != (Point{}) || !r.IsBoundary(0) {
		t.Fatalf("zero rope has wrong line or boundary state")
	}
	for range r.Chunks(0) {
		t.Fatalf("empty rope yields a chunk")
	}
	if _, err := r.Insert(0, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Delete(0, 3); err != nil {
		t.Fatal(err)
	}
	if r.root != nil || r.Len() != 0 {
		t.Fatalf("deleting all text did not return to the empty state")
	}
}

// TestErrors checks the rejected arguments of Insert, Delete, New, and the
// UTF-16 conversion. A rejected edit must leave the rope unchanged.
func TestErrors(t *testing.T) {
	r, err := NewString("aé𝄞b")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		err  error
		run  func() error
	}{
		{"insert before start", ErrRange, func() error { _, e := r.Insert(-1, []byte("x")); return e }},
		{"insert past end", ErrRange, func() error { _, e := r.Insert(r.Len()+1, []byte("x")); return e }},
		{"delete reversed", ErrRange, func() error { _, e := r.Delete(3, 2); return e }},
		{"delete past end", ErrRange, func() error { _, e := r.Delete(0, r.Len()+1); return e }},
		{"insert inside a sequence", ErrBoundary, func() error { _, e := r.Insert(2, []byte("x")); return e }},
		{"delete starting inside a sequence", ErrBoundary, func() error { _, e := r.Delete(2, 3); return e }},
		{"delete ending inside a sequence", ErrBoundary, func() error { _, e := r.Delete(0, 2); return e }},
		{"invalid UTF-8 insert", ErrInvalidUTF8, func() error { _, e := r.Insert(0, []byte{0xff}); return e }},
		{"truncated sequence insert", ErrInvalidUTF8, func() error { _, e := r.Insert(0, []byte{0xC3}); return e }},
		{"UTF-16 offset negative", ErrRange, func() error { _, e := r.UTF16ToByte(-1); return e }},
		{"UTF-16 offset past end", ErrRange, func() error { _, e := r.UTF16ToByte(6); return e }},
		{"UTF-16 offset inside a surrogate pair", ErrBoundary, func() error { _, e := r.UTF16ToByte(3); return e }},
		{"replace with reversed range", ErrRange, func() error { _, e := r.Replace(3, 2, []byte("x")); return e }},
		{"replace with range past end", ErrRange, func() error { _, e := r.Replace(0, r.Len()+1, nil); return e }},
		{"replace starting inside a sequence", ErrBoundary, func() error { _, e := r.Replace(2, 7, []byte("x")); return e }},
		{"replace ending inside a sequence", ErrBoundary, func() error { _, e := r.Replace(0, 4, []byte("x")); return e }},
		{"replace with invalid UTF-8", ErrInvalidUTF8, func() error { _, e := r.Replace(0, 1, []byte{0xff}); return e }},
		{"point with row past last", ErrRange, func() error { _, e := r.OffsetOf(Point{Row: 1, Column: 0}); return e }},
		{"point with negative column", ErrRange, func() error { _, e := r.OffsetOf(Point{Row: 0, Column: -1}); return e }},
		{"point past line", ErrRange, func() error { _, e := r.OffsetOf(Point{Row: 0, Column: 9}); return e }},
		{"point inside a sequence", ErrBoundary, func() error { _, e := r.OffsetOf(Point{Row: 0, Column: 2}); return e }},
	}
	for _, c := range cases {
		if err := c.run(); !errors.Is(err, c.err) {
			t.Errorf("%s: error = %v, want %v", c.name, err, c.err)
		}
	}
	if r.String() != "aé𝄞b" {
		t.Fatalf("rejected edits changed the text to %q", r.String())
	}
	if _, err := New([]byte{'a', 0xff}); !errors.Is(err, ErrInvalidUTF8) {
		t.Errorf("New with invalid UTF-8: error = %v", err)
	}
	if _, err := NewString("a\xff"); !errors.Is(err, ErrInvalidUTF8) {
		t.Errorf("NewString with invalid UTF-8: error = %v", err)
	}
}

// TestOutOfRangeReadsPanic checks that read methods panic on programmer error,
// as slice indexing does.
func TestOutOfRangeReadsPanic(t *testing.T) {
	r, _ := NewString("abc\n")
	mustPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s did not panic", name)
			}
		}()
		f()
	}
	mustPanic("Slice past end", func() { r.Slice(0, 5) })
	mustPanic("Slice reversed", func() { r.Slice(2, 1) })
	mustPanic("PointAt past end", func() { r.PointAt(5) })
	mustPanic("LineStart past last line", func() { r.LineStart(2) })
	mustPanic("Chunks past end", func() { r.Chunks(5) })
	mustPanic("ByteToUTF16 inside a sequence", func() {
		s, _ := NewString("é")
		s.ByteToUTF16(1)
	})
}

// TestLineBreaks checks the CRLF rule: the \r belongs to the break and counts
// toward the column of its line, and a lone \r is ordinary text.
func TestLineBreaks(t *testing.T) {
	cases := []struct {
		text   string
		lines  int
		points map[int]Point
	}{
		{"a\r\nb\r\n", 3, map[int]Point{0: {0, 0}, 1: {0, 1}, 2: {0, 2}, 3: {1, 0}, 6: {2, 0}}},
		{"a\rb\n", 2, map[int]Point{2: {0, 2}, 4: {1, 0}}},
		{"", 1, map[int]Point{0: {0, 0}}},
		{"\n\n", 3, map[int]Point{0: {0, 0}, 1: {1, 0}, 2: {2, 0}}},
	}
	for _, c := range cases {
		r, err := NewString(c.text)
		if err != nil {
			t.Fatal(err)
		}
		if r.LineCount() != c.lines {
			t.Errorf("%q: LineCount = %d, want %d", c.text, r.LineCount(), c.lines)
		}
		for off, want := range c.points {
			if got := r.PointAt(off); got != want {
				t.Errorf("%q: PointAt(%d) = %+v, want %+v", c.text, off, got, want)
			}
		}
	}
}

// TestEditRecordFields checks the edit record for an insert with line breaks
// and for a delete that spans a line break.
func TestEditRecordFields(t *testing.T) {
	r, _ := NewString("ab\ncd\nef")
	e, err := r.Insert(1, []byte("x\ny"))
	if err != nil {
		t.Fatal(err)
	}
	want := Edit{
		StartByte: 1, OldEndByte: 1, NewEndByte: 4,
		StartPoint: Point{0, 1}, OldEndPoint: Point{0, 1}, NewEndPoint: Point{1, 1},
	}
	if e != want {
		t.Fatalf("insert edit = %+v, want %+v", e, want)
	}
	e, err = r.Delete(2, 7)
	if err != nil {
		t.Fatal(err)
	}
	want = Edit{
		StartByte: 2, OldEndByte: 7, NewEndByte: 2,
		StartPoint: Point{0, 2}, OldEndPoint: Point{2, 1}, NewEndPoint: Point{0, 2},
	}
	if e != want {
		t.Fatalf("delete edit = %+v, want %+v", e, want)
	}
	if r.String() != "axd\nef" {
		t.Fatalf("text = %q", r.String())
	}
}

// TestOffsetOfEdges checks point-to-offset conversion at line feeds, CRLF
// breaks, the end of the text, and the rows and columns that are out of range.
func TestOffsetOfEdges(t *testing.T) {
	r, _ := NewString("a\r\nb\n")
	valid := map[Point]int{
		{0, 0}: 0, {0, 1}: 1, {0, 2}: 2, // a, the '\r', the '\n'
		{1, 0}: 3, {1, 1}: 4, // b, the '\n'
		{2, 0}: 5, // empty last row, at the end of the text
	}
	for p, want := range valid {
		if got, err := r.OffsetOf(p); err != nil || got != want {
			t.Errorf("OffsetOf(%+v) = %d, %v, want %d", p, got, err, want)
		}
	}
	for _, p := range []Point{{2, 1}, {3, 0}, {-1, 0}, {0, 3}, {1, 2}} {
		if _, err := r.OffsetOf(p); err != ErrRange {
			t.Errorf("OffsetOf(%+v) error = %v, want ErrRange", p, err)
		}
	}
	var empty Rope
	if off, err := empty.OffsetOf(Point{}); err != nil || off != 0 {
		t.Errorf("empty OffsetOf(0,0) = %d, %v", off, err)
	}
}

// TestReplaceEdit checks that Replace returns one record for a combined delete and
// insert, and that its points match the text.
func TestReplaceEdit(t *testing.T) {
	r, _ := NewString("ab\ncd\nef")
	e, err := r.Replace(1, 6, []byte("X\nY"))
	if err != nil {
		t.Fatal(err)
	}
	want := Edit{
		StartByte: 1, OldEndByte: 6, NewEndByte: 4,
		StartPoint: Point{0, 1}, OldEndPoint: Point{2, 0}, NewEndPoint: Point{1, 1},
	}
	if e != want {
		t.Fatalf("Replace edit = %+v, want %+v", e, want)
	}
	if r.String() != "aX\nYef" {
		t.Fatalf("text = %q", r.String())
	}
}

// TestUTF16Surrogates checks conversions around a four-byte character.
func TestUTF16Surrogates(t *testing.T) {
	r, _ := NewString("a𝄞b")
	if r.UTF16Len() != 4 {
		t.Fatalf("UTF16Len = %d, want 4", r.UTF16Len())
	}
	for off, want := range map[int]int{0: 0, 1: 1, 5: 3, 6: 4} {
		if got := r.ByteToUTF16(off); got != want {
			t.Errorf("ByteToUTF16(%d) = %d, want %d", off, got, want)
		}
	}
	for u, want := range map[int]int{0: 0, 1: 1, 3: 5, 4: 6} {
		if got, err := r.UTF16ToByte(u); err != nil || got != want {
			t.Errorf("UTF16ToByte(%d) = %d, %v, want %d", u, got, err, want)
		}
	}
}

// TestSnapshotIsolation keeps snapshots taken at several points of an edit
// history and checks that each one still reads as it did when taken.
func TestSnapshotIsolation(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	r, _ := NewString(strings.Repeat("hello wörld\r\n€ 𝄞\n", 2000))
	type version struct {
		snap  Text
		text  string
		lines int
		units int
	}
	var versions []version
	for i := 0; i < 400; i++ {
		if i%20 == 0 {
			versions = append(versions, version{r.Snapshot(), r.String(), r.LineCount(), r.UTF16Len()})
		}
		randomEdit(t, rng, r)
	}
	for i, v := range versions {
		if v.snap.String() != v.text || v.snap.LineCount() != v.lines || v.snap.UTF16Len() != v.units {
			t.Fatalf("snapshot %d changed after later edits", i)
		}
		checkTree(t, v.snap.root)
	}
}

// TestSnapshotConcurrentReads reads a snapshot from several goroutines while the
// rope is edited. Run with -race, this checks that snapshot reads do not share
// mutable state with edits.
func TestSnapshotConcurrentReads(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	r, _ := NewString(strings.Repeat("line of text é\n", 3000))
	snap := r.Snapshot()
	want := snap.String()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				if snap.String() != want {
					t.Error("concurrent snapshot read saw changed text")
					return
				}
				_ = snap.LineStart(snap.LineCount() - 1)
				_ = snap.ByteToUTF16(snap.Len() / 2)
			}
		}()
	}
	for i := 0; i < 300; i++ {
		randomEdit(t, rng, r)
	}
	wg.Wait()
}

// randomEdit applies one random insert or delete through the public API.
func randomEdit(t *testing.T, rng *rand.Rand, r *Rope) {
	t.Helper()
	m := r.Bytes()
	bounds := modelBoundaries(m)
	a := rng.IntN(len(bounds))
	if rng.IntN(2) == 0 {
		if _, err := r.Insert(bounds[a], randomText(rng)); err != nil {
			t.Fatal(err)
		}
		return
	}
	b := min(a+rng.IntN(200), len(bounds)-1)
	if _, err := r.Delete(bounds[a], bounds[b]); err != nil {
		t.Fatal(err)
	}
}

// TestChunksAndReader checks iteration from several offsets, including offsets
// on leaf boundaries and the end of the text.
func TestChunksAndReader(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 9))
	var sb strings.Builder
	for sb.Len() < 300000 {
		sb.WriteString(testPieces[rng.IntN(len(testPieces))])
	}
	r, err := NewString(sb.String())
	if err != nil {
		t.Fatal(err)
	}
	checkTree(t, r.root)
	text := sb.String()
	for _, from := range []int{0, 1, 511, 1024, len(text) - 1, len(text)} {
		var got []byte
		for c := range r.Chunks(from) {
			if len(c) == 0 {
				t.Fatalf("Chunks(%d) yielded an empty chunk", from)
			}
			got = append(got, c...)
		}
		if string(got) != text[from:] {
			t.Fatalf("Chunks(%d) differs from the text", from)
		}
		data, err := io.ReadAll(r.Reader(from))
		if err != nil || string(data) != text[from:] {
			t.Fatalf("Reader(%d) differs from the text: %v", from, err)
		}
	}
}

// TestBuildAndSmallLeaves checks the leaf size invariants after a large build
// and after many small edits near a leaf boundary.
func TestBuildAndSmallLeaves(t *testing.T) {
	r, err := New(bytes.Repeat([]byte("abcdefghij\n"), 300000))
	if err != nil {
		t.Fatal(err)
	}
	checkTree(t, r.root)
	if r.LineCount() != 300001 {
		t.Fatalf("LineCount = %d", r.LineCount())
	}
	rng := rand.New(rand.NewPCG(4, 4))
	for i := 0; i < 2000; i++ {
		off := rng.IntN(r.Len())
		for !r.IsBoundary(off) {
			off++
		}
		if _, err := r.Insert(off, []byte("é")); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Delete(off, off+2); err != nil {
			t.Fatal(err)
		}
	}
	checkTree(t, r.root)
}
