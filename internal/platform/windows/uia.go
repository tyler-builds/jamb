//go:build windows

package windows

import (
	"runtime"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/win32"
)

// The constants below are from UIAutomationClient.h and UIAutomationCoreApi.h
// in the Windows 10 SDK (10.0.26100.0).
const (
	// Control types.
	ctButton      = 50000
	ctCalendar    = 50001 // UIA_CalendarControlTypeId
	ctCheckBox    = 50002
	ctComboBox    = 50003
	ctEdit        = 50004
	ctHyperlink   = 50005
	ctImage       = 50006
	ctListItem    = 50007
	ctList        = 50008
	ctMenu        = 50009
	ctMenuBar     = 50010
	ctMenuItem    = 50011
	ctProgressBar = 50012
	ctRadioButton = 50013
	ctSlider      = 50015
	ctSpinner     = 50016
	ctStatusBar   = 50017 // UIA_StatusBarControlTypeId
	ctTab         = 50018
	ctTabItem     = 50019
	ctText        = 50020
	ctToolBar     = 50021 // UIA_ToolBarControlTypeId
	ctToolTip     = 50022
	ctTree        = 50023
	ctTreeItem    = 50024
	ctCustom      = 50025
	ctGroup       = 50026
	ctDocument    = 50030
	ctWindow      = 50032
	ctPane        = 50033
	ctSeparator   = 50038

	// Properties.
	propControlType          = 30003
	propLocalizedControlType = 30004
	propName                 = 30005
	propAcceleratorKey       = 30006
	propAccessKey            = 30007
	propHasKeyboardFocus     = 30008
	propIsKeyboardFocusable  = 30009
	propIsEnabled            = 30010
	propAutomationID         = 30011
	propClassName            = 30012
	propHelpText             = 30013
	propIsControlElement     = 30016
	propIsContentElement     = 30017
	propIsPassword           = 30019
	propIsOffscreen          = 30022
	propOrientation          = 30023
	propIsRequiredForForm    = 30025 // UIA_IsRequiredForFormPropertyId
	propIsDataValidForForm   = 30103 // UIA_IsDataValidForFormPropertyId
	propPositionInSet        = 30152
	propSizeOfSet            = 30153
	propLevel                = 30154
	propIsDialog             = 30174

	// Patterns and events.
	patternInvoke         = 10000
	patternSelection      = 10001
	patternValue          = 10002
	patternScroll         = 10004
	patternExpandCollapse = 10005
	patternSelectionItem  = 10010
	patternToggle         = 10015
	patternScrollItem     = 10017
	patternWindow         = 10009
	patternRangeValue     = 10003

	eventStructureChanged       = 20002
	eventPropertyChanged        = 20004
	eventAutomationFocusChanged = 20005
	eventMenuClosed             = 20007
	eventMenuOpened             = 20003
	eventWindowOpened           = 20016
	eventWindowClosed           = 20017

	// Properties of the patterns, and IsXPatternAvailable.
	propLabeledBy                = 30018
	propIsInvokeAvailable        = 30031
	propIsExpandCollapseAvail    = 30028
	propIsScrollAvailable        = 30034
	propIsScrollItemAvailable    = 30035
	propIsSelectionItemAvail     = 30036
	propIsSelectionAvailable     = 30037
	propIsToggleAvailable        = 30041
	propIsValueAvailable         = 30043
	propIsWindowAvailable        = 30044
	propWindowIsModal            = 30077
	propValue                    = 30045
	propIsReadOnly               = 30046
	propIsRangeValueAvail        = 30033
	propRangeValue               = 30047
	propRangeIsReadOnly          = 30048
	propRangeMinimum             = 30049
	propRangeMaximum             = 30050
	propRangeLargeChange         = 30051
	propRangeSmallChange         = 30052
	propScrollHorizontalPercent  = 30053
	propScrollHorizontalView     = 30054
	propScrollVerticalPercent    = 30055
	propScrollVerticalView       = 30056
	propScrollHorizontallyScroll = 30057
	propScrollVerticallyScroll   = 30058
	propExpandCollapseState      = 30070
	propIsSelected               = 30079
	propToggleState              = 30086
	propDescribedBy              = 30105
	propHeadingLevel             = 30173

	// The states of the patterns.
	toggleStateOff           = 0
	toggleStateOn            = 1
	toggleStateIndeterminate = 2
	expandCollapseCollapsed  = 0
	expandCollapseExpanded   = 1
	expandCollapseLeafNode   = 3     // a tree item with no children, which cannot open or close
	headingLevel1            = 80051 // HeadingLevel1; the others follow it

	// WindowVisualState and WindowInteractionState values.
	windowVisualNormal  = 0
	windowReadyForInput = 2

	// ScrollAmount values.
	scrollLargeDecrement = 0
	scrollSmallDecrement = 1
	scrollLargeIncrement = 3
	scrollSmallIncrement = 4

	// NotificationKind_Other, and the NotificationProcessing values that
	// announcements use (see announce).
	notificationKindOther           = 4
	notificationImportantMostRecent = 1
	notificationAll                 = 2
	notificationMostRecent          = 3

	// ProviderOptions.
	providerClientSide             = 0x1
	providerServerSide             = 0x2
	providerNonClientArea          = 0x4
	providerOverride               = 0x8
	providerOwnsSetFocus           = 0x10
	providerUseComThreading        = 0x20
	providerRefuseNonClientSupport = 0x40
	providerHasNativeIAccessible   = 0x80
	providerUseClientCoordinates   = 0x100

	// NavigateDirection.
	navParent          = 0
	navNextSibling     = 1
	navPreviousSibling = 2
	navFirstChild      = 3
	navLastChild       = 4

	// StructureChangeType_ChildrenInvalidated.
	structureChildrenInvalidated = 2

	uiaAppendRuntimeID = 3
	uiaRootObjectID    = -25 // the lParam of WM_GETOBJECT that asks for the root object

	// VARIANT types and values.
	vtEmpty    = 0
	vtI4       = 3
	vtR8       = 5
	vtBSTR     = 8
	vtUnknown  = 13
	vtBool     = 11
	vtArray    = 0x2000
	variantOn  = -1 // VARIANT_TRUE
	variantOff = 0  // VARIANT_FALSE
)

