//go:build windows

package windows

import (
	"errors"
	"image"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/tyler-builds/jamb/internal/win32"
)

// trayMsg is the message the notification area sends the tray window for its
// icons. With NOTIFYICON_VERSION_4 the event is in the low word of lParam, the
// icon's ID in the high word, and the anchor point of the icon in wParam.
const trayMsg = win32.WM_APP + 4

// tipLimit is the most UTF-16 units of a tooltip. szTip holds 128, with the
// terminating NUL.
const tipLimit = 127

// trayHost is the hidden window that receives the notification area's messages
// for every icon of the process. It is a top-level window, not a message-only
// one, because the shell sends TaskbarCreated only to top-level windows. The UI
// thread owns it.
var trayHost struct {
	hwnd    uintptr
	dpi     uint32
	created uint32 // the TaskbarCreated message: the notification area was made again
	icons   map[uint32]*TrayIcon
}

// TrayOptions describes a notification area icon.
type TrayOptions struct {
	// ID is the icon's uID, which is unique in the process and fits in 16 bits.
	ID uint32
	// GUID is the icon's identity in the byte order of an RFC 4122 UUID. Windows
	// keeps the user's choice to show or hide the icon by it. Nil identifies the
	// icon by ID alone.
	GUID    *[16]byte
	Tooltip string
	// Icon is the image of the icon; nil is the executable's icon.
	Icon image.Image
	// Click runs for a click, and for Enter or Space on the focused icon.
	Click func()
	// DoubleClick runs for a double click.
	DoubleClick func()
	// Menu returns the menu, which is shown on a right click, and with the
	// context menu key or Shift+F10. It is called each time the menu opens.
	Menu func() []MenuItem
	// Chosen runs with the ID of the menu item the user chose, after the menu closes.
	Chosen func(id uint32)
}

// MenuItem is an item of a tray menu. ID is the command that Chosen receives.
// A separator, and an item with a Submenu, is never chosen.
type MenuItem struct {
	ID        uint32
	Label     string
	Disabled  bool
	Checked   bool
	Separator bool
	Submenu   []MenuItem
}

// TrayIcon is an icon of the notification area. Its methods run on the UI thread.
type TrayIcon struct {
	opts    TrayOptions
	icon    uintptr // the HICON the shell has; owned when owned is set
	owned   bool
	shown   bool
	useGUID bool // the icon is identified by its GUID, not its ID, in the shell
}

// NewTrayIcon returns an icon that is not in the notification area yet; see Show.
func NewTrayIcon(o TrayOptions) *TrayIcon { return &TrayIcon{opts: o} }

// Show adds the icon to the notification area. The icon is drawn at the small
// icon size of the tray window's DPI. It is an error when the shell refuses it.
func (t *TrayIcon) Show() error {
	if t.shown {
		return nil
	}
	if err := ensureTrayHost(); err != nil {
		return err
	}
	if err := t.updateIcon(); err != nil {
		return err
	}
	if !t.add() {
		t.dropIcon()
		return errors.New("platform/windows: Shell_NotifyIconW(NIM_ADD) refused the icon")
	}
	t.shown = true
	trayHost.icons[t.opts.ID] = t
	return nil
}

// add sends the icon to the shell with its identity, and then selects version 4
// for it, which sends the events Click, DoubleClick and Menu are for. When the
// icon has a GUID, but Windows refuses it, the icon is added by its ID instead. A
// GUID is tied to the executable's path, so the refusal is what a moved program
// gets. It reports whether the shell has the icon.
func (t *TrayIcon) add() bool {
	t.useGUID = t.opts.GUID != nil
	ok := t.notify(win32.NIM_ADD)
	if !ok && t.useGUID {
		t.useGUID = false
		ok = t.notify(win32.NIM_ADD)
	}
	if !ok {
		return false
	}
	nid := t.data()
	nid.UVersion = win32.NOTIFYICON_VERSION_4
	win32.ShellNotifyIcon(win32.NIM_SETVERSION, &nid)
	return true
}

// SetTooltip sets the tooltip. Longer tooltips are cut to 127 UTF-16 units.
func (t *TrayIcon) SetTooltip(s string) {
	t.opts.Tooltip = s
	if t.shown {
		t.notify(win32.NIM_MODIFY)
	}
}

// SetIcon sets the image of the icon; nil is the executable's icon. An icon that
// is not shown makes its HICON in Show. It is an error when the image cannot be
// drawn, and the icon keeps its image then.
func (t *TrayIcon) SetIcon(img image.Image) error {
	prev := t.opts.Icon
	t.opts.Icon = img
	if !t.shown {
		return nil
	}
	if err := t.updateIcon(); err != nil {
		t.opts.Icon = prev
		return err
	}
	return nil
}

