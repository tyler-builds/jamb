//go:build windows

package windows

import (
	"syscall"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/win32"
)

// The Text pattern of a text field or text area is an ITextProvider object for
// the node, and its ranges are ITextRangeProvider objects, which UI Automation
// creates through the calls below. Both are COM objects in the registry (see
// uiacom.go), so they follow the same rules as the other objects: the range
// objects are made per call, reference counted, and freed when the last
// reference is released.
//
// Semantics follow Microsoft's documentation of the Text pattern. The rules that
// matter here are cited at each method; the access package implements them on
// the text, in points (see access.TextRange).
//
// A range keeps the snapshot it was made from. Its text and its geometry are the
// ones that were reported to the client, even if the field has changed since, and
// Select acts on the current text. The provider and its ranges never read the
// runtime: they read the snapshot, which is immutable.

// Text pattern identifiers, from UIAutomationClient.h.
const (
	patternText  = 10014 // UIA_TextPatternId
	patternText2 = 10024 // UIA_TextPattern2Id

	propIsTextPatternAvailable   = 30040      // UIA_IsTextPatternAvailablePropertyId
	propIsTextPattern2Available  = 30119      // UIA_IsTextPattern2AvailablePropertyId
	eventTextSelectionChanged    = 20014      // UIA_Text_TextSelectionChangedEventId
	eventTextChanged             = 20015      // UIA_Text_TextChangedEventId
	attrFontName                 = 40005      // UIA_FontNameAttributeId
	attrFontSize                 = 40006      // UIA_FontSizeAttributeId
	attrFontWeight               = 40007      // UIA_FontWeightAttributeId
	attrIsItalic                 = 40014      // UIA_IsItalicAttributeId
	attrIsReadOnly               = 40015      // UIA_IsReadOnlyAttributeId
	supportedTextSelectionSingle = 1          // SupportedTextSelection_Single
	textEndpointStart            = 0          // TextPatternRangeEndpoint_Start
	hrInvalidArg                 = 0x80070057 // E_INVALIDARG
)

// The interface IIDs, from UIAutomationCore.idl. TestPatternIIDsMatchTheSDK
// checks them with the other IIDs.
var (
	iidTextProvider      = guid{0x3589c92c, 0x63f3, 0x4367, [8]byte{0x99, 0xbb, 0xad, 0xa6, 0x53, 0xb7, 0x7c, 0xf2}}
	iidTextProvider2     = guid{0x0dc5e6ed, 0x3e16, 0x4bf1, [8]byte{0x8f, 0x9a, 0xa9, 0x79, 0x87, 0x8b, 0xc1, 0x95}}
	iidTextRangeProvider = guid{0x5347ad7b, 0xc355, 0x46f8, [8]byte{0xaf, 0xf5, 0x90, 0x90, 0x33, 0x58, 0x2f, 0x63}}
)

// The vtables. ITextProvider is IUnknown plus six methods; ITextProvider2 adds
// two to it; ITextRangeProvider is IUnknown plus eighteen. The order is the
// declaration order in the IDL.
var (
	textVtbl      [9]uintptr  // ITextProvider
	text2Vtbl     [11]uintptr // ITextProvider2
	textRangeVtbl [21]uintptr // ITextRangeProvider
)

var (
	ifaceText      = iface{iidTextProvider, textVtbl[:]}
	ifaceText2     = iface{iidTextProvider2, text2Vtbl[:]}
	ifaceTextRange = iface{iidTextRangeProvider, textRangeVtbl[:]}
)