// HRESULTs. A failure has the top bit set.
const (
	hrOK                  = 0
	hrNotImpl             = 0x80004001
	hrNoInterface         = 0x80004002
	hrPointer             = 0x80004003
	hrFail                = 0x80004005
	hrElementNotAvailable = 0x80040201 // UIA_E_ELEMENTNOTAVAILABLE
	hrInvalidOperation    = 0x80131509 // UIA_E_INVALIDOPERATION
	hrNotSupported        = 0x80040204 // UIA_E_NOTSUPPORTED
)

// controlType returns the UI Automation control type of a role.
//
//	role                      control type      LocalizedControlType
//	window                    Window            window
//	dialog                    Window            dialog (IsDialog is set)
//	alert dialog              Window            alert dialog (IsDialog is set)
//	group                     Group             group
//	button                    Button            button
//	check box, switch         CheckBox          check box, switch
//	radio button              RadioButton       radio button
//	text field, text area     Edit              text field, text area
//	combo box                 ComboBox          combo box
//	slider                    Slider            slider
//	spinner                   Spinner           spinner
//	static text, heading      Text              static text, heading
//	image                     Image             image
//	link                      Hyperlink         link
//	list, list item           List, ListItem    list, list item
//	menu bar, menu            MenuBar, Menu     menu bar, menu
//	menu item (all kinds)     MenuItem          menu item, menu item check, menu item radio
//	separator                 Separator         separator
//	tooltip                   ToolTip           tooltip
//	scroll area, tab panel    Pane              scroll area, tab panel
//	tab list, tab             Tab, TabItem      tab list, tab
//	tree                      Tree              tree
//	tree item                 TreeItem          tree item
//	calendar                  Calendar          calendar (Grid and Table patterns)
//	progress bar, busy ring   ProgressBar      progress bar (read-only RangeValue when determinate)
//	status bar                StatusBar         status bar
//	tool bar                  ToolBar           tool bar
//	unknown                   Pane              unknown
//
// A text area is an Edit, not a Document: a Document is for read-only rich
// text, and the ui runtime's text areas are editable. Unknown roles are Panes,
// which carry no pattern and so no behavior a client would expect.
func controlType(r access.Role) int32 {
	if r.IsDialog() {
		return ctWindow
	}
	switch r {
	case access.RoleWindow:
		return ctWindow
	case access.RoleGroup:
		return ctGroup
	case access.RoleButton:
		return ctButton
	case access.RoleCheckBox, access.RoleSwitch:
		return ctCheckBox
	case access.RoleRadioButton:
		return ctRadioButton
	case access.RoleTextField, access.RoleTextArea:
		return ctEdit
	case access.RoleComboBox:
		return ctComboBox
	case access.RoleSlider:
		return ctSlider
	case access.RoleSpinner:
		return ctSpinner
	case access.RoleStaticText, access.RoleHeading:
		return ctText
	case access.RoleImage:
		return ctImage
	case access.RoleLink:
		return ctHyperlink
	case access.RoleList:
		return ctList
	case access.RoleListItem:
		return ctListItem
	case access.RoleMenuBar:
		return ctMenuBar
	case access.RoleMenu:
		return ctMenu
	case access.RoleMenuItem, access.RoleMenuItemCheck, access.RoleMenuItemRadio:
		return ctMenuItem
	case access.RoleSeparator:
		return ctSeparator
	case access.RoleTooltip:
		return ctToolTip
	case access.RoleScrollArea, access.RoleTabPanel:
		return ctPane
	case access.RoleTabList:
		return ctTab
	case access.RoleTab:
		return ctTabItem
	case access.RoleTree:
		return ctTree
	case access.RoleTreeItem:
		return ctTreeItem
	case access.RoleProgressBar:
		return ctProgressBar
	case access.RoleDataGrid:
		return ctDataGrid
	case access.RoleCalendar:
		return ctCalendar
	case access.RoleDataItem:
		return ctDataItem
	case access.RoleHeaderItem:
		return ctHeaderItem
	case access.RoleStatusBar:
		return ctStatusBar
	case access.RoleToolBar:
		return ctToolBar
	}
	return ctPane
}

// variant is a VARIANT. The type tag is at offset 0 and the value at offset 8,
// so the struct is 24 bytes on amd64. The value is kept as words so the struct
// is 8-byte aligned.
type variant struct {
	vt   uint16
	_    [3]uint16
	data [2]uint64
}

func variantI4(n int32) variant {
	v := variant{vt: vtI4}
	*(*int32)(unsafe.Pointer(&v.data[0])) = n
	return v
}

func variantBool(b bool) variant {
	v := variant{vt: vtBool}
	if b {
		*(*int16)(unsafe.Pointer(&v.data[0])) = variantOn
	} else {
		*(*int16)(unsafe.Pointer(&v.data[0])) = variantOff
	}
	return v
}

// variantBSTR returns a VARIANT that holds a new BSTR. The VARIANT owns the
// BSTR once UI Automation has it, so it is freed by UI Automation.
func variantBSTR(s string) variant {
	v := variant{vt: vtBSTR}
	*(*uintptr)(unsafe.Pointer(&v.data[0])) = bstrOf(s)
	return v
}

// bstrOf returns a new BSTR holding s. The caller frees it with SysFreeString,
// unless it is given to UI Automation in a VARIANT.
func bstrOf(s string) uintptr {
	// The BSTR is allocated by the system and filled in place, so a call makes no
	// Go buffer. Garbage made on each query is what the Go collector had to run
	// over, and it grew the process's private memory between collections.
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	b := win32.SysAllocStringN(n)
	if b == 0 {
		return 0
	}
	units := unsafe.Slice((*uint16)(ptr(b)), n)
	i := 0
	for _, r := range s {
		if r >= 0x10000 {
			r1, r2 := utf16.EncodeRune(r)
			units[i], units[i+1] = uint16(r1), uint16(r2)
			i += 2
		} else {
			units[i] = uint16(r)
			i++
		}
	}
	return b
}

// newVariant returns a VARIANT on the heap. A pointer to a VARIANT that is given to
// COM must not be on the goroutine stack: a callback can grow the stack and move
// the variable, and the callback then writes to the old copy. Returning the
// pointer makes the compiler allocate on the heap, which the call does not
// change; passing the pointer as a uintptr would not keep a stack variable.
//
//go:noinline
func newVariant() *variant { return new(variant) }

// newWord returns a pointer-sized word on the heap, for the same reason as
// newVariant.
//
//go:noinline
func newWord() *uintptr { return new(uintptr) }

