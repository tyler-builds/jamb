// Package access is jamb's platform-neutral accessibility model. The ui
// runtime describes its UI as a tree of Nodes, each with a role, a name,
// states, actions and bounds. A platform adapter (platform/windows for
// UI Automation) translates the tree to the operating system's API, and
// translates the operating system's requests back into Actions on the UI
// thread. The package has no dependencies, so it can be used by any adapter.
//
// The tree is published as an immutable *Tree snapshot. Adapters read it from
// any goroutine; the ui runtime builds a new snapshot on the UI thread only
// when something changed, and Diff reports what differs from the previous one.
// See docs/accessibility.md for the design.
package access

import (
	"fmt"
	"strings"
)

// ID identifies a node. It is stable across frames for the same element, and
// zero means no node. The ui runtime derives IDs from its element IDs.
type ID uint64

// RootID is the ID of the window's root node. The ui runtime always makes it
// the tree's root, and it is never the ID of an element.
const RootID ID = 1

// Role says what kind of control a node is. The values follow the roles that
// UI Automation and WAI-ARIA name, so that each adapter maps them directly.
type Role uint8

const (
	RoleUnknown       Role = iota // no role; the node is not exposed as a control
	RoleWindow                    // the window, the root of the tree
	RoleGroup                     // a container with a name or a role of its own
	RoleButton                    // a push button, including icon-only buttons
	RoleCheckBox                  // a check box with checked, mixed or unchecked state
	RoleSwitch                    // an on/off switch
	RoleRadioButton               // one choice in a group of radio buttons
	RoleTextField                 // a single-line text field
	RoleTextArea                  // a multi-line text field
	RoleStaticText                // text that is not part of a named control
	RoleImage                     // an image with alternative text
	RoleLink                      // a link: a clickable run of text
	RoleList                      // a list of items, such as a UniformList or ListOf
	RoleListItem                  // one row of a list, with its position in the set
	RoleMenuBar                   // the application's menu bar
	RoleMenu                      // a menu, or the panel of a submenu
	RoleMenuItem                  // a command in a menu
	RoleMenuItemCheck             // a checkable command in a menu (check mark)
	RoleMenuItemRadio             // a choice in a menu (radio dot)
	RoleSeparator                 // a rule between groups of menu items
	RoleDialog                    // a modal dialog, named by its title
	RoleTooltip                   // a tooltip that describes the element it hangs from
	RoleScrollArea                // a scrolling container
	RoleHeading                   // a heading; its Heading field gives the level
	RoleTabList                   // a row of tabs
	RoleTab                       // one tab of a tab list
	RoleTabPanel                  // the content of a tab
)

var roleNames = [...]string{
	RoleUnknown:       "unknown",
	RoleWindow:        "window",
	RoleGroup:         "group",
	RoleButton:        "button",
	RoleCheckBox:      "check box",
	RoleSwitch:        "switch",
	RoleRadioButton:   "radio button",
	RoleTextField:     "text field",
	RoleTextArea:      "text area",
	RoleStaticText:    "static text",
	RoleImage:         "image",
	RoleLink:          "link",
	RoleList:          "list",
	RoleListItem:      "list item",
	RoleMenuBar:       "menu bar",
	RoleMenu:          "menu",
	RoleMenuItem:      "menu item",
	RoleMenuItemCheck: "menu item check",
	RoleMenuItemRadio: "menu item radio",
	RoleSeparator:     "separator",
	RoleDialog:        "dialog",
	RoleTooltip:       "tooltip",
	RoleScrollArea:    "scroll area",
	RoleHeading:       "heading",
	RoleTabList:       "tab list",
	RoleTab:           "tab",
	RoleTabPanel:      "tab panel",
}

// String returns the role's name, such as "check box".
func (r Role) String() string {
	if int(r) < len(roleNames) {
		return roleNames[r]
	}
	return fmt.Sprintf("role(%d)", uint8(r))
}

// Interactive reports whether a user operates a node of this role, so that it
// needs an accessible name (see the development warnings).
func (r Role) Interactive() bool {
	switch r {
	case RoleButton, RoleCheckBox, RoleSwitch, RoleRadioButton, RoleTextField,
		RoleTextArea, RoleLink, RoleMenuItem, RoleMenuItemCheck, RoleMenuItemRadio,
		RoleTab:
		return true
	}
	return false
}

// State is a set of flags that describe a node's condition.
type State uint32

const (
	StateFocusable State = 1 << iota // the node can take keyboard focus
	StateFocused                     // the node has keyboard focus
	StateDisabled                    // the node is shown but cannot be operated
	StateChecked                     // a check box, switch or radio button is on; a menu item is checked
	StateMixed                       // a check box is in the indeterminate state
	StateExpanded                    // the node is open (a menu, a tree item)
	StateSelected                    // the node is selected (a list item, a tab)
	StateReadOnly                    // a text field can be read but not edited
	StateRequired                    // a field must be filled in
	StateInvalid                     // a field's value is not valid
	StateOffscreen                   // the node is clipped out of view by a scroll container
	StateMultiLine                   // a text field takes several lines
)

