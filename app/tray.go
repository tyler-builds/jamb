package app

import (
	"fmt"
	"hash/fnv"
	"image"
	"os"
	"sync/atomic"

	"github.com/tyler-builds/jamb/ui"
)

// TrayOptions describes a notification area icon; see NewTray.
type TrayOptions struct {
	// Icon is the image of the icon, drawn at the small icon size of the current
	// DPI. Nil uses the executable's icon, which jamb package embeds, or the
	// standard application icon when the executable has none.
	Icon image.Image
	// Tooltip is the text shown when the pointer rests on the icon. Windows shows
	// at most 127 UTF-16 units of it.
	Tooltip string
	// OnClick runs on the UI thread for a click, and for Enter or Space when the
	// icon has the keyboard focus. A double click runs OnClick for its first click,
	// as Windows reports both, and then OnDoubleClick. An app that wants different
	// actions for the two should make OnClick idempotent, or rely on OnDoubleClick
	// alone and leave OnClick nil.
	OnClick func()
	// OnDoubleClick runs on the UI thread for a double click.
	OnDoubleClick func()
	// Menu returns the menu, which is shown on a right click, and with the context
	// menu key or Shift+F10. It runs on the UI thread each time the menu opens, so
	// its checked items show the app's state. Nil means the icon has no menu.
	Menu func() []TrayItem
}

// TrayItem is an item of a tray menu. A tray menu is Windows' native menu, so it is a
// subset of what the menus of ui can show: an item has a label, a disabled state, a check
// mark and a submenu, but no icon, shortcut or action. Its OnClick runs on the UI thread
// after the menu closes, when the user chooses the item. A separator is made by
// TraySeparator.
type TrayItem struct {
	// Label is the text of the item.
	Label string
	// Disabled shows the item, but it cannot be chosen.
	Disabled bool
	// Checked shows a check mark beside the label.
	Checked bool
	// Submenu opens a menu of its items to the right, instead of running OnClick.
	Submenu []TrayItem
	// OnClick runs when the item is chosen.
	OnClick func()

	separator bool
}

// TraySeparator returns a rule between groups of menu items.
func TraySeparator() TrayItem { return TrayItem{separator: true} }

// Tray is a notification area icon: the icon at the edge of the taskbar, which
// the user can click and right click. Make one with NewTray and add it with Show.
//
// The methods are safe from any goroutine. They run on the UI thread in the order
// they are called, so a call before the app runs waits for it to start.
//
// A tray icon needs no window. The icon, its tooltip and its menu are Windows'
// own: the menu follows the system theme, and assistive technology reads it as
// the system's menus. Windows keeps the user's choice to show or hide an icon
// by its identity: with an AppID, the identity is derived from the AppID and the
// icon's place in the order of NewTray calls, so keep that order from run to
// run. Windows also ties that identity to the executable's path, and an icon
// whose executable moved is added by its ID instead.
type Tray struct {
	id   uint32
	opts TrayOptions // set by NewTray only; the callbacks are read from it

	// The fields below are read and written on the UI thread only.
	tooltip string
	icon    image.Image
	native  trayIcon
	actions map[uint32]func() // the actions of the menu last shown, by item ID
}

// trayIcon is the platform's notification area icon. Windows has one (see
// tray_windows.go); elsewhere the app has none.
type trayIcon interface {
	Show() error
	SetTooltip(s string)
	SetIcon(img image.Image) error
	Remove()
}

// shownTrays is the number of the app's tray icons that are shown. It is read and
// written on the UI thread only.
var shownTrays int

// closeHides reports whether a close the user asks for hides the window instead:
// CloseToTray holds true, a tray icon is shown to bring the window back, and the
// close is not the app's own (asked).
func closeHides(closeToTray *ui.Signal[bool], shown int, asked bool) bool {
	return !asked && closeToTray != nil && closeToTray.Peek() && shown > 0
}

// trayCount numbers the trays: the number is the icon's ID, which is unique in
// the process. The shell reads IDs as 16-bit values, so the 65535th is the last.
var trayCount atomic.Uint32

// NewTray returns a notification area icon. It is not shown until Show is called.
func NewTray(o TrayOptions) *Tray {
	return &Tray{id: trayCount.Add(1), tooltip: o.Tooltip, icon: o.Icon, actions: map[uint32]func(){}, opts: o}
}

// Show adds the icon to the notification area. Showing an icon that is shown
// does nothing. A failure, such as a refused icon, is printed, and the icon is
// not shown; Show may be called again. It is safe from any goroutine.
func (t *Tray) Show() {
	postUI(func() {
		if t.native != nil {
			return
		}
		if t.id > 0xFFFF {
			fmt.Fprintln(os.Stderr, "jamb: NewTray: more than 65535 tray icons")
			return
		}
		n := newTrayIcon(t)
		if err := n.Show(); err != nil {
			fmt.Fprintln(os.Stderr, "jamb: tray icon:", err)
			return
		}
		t.native = n
		shownTrays++
	})
}

// SetTooltip sets the tooltip of the icon. It is safe from any goroutine.
func (t *Tray) SetTooltip(s string) {
	postUI(func() {
		t.tooltip = s
		if t.native != nil {
			t.native.SetTooltip(s)
		}
	})
}

// SetIcon sets the image of the icon; nil is the executable's icon. It is safe
// from any goroutine.
func (t *Tray) SetIcon(img image.Image) {
	postUI(func() {
		t.icon = img
		if t.native != nil {
			if err := t.native.SetIcon(img); err != nil {
				fmt.Fprintln(os.Stderr, "jamb: tray icon:", err)
			}
		}
	})
}

// Hide takes the icon out of the notification area, and Show adds it again. An
// app's icons are also removed when the app quits. It is safe from any goroutine.
func (t *Tray) Hide() {
	postUI(func() {
		if t.native != nil {
			t.native.Remove()
			t.native = nil
			shownTrays--
		}
	})
}

// Shown reports whether the icon is in the notification area: Show has added
// it, and Hide has not taken it out. An icon stays shown when Explorer restarts,
// as it is added to the new notification area again. It is safe from any
// goroutine: from another goroutine, it waits for the UI thread to run the calls
// made before it, and it reports false once the app has quit.
func (t *Tray) Shown() bool { return readUI(false, func() bool { return t.native != nil }) }

// trayGUIDNamespace is the namespace of the tray icon GUIDs: a fixed UUID made
// for jamb. It must not change, or the icons lose the user's choices.
var trayGUIDNamespace = [16]byte{0x6f, 0x4a, 0x2c, 0x9e, 0x3b, 0x1d, 0x4e, 0x7a, 0x8c, 0x5f, 0x0b, 0x2d, 0x9a, 0x61, 0x7e, 0x34}

// trayGUID returns the identity of tray icon id of the app with appID, in the byte
// order of an RFC 9562 UUID: the 128-bit FNV-1a hash of the namespace and the name
// "jamb:tray:" + appID + ":" + id, marked as a version 8 (custom) UUID. The same app
// and the same icon give the same GUID in every run. It is not a cryptographic hash,
// which the identity does not need, so the package links no crypto package (whose
// import would add the 32 MB buffer of Go's FIPS module to every app).
func trayGUID(appID string, id uint32) [16]byte {
	h := fnv.New128a()
	h.Write(trayGUIDNamespace[:])
	fmt.Fprintf(h, "jamb:tray:%s:%d", appID, id)
	var u [16]byte
	copy(u[:], h.Sum(nil))
	u[6] = u[6]&0x0f | 0x80 // version 8, custom
	u[8] = u[8]&0x3f | 0x80 // the RFC 9562 variant
	return u
}