// variantDouble returns a VARIANT that holds f.
func variantDouble(f float64) variant {
	v := variant{vt: vtR8}
	*(*float64)(unsafe.Pointer(&v.data[0])) = f
	return v
}

// variantUnknown returns a VARIANT that holds the interface pointer p, which the
// VARIANT owns once UI Automation has it.
func variantUnknown(p uintptr) variant {
	v := variant{vt: vtUnknown}
	*(*uintptr)(unsafe.Pointer(&v.data[0])) = p
	return v
}

// variantUnknownArray returns a VARIANT that holds the SAFEARRAY psa of
// interface pointers, which the VARIANT owns once UI Automation has it.
func variantUnknownArray(psa uintptr) variant {
	v := variant{vt: vtUnknown | vtArray}
	*(*uintptr)(unsafe.Pointer(&v.data[0])) = psa
	return v
}

// wideString copies the NUL-terminated UTF-16 string at p, such as the value a
// client passes to SetValue. A null pointer is the empty string.
func wideString(p uintptr) string {
	if p == 0 {
		return ""
	}
	n := 0
	for *(*uint16)(ptr(p + uintptr(2*n))) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(ptr(p)), n))
}

// nodeProperty returns the VARIANT of property id for node n. focused says
// whether n has keyboard focus. Properties the node does not have are VT_EMPTY.
func nodeProperty(id int32, n *access.Node, focused bool) variant {
	switch id {
	case propControlType:
		return variantI4(controlType(n.Role))
	case propLocalizedControlType:
		return variantBSTR(n.Role.String())
	case propName:
		return variantBSTR(n.Name)
	case propAutomationID:
		return variantBSTR(n.Key)
	case propHelpText:
		return variantBSTR(n.Description)
	case propIsEnabled:
		return variantBool(!n.States.Has(access.StateDisabled))
	case propHasKeyboardFocus:
		return variantBool(focused)
	case propIsKeyboardFocusable:
		return variantBool(n.States.Has(access.StateFocusable))
	case propIsOffscreen:
		return variantBool(n.States.Has(access.StateOffscreen))
	case propIsRequiredForForm:
		return variantBool(n.States.Has(access.StateRequired))
	case propIsDataValidForForm:
		// A field that is required or invalid says whether its value is valid. Any other
		// element leaves the property empty, which is the answer of an element that is
		// not a form field.
		if n.States.Has(access.StateRequired) || n.States.Has(access.StateInvalid) {
			return variantBool(!n.States.Has(access.StateInvalid))
		}
	case propIsControlElement, propIsContentElement:
		return variantBool(true)
	case propPositionInSet:
		if n.PosInSet > 0 {
			return variantI4(int32(n.PosInSet))
		}
	case propSizeOfSet:
		if n.SetSize > 0 {
			return variantI4(int32(n.SetSize))
		}
	case propLevel:
		if n.Level > 0 {
			return variantI4(int32(n.Level))
		}
	case propIsDialog:
		if n.Role.IsDialog() {
			return variantBool(true)
		}
	case propValue:
		if hasValueNode(n) {
			return variantBSTR(n.Value)
		}
	case propIsPassword:
		if isText(n.Role) {
			return variantBool(n.States.Has(access.StatePassword))
		}
	case propIsReadOnly:
		if hasValueNode(n) {
			return variantBool(valueReadOnly(n))
		}
	case propRangeValue:
		if n.Range != nil {
			return variantDouble(n.Range.Value)
		}
	case propRangeIsReadOnly:
		if n.Range != nil {
			return variantBool(n.States.Has(access.StateReadOnly))
		}
	case propRangeMinimum:
		if n.Range != nil {
			return variantDouble(n.Range.Min)
		}
	case propRangeMaximum:
		if n.Range != nil {
			return variantDouble(n.Range.Max)
		}
	case propRangeLargeChange:
		if n.Range != nil {
			return variantDouble(n.Range.LargeChange)
		}
	case propRangeSmallChange:
		if n.Range != nil {
			return variantDouble(n.Range.SmallChange)
		}
	case propToggleState:
		if n.States.Has(access.StateToggle) {
			return variantI4(toggleStateOf(n.States))
		}
	case propExpandCollapseState:
		if n.States.Has(access.StateExpandable) || n.Role == access.RoleTreeItem {
			return variantI4(expandStateOf(n))
		}
	case propIsSelected:
		if n.States.Has(access.StateSelectable) {
			return variantBool(n.States.Has(access.StateSelected))
		}
	case propItemStatus:
		if n.ItemStatus != "" {
			return variantBSTR(n.ItemStatus)
		}
	case propWindowIsModal:
		if n.Role.IsDialog() {
			return variantBool(true)
		}
	case propHeadingLevel:
		if n.Heading > 0 {
			return variantI4(headingLevel1 + int32(n.Heading) - 1)
		}
	case propScrollHorizontalPercent, propScrollHorizontalView, propScrollVerticalPercent,
		propScrollVerticalView, propScrollHorizontallyScroll, propScrollVerticallyScroll:
		if n.Scroll.Container {
			return scrollProperty(id, scrollState(n.Scroll))
		}
	}
	if k, ok := availableKind(id); ok {
		return variantBool(supports(k, n))
	}
	return variant{}
}

// isText reports whether a role is an editable text field.
func isText(r access.Role) bool { return r == access.RoleTextField || r == access.RoleTextArea }

// hasValue reports whether a role has the Value pattern: a text field, or a combo
// box, whose Value is the label it has chosen.
func hasValue(r access.Role) bool { return isText(r) || r == access.RoleComboBox }

// hasValueNode reports whether a node has the Value pattern: its role does (see
// hasValue), or it takes a value through the Set Value action. An editable data
// item does: Microsoft's DataItem guidance requires the Value pattern when the
// item's primary text is editable (uiauto-supportdataitemcontroltype). A data item
// that cannot be edited has no Value pattern, which that guidance does not require.
// A slider with a value text, such as a Rating's "3 of 5 stars", has a read-only
// Value pattern beside its RangeValue, as the Slider control type allows and the
// WinUI RatingControl does (see valueReadOnly).
func hasValueNode(n *access.Node) bool {
	if hasValue(n.Role) || sliderText(n) {
		return true
	}
	for _, a := range n.Actions {
		if a == access.ActionSetValue {
			return true
		}
	}
	return false
}

