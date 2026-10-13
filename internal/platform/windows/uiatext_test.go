//go:build windows

package windows

import (
	"github.com/tyler-builds/jamb/internal/text"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/win32"
)

// asciiText returns the content of an ASCII text on one visual line: one point
// per byte, eight units to a character, and runs of word and other characters.
func asciiText(s string) *access.TextContent {
	c := &access.TextContent{Text: s, Paras: []int{0},
		Lines: []access.TextLine{{Start: 0, Top: 0, Height: 16, Right: float32(len(s)) * 8}}}
	for i := 0; i <= len(s); i++ {
		c.Points = append(c.Points, i)
		c.X = append(c.X, float32(i)*8)
	}
	// ASCII bytes are points, so the word starts are the points.
	c.Words = text.WordStarts(s)
	return c
}

// textBridge returns a bridge with a window and one text field, node 2, that
// holds s. The field has the focus, and its caret is at the start.
func textBridge(s string) *access.Bridge {
	field := &access.Node{ID: 2, Role: access.RoleTextField, Name: "Search", Parent: access.RootID,
		Text: &access.TextInfo{Content: asciiText(s), OriginX: 10, OriginY: 20, Scale: 1,
			Clip: access.Rect{W: 500, H: 500}}}
	root := &access.Node{ID: access.RootID, Role: access.RoleWindow, Children: []access.ID{2}}
	return testTree(2, root, field)
}

// rangeOut calls a method that returns a range (or a SAFEARRAY), and fails the
// test if it does not succeed. The object is released when the test ends.
func rangeOut(t *testing.T, method, obj uintptr, args ...uintptr) uintptr {
	t.Helper()
	r, hr := getOut(method, obj, args...)
	if hr != hrOK || r == 0 {
		t.Fatalf("call failed: HRESULT 0x%08X", uint32(hr))
	}
	t.Cleanup(func() { releaseCall(r) })
	return r
}

// rangeText returns the text of a range object, cut to maxLength units.
func rangeText(t *testing.T, r uintptr, maxLength int32) string {
	t.Helper()
	s, hr := getOut(comMethod(r, 12), r, uintptr(maxLength))
	if hr != hrOK {
		t.Fatalf("GetText failed: HRESULT 0x%08X", uint32(hr))
	}
	out := bstrString(s)
	win32.SysFreeString(s)
	return out
}

// textPatternOf returns the Text pattern of the node 2 of b, and the
// ITextProvider2 view of the same object, which the test releases.
func textPatternOf(t *testing.T, b *access.Bridge) (uintptr, uintptr) {
	t.Helper()
	p := patternOn(t, b, 2, patternText)
	var p2 uintptr
	if hr, _, _ := syscall.SyscallN(comMethod(p, 0), p, uintptr(unsafe.Pointer(&iidTextProvider2)), uintptr(unsafe.Pointer(&p2))); hr != hrOK || p2 == 0 {
		t.Fatalf("the text pattern does not answer ITextProvider2 (HRESULT 0x%08X)", hr)
	}
	t.Cleanup(func() { releaseCall(p2) })
	return p, p2
}

// TestTextIIDsMatchTheSDK checks the text interfaces' IIDs against the uuid
// attributes in UIAutomationCore.idl, as TestPatternIIDsMatchTheSDK does for the
// others.
func TestTextIIDsMatchTheSDK(t *testing.T) {
	cases := []struct {
		name string
		g    guid
		uuid string
	}{
		{"ITextProvider", iidTextProvider, "3589c92c-63f3-4367-99bb-ada653b77cf2"},
		{"ITextProvider2", iidTextProvider2, "0dc5e6ed-3e16-4bf1-8f9a-a979878bc195"},
		{"ITextRangeProvider", iidTextRangeProvider, "5347ad7b-c355-46f8-aff5-909033582f63"},
	}
	for _, c := range cases {
		if got := guidText(c.g); got != c.uuid {
			t.Errorf("%s has IID %s, the SDK has %s", c.name, got, c.uuid)
		}
	}
}