// Remove takes the icon out of the notification area. Show adds it again.
func (t *TrayIcon) Remove() {
	if !t.shown {
		return
	}
	t.notify(win32.NIM_DELETE)
	t.shown = false
	delete(trayHost.icons, t.opts.ID)
	t.dropIcon()
}

// updateIcon makes the HICON of the icon's image at the tray window's DPI, and
// gives it to the shell when the icon is shown. The previous icon is destroyed
// once the shell has the new one.
func (t *TrayIcon) updateIcon() error {
	icon, owned, err := makeIcon(t.opts.Icon, iconSize(trayHost.dpi, smallIconMetric))
	if err != nil {
		return err
	}
	prev, prevOwned := t.icon, t.owned
	t.icon, t.owned = icon, owned
	if t.shown {
		t.notify(win32.NIM_MODIFY)
	}
	if prevOwned {
		win32.DestroyIcon(prev)
	}
	return nil
}

// dropIcon destroys the icon's HICON, if the icon owns it.
func (t *TrayIcon) dropIcon() {
	if t.owned {
		win32.DestroyIcon(t.icon)
	}
	t.icon, t.owned = 0, false
}

// notify sends msg with the icon's data, and reports whether the shell accepted it.
func (t *TrayIcon) notify(msg uint32) bool {
	nid := t.data()
	return win32.ShellNotifyIcon(msg, &nid)
}

// data returns the NOTIFYICONDATAW of the icon, with the fields its messages use.
func (t *TrayIcon) data() win32.NOTIFYICONDATAW {
	nid := win32.NOTIFYICONDATAW{
		CbSize:           uint32(unsafe.Sizeof(win32.NOTIFYICONDATAW{})),
		HWnd:             trayHost.hwnd,
		UID:              t.opts.ID,
		UFlags:           win32.NIF_MESSAGE | win32.NIF_ICON | win32.NIF_TIP | win32.NIF_SHOWTIP,
		UCallbackMessage: trayMsg,
		HIcon:            t.icon,
		SzTip:            tipUnits(t.opts.Tooltip),
	}
	if t.useGUID {
		nid.UFlags |= win32.NIF_GUID
		nid.GuidItem = guidMemory(*t.opts.GUID)
	}
	return nid
}

// event runs the action for a notification from the shell. wp is the anchor
// point of the icon, which a menu opens at.
func (t *TrayIcon) event(ev uint32, wp uintptr) {
	switch ev {
	case win32.NIN_SELECT, win32.NIN_KEYSELECT:
		if t.opts.Click != nil {
			t.opts.Click()
		}
	case win32.WM_LBUTTONDBLCLK:
		if t.opts.DoubleClick != nil {
			t.opts.DoubleClick()
		}
	case win32.WM_CONTEXTMENU, win32.WM_RBUTTONUP:
		t.showMenu(int32(int16(win32.LOWORD(wp))), int32(int16(win32.HIWORD(wp))))
	}
}

// showMenu shows the icon's menu at the screen point (x, y), and runs Chosen
// with the item the user picks. The window must be in the foreground for the
// menu to close when the user clicks elsewhere, and WM_NULL is posted after it
// closes, as Windows requires for a menu of a notification icon.
func (t *TrayIcon) showMenu(x, y int32) {
	if t.opts.Menu == nil {
		return
	}
	menu := buildMenu(t.opts.Menu())
	defer win32.DestroyMenu(menu)
	win32.SetForegroundWindow(trayHost.hwnd)
	vertical := uint32(win32.TPM_TOPALIGN)
	if work := win32.WorkAreaAt(x, y); y > (work.Top+work.Bottom)/2 {
		vertical = win32.TPM_BOTTOMALIGN
	}
	id := win32.TrackPopupMenu(menu, win32.TPM_RIGHTBUTTON|win32.TPM_RETURNCMD|win32.TPM_LEFTALIGN|vertical, x, y, trayHost.hwnd)
	win32.PostMessage(trayHost.hwnd, win32.WM_NULL, 0, 0)
	if id != 0 && t.opts.Chosen != nil {
		t.opts.Chosen(id)
	}
}

// buildMenu returns a popup menu of the items, with its submenus. The caller
// destroys it.
func buildMenu(items []MenuItem) uintptr {
	menu := win32.CreatePopupMenu()
	for _, it := range items {
		switch {
		case it.Separator:
			win32.AppendMenu(menu, win32.MF_SEPARATOR, 0, "")
		case len(it.Submenu) > 0:
			win32.AppendMenu(menu, menuFlags(it)|win32.MF_POPUP, buildMenu(it.Submenu), menuText(it.Label))
		default:
			win32.AppendMenu(menu, menuFlags(it), uintptr(it.ID), menuText(it.Label))
		}
	}
	return menu
}