// sliderText reports whether n is a slider with a value text.
func sliderText(n *access.Node) bool { return n.Role == access.RoleSlider && n.Value != "" }

// valueReadOnly reports whether the Value of n can only be read: a read-only node,
// or the value text of a slider, which is set through RangeValue.
func valueReadOnly(n *access.Node) bool { return n.States.Has(access.StateReadOnly) || sliderText(n) }

// toggleStateOf returns the ToggleState of a node: on when checked, indeterminate
// when mixed, off otherwise.
func toggleStateOf(s access.State) int32 {
	switch {
	case s.Has(access.StateMixed):
		return toggleStateIndeterminate
	case s.Has(access.StateChecked):
		return toggleStateOn
	}
	return toggleStateOff
}

// expandStateOf returns the ExpandCollapseState of a node that can expand, or of a
// tree item. A tree item with no children is a leaf node: UI Automation requires
// every tree item to support ExpandCollapse, and a leaf reports LeafNode.
func expandStateOf(n *access.Node) int32 {
	if n.Role == access.RoleTreeItem && !n.States.Has(access.StateExpandable) {
		return expandCollapseLeafNode
	}
	if n.States.Has(access.StateExpanded) {
		return expandCollapseExpanded
	}
	return expandCollapseCollapsed
}

// patternKindOf returns the kind of pattern object for a pattern ID.
func patternKindOf(id int32) (coreKind, bool) {
	switch id {
	case patternInvoke:
		return kindInvoke, true
	case patternToggle:
		return kindToggle, true
	case patternExpandCollapse:
		return kindExpand, true
	case patternSelectionItem:
		return kindSelectItem, true
	case patternSelection:
		return kindSelection, true
	case patternValue:
		return kindValue, true
	case patternRangeValue:
		return kindRange, true
	case patternScroll:
		return kindScroll, true
	case patternScrollItem:
		return kindScrollItem, true
	case patternWindow:
		return kindWindow, true
	case patternGrid:
		return kindGrid, true
	case patternTable:
		return kindTable, true
	case patternGridItem:
		return kindGridItem, true
	case patternTableItem:
		return kindTableItem, true
	case patternText, patternText2:
		return kindText, true
	}
	return 0, false
}

// availableKind returns the pattern kind that an IsXPatternAvailable property
// reports on.
func availableKind(id int32) (coreKind, bool) {
	switch id {
	case propIsInvokeAvailable:
		return kindInvoke, true
	case propIsToggleAvailable:
		return kindToggle, true
	case propIsExpandCollapseAvail:
		return kindExpand, true
	case propIsSelectionItemAvail:
		return kindSelectItem, true
	case propIsSelectionAvailable:
		return kindSelection, true
	case propIsValueAvailable:
		return kindValue, true
	case propIsRangeValueAvail:
		return kindRange, true
	case propIsScrollAvailable:
		return kindScroll, true
	case propIsScrollItemAvailable:
		return kindScrollItem, true
	case propIsWindowAvailable:
		return kindWindow, true
	case propIsGridPatternAvail:
		return kindGrid, true
	case propIsTablePatternAvail:
		return kindTable, true
	case propIsGridItemPatternAvail:
		return kindGridItem, true
	case propIsTableItemPatternAvail:
		return kindTableItem, true
	case propIsTextPatternAvailable, propIsTextPattern2Available:
		return kindText, true
	}
	return 0, false
}

// supports reports whether node n has the pattern of kind k. A pattern follows
// the node's role and states, so a disabled control still has its pattern, and
// reports it as disabled.
func supports(k coreKind, n *access.Node) bool {
	switch k {
	case kindInvoke:
		// A toggle is changed by Toggle, not by Invoke, so it has Toggle alone. A
		// disabled button keeps Invoke, as a disabled element keeps its patterns.
		if n.States.Has(access.StateToggle) {
			return false
		}
		return n.Has(access.ActionClick) || n.Role == access.RoleButton && n.States.Has(access.StateDisabled)
	case kindToggle:
		return n.States.Has(access.StateToggle)
	case kindExpand:
		return n.States.Has(access.StateExpandable) || n.Role == access.RoleTreeItem
	case kindSelectItem:
		return n.States.Has(access.StateSelectable)
	case kindSelection:
		return n.States.Has(access.StateSelectionContainer)
	case kindValue:
		return hasValueNode(n)
	case kindRange:
		return n.Range != nil
	case kindScroll:
		return n.Scroll.Container
	case kindScrollItem:
		return n.Has(access.ActionScrollIntoView)
	case kindGrid, kindTable:
		return n.Role == access.RoleDataGrid || n.Role == access.RoleCalendar
	case kindGridItem, kindTableItem:
		return n.Role == access.RoleDataItem && n.GridRow > 0
	case kindText:
		return isText(n.Role)
	case kindWindow:
		return n.Role.IsDialog()
	}
	return false
}

// scrollState is the Scroll pattern's view of a container.
type scrollState access.Scroll

// canX reports whether the container scrolls horizontally.
func (s scrollState) canX() bool { return s.CanX }

// canY reports whether the container scrolls vertically.
func (s scrollState) canY() bool { return s.CanY }

// percentX returns the horizontal position, or -1 when the container does not
// scroll horizontally, as UI Automation defines it.
func (s scrollState) percentX() float64 {
	if !s.CanX {
		return -1
	}
	return s.X
}

// percentY returns the vertical position, or -1 when the container does not
// scroll vertically.
func (s scrollState) percentY() float64 {
	if !s.CanY {
		return -1
	}
	return s.Y
}

// viewX returns the visible part of the content horizontally, in percent.
func (s scrollState) viewX() float64 { return s.ViewX }

// viewY returns the visible part of the content vertically, in percent.
func (s scrollState) viewY() float64 { return s.ViewY }

// scrollProperty returns the VARIANT of a scroll property of a container.
func scrollProperty(id int32, s scrollState) variant {
	switch id {
	case propScrollHorizontalPercent:
		return variantDouble(s.percentX())
	case propScrollHorizontalView:
		return variantDouble(s.viewX())
	case propScrollVerticalPercent:
		return variantDouble(s.percentY())
	case propScrollVerticalView:
		return variantDouble(s.viewY())
	case propScrollHorizontallyScroll:
		return variantBool(s.canX())
	case propScrollVerticallyScroll:
		return variantBool(s.canY())
	}
	return variant{}
}