// initTextVtbls fills the vtables of the text objects, with the IUnknown callbacks
// that initVtbls made. It runs once, from initVtbls.
func initTextVtbls(qi, ar, rl uintptr) {
	textVtbl = [...]uintptr{qi, ar, rl,
		syscall.NewCallback(uiaTextSelection),
		syscall.NewCallback(uiaTextVisibleRanges),
		syscall.NewCallback(uiaTextRangeFromChild),
		syscall.NewCallback(uiaTextRangeFromPoint),
		syscall.NewCallback(uiaTextDocumentRange),
		syscall.NewCallback(uiaTextSupportedSelection),
	}
	text2Vtbl = [...]uintptr{qi, ar, rl,
		syscall.NewCallback(uiaTextSelection),
		syscall.NewCallback(uiaTextVisibleRanges),
		syscall.NewCallback(uiaTextRangeFromChild),
		syscall.NewCallback(uiaTextRangeFromPoint),
		syscall.NewCallback(uiaTextDocumentRange),
		syscall.NewCallback(uiaTextSupportedSelection),
		syscall.NewCallback(uiaTextRangeFromAnnotation),
		syscall.NewCallback(uiaTextCaretRange),
	}
	textRangeVtbl = [...]uintptr{qi, ar, rl,
		syscall.NewCallback(uiaRangeClone),
		syscall.NewCallback(uiaRangeCompare),
		syscall.NewCallback(uiaRangeCompareEndpoints),
		syscall.NewCallback(uiaRangeExpand),
		syscall.NewCallback(uiaRangeFindAttribute),
		syscall.NewCallback(uiaRangeFindText),
		syscall.NewCallback(uiaRangeGetAttributeValue),
		syscall.NewCallback(uiaRangeBoundingRectangles),
		syscall.NewCallback(uiaRangeEnclosingElement),
		syscall.NewCallback(uiaRangeGetText),
		syscall.NewCallback(uiaRangeMove),
		syscall.NewCallback(uiaRangeMoveEndpointByUnit),
		syscall.NewCallback(uiaRangeMoveEndpointByRange),
		syscall.NewCallback(uiaRangeSelect),
		syscall.NewCallback(uiaRangeAddToSelection),
		syscall.NewCallback(uiaRangeRemoveFromSelection),
		syscall.NewCallback(uiaRangeScrollIntoView),
		syscall.NewCallback(uiaRangeGetChildren),
	}
}

// textRange returns the range of a range object, or false when c is not one.
// The range is replaced as a whole when UI Automation moves it, so it is kept in
// an atomic pointer.
func (c *core) textRange() (access.TextRange, bool) {
	if c.kind != kindTextRange {
		return access.TextRange{}, false
	}
	regMu.Lock()
	r := c.rng
	regMu.Unlock()
	return r, true
}

// setTextRange replaces the range of a range object.
func (c *core) setTextRange(r access.TextRange) {
	regMu.Lock()
	c.rng = r
	regMu.Unlock()
}

// newTextRange makes a range object for node id, holding r, and returns its view
// for iid. The caller releases the reference that it returns, and 0 means memory
// ran out.
func newTextRange(hwnd uintptr, b *access.Bridge, id access.ID, r access.TextRange, iid guid) uintptr {
	regMu.Lock()
	defer regMu.Unlock()
	v := createLocked(kindTextRange, id, b, hwnd, iid)
	if v != 0 {
		cores[lastHandle].rng = r // the registry lock is held, so this is set directly
	}
	return v
}

// rangeArray returns a SAFEARRAY of new range objects, one for each r. The array
// holds its own references, so the references made here are released. It returns
// 0 if memory runs out.
func rangeArray(c *core, rs []access.TextRange) uintptr {
	ps := make([]uintptr, 0, len(rs))
	for _, r := range rs {
		if p := newTextRange(c.hwnd, c.bridge, c.id, r, iidTextRangeProvider); p != 0 {
			ps = append(ps, p)
		}
	}
	psa := win32.SafeArrayCreateVector(vtUnknown, 0, uint32(len(ps)))
	if psa == 0 {
		releaseAll(ps)
		return 0
	}
	for i, p := range ps {
		win32.SafeArrayPutUnknown(psa, int32(i), p)
	}
	releaseAll(ps)
	return psa
}

// ownRange returns the core of a range object that this package made, or nil. A
// pointer from a client is checked against the vtable before it is read.
func ownRange(v uintptr) *core {
	if v == 0 || *(*uintptr)(ptr(v)) != uintptr(unsafe.Pointer(&textRangeVtbl[0])) {
		return nil
	}
	c := coreOf(v)
	if c == nil || c.kind != kindTextRange {
		return nil
	}
	return c
}