// Has reports whether all the flags in f are set.
func (s State) Has(f State) bool { return s&f == f }

var stateNames = [...]struct {
	flag State
	name string
}{
	{StateFocusable, "focusable"},
	{StateFocused, "focused"},
	{StateDisabled, "disabled"},
	{StateChecked, "checked"},
	{StateMixed, "mixed"},
	{StateExpanded, "expanded"},
	{StateSelected, "selected"},
	{StateReadOnly, "read-only"},
	{StateRequired, "required"},
	{StateInvalid, "invalid"},
	{StateOffscreen, "offscreen"},
	{StateMultiLine, "multi-line"},
}

// String lists the flags that are set, such as "focusable focused".
func (s State) String() string {
	var parts []string
	for _, sn := range stateNames {
		if s.Has(sn.flag) {
			parts = append(parts, sn.name)
		}
	}
	return strings.Join(parts, " ")
}

// Action is an operation a user or an assistive technology can request on a
// node. The ui runtime runs the request on the UI thread.
type Action uint8

const (
	ActionClick          Action = iota // invoke: press the control
	ActionFocus                        // move keyboard focus to the node
	ActionExpand                       // open the node
	ActionCollapse                     // close the node
	ActionToggle                       // change a check box or switch
	ActionSelect                       // select a list item, tab or radio button
	ActionSetValue                     // replace a text field's value
	ActionScrollIntoView               // scroll the node into view
	ActionIncrement                    // raise a value (a slider or spin box)
	ActionDecrement                    // lower a value
)

var actionNames = [...]string{
	ActionClick:          "click",
	ActionFocus:          "focus",
	ActionExpand:         "expand",
	ActionCollapse:       "collapse",
	ActionToggle:         "toggle",
	ActionSelect:         "select",
	ActionSetValue:       "set value",
	ActionScrollIntoView: "scroll into view",
	ActionIncrement:      "increment",
	ActionDecrement:      "decrement",
}

// String returns the action's name, such as "click".
func (a Action) String() string {
	if int(a) < len(actionNames) {
		return actionNames[a]
	}
	return fmt.Sprintf("action(%d)", uint8(a))
}

// Rect is a rectangle in physical pixels, relative to the top-left corner of
// the window's client area. Adapters convert it to screen coordinates.
type Rect struct {
	X, Y, W, H float32
}

// Empty reports whether the rectangle has no area.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Contains reports whether the point (x, y) lies inside the rectangle.
func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

// Politeness says how urgently an announcement is spoken.
type Politeness uint8

const (
	// Polite waits until the assistive technology is idle, as for "Saved".
	Polite Politeness = iota
	// Assertive interrupts the current speech, as for an error.
	Assertive
)

// String returns "polite" or "assertive".
func (p Politeness) String() string {
	if p == Assertive {
		return "assertive"
	}
	return "polite"
}

// Announcement is text to speak for a status message that has no focused
// element to carry it, such as "3 results".
type Announcement struct {
	Text       string
	Politeness Politeness
}

// Node is one element of the access tree. A Tree holds its nodes; a Node is
// not changed after the tree is published.
type Node struct {
	ID          ID
	Role        Role
	Name        string // the accessible name: what a screen reader says first
	Description string // help text, read after the name
	// Key is the element's key in the ui package, exposed as the automation
	// ID so that tests can find a node by something other than its name.
	Key         string
	Value       string // the current value of a text field (phase B)
	States      State
	Actions     []Action
	Bounds      Rect
	Parent      ID
	Children    []ID
	LabelledBy  []ID // nodes whose names name this one, in order
	DescribedBy []ID // nodes whose text describes this one, in order
	Controls    []ID // nodes this one opens or controls, such as a menu
	Heading     int  // the level of a RoleHeading, 1 to 6
	PosInSet    int  // 1-based position among its siblings in a set, such as a row of a list; 0 when not in a set
	SetSize     int  // the number of items in the set; 0 when not in a set
	Level       int  // the nesting depth of a menu or tree item, 1 for the top; 0 when not set
}

// Has reports whether the node supports action a.
func (n *Node) Has(a Action) bool {
	for _, x := range n.Actions {
		if x == a {
			return true
		}
	}
	return false
}

// Equal reports whether two nodes describe the same thing. Diff uses it to
// find the nodes that changed.
func (n *Node) Equal(o *Node) bool {
	if n == o {
		return true
	}
	if n == nil || o == nil {
		return false
	}
	return n.ID == o.ID && n.Role == o.Role && n.Name == o.Name && n.Description == o.Description &&
		n.Key == o.Key && n.Value == o.Value && n.States == o.States && n.Bounds == o.Bounds &&
		n.Parent == o.Parent && n.Heading == o.Heading && n.PosInSet == o.PosInSet &&
		n.SetSize == o.SetSize && n.Level == o.Level &&
		equalIDs(n.Children, o.Children) && equalIDs(n.LabelledBy, o.LabelledBy) &&
		equalIDs(n.DescribedBy, o.DescribedBy) && equalIDs(n.Controls, o.Controls) &&
		equalActions(n.Actions, o.Actions)
}

func equalIDs(a, b []ID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalActions(a, b []Action) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