// runtimeIDOf returns the runtime ID of a node: UiaAppendRuntimeId, then the
// high and low 32 bits of the node ID.
func runtimeIDOf(id access.ID) [3]int32 {
	return [3]int32{uiaAppendRuntimeID, int32(uint32(id >> 32)), int32(uint32(id))}
}

// newRuntimeID returns a new SAFEARRAY of the runtime ID of id, which the
// caller owns, or 0 on failure.
func newRuntimeID(id access.ID) uintptr {
	vals := runtimeIDOf(id)
	psa := win32.SafeArrayCreateVector(vtI4, 0, uint32(len(vals)))
	if psa == 0 {
		return 0
	}
	for i, v := range vals {
		if win32.SafeArrayPutInt32(psa, int32(i), v) != hrOK {
			win32.SafeArrayDestroy(psa)
			return 0
		}
	}
	return psa
}

// screenRect converts a rectangle in client pixels to screen pixels, given the
// screen position of the client area's top-left corner. An empty rectangle is
// all zeros.
func screenRect(origin win32.POINT, r access.Rect) (x, y, w, h float64) {
	if r.Empty() {
		return 0, 0, 0, 0
	}
	return float64(origin.X) + float64(r.X), float64(origin.Y) + float64(r.Y), float64(r.W), float64(r.H)
}

// clientPoint converts a screen point to client pixels, given the screen
// position of the client area's top-left corner.
func clientPoint(origin win32.POINT, x, y float64) (float64, float64) {
	return x - float64(origin.X), y - float64(origin.Y)
}

// clientOrigin returns the screen position of the top-left corner of the client
// area of hwnd, in physical pixels. It is zero if hwnd is not a window.
// boundsMu guards boundsScratch, which clientOrigin and clientSize use for the
// structures the window calls fill in. A local would escape to the heap through
// the call, once per call.
var (
	boundsMu      sync.Mutex
	boundsScratch struct {
		pt win32.POINT
		r  win32.RECT
	}
)

func clientOrigin(hwnd uintptr) win32.POINT {
	boundsMu.Lock()
	defer boundsMu.Unlock()
	boundsScratch.pt = win32.POINT{}
	win32.ClientToScreen(hwnd, &boundsScratch.pt)
	return boundsScratch.pt
}

// clientSize returns the size of the client area of hwnd in physical pixels.
func clientSize(hwnd uintptr) (w, h int32) {
	boundsMu.Lock()
	defer boundsMu.Unlock()
	win32.GetClientRectTo(hwnd, &boundsScratch.r)
	r := boundsScratch.r
	return r.Right - r.Left, r.Bottom - r.Top
}

// navigateTarget returns the node that navigation in direction dir reaches from
// node id, or 0 for none. The walk over the children allocates nothing, since
// a client navigates on every node it visits.
func navigateTarget(t *access.Tree, id access.ID, dir int32) access.ID {
	switch dir {
	case navFirstChild:
		return edgeChild(t, t.Node(id), false)
	case navLastChild:
		return edgeChild(t, t.Node(id), true)
	case navParent, navNextSibling, navPreviousSibling:
		if id == access.RootID {
			return 0
		}
		parent := access.RootID
		if n := t.Node(id); n != nil && n.Parent != 0 {
			parent = n.Parent
		}
		if dir == navParent {
			if parent == access.RootID || t.Node(parent) != nil {
				return parent
			}
			return 0
		}
		return siblingOf(t, t.Node(parent), id, dir == navNextSibling)
	}
	return 0
}

// edgeChild returns the first (or last) child of n that is in t, or 0.
func edgeChild(t *access.Tree, n *access.Node, last bool) access.ID {
	if n == nil {
		return 0
	}
	if last {
		for i := len(n.Children) - 1; i >= 0; i-- {
			if t.Node(n.Children[i]) != nil {
				return n.Children[i]
			}
		}
		return 0
	}
	for _, c := range n.Children {
		if t.Node(c) != nil {
			return c
		}
	}
	return 0
}

// siblingOf returns the child of parent that follows (or precedes) id among the
// children that are in t, or 0. It is 0 when id is not one of them.
func siblingOf(t *access.Tree, parent *access.Node, id access.ID, next bool) access.ID {
	if parent == nil {
		return 0
	}
	var prev access.ID
	found := false
	for _, c := range parent.Children {
		if t.Node(c) == nil {
			continue
		}
		if found {
			return c // the one after id
		}
		if c == id {
			if !next {
				return prev
			}
			found = true
			continue
		}
		prev = c
	}
	return 0
}

// hitTestTree returns the deepest node under the client point (x, y) in the
// subtree of id, or 0. Children are searched in reverse paint order, so the one
// painted last, which is on top, wins. A node's descendants are searched even
// when they lie outside its bounds, as overlays do.
func hitTestTree(t *access.Tree, id access.ID, x, y float32) access.ID {
	n := t.Node(id)
	if n == nil {
		return 0
	}
	for i := len(n.Children) - 1; i >= 0; i-- {
		c := t.Node(n.Children[i])
		if c == nil {
			continue
		}
		if hit := hitTestTree(t, c.ID, x, y); hit != 0 {
			return hit
		}
		if c.Bounds.Contains(x, y) {
			return c.ID
		}
	}
	return 0
}

// hitTestPoint returns what the client point (x, y) hits in a client area of w
// by h pixels: 0 when the point is outside the area, access.RootID when no node
// is under it, and otherwise the deepest node under it.
func hitTestPoint(t *access.Tree, x, y float64, w, h int32) access.ID {
	if x < 0 || y < 0 || x >= float64(w) || y >= float64(h) {
		return 0
	}
	if id := hitTestTree(t, access.RootID, float32(x), float32(y)); id != 0 {
		return id
	}
	return access.RootID
}

// current returns the snapshot node that c stands for, and the snapshot. The
// root answers as the window: its node is the tree's root node, named by the
// window title when the node has no name of its own. ok is false when the node
// is no longer in the snapshot. The snapshot is read only; it is immutable.
func (c *core) current() (n *access.Node, t *access.Tree, ok bool) {
	t = c.bridge.Tree()
	if c.kind == kindRoot {
		return c.rootNode(t), t, true
	}
	n = t.Node(c.id)
	return n, t, n != nil
}