// textOf returns the Text pattern state of the node of c in the current snapshot.
// The provider's node is the one the pattern was asked for.
func (c *core) textOf() (*access.TextInfo, *access.Tree, uintptr) {
	n, t, ok := c.current()
	if !ok {
		return nil, nil, hrElementNotAvailable
	}
	if n.Text == nil {
		return nil, nil, hrNotSupported
	}
	return n.Text, t, hrOK
}

// rangeHR is the HRESULT for a range that was made: a failure when memory ran out.
func rangeHR(v uintptr) uintptr {
	if v == 0 {
		return hrFail
	}
	return hrOK
}

// textUnit converts a TextUnit value, and reports whether it is one of the seven.
func textUnit(v uintptr) (access.TextUnit, bool) {
	u := int32(v)
	if u < 0 || u > int32(access.UnitDocument) {
		return 0, false
	}
	return access.TextUnit(u), true
}

// ---- ITextProvider and ITextProvider2 ----

// uiaTextSelection returns the selection of the field, as UI Automation's
// GetSelection describes it: a degenerate range at the insertion point when
// nothing is selected, and no range when the field has no insertion point. Only a
// focused field has one here, since an unfocused field's selection is not shown.
func uiaTextSelection(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	info, t, hr := c.textOf()
	if hr != hrOK {
		return hr
	}
	var rs []access.TextRange
	if t.Focus == c.id {
		rs = append(rs, info.Selection())
	}
	psa := rangeArray(c, rs)
	if psa == 0 && len(rs) > 0 {
		return hrFail
	}
	putPtr(out, psa)
	return hrOK
}

// uiaTextVisibleRanges returns the visible text, which for a text field is all
// of it. Scrolling a multi-line field does not split the text into visible parts
// here; the rectangles of the range are clipped to the field instead.
func uiaTextVisibleRanges(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	info, _, hr := c.textOf()
	if hr != hrOK {
		return hr
	}
	psa := rangeArray(c, []access.TextRange{info.Whole()})
	if psa == 0 {
		return hrFail
	}
	putPtr(out, psa)
	return hrOK
}

// uiaTextRangeFromChild returns no range: a text field has no child that holds
// text of its own (ITextChildProvider is not implemented).
func uiaTextRangeFromChild(this, child, out uintptr) uintptr {
	_, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	putPtr(out, 0)
	return hrOK
}

// uiaTextRangeFromPoint returns the degenerate range at the point nearest to the
// screen point, whose two coordinates are doubles passed by reference, as the
// x64 calling convention passes a 16-byte structure.
func uiaTextRangeFromPoint(this, pt, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	info, _, hr := c.textOf()
	if hr != hrOK {
		return hr
	}
	if pt == 0 {
		return hrPointer
	}
	p := (*[2]float64)(ptr(pt))
	x, y := clientPoint(clientOrigin(c.hwnd), p[0], p[1])
	at := info.PointAt(float32(x), float32(y))
	r := access.TextRange{Info: info, Start: at, End: at}
	putPtr(out, newTextRange(c.hwnd, c.bridge, c.id, r, iidTextRangeProvider))
	return rangeHR(*(*uintptr)(ptr(out)))
}

// uiaTextDocumentRange returns the range of all the text.
func uiaTextDocumentRange(this, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	info, _, hr := c.textOf()
	if hr != hrOK {
		return hr
	}
	putPtr(out, newTextRange(c.hwnd, c.bridge, c.id, info.Whole(), iidTextRangeProvider))
	return rangeHR(*(*uintptr)(ptr(out)))
}

// uiaTextSupportedSelection reports that a field has one selection.
func uiaTextSupportedSelection(this, out uintptr) uintptr {
	_, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	*(*int32)(ptr(out)) = supportedTextSelectionSingle
	return hrOK
}

// uiaTextRangeFromAnnotation is not supported: a field has no annotations.
func uiaTextRangeFromAnnotation(this, annotation, out uintptr) uintptr {
	_, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	return hrNotSupported
}