func TestTextPatternOnlyOnTextFields(t *testing.T) {
	b := textBridge("hello")
	textPatternOf(t, b) // answers ITextProvider2 as well as ITextProvider
	b2 := testTree(0, &access.Node{ID: access.RootID, Role: access.RoleWindow, Children: []access.ID{3}},
		&access.Node{ID: 3, Role: access.RoleButton, Name: "Go", Parent: access.RootID})
	testHwnd++
	v := create(kindNode, 3, b2, testHwnd, iidRawElementProviderSimple)
	defer releaseCall(v)
	if got, hr := getOut(comMethod(v, 4), v, patternText); hr != hrOK || got != 0 {
		t.Errorf("a button answered the text pattern: %x, HRESULT 0x%08X", got, uint32(hr))
	}
}

func TestTextRangeReadsItsText(t *testing.T) {
	p, _ := textPatternOf(t, textBridge("hello world"))
	doc := rangeOut(t, comMethod(p, 7), p) // DocumentRange
	if got := rangeText(t, doc, -1); got != "hello world" {
		t.Errorf("document text = %q", got)
	}
	if got := rangeText(t, doc, 5); got != "hello" {
		t.Errorf("GetText(5) = %q, want hello", got)
	}
	if got := rangeText(t, doc, 0); got != "" {
		t.Errorf("GetText(0) = %q, want empty", got)
	}
}

func TestTextCaretRangeIsDegenerateAtTheCaret(t *testing.T) {
	_, p2 := textPatternOf(t, textBridge("hello world"))
	active := newWord()
	out := newWord()
	if hr, _, _ := syscall.SyscallN(comMethod(p2, 10), p2, uintptr(unsafe.Pointer(active)), uintptr(unsafe.Pointer(out))); hr != hrOK || *out == 0 {
		t.Fatalf("GetCaretRange failed: HRESULT 0x%08X", hr)
	}
	caret := *out
	t.Cleanup(func() { releaseCall(caret) })
	if uint32(*active) == 0 {
		t.Errorf("the focused field reports its caret as not active")
	}
	if got := rangeText(t, caret, -1); got != "" {
		t.Errorf("caret range has text %q, want none", got)
	}
}

// TestTextRangeMoveAndSelect moves a degenerate range by words, as a screen
// reader does with Ctrl+Right, reads the word it lands in, finds text, and
// selects it. The runtime receives the selection as byte offsets.
func TestTextRangeMoveAndSelect(t *testing.T) {
	var got access.Request
	b := textBridge("foo bar baz")
	b.SetHooks(nil, func(id access.ID, req access.Request) { got = req })
	p, p2 := textPatternOf(t, b)
	doc := rangeOut(t, comMethod(p, 7), p) // DocumentRange
	active, caret := newWord(), newWord()
	if hr, _, _ := syscall.SyscallN(comMethod(p2, 10), p2, uintptr(unsafe.Pointer(active)), uintptr(unsafe.Pointer(caret))); hr != hrOK {
		t.Fatalf("GetCaretRange failed: HRESULT 0x%08X", hr)
	}
	pos := *caret
	t.Cleanup(func() { releaseCall(pos) })
	moved, hr := getOut(comMethod(pos, 13), pos, uintptr(access.UnitWord), 1) // Move
	if hr != hrOK || int32(moved) != 1 {
		t.Fatalf("Move by a word = %d, HRESULT 0x%08X; want 1", int32(moved), uint32(hr))
	}
	// The caret is now at the start of "bar ", the unit after "foo ", so the unit there is "bar " with its space.
	word := rangeOut(t, comMethod(pos, 3), pos) // Clone
	if hr, _, _ := syscall.SyscallN(comMethod(word, 6), word, uintptr(access.UnitWord)); hr != hrOK {
		t.Fatalf("ExpandToEnclosingUnit failed: HRESULT 0x%08X", hr)
	}
	if got := rangeText(t, word, -1); got != "bar " {
		t.Errorf("the word at the caret = %q, want %q", got, "bar ")
	}
	needle := bstrOf("BAR")
	defer win32.SysFreeString(needle)
	found, hr := getOut(comMethod(doc, 8), doc, needle, 0, 1) // FindText over the document, ignoring case
	if hr != hrOK || found == 0 {
		t.Fatalf("FindText failed: HRESULT 0x%08X", uint32(hr))
	}
	defer releaseCall(found)
	if got := rangeText(t, found, -1); got != "bar" {
		t.Errorf("FindText found %q, want bar", got)
	}
	if hr, _, _ := syscall.SyscallN(comMethod(found, 16), found); hr != hrOK { // Select
		t.Fatalf("Select failed: HRESULT 0x%08X", hr)
	}
	if got.Action != access.ActionSetTextSelection || got.Anchor != 4 || got.Caret != 7 {
		t.Errorf("Select requested %+v, want the bytes 4 to 7", got)
	}
}