// rootNode returns the root's node for the snapshot t. It is made once for each
// snapshot, not for each call, because UI Automation asks for the root's
// properties and children very often. The window title is read at that time, so
// a title change is seen with the next snapshot.
func (c *core) rootNode(t *access.Tree) *access.Node {
	if s := c.root.Load(); s != nil && s.tree == t {
		return &s.node
	}
	s := &rootSnap{tree: t, node: access.Node{ID: access.RootID, Role: access.RoleWindow}}
	if src := t.Node(access.RootID); src != nil {
		s.node = *src
	}
	if s.node.Name == "" {
		s.node.Name = win32.GetWindowText(c.hwnd)
	}
	c.root.Store(s)
	return &s.node
}

// navigate returns a new reference to the fragment that dir reaches from c, or
// 0 for none.
func (c *core) navigate(dir int32) (uintptr, uintptr) {
	_, t, ok := c.current()
	if !ok {
		return 0, hrElementNotAvailable
	}
	// The target is found in the same snapshot that showed c, so the two agree.
	id := navigateTarget(t, c.id, dir)
	if id == 0 {
		return 0, hrOK
	}
	p := c.provide(id, iidRawElementProviderFragment)
	if p == 0 {
		return 0, hrElementNotAvailable // the target left the tree since t was published
	}
	return p, hrOK
}

// runtimeID returns a new SAFEARRAY that the caller owns. The root has no
// runtime ID and returns 0.
func (c *core) runtimeID() (uintptr, uintptr) {
	if c.kind == kindRoot {
		return 0, hrOK
	}
	if _, _, ok := c.current(); !ok {
		return 0, hrElementNotAvailable
	}
	psa := newRuntimeID(c.id)
	if psa == 0 {
		return 0, hrFail
	}
	return psa, hrOK
}

// boundingRect returns the screen rectangle of c in physical pixels. The root's
// is the client area.
func (c *core) boundingRect() (x, y, w, h float64, hr uintptr) {
	n, _, ok := c.current()
	if !ok {
		return 0, 0, 0, 0, hrElementNotAvailable
	}
	origin := clientOrigin(c.hwnd)
	if c.kind == kindRoot {
		cw, ch := clientSize(c.hwnd)
		x, y, w, h = screenRect(origin, access.Rect{W: float32(cw), H: float32(ch)})
		return x, y, w, h, hrOK
	}
	x, y, w, h = screenRect(origin, n.Bounds)
	return x, y, w, h, hrOK
}

// property returns the VARIANT of property id for c.
func (c *core) property(id int32) (variant, uintptr) {
	n, t, ok := c.current()
	if !ok {
		return variant{}, hrElementNotAvailable
	}
	switch id {
	case propLabeledBy:
		// The element that labels this one is a provider, not a value.
		if c.kind == kindNode && len(n.LabelledBy) > 0 {
			if p := c.provide(n.LabelledBy[0], iidRawElementProviderSimple); p != 0 {
				return variantUnknown(p), hrOK
			}
		}
		return variant{}, hrOK
	case propDescribedBy:
		return c.describedBy(n)
	}
	return nodeProperty(id, n, t != nil && t.Focus == n.ID), hrOK
}

// describedBy returns the VARIANT of the DescribedBy property of n: an array of
// the providers that describe it, or VT_EMPTY when nothing describes it.
func (c *core) describedBy(n *access.Node) (variant, uintptr) {
	var ps []uintptr
	if c.kind == kindNode {
		for _, id := range n.DescribedBy {
			if p := c.provide(id, iidRawElementProviderSimple); p != 0 {
				ps = append(ps, p)
			}
		}
	}
	if len(ps) == 0 {
		return variant{}, hrOK
	}
	psa := win32.SafeArrayCreateVector(vtUnknown, 0, uint32(len(ps)))
	if psa == 0 {
		releaseAll(ps)
		return variant{}, hrFail
	}
	for i, p := range ps {
		win32.SafeArrayPutUnknown(psa, int32(i), p)
	}
	releaseAll(ps) // the array holds its own references
	return variantUnknownArray(psa), hrOK
}

// releaseAll releases the references in ps.
func releaseAll(ps []uintptr) {
	for _, p := range ps {
		release(p)
	}
}

// pattern returns a new reference to the pattern object of kind c.id supports
// for the pattern id, and 0 when the node does not have that pattern.
func (c *core) pattern(id int32) (uintptr, uintptr) {
	k, ok := patternKindOf(id)
	if c.kind != kindNode || !ok {
		return 0, hrOK
	}
	n, _, ok := c.current()
	if !ok {
		return 0, hrElementNotAvailable
	}
	if !supports(k, n) {
		return 0, hrOK
	}
	p := cachedObject(k, c.id, c.bridge, c.hwnd, kindIfaces[k][0].iid)
	if p == 0 {
		return 0, hrElementNotAvailable // the node left the tree since n was read
	}
	return p, hrOK
}

// perform asks the runtime to run action a on c, from any thread. It returns
// the HRESULT: the node may have gone.
func (c *core) perform(a access.Action) uintptr {
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Perform(c.id, a)
	return hrOK
}

// toggle runs the toggle of a check box, switch or toggle button.
func (c *core) toggle() uintptr { return c.perform(access.ActionToggle) }

// toggleState returns the ToggleState of c.
func (c *core) toggleState() (int32, uintptr) {
	n, _, ok := c.current()
	if !ok {
		return 0, hrElementNotAvailable
	}
	return toggleStateOf(n.States), hrOK
}

// expand opens or closes c.
func (c *core) expand(open bool) uintptr {
	if open {
		return c.perform(access.ActionExpand)
	}
	return c.perform(access.ActionCollapse)
}

// expandState returns the ExpandCollapseState of c.
func (c *core) expandState() (int32, uintptr) {
	n, _, ok := c.current()
	if !ok {
		return 0, hrElementNotAvailable
	}
	return expandStateOf(n), hrOK
}

// selectItem chooses c in its container. An item that is chosen already stays so.
func (c *core) selectItem() uintptr { return c.perform(access.ActionSelect) }

// isSelected reports whether c is chosen.
func (c *core) isSelected() (bool, uintptr) {
	n, _, ok := c.current()
	if !ok {
		return false, hrElementNotAvailable
	}
	return n.States.Has(access.StateSelected), hrOK
}