// uiaTextCaretRange returns the degenerate range at the caret. isActive says
// whether the field has keyboard focus, in which case the caret is the system
// caret's position; otherwise the caret may be elsewhere (ITextProvider2).
func uiaTextCaretRange(this, isActive, out uintptr) uintptr {
	c, hr := enter(this, out)
	if hr != hrOK {
		return hr
	}
	if isActive == 0 {
		return hrPointer
	}
	info, t, hr := c.textOf()
	if hr != hrOK {
		return hr
	}
	focused := t.Focus == c.id
	*(*int32)(ptr(isActive)) = boolOf(focused)
	putPtr(out, newTextRange(c.hwnd, c.bridge, c.id, info.CaretRange(), iidTextRangeProvider))
	return rangeHR(*(*uintptr)(ptr(out)))
}

// ---- ITextRangeProvider ----

// rangeEnter finds the range for a method that fills an out parameter.
func rangeEnter(this, out uintptr) (*core, access.TextRange, uintptr) {
	c, hr := enter(this, out)
	if hr != hrOK {
		return nil, access.TextRange{}, hr
	}
	r, ok := c.textRange()
	if !ok {
		return nil, access.TextRange{}, hrFail
	}
	return c, r, hrOK
}

// rangeThis finds the range for a method that has no out parameter.
func rangeThis(this uintptr) (*core, access.TextRange, uintptr) {
	c := coreOf(this)
	if c == nil {
		return nil, access.TextRange{}, hrFail
	}
	r, ok := c.textRange()
	if !ok {
		return nil, access.TextRange{}, hrFail
	}
	return c, r, hrOK
}

// uiaRangeClone returns a new range with the same endpoints.
func uiaRangeClone(this, out uintptr) uintptr {
	c, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	putPtr(out, newTextRange(c.hwnd, c.bridge, c.id, r, iidTextRangeProvider))
	return rangeHR(*(*uintptr)(ptr(out)))
}

// uiaRangeCompare reports whether two ranges have the same endpoints. It compares
// the endpoints, not the text (ITextRangeProvider::Compare).
func uiaRangeCompare(this, other, out uintptr) uintptr {
	_, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	o := ownRange(other)
	if o == nil {
		return hrInvalidArg
	}
	s, _ := o.textRange()
	*(*int32)(ptr(out)) = boolOf(r.Same(s))
	return hrOK
}

// uiaRangeCompareEndpoints compares an endpoint of this range with an endpoint of
// another, and returns a negative, zero or positive value
// (ITextRangeProvider::CompareEndpoints).
func uiaRangeCompareEndpoints(this, endpoint, target, targetEndpoint, out uintptr) uintptr {
	_, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	o := ownRange(target)
	if o == nil {
		return hrInvalidArg
	}
	s, _ := o.textRange()
	if s.Content() != r.Content() {
		return hrInvalidArg
	}
	*(*int32)(ptr(out)) = int32(r.CompareEndpoints(int32(endpoint) == textEndpointStart, s, int32(targetEndpoint) == textEndpointStart))
	return hrOK
}

// uiaRangeExpand expands the range to whole units, in place
// (ITextRangeProvider::ExpandToEnclosingUnit). A page is the whole text, as a
// document is, since a text field has one page.
func uiaRangeExpand(this, unit uintptr) uintptr {
	c, r, hr := rangeThis(this)
	if hr != hrOK {
		return hr
	}
	u, ok := textUnit(unit)
	if !ok {
		return hrInvalidArg
	}
	c.setTextRange(r.Expand(u))
	return hrOK
}

// uiaRangeFindAttribute returns the whole range when its attribute has the value
// val, and no range otherwise. The attributes of a field are the same throughout,
// so the answer does not depend on the range's extent.
func uiaRangeFindAttribute(this, attr uintptr, val uintptr, backward uintptr, out uintptr) uintptr {
	c, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	if val == 0 {
		return hrPointer
	}
	got, ok := attributeValue(r.Info, int32(attr))
	if !ok || !sameVariant(got, *(*variant)(ptr(val))) {
		putPtr(out, 0)
		return hrOK
	}
	putPtr(out, newTextRange(c.hwnd, c.bridge, c.id, r, iidTextRangeProvider))
	return rangeHR(*(*uintptr)(ptr(out)))
}

