package rope

import (
	"bytes"
	"sync"
	"testing"
	"unicode/utf8"
)

// Sizes of the benchmark texts. The 100 MB case is about 100,000,000 bytes and
// the 2.6 MB case is 2,600,000 bytes.
const (
	benchLarge = 100_000_000
	benchSmall = 2_600_000
)

// sink keeps benchmark results live so the compiler cannot discard the calls.
var (
	sinkInt   int
	sinkPoint Point
)

// benchData returns about n bytes of deterministic text. The text is source-like,
// with lines of varying length and some two-, three-, and four-byte characters.
// It ends on a line feed, so it is always valid UTF-8.
func benchData(n int) []byte {
	lines := []string{
		"package rope // a persistent rope\n",
		"\tx := \"é€𝄞\" + y // multi-byte text\r\n",
		"func (t Text) Len() int { return t.root.bytes }\n",
		"// Lines end at \\n, and CRLF is one break.\n",
		"\n",
	}
	buf := make([]byte, 0, n+128)
	for i := 0; len(buf) < n; i++ {
		buf = append(buf, lines[i%len(lines)]...)
	}
	return buf[:bytes.LastIndexByte(buf[:n], '\n')+1]
}

// benchRopes caches read-only ropes for the read benchmarks, keyed by size.
var (
	benchMu    sync.Mutex
	benchRopes = map[int]*Rope{}
)

// benchText returns a snapshot of a shared rope of about size bytes. The rope is
// built once and must not be edited by benchmarks.
func benchText(b *testing.B, size int) Text {
	b.Helper()
	benchMu.Lock()
	defer benchMu.Unlock()
	if r, ok := benchRopes[size]; ok {
		return r.Snapshot()
	}
	r, err := New(benchData(size))
	if err != nil {
		b.Fatal(err)
	}
	benchRopes[size] = r
	return r.Snapshot()
}

// midBoundary returns a character boundary near the middle of t.
func midBoundary(t Text) int {
	off := t.Len() / 2
	for !t.IsBoundary(off) {
		off++
	}
	return off
}

// sampleBoundaries returns k pseudo-random character boundaries of t. They are
// computed before timing starts.
func sampleBoundaries(t Text, k int) []int {
	out := make([]int, k)
	for i := range out {
		off := (i*2654435761 + 12345) % (t.Len() + 1)
		for !t.IsBoundary(off) {
			off++
		}
		out[i] = off
	}
	return out
}

func benchBuild(b *testing.B, size int) {
	data := benchData(size)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := New(data)
		if err != nil {
			b.Fatal(err)
		}
		sinkInt = r.Len()
	}
}

func benchInsertMid(b *testing.B, size int) {
	r, err := New(benchData(size))
	if err != nil {
		b.Fatal(err)
	}
	mid := midBoundary(r.Snapshot())
	one := []byte{'x'}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e, err := r.Insert(mid, one)
		if err != nil {
			b.Fatal(err)
		}
		sinkInt = e.NewEndByte
	}
}

func benchLineStart(b *testing.B, size int) {
	t := benchText(b, size)
	lines := t.LineCount()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkInt = t.LineStart((i * 2654435761) % lines)
	}
}

func benchPointAt(b *testing.B, size int) {
	t := benchText(b, size)
	offs := sampleBoundaries(t, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkPoint = t.PointAt(offs[i%len(offs)])
	}
}

func benchByteToUTF16(b *testing.B, size int) {
	t := benchText(b, size)
	offs := sampleBoundaries(t, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkInt = t.ByteToUTF16(offs[i%len(offs)])
	}
}

func benchIterate(b *testing.B, size int) {
	t := benchText(b, size)
	nl := []byte{'\n'}
	b.SetBytes(int64(t.Len()))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := 0
		// Counting line feeds reads every byte, as a parser would.
		for c := range t.Chunks(0) {
			total += bytes.Count(c, nl)
		}
		sinkInt = total
	}
}

func BenchmarkBuild100MB(b *testing.B)   { benchBuild(b, benchLarge) }
func BenchmarkBuild2_6MB(b *testing.B)   { benchBuild(b, benchSmall) }
func BenchmarkInsert100MB(b *testing.B)  { benchInsertMid(b, benchLarge) }
func BenchmarkInsert2_6MB(b *testing.B)  { benchInsertMid(b, benchSmall) }
func BenchmarkLine100MB(b *testing.B)    { benchLineStart(b, benchLarge) }
func BenchmarkLine2_6MB(b *testing.B)    { benchLineStart(b, benchSmall) }
func BenchmarkPoint100MB(b *testing.B)   { benchPointAt(b, benchLarge) }
func BenchmarkPoint2_6MB(b *testing.B)   { benchPointAt(b, benchSmall) }
func BenchmarkUTF16100MB(b *testing.B)   { benchByteToUTF16(b, benchLarge) }
func BenchmarkUTF162_6MB(b *testing.B)   { benchByteToUTF16(b, benchSmall) }
func BenchmarkIterate100MB(b *testing.B) { benchIterate(b, benchLarge) }
func BenchmarkIterate2_6MB(b *testing.B) { benchIterate(b, benchSmall) }

// TestBenchDataIsValid guards the benchmark fixture, since the benchmarks do not
// check their input.
func TestBenchDataIsValid(t *testing.T) {
	data := benchData(100000)
	if !utf8.Valid(data) || len(data) < 99000 || data[len(data)-1] != '\n' {
		t.Fatalf("benchmark fixture is invalid or too short: %d bytes", len(data))
	}
}