// selectionContainer returns a new reference to the nearest selection container
// above c, or 0 when there is none.
func (c *core) selectionContainer() (uintptr, uintptr) {
	n, t, ok := c.current()
	if !ok {
		return 0, hrElementNotAvailable
	}
	for id := n.Parent; id != 0; {
		p := t.Node(id)
		if p == nil {
			break
		}
		if p.States.Has(access.StateSelectionContainer) {
			v := c.provide(p.ID, iidRawElementProviderSimple)
			if v == 0 {
				return 0, hrElementNotAvailable
			}
			return v, hrOK
		}
		id = p.Parent
	}
	return 0, hrOK
}

// selection returns a new SAFEARRAY of the chosen items below the container c,
// which the caller owns. An empty container gives an empty array.
func (c *core) selection() (uintptr, uintptr) {
	n, t, ok := c.current()
	if !ok {
		return 0, hrElementNotAvailable
	}
	var ids []access.ID
	collectSelected(t, n, &ids)
	ps := make([]uintptr, 0, len(ids))
	for _, id := range ids {
		if p := c.provide(id, iidRawElementProviderSimple); p != 0 {
			ps = append(ps, p)
		}
	}
	psa := win32.SafeArrayCreateVector(vtUnknown, 0, uint32(len(ps)))
	if psa == 0 {
		releaseAll(ps)
		return 0, hrFail
	}
	for i, p := range ps {
		win32.SafeArrayPutUnknown(psa, int32(i), p)
	}
	releaseAll(ps)
	return psa, hrOK
}

// collectSelected appends the chosen items below n to out. It does not look
// inside a nested selection container, which answers for its own items.
func collectSelected(t *access.Tree, n *access.Node, out *[]access.ID) {
	for _, id := range n.Children {
		c := t.Node(id)
		if c == nil {
			continue
		}
		if c.States.Has(access.StateSelectable) && c.States.Has(access.StateSelected) {
			*out = append(*out, id)
		}
		if !c.States.Has(access.StateSelectionContainer) {
			collectSelected(t, c, out)
		}
	}
}

// setValue replaces the text of c with the NUL-terminated string at val. The
// runtime sets the field's signal on the UI thread, as typing does.
func (c *core) setValue(val uintptr) uintptr {
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Do(c.id, access.Request{Action: access.ActionSetValue, Value: wideString(val)})
	return hrOK
}

// value returns a new BSTR holding the value of c, which the caller owns.
func (c *core) value() (uintptr, uintptr) {
	n, _, ok := c.current()
	if !ok {
		return 0, hrElementNotAvailable
	}
	return bstrOf(n.Value), hrOK
}

// isReadOnly reports whether the text of c can only be read.
func (c *core) isReadOnly() (bool, uintptr) {
	n, _, ok := c.current()
	if !ok {
		return false, hrElementNotAvailable
	}
	return valueReadOnly(n), hrOK
}

// setRangeValue sets the number of the slider or spin box c. The runtime sets it
// on the UI thread, as setValue does for a text field.
func (c *core) setRangeValue(v float64) uintptr {
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Do(c.id, access.Request{Action: access.ActionSetRangeValue, X: v})
	return hrOK
}

// rangeOf returns the range of c. A node without a Range has the zero range.
func (c *core) rangeOf() (access.Range, uintptr) {
	n, _, ok := c.current()
	if !ok {
		return access.Range{}, hrElementNotAvailable
	}
	if n.Range == nil {
		return access.Range{}, hrOK
	}
	return *n.Range, hrOK
}

// scroll moves the container c by the ScrollAmount values of each axis: a small
// amount is a tenth of the view, a large one nine tenths (see ActionScroll).
func (c *core) scroll(horizontal, vertical int32) uintptr {
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Do(c.id, access.Request{Action: access.ActionScroll,
		X: scrollFraction(horizontal), Y: scrollFraction(vertical)})
	return hrOK
}

// scrollFraction returns the fraction of the view that a ScrollAmount moves.
func scrollFraction(amount int32) float64 {
	switch amount {
	case scrollLargeDecrement:
		return -0.9
	case scrollSmallDecrement:
		return -0.1
	case scrollLargeIncrement:
		return 0.9
	case scrollSmallIncrement:
		return 0.1
	}
	return 0 // NoAmount
}

// setScrollPercent moves the container c to horizontal and vertical percents of
// its range. A negative percent leaves its axis where it is.
func (c *core) setScrollPercent(horizontal, vertical float64) uintptr {
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Do(c.id, access.Request{Action: access.ActionSetScrollPercent, X: horizontal, Y: vertical})
	return hrOK
}

// scrollState returns the scroll state of the container c.
func (c *core) scrollState() (scrollState, uintptr) {
	n, _, ok := c.current()
	if !ok {
		return scrollState{}, hrElementNotAvailable
	}
	return scrollState(n.Scroll), hrOK
}

// scrollIntoView scrolls c into view in its scroll container.
func (c *core) scrollIntoView() uintptr { return c.perform(access.ActionScrollIntoView) }

// setFocus asks the runtime to move keyboard focus to c. The root has nothing to
// focus and does nothing.
func (c *core) setFocus() uintptr {
	if c.kind == kindRoot {
		return hrOK
	}
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Perform(c.id, access.ActionFocus)
	return hrOK
}

// invoke asks the runtime to click c. The runtime runs the click on the UI
// thread, so this returns at once.
func (c *core) invoke() uintptr {
	if _, _, ok := c.current(); !ok {
		return hrElementNotAvailable
	}
	c.bridge.Perform(c.id, access.ActionClick)
	return hrOK
}

// focused returns a new reference to the node with keyboard focus, or 0.
func (c *core) focused() (uintptr, uintptr) {
	t := c.bridge.Tree()
	if t == nil || t.Focus == 0 || t.Node(t.Focus) == nil {
		return 0, hrOK
	}
	return c.provide(t.Focus, iidRawElementProviderFragment), hrOK
}

// The window's accessibility. The window answers WM_GETOBJECT with its root
// object, and listens for the snapshots that its runtime publishes. Both run on
// the UI thread.

// answerAccess answers WM_GETOBJECT for UI Automation. It returns false, and
// the window procedure uses DefWindowProc, when the request is for another
// object, or when the handler has no access bridge yet.
func (w *window) answerAccess(wp, lp uintptr) (uintptr, bool) {
	if int32(lp) != uiaRootObjectID {
		return 0, false
	}
	ah, ok := w.handler.(platform.AccessHandler)
	if !ok {
		return 0, false
	}
	b := ah.AccessBridge()
	if b == nil {
		return 0, false
	}
	if w.uiaRoot == 0 {
		// The window keeps one reference to its root, released when the
		// window is destroyed. UI Automation takes its own reference when it
		// receives the provider.
		w.uiaRoot = rootView(w.hwnd, b, iidRawElementProviderSimple)
		w.uiaBridge = b
		b.Listen(func(u access.Update) { w.onAccess(b, u) })
		b.Activate()
	}
	return win32.UiaReturnRawElementProvider(w.hwnd, wp, lp, w.uiaRoot), true
}