// uiaRangeFindText returns the first match of the text within the range, or the
// last one when backward is set; no range is returned when there is none.
// ignoreCase folds case.
func uiaRangeFindText(this, text, backward, ignoreCase, out uintptr) uintptr {
	c, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	m, ok := r.Find(wideString(text), uint32(backward) != 0, uint32(ignoreCase) != 0)
	if !ok {
		putPtr(out, 0)
		return hrOK
	}
	putPtr(out, newTextRange(c.hwnd, c.bridge, c.id, m, iidTextRangeProvider))
	return rangeHR(*(*uintptr)(ptr(out)))
}

// attributeValue returns the value of a text attribute of the field, and whether
// the field has it. Colors are not reported; the font attributes are what the
// editor knows about. A value that is not known is NotSupported, not an error.
func attributeValue(info *access.TextInfo, attr int32) (variant, bool) {
	switch attr {
	case attrIsReadOnly:
		return variantBool(info.ReadOnly), true
	case attrFontName:
		if info.FontName != "" {
			return variantBSTR(info.FontName), true
		}
	case attrFontSize:
		if info.FontSize > 0 {
			return variantDouble(info.FontSize), true
		}
	case attrFontWeight:
		if info.FontWeight > 0 {
			return variantI4(int32(info.FontWeight)), true
		}
	case attrIsItalic:
		return variantBool(info.Italic), true
	}
	return variant{}, false
}

// sameVariant reports whether two VARIANTs hold the same value of the same type.
func sameVariant(a, b variant) bool {
	if a.vt != b.vt {
		return false
	}
	if a.vt == vtBSTR {
		return wideString(variantBSTRValue(a)) == wideString(variantBSTRValue(b))
	}
	return a.data == b.data
}

// uiaRangeGetAttributeValue returns the value of a text attribute over the range.
// A text field's attributes are the same throughout, so the value is the field's.
// An attribute the field does not have is the NotSupported value.
func uiaRangeGetAttributeValue(this, attr, out uintptr) uintptr {
	_, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	v, ok := attributeValue(r.Info, int32(attr))
	if !ok {
		p := newWord()
		if hr := win32.UiaGetReservedNotSupportedValue(uintptr(unsafe.Pointer(p))); hr != hrOK {
			return hr
		}
		v = variantUnknown(*p)
	}
	*(*variant)(ptr(out)) = v
	return hrOK
}

// uiaRangeBoundingRectangles returns the rectangles of the range, one per line it
// touches, as x, y, width and height in screen coordinates. A degenerate range
// has none, and neither has one that is off the visible part of the field.
func uiaRangeBoundingRectangles(this, out uintptr) uintptr {
	c, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	rects := r.Rects()
	psa := win32.SafeArrayCreateVector(vtR8, 0, uint32(4*len(rects)))
	if psa == 0 {
		return hrFail
	}
	if len(rects) > 0 {
		data := newWord()
		if hr := win32.SafeArrayAccessData(psa, data); hr != hrOK {
			win32.SafeArrayDestroy(psa)
			return hr
		}
		vals := unsafe.Slice((*float64)(ptr(*data)), 4*len(rects))
		origin := clientOrigin(c.hwnd)
		for i, rc := range rects {
			x, y, w, h := screenRect(origin, rc)
			vals[4*i], vals[4*i+1], vals[4*i+2], vals[4*i+3] = x, y, w, h
		}
		win32.SafeArrayUnaccessData(psa)
	}
	putPtr(out, psa)
	return hrOK
}

// uiaRangeEnclosingElement returns the element that holds the text: the field. A
// field that has left the tree has no element to return.
func uiaRangeEnclosingElement(this, out uintptr) uintptr {
	c, _, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	p := c.provide(c.id, iidRawElementProviderSimple)
	if p == 0 {
		return hrElementNotAvailable
	}
	putPtr(out, p)
	return hrOK
}