// menuFlags returns the MF_ flags of an item that is not a separator.
func menuFlags(it MenuItem) uint32 {
	if it.Separator {
		return win32.MF_SEPARATOR
	}
	flags := uint32(win32.MF_STRING)
	if it.Disabled {
		flags |= win32.MF_GRAYED
	}
	if it.Checked {
		flags |= win32.MF_CHECKED
	}
	return flags
}

// menuText returns a label as a menu shows it: a '&' marks an access key in a
// menu, so it is doubled to show as itself.
func menuText(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '&' {
			out = append(out, '&')
		}
		out = append(out, s[i])
	}
	return string(out)
}

// tipUnits returns the tooltip as the UTF-16 units of szTip, cut to tipLimit
// units without splitting a surrogate pair.
func tipUnits(s string) (tip [128]uint16) {
	u := utf16.Encode([]rune(s))
	if len(u) > tipLimit {
		u = u[:tipLimit]
		if last := u[len(u)-1]; last >= 0xD800 && last <= 0xDBFF {
			u = u[:len(u)-1]
		}
	}
	copy(tip[:], u)
	return tip
}

// guidMemory returns the bytes of a GUID as they lie in memory. The first three
// fields of an RFC 4122 UUID are big-endian, and the GUID structure holds them
// little-endian.
func guidMemory(u [16]byte) [16]byte {
	return [16]byte{u[3], u[2], u[1], u[0], u[5], u[4], u[7], u[6],
		u[8], u[9], u[10], u[11], u[12], u[13], u[14], u[15]}
}

// ensureTrayHost creates the tray window, once, and reads the DPI it draws the
// icons at.
func ensureTrayHost() error {
	if trayHost.hwnd != 0 {
		return nil
	}
	className, _ := syscall.UTF16PtrFromString("JambTray")
	title, _ := syscall.UTF16PtrFromString("")
	inst := win32.GetModuleHandle()
	wc := win32.WNDCLASSEXW{
		LpfnWndProc:   syscall.NewCallback(trayProc),
		HInstance:     inst,
		LpszClassName: className,
	}
	wc.CbSize = uint32(sizeofWndClass)
	if _, err := win32.RegisterClassEx(&wc); err != nil {
		return err
	}
	hwnd, err := win32.CreateWindowEx(win32.WS_EX_TOOLWINDOW, className, title, win32.WS_POPUP, 0, 0, 0, 0, 0, 0, inst)
	if err != nil {
		return err
	}
	trayHost.hwnd = hwnd
	trayHost.dpi = win32.GetDpiForWindow(hwnd)
	trayHost.created = win32.RegisterWindowMessage("TaskbarCreated")
	trayHost.icons = map[uint32]*TrayIcon{}
	return nil
}

// trayProc is the window procedure of the tray window.
func trayProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	switch {
	case msg == trayMsg:
		if t := trayHost.icons[uint32(win32.HIWORD(lp))]; t != nil {
			t.event(uint32(win32.LOWORD(lp)), wp)
		}
		return 0
	case trayHost.created != 0 && msg == trayHost.created:
		// The notification area was made again, and has none of the icons.
		refreshTrayDPI()
		for _, t := range trayHost.icons {
			t.add()
		}
		return 0
	case msg == win32.WM_DPICHANGED, msg == win32.WM_DISPLAYCHANGE, msg == win32.WM_SETTINGCHANGE:
		refreshTrayDPI()
	}
	return win32.DefWindowProc(hwnd, msg, wp, lp)
}

// refreshTrayDPI redraws the icons at the tray window's DPI, when it has changed.
func refreshTrayDPI() {
	if trayHost.hwnd == 0 {
		return
	}
	dpi := win32.GetDpiForWindow(trayHost.hwnd)
	if dpi == trayHost.dpi {
		return
	}
	trayHost.dpi = dpi
	for _, t := range trayHost.icons {
		t.updateIcon()
	}
}

// shutdownTray takes every icon out of the notification area, and destroys the
// tray window. Run calls it on the UI thread when the loop ends.
func shutdownTray() {
	var shown []*TrayIcon
	for _, t := range trayHost.icons {
		shown = append(shown, t)
	}
	for _, t := range shown {
		t.Remove()
	}
	if trayHost.hwnd != 0 {
		win32.DestroyWindow(trayHost.hwnd)
		trayHost.hwnd = 0
	}
}