// onAccess reports a published snapshot to UI Automation. It drops the objects
// of nodes that left the tree, and raises the events the snapshot implies, but
// only when a client listens. The events are derived from the Update, which
// the runtime makes once per published snapshot, so they are coalesced per frame.
func (w *window) onAccess(b *access.Bridge, u access.Update) {
	for _, id := range u.Removed {
		dropObjects(w.hwnd, id)
	}
	if w.uiaRoot == 0 || !win32.UiaClientsAreListening() {
		return
	}
	t := b.Tree()
	if u.FocusChanged && u.Focus != 0 {
		w.raiseOn(b, u.Focus, eventAutomationFocusChanged)
	}
	if u.Structure {
		win32.UiaRaiseStructureChangedEvent(w.uiaRoot, structureChildrenInvalidated, 0, 0)
	}
	for _, pc := range u.Properties {
		if n := t.Node(pc.ID); n != nil {
			w.raiseProperty(b, n, pc.Property, t.Focus == n.ID)
		}
	}
	// The text events are raised on the field, which holds the text pattern.
	for _, id := range u.TextSelection {
		w.raiseOn(b, id, eventTextSelectionChanged)
	}
	for _, id := range u.TextChanged {
		w.raiseOn(b, id, eventTextChanged)
	}
	for _, p := range u.Opened {
		event := int32(eventMenuOpened)
		if p.Role.IsDialog() {
			event = eventWindowOpened
		}
		w.raiseOn(b, p.ID, event)
	}
	for _, p := range u.Closed {
		// The popup has left the tree, so its closing is raised on the window.
		event := int32(eventMenuClosed)
		if p.Role.IsDialog() {
			event = eventWindowClosed
		}
		win32.UiaRaiseAutomationEvent(w.uiaRoot, event)
	}
	for _, a := range u.Announcements {
		w.announce(a)
	}
}

// raiseOn raises the automation event for node id, if the node is in the
// snapshot. The provider is the node's object, so the client sees the element
// it is told about.
func (w *window) raiseOn(b *access.Bridge, id access.ID, event int32) {
	if b.Tree().Node(id) == nil {
		return
	}
	p := cachedObject(kindNode, id, b, w.hwnd, iidRawElementProviderSimple)
	if p == 0 {
		return
	}
	win32.UiaRaiseAutomationEvent(p, event)
	release(p)
}

// raiseProperty raises the property-changed event of n for prop, with the value
// that the property has now. The old value is empty: the adapter does not keep
// the values it reported before. The new value is copied by UI Automation, so
// the BSTR it holds is freed here.
func (w *window) raiseProperty(b *access.Bridge, n *access.Node, prop access.Property, focused bool) {
	id := propertyID(prop)
	p := cachedObject(kindNode, n.ID, b, w.hwnd, iidRawElementProviderSimple)
	if p == 0 {
		return
	}
	oldV, newV := newVariant(), newVariant()
	*newV = nodeProperty(id, n, focused)
	win32.UiaRaiseAutomationPropertyChangedEvent(p, id, uintptr(unsafe.Pointer(oldV)), uintptr(unsafe.Pointer(newV)))
	runtime.KeepAlive(oldV) // the VARIANTs are given as uintptrs, which do not keep them alive
	runtime.KeepAlive(newV)
	if newV.vt == vtBSTR {
		win32.SysFreeString(variantBSTRValue(*newV))
	}
	release(p)
}

// propertyID returns the UI Automation property that an access property is.
func propertyID(prop access.Property) int32 {
	switch prop {
	case access.PropertyValue:
		return propValue
	case access.PropertyToggle:
		return propToggleState
	case access.PropertyExpanded:
		return propExpandCollapseState
	case access.PropertySelected:
		return propIsSelected
	case access.PropertyEnabled:
		return propIsEnabled
	case access.PropertyRangeValue:
		return propRangeValue
	}
	return propName
}

// variantBSTRValue returns the BSTR that a VARIANT holds.
func variantBSTRValue(v variant) uintptr { return *(*uintptr)(unsafe.Pointer(&v.data[0])) }

// announce raises a notification event on the window for an announcement. A
// polite announcement is queued behind the ones before it, and an assertive one
// interrupts the speech in progress and then says the most recent one. The
// activity ID names the source of the notification.
func (w *window) announce(a access.Announcement) {
	activity := a.Source
	if activity == "" {
		activity = "jamb"
	}
	display, activityID := bstrOf(a.Text), bstrOf(activity)
	win32.UiaRaiseNotificationEvent(w.uiaRoot, notificationKindOther, notificationProcessingOf(a), display, activityID)
	win32.SysFreeString(display)
	win32.SysFreeString(activityID)
}

// notificationProcessingOf returns the NotificationProcessing of an announcement
// a. Assertive is ImportantMostRecent: presented as soon as possible, and a newer
// message supersedes an older one. Polite is All: presented when possible, and
// every message is delivered. A polite announcement with a source is MostRecent,
// so a newer message from the same source supersedes the ones still waiting.
// Microsoft warns that ImportantAll can flood the user, and
// ImportantCurrentThenMostRecent exists only from Windows build 26100.
func notificationProcessingOf(a access.Announcement) int32 {
	switch {
	case a.Politeness == access.Assertive:
		return notificationImportantMostRecent
	case a.Source != "":
		return notificationMostRecent
	}
	return notificationAll
}

// endAccess ends the window's accessibility when it is destroyed. It stops the
// listener and releases the window's reference to its root.
func (w *window) endAccess() {
	if w.uiaBridge != nil {
		w.uiaBridge.Listen(nil)
		w.uiaBridge = nil
	}
	if w.uiaRoot == 0 {
		return
	}
	win32.UiaReturnRawElementProvider(w.hwnd, 0, 0, 0)
	win32.UiaDisconnectProvider(w.uiaRoot)
	dropAllObjects(w.hwnd)
	forgetRoot(w.hwnd)
	forgetHost(w.hwnd)
	release(w.uiaRoot)
	w.uiaRoot = 0
}