// uiaRangeGetText returns the text of the range, cut to maxLength UTF-16 code
// units (-1 for no limit). The cut never splits a surrogate pair.
func uiaRangeGetText(this, maxLength, out uintptr) uintptr {
	_, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	putPtr(out, bstrOf(access.UTF16Prefix(r.Text(), int(int32(maxLength)))))
	return hrOK
}

// uiaRangeMove moves the range by count units, in place, and returns the number
// of units moved (ITextRangeProvider::Move).
func uiaRangeMove(this, unit, count, out uintptr) uintptr {
	c, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	u, ok := textUnit(unit)
	if !ok {
		return hrInvalidArg
	}
	m, moved := r.Move(u, int(int32(count)))
	c.setTextRange(m)
	*(*int32)(ptr(out)) = int32(moved)
	return hrOK
}

// uiaRangeMoveEndpointByUnit moves one endpoint by count units, in place, and
// returns the number of units moved (ITextRangeProvider::MoveEndpointByUnit).
func uiaRangeMoveEndpointByUnit(this, endpoint, unit, count, out uintptr) uintptr {
	c, r, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	u, ok := textUnit(unit)
	if !ok {
		return hrInvalidArg
	}
	m, moved := r.MoveEndpoint(int32(endpoint) == textEndpointStart, u, int(int32(count)))
	c.setTextRange(m)
	*(*int32)(ptr(out)) = int32(moved)
	return hrOK
}

// uiaRangeMoveEndpointByRange sets one endpoint of this range to an endpoint of
// the target range, in place (ITextRangeProvider::MoveEndpointByRange).
func uiaRangeMoveEndpointByRange(this, endpoint, target, targetEndpoint uintptr) uintptr {
	c, r, hr := rangeThis(this)
	if hr != hrOK {
		return hr
	}
	o := ownRange(target)
	if o == nil {
		return hrInvalidArg
	}
	s, _ := o.textRange()
	if s.Content() != r.Content() {
		return hrInvalidArg
	}
	c.setTextRange(r.MoveEndpointByRange(int32(endpoint) == textEndpointStart, s, int32(targetEndpoint) == textEndpointStart))
	return hrOK
}

// uiaRangeSelect selects the text of the range, and moves the insertion point to
// the end when the range is degenerate (ITextRangeProvider::Select). The field
// takes focus. The offsets are bytes, which the runtime clamps to its text.
func uiaRangeSelect(this uintptr) uintptr {
	c, r, hr := rangeThis(this)
	if hr != hrOK {
		return hr
	}
	content := r.Content()
	return c.performText(access.Request{Action: access.ActionSetTextSelection,
		Anchor: content.ByteOf(r.Start), Caret: content.ByteOf(r.End)})
}

// uiaRangeAddToSelection is not supported: a field has one selection.
func uiaRangeAddToSelection(this uintptr) uintptr { return hrNotSupported }

// uiaRangeRemoveFromSelection is not supported: a field has one selection.
func uiaRangeRemoveFromSelection(this uintptr) uintptr { return hrNotSupported }

// uiaRangeScrollIntoView scrolls the field so that it is visible. A range is
// within its field, so the alignment is not used.
func uiaRangeScrollIntoView(this, alignToTop uintptr) uintptr {
	c := coreOf(this)
	if c == nil {
		return hrFail
	}
	return c.perform(access.ActionScrollIntoView)
}

// uiaRangeGetChildren returns an empty array: a field's text has no children
// (ITextRangeProvider::GetChildren).
func uiaRangeGetChildren(this, out uintptr) uintptr {
	_, _, hr := rangeEnter(this, out)
	if hr != hrOK {
		return hr
	}
	psa := win32.SafeArrayCreateVector(vtUnknown, 0, 0)
	if psa == 0 {
		return hrFail
	}
	putPtr(out, psa)
	return hrOK
}

// performText asks the runtime to run a text action with its request, from any
// thread. It fails when the node has gone.
func (c *core) performText(req access.Request) uintptr {
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Do(c.id, req)
	return hrOK
}