func TestTextRangeBoundsAndAttributes(t *testing.T) {
	p, _ := textPatternOf(t, textBridge("hi there"))
	doc := rangeOut(t, comMethod(p, 7), p)
	psa, hr := getOut(comMethod(doc, 10), doc) // GetBoundingRectangles
	if hr != hrOK || psa == 0 {
		t.Fatalf("GetBoundingRectangles failed: HRESULT 0x%08X", uint32(hr))
	}
	defer win32.SafeArrayDestroy(psa)
	if ub := win32.SafeArrayUBound(psa); ub != 3 {
		t.Fatalf("bounds array upper bound %d, want 3 (one rectangle)", ub)
	}
	data := newWord()
	win32.SafeArrayAccessData(psa, data)
	vals := unsafe.Slice((*float64)(ptr(*data)), 4)
	want := []float64{10, 20, 64, 16} // the origin of the field, and the eight characters of eight units
	for i := range want {
		if vals[i] != want[i] {
			t.Errorf("rectangle value %d = %v, want %v", i, vals[i], want[i])
		}
	}
	win32.SafeArrayUnaccessData(psa)

	// IsReadOnly is a VT_BOOL that the field reports as false.
	v := newVariant()
	if hr, _, _ := syscall.SyscallN(comMethod(doc, 9), doc, attrIsReadOnly, uintptr(unsafe.Pointer(v))); hr != hrOK || v.vt != vtBool || variantBoolOf(*v) != variantOff {
		t.Errorf("IsReadOnly: HRESULT 0x%08X, type %d; want S_OK with VT_BOOL false", hr, v.vt)
	}
	// An attribute the field does not have is NotSupported: a reserved VARIANT of
	// VT_UNKNOWN, not an error.
	v = newVariant()
	if hr, _, _ := syscall.SyscallN(comMethod(doc, 9), doc, 99999, uintptr(unsafe.Pointer(v))); hr != hrOK || v.vt != vtUnknown {
		t.Errorf("unknown attribute: HRESULT 0x%08X, type %d; want S_OK with VT_UNKNOWN", hr, v.vt)
	}
}

func TestDegenerateRangeHasNoBounds(t *testing.T) {
	_, p2 := textPatternOf(t, textBridge("hello"))
	active, caret := newWord(), newWord()
	syscall.SyscallN(comMethod(p2, 10), p2, uintptr(unsafe.Pointer(active)), uintptr(unsafe.Pointer(caret)))
	c := *caret
	t.Cleanup(func() { releaseCall(c) })
	psa, _ := getOut(comMethod(c, 10), c)
	defer win32.SafeArrayDestroy(psa)
	if ub := win32.SafeArrayUBound(psa); ub != -1 {
		t.Errorf("a degenerate range has bounds with upper bound %d, want none", ub)
	}
}

// TestTextRangesAreFreed makes and releases range objects in a loop, and checks
// that the registry holds no more objects afterwards, so each one was freed.
func TestTextRangesAreFreed(t *testing.T) {
	p, _ := textPatternOf(t, textBridge("hello"))
	before := registrySize()
	for i := 0; i < 500; i++ {
		doc, hr := getOut(comMethod(p, 7), p)
		if hr != hrOK {
			t.Fatalf("DocumentRange failed: HRESULT 0x%08X", uint32(hr))
		}
		clone, hr := getOut(comMethod(doc, 3), doc)
		if hr != hrOK {
			t.Fatalf("Clone failed: HRESULT 0x%08X", uint32(hr))
		}
		syscall.SyscallN(comMethod(clone, 6), clone, uintptr(access.UnitLine)) // Expand
		releaseCall(clone)
		releaseCall(doc)
	}
	if after := registrySize(); after != before {
		t.Errorf("registry holds %d objects after the ranges were released, had %d", after, before)
	}
}

func TestTextRangeChildrenAndUnsupportedCalls(t *testing.T) {
	p, _ := textPatternOf(t, textBridge("abc"))
	doc := rangeOut(t, comMethod(p, 7), p)
	if hr, _, _ := syscall.SyscallN(comMethod(doc, 17), doc); uint32(hr) != hrNotSupported { // AddToSelection
		t.Errorf("AddToSelection HRESULT 0x%08X, want not supported", uint32(hr))
	}
	kids, _ := getOut(comMethod(doc, 20), doc) // GetChildren
	defer win32.SafeArrayDestroy(kids)
	if ub := win32.SafeArrayUBound(kids); ub != -1 {
		t.Errorf("children upper bound %d, want an empty array", ub)
	}
}

// TestBSTRHoldsUTF16 checks that a BSTR made in place holds the same UTF-16 as the
// string, including a surrogate pair, an empty string, and invalid UTF-8.
func TestBSTRHoldsUTF16(t *testing.T) {
	for _, s := range []string{"", "plain", "a\U0001F600b", "\xffx"} {
		b := bstrOf(s)
		got := wideString(b)
		want := string(utf16.Decode(utf16.Encode([]rune(s))))
		win32.SysFreeString(b)
		if got != want {
			t.Errorf("bstrOf(%q) holds %q, want %q", s, got, want)
		}
	}
}

// TestRangeArrayOwnsItsRanges checks the ownership of the SAFEARRAY that
// GetSelection returns. The array takes its own reference to each range, so the
// ranges survive the release of the references made to fill it, and they are freed
// once the array is destroyed.
func TestRangeArrayOwnsItsRanges(t *testing.T) {
	b := textBridge("abc")
	view := cachedObject(kindText, 2, b, 0x7f22, iidTextProvider)
	defer releaseCall(view)
	c := coreOf(view)
	before := registrySize()
	info := b.Tree().Node(2).Text
	psa := rangeArray(c, []access.TextRange{info.Whole()})
	if psa == 0 {
		t.Fatal("no array")
	}
	elem := newWord()
	if hr := win32.SafeArrayGetUnknown(psa, 0, elem); hr != hrOK || *elem == 0 {
		t.Fatalf("element 0: HRESULT 0x%08X", uint32(hr))
	}
	// The range is alive after the release of the array's build-time references:
	// AddRef on a freed object would return 0.
	if n, _, _ := syscall.SyscallN(comMethod(*elem, 1), *elem); n == 0 {
		t.Fatal("the range was freed while the array still holds it")
	}
	releaseCall(*elem) // the reference SafeArrayGetUnknown made
	releaseCall(*elem) // the reference the AddRef above made
	win32.SafeArrayDestroy(psa)
	if after := registrySize(); after != before {
		t.Errorf("registry holds %d objects after the array was destroyed, had %d", after, before)
	}
}
